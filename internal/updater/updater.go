// Package updater checks official Yoru releases and downloads verified installers.
package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	defaultAPIURL       = "https://api.github.com/repos/d0kur0/Yoru/releases/latest"
	defaultAssetBaseURL = "https://github.com/d0kur0/Yoru/releases/download"
	maxReleaseJSON      = 2 << 20
	maxChecksums        = 1 << 20
	maxInstaller        = 512 << 20
)

var (
	versionRE = regexp.MustCompile(`^v?(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)\.(0|[1-9][0-9]*)$`)
	sha256RE  = regexp.MustCompile(`^[[:xdigit:]]{64}$`)
)

// Options supplies test endpoints or overrides the download location. Zero values
// select the official GitHub release, the running platform, and the user cache.
type Options struct {
	Client       *http.Client
	APIURL       string
	AssetBaseURL string
	CacheDir     string
	OS           string
	Arch         string
	Progress     func(downloaded, total int64)
}

type Updater struct{ options Options }

type Release struct {
	Version   string `json:"version"`
	Notes     string `json:"notes"`
	URL       string `json:"url"`
	AssetName string `json:"assetName"`
	SHA256    string `json:"sha256"`
	Size      int64  `json:"size"`
}

type Downloaded struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

func New(options Options) *Updater { return &Updater{options: options} }

type githubRelease struct {
	Tag        string `json:"tag_name"`
	Body       string `json:"body"`
	Draft      bool   `json:"draft"`
	Prerelease bool   `json:"prerelease"`
	Assets     []struct {
		Name string `json:"name"`
		URL  string `json:"browser_download_url"`
		Size int64  `json:"size"`
	} `json:"assets"`
}

// Check reports a release only when its stable semantic version is newer.
// The returned bool is false for an equal or older release.
func (u *Updater) Check(ctx context.Context, currentVersion string) (Release, bool, error) {
	current, err := parseVersion(currentVersion)
	if err != nil {
		return Release{}, false, fmt.Errorf("current version: %w", err)
	}
	_, err = assetName(u.platform(), u.arch(), "")
	if err != nil {
		return Release{}, false, err
	}
	ctx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()
	apiURL := u.apiURL()
	data, err := u.get(ctx, apiURL, maxReleaseJSON, apiURL)
	if err != nil {
		return Release{}, false, fmt.Errorf("release metadata: %w", err)
	}
	var gh githubRelease
	if err := json.Unmarshal(data, &gh); err != nil {
		return Release{}, false, fmt.Errorf("invalid release metadata: %w", err)
	}
	if gh.Draft || gh.Prerelease || !strings.HasPrefix(gh.Tag, "v") {
		return Release{}, false, errors.New("latest release is not stable")
	}
	latest, err := parseVersion(gh.Tag)
	if err != nil {
		return Release{}, false, fmt.Errorf("invalid release tag: %w", err)
	}
	if latest[0] > 65535 || latest[1] > 65535 || latest[2] > 65535 {
		return Release{}, false, errors.New("release version exceeds supported range")
	}
	if compareVersion(latest, current) <= 0 {
		return Release{}, false, nil
	}
	name, _ := assetName(u.platform(), u.arch(), gh.Tag[1:])
	installerURL, err := u.assetURL(gh.Tag, name)
	if err != nil {
		return Release{}, false, err
	}
	checksumsURL, err := u.assetURL(gh.Tag, "SHA256SUMS.txt")
	if err != nil {
		return Release{}, false, err
	}
	var installerFound, checksumsFound bool
	var size int64
	for _, asset := range gh.Assets {
		switch asset.Name {
		case name:
			if installerFound || asset.URL != installerURL || asset.Size < 0 || asset.Size > maxInstaller {
				return Release{}, false, errors.New("invalid installer asset")
			}
			installerFound, size = true, asset.Size
		case "SHA256SUMS.txt":
			if checksumsFound || asset.URL != checksumsURL || asset.Size < 0 || asset.Size > maxChecksums {
				return Release{}, false, errors.New("invalid checksums asset")
			}
			checksumsFound = true
		}
	}
	if !installerFound || !checksumsFound {
		return Release{}, false, errors.New("release assets are incomplete")
	}
	checksums, err := u.get(ctx, checksumsURL, maxChecksums, checksumsURL)
	if err != nil {
		return Release{}, false, fmt.Errorf("release checksums: %w", err)
	}
	hash, err := checksumFor(checksums, name)
	if err != nil {
		return Release{}, false, err
	}
	return Release{Version: gh.Tag[1:], Notes: gh.Body, URL: installerURL, AssetName: name, SHA256: hash, Size: size}, true, nil
}

// Download writes the installer to the user cache and returns its verified path.
// It never starts or installs the downloaded file.
func (u *Updater) Download(ctx context.Context, release Release) (Downloaded, error) {
	version, err := parseVersion(release.Version)
	if err != nil {
		return Downloaded{}, err
	}
	if version[0] > 65535 || version[1] > 65535 || version[2] > 65535 {
		return Downloaded{}, errors.New("release version exceeds supported range")
	}
	name, err := assetName(u.platform(), u.arch(), release.Version)
	if err != nil {
		return Downloaded{}, err
	}
	expectedURL, err := u.assetURL("v"+release.Version, name)
	if err != nil {
		return Downloaded{}, err
	}
	if release.AssetName != name || release.URL != expectedURL || !sha256RE.MatchString(release.SHA256) || release.Size < 0 || release.Size > maxInstaller {
		return Downloaded{}, errors.New("invalid release installer")
	}
	cacheDir, err := u.cacheDir()
	if err != nil {
		return Downloaded{}, err
	}
	if err := privateDir(cacheDir); err != nil {
		return Downloaded{}, err
	}
	finalDir := filepath.Join(cacheDir, release.Version+"-"+strings.ToLower(release.SHA256[:12]))
	if err := privateDir(finalDir); err != nil {
		return Downloaded{}, err
	}
	finalPath := filepath.Join(finalDir, name)
	if info, err := os.Lstat(finalPath); err == nil {
		if valid, err := verifyFile(finalPath, release.SHA256); err == nil && valid {
			if u.options.Progress != nil {
				if info, err := os.Stat(finalPath); err == nil {
					u.options.Progress(info.Size(), info.Size())
				}
			}
			return Downloaded{Version: release.Version, Path: finalPath, SHA256: strings.ToLower(release.SHA256)}, nil
		}
		if !info.Mode().IsRegular() && info.Mode()&os.ModeSymlink == 0 {
			return Downloaded{}, errors.New("cached installer is not a file")
		}
		if err := os.Remove(finalPath); err != nil {
			return Downloaded{}, fmt.Errorf("remove corrupted cached installer: %w", err)
		}
	} else if !errors.Is(err, os.ErrNotExist) {
		return Downloaded{}, err
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Minute)
	defer cancel()
	res, err := u.request(ctx, expectedURL, expectedURL)
	if err != nil {
		return Downloaded{}, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return Downloaded{}, fmt.Errorf("installer download returned HTTP %d", res.StatusCode)
	}
	if res.ContentLength > maxInstaller || (release.Size > 0 && res.ContentLength > 0 && res.ContentLength != release.Size) {
		return Downloaded{}, errors.New("invalid installer size")
	}
	file, err := os.CreateTemp(finalDir, ".download-*")
	if err != nil {
		return Downloaded{}, err
	}
	defer os.Remove(file.Name())
	defer file.Close()
	h := sha256.New()
	total := res.ContentLength
	if total <= 0 {
		total = release.Size
	}
	buf := make([]byte, 128<<10)
	var written int64
	reader := io.LimitReader(res.Body, maxInstaller+1)
	for {
		n, readErr := reader.Read(buf)
		if n > 0 {
			written += int64(n)
			if written > maxInstaller {
				return Downloaded{}, errors.New("installer exceeds size limit")
			}
			if _, err := file.Write(buf[:n]); err != nil {
				return Downloaded{}, err
			}
			_, _ = h.Write(buf[:n])
			if u.options.Progress != nil {
				u.options.Progress(written, total)
			}
		}
		if readErr == io.EOF {
			break
		}
		if readErr != nil {
			return Downloaded{}, readErr
		}
	}
	if written == 0 || (release.Size > 0 && written != release.Size) || !strings.EqualFold(hex.EncodeToString(h.Sum(nil)), release.SHA256) {
		return Downloaded{}, errors.New("installer SHA-256 or size mismatch")
	}
	if err := file.Sync(); err != nil {
		return Downloaded{}, err
	}
	if err := file.Close(); err != nil {
		return Downloaded{}, err
	}
	if err := os.Rename(file.Name(), finalPath); err != nil {
		return Downloaded{}, err
	}
	return Downloaded{Version: release.Version, Path: finalPath, SHA256: strings.ToLower(release.SHA256)}, nil
}

func (u *Updater) platform() string {
	if u.options.OS != "" {
		return u.options.OS
	}
	return runtime.GOOS
}

func (u *Updater) arch() string {
	if u.options.Arch != "" {
		return u.options.Arch
	}
	return runtime.GOARCH
}

func (u *Updater) apiURL() string {
	if u.options.APIURL != "" {
		return u.options.APIURL
	}
	return defaultAPIURL
}

func (u *Updater) assetBaseURL() string {
	if u.options.AssetBaseURL != "" {
		return u.options.AssetBaseURL
	}
	return defaultAssetBaseURL
}

func (u *Updater) cacheDir() (string, error) {
	if u.options.CacheDir != "" {
		return u.options.CacheDir, nil
	}
	base, err := os.UserCacheDir()
	if err != nil {
		return "", err
	}
	return filepath.Join(base, "Yoru", "updates"), nil
}

func assetName(platform, arch, version string) (string, error) {
	if platform == "windows" && arch == "amd64" {
		return "Yoru-" + version + "-windows-x64-setup.exe", nil
	}
	if platform == "darwin" && arch == "amd64" {
		return "Yoru-" + version + "-macos-x64.dmg", nil
	}
	if platform == "darwin" && arch == "arm64" {
		return "Yoru-" + version + "-macos-arm64.dmg", nil
	}
	return "", fmt.Errorf("unsupported platform: %s/%s", platform, arch)
}

func parseVersion(raw string) ([3]uint64, error) {
	matches := versionRE.FindStringSubmatch(raw)
	if matches == nil {
		return [3]uint64{}, errors.New("invalid stable semantic version")
	}
	var value [3]uint64
	for i := range value {
		n, err := strconv.ParseUint(matches[i+1], 10, 64)
		if err != nil {
			return value, errors.New("version component exceeds supported range")
		}
		value[i] = n
	}
	return value, nil
}

func compareVersion(a, b [3]uint64) int {
	for i := range a {
		if a[i] < b[i] {
			return -1
		}
		if a[i] > b[i] {
			return 1
		}
	}
	return 0
}

func (u *Updater) assetURL(tag, name string) (string, error) {
	base, err := url.Parse(u.assetBaseURL())
	if err != nil || base.Host == "" || (base.Scheme != "https" && base.Scheme != "http") || base.User != nil || base.RawQuery != "" || base.Fragment != "" {
		return "", errors.New("invalid asset base URL")
	}
	if base.Scheme == "http" && u.options.AssetBaseURL == "" {
		return "", errors.New("insecure official asset URL")
	}
	return strings.TrimRight(base.String(), "/") + "/" + url.PathEscape(tag) + "/" + url.PathEscape(name), nil
}

func checksumFor(data []byte, name string) (string, error) {
	var found string
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSuffix(line, "\r")
		if line == "" {
			continue
		}
		if len(line) < 67 || line[64:66] != "  " || !sha256RE.MatchString(line[:64]) {
			return "", errors.New("malformed SHA256SUMS.txt")
		}
		if line[66:] == name {
			if found != "" {
				return "", errors.New("duplicate installer checksum")
			}
			found = strings.ToLower(line[:64])
		}
	}
	if found == "" {
		return "", errors.New("installer checksum is missing")
	}
	return found, nil
}

func (u *Updater) get(ctx context.Context, address string, limit int64, origin string) ([]byte, error) {
	res, err := u.request(ctx, address, origin)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("HTTP %d", res.StatusCode)
	}
	if res.ContentLength > limit {
		return nil, errors.New("response exceeds size limit")
	}
	data, err := io.ReadAll(io.LimitReader(res.Body, limit+1))
	if err != nil {
		return nil, err
	}
	if int64(len(data)) > limit {
		return nil, errors.New("response exceeds size limit")
	}
	return data, nil
}

func (u *Updater) request(ctx context.Context, address string, origin string) (*http.Response, error) {
	initial, err := url.Parse(origin)
	if err != nil || initial.Host == "" || (initial.Scheme != "https" && initial.Scheme != "http") || initial.User != nil {
		return nil, errors.New("invalid download URL")
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("User-Agent", "Yoru-Updater")
	req.Header.Set("Accept", "application/vnd.github+json")
	client := http.Client{Timeout: 10 * time.Minute}
	if u.options.Client != nil {
		client = *u.options.Client
	}
	client.CheckRedirect = func(next *http.Request, via []*http.Request) error {
		if len(via) >= 6 || next.URL.User != nil {
			return errors.New("unsafe download redirect")
		}
		if initial.Scheme == "http" {
			if next.URL.Scheme != "http" || next.URL.Host != initial.Host {
				return errors.New("unsafe download redirect")
			}
			return nil
		}
		if next.URL.Scheme != "https" {
			return errors.New("unsafe download redirect")
		}
		if origin == u.apiURL() {
			if next.URL.Host != initial.Host {
				return errors.New("unsafe metadata redirect")
			}
		} else if u.options.AssetBaseURL == "" {
			switch next.URL.Hostname() {
			case "github.com", "release-assets.githubusercontent.com", "objects.githubusercontent.com":
			default:
				return errors.New("unsafe asset redirect")
			}
		} else if next.URL.Host != initial.Host {
			return errors.New("unsafe download redirect")
		}
		return nil
	}
	return client.Do(req)
}

func verifyFile(path, want string) (bool, error) {
	info, err := os.Lstat(path)
	if err != nil {
		return false, err
	}
	if !info.Mode().IsRegular() || info.Size() == 0 || info.Size() > maxInstaller {
		return false, nil
	}
	file, err := os.Open(path)
	if err != nil {
		return false, err
	}
	defer file.Close()
	h := sha256.New()
	if _, err := io.Copy(h, io.LimitReader(file, maxInstaller+1)); err != nil {
		return false, err
	}
	return strings.EqualFold(hex.EncodeToString(h.Sum(nil)), want), nil
}

func privateDir(path string) error {
	if err := os.MkdirAll(path, 0700); err != nil {
		return err
	}
	info, err := os.Lstat(path)
	if err != nil {
		return err
	}
	if !info.IsDir() || info.Mode()&os.ModeSymlink != 0 {
		return errors.New("update cache directory is not private")
	}
	return nil
}
