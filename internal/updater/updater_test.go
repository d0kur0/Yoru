package updater

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func testUpdater(t *testing.T, installer []byte, mutate func(map[string]any)) (*Updater, *int) {
	t.Helper()
	const version = "1.2.4"
	name := "Yoru-" + version + "-windows-x64-setup.exe"
	hash := sha256.Sum256(installer)
	sums := fmt.Sprintf("%x  %s\n", hash, name)
	var base string
	requests := new(int)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/latest":
			gh := map[string]any{
				"tag_name":   "v" + version,
				"body":       "Changes",
				"draft":      false,
				"prerelease": false,
				"assets": []map[string]any{
					{"name": name, "browser_download_url": base + "/v" + version + "/" + name, "size": len(installer)},
					{"name": "SHA256SUMS.txt", "browser_download_url": base + "/v" + version + "/SHA256SUMS.txt", "size": len(sums)},
				},
			}
			if mutate != nil {
				mutate(gh)
			}
			_ = json.NewEncoder(w).Encode(gh)
		case "/v" + version + "/SHA256SUMS.txt":
			_, _ = w.Write([]byte(sums))
		case "/v" + version + "/" + name:
			*requests++
			_, _ = w.Write(installer)
		default:
			http.NotFound(w, r)
		}
	}))
	t.Cleanup(server.Close)
	base = server.URL
	u := New(Options{APIURL: server.URL + "/latest", AssetBaseURL: server.URL, CacheDir: t.TempDir(), OS: "windows", Arch: "amd64"})
	return u, requests
}

func TestCheckAndDownload(t *testing.T) {
	u, requests := testUpdater(t, []byte("signed release bytes"), nil)
	var progress []int64
	u.options.Progress = func(done, total int64) {
		if total != int64(len("signed release bytes")) {
			t.Errorf("wrong total: %d", total)
		}
		progress = append(progress, done)
	}
	release, newer, err := u.Check(context.Background(), "1.2.3")
	if err != nil || !newer || release.Version != "1.2.4" || release.AssetName != "Yoru-1.2.4-windows-x64-setup.exe" {
		t.Fatalf("unexpected release: %+v, %t, %v", release, newer, err)
	}
	got, err := u.Download(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if got.Version != release.Version || got.SHA256 != release.SHA256 || filepath.Base(got.Path) != release.AssetName {
		t.Fatalf("unexpected download: %+v", got)
	}
	data, err := os.ReadFile(got.Path)
	if err != nil || string(data) != "signed release bytes" {
		t.Fatalf("bad downloaded bytes: %s, %v", data, err)
	}
	if len(progress) == 0 || progress[len(progress)-1] != int64(len(data)) {
		t.Fatalf("missing final progress: %v", progress)
	}
	if _, err := u.Download(context.Background(), release); err != nil || *requests != 1 {
		t.Fatalf("verified cache not reused: requests=%d err=%v", *requests, err)
	}
}

func TestCheckNeverDowngrades(t *testing.T) {
	u, _ := testUpdater(t, []byte("installer"), nil)
	for _, current := range []string{"1.2.4", "v1.3.0", "2.0.0"} {
		_, newer, err := u.Check(context.Background(), current)
		if err != nil || newer {
			t.Errorf("current=%s newer=%t err=%v", current, newer, err)
		}
	}
	if _, _, err := u.Check(context.Background(), "1.2.4-beta.1"); err == nil {
		t.Fatal("accepted prerelease current version")
	}
}

func TestCheckRejectsUntrustedRelease(t *testing.T) {
	cases := map[string]func(map[string]any){
		"draft":      func(gh map[string]any) { gh["draft"] = true },
		"prerelease": func(gh map[string]any) { gh["prerelease"] = true },
		"bad tag":    func(gh map[string]any) { gh["tag_name"] = "v1.2.4-beta.1" },
		"asset URL": func(gh map[string]any) {
			gh["assets"].([]map[string]any)[0]["browser_download_url"] = "https://example.com/payload"
		},
		"missing checksum": func(gh map[string]any) { gh["assets"] = gh["assets"].([]map[string]any)[:1] },
	}
	for label, mutate := range cases {
		t.Run(label, func(t *testing.T) {
			u, _ := testUpdater(t, []byte("installer"), mutate)
			if _, _, err := u.Check(context.Background(), "1.2.3"); err == nil {
				t.Fatal("accepted unsafe release")
			}
		})
	}
}

func TestDownloadRejectsCorruption(t *testing.T) {
	u, _ := testUpdater(t, []byte("installer"), nil)
	release, _, err := u.Check(context.Background(), "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	release.SHA256 = strings.Repeat("a", 64)
	if _, err := u.Download(context.Background(), release); err == nil {
		t.Fatal("accepted checksum mismatch")
	}
	entries, err := os.ReadDir(filepath.Join(u.options.CacheDir, release.Version+"-"+release.SHA256[:12]))
	if err != nil || len(entries) != 0 {
		t.Fatalf("partial installer retained: %v, %v", entries, err)
	}
}

func TestDownloadRecoversCorruptedCache(t *testing.T) {
	u, requests := testUpdater(t, []byte("installer"), nil)
	release, _, err := u.Check(context.Background(), "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	got, err := u.Download(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(got.Path, []byte("corrupted"), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = u.Download(context.Background(), release)
	if err != nil || *requests != 2 {
		t.Fatalf("corrupted cache was not replaced: requests=%d err=%v", *requests, err)
	}
	data, err := os.ReadFile(got.Path)
	if err != nil || string(data) != "installer" {
		t.Fatalf("bad replacement: %q, %v", data, err)
	}
}

func TestDownloadRejectsForgedURLAndSize(t *testing.T) {
	u, _ := testUpdater(t, []byte("installer"), nil)
	release, _, err := u.Check(context.Background(), "1.2.3")
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []func(*Release){
		func(r *Release) { r.URL = "https://example.com/installer" },
		func(r *Release) { r.AssetName = "other.exe" },
		func(r *Release) { r.Size = maxInstaller + 1 },
	} {
		bad := release
		change(&bad)
		if _, err := u.Download(context.Background(), bad); err == nil {
			t.Fatalf("accepted forged release: %+v", bad)
		}
	}
}

func TestChecksumParsing(t *testing.T) {
	name := "Yoru-1.2.4-windows-x64-setup.exe"
	hash := hex.EncodeToString(make([]byte, 32))
	for _, data := range []string{
		hash + "  another.exe\n",
		hash + "  " + name + "\n" + hash + "  " + name + "\n",
		"invalid  " + name + "\n",
	} {
		if _, err := checksumFor([]byte(data), name); err == nil {
			t.Fatalf("accepted malformed checksums: %q", data)
		}
	}
}

func TestRejectsCrossHostRedirect(t *testing.T) {
	other := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte("untrusted"))
	}))
	defer other.Close()
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Redirect(w, r, other.URL+"/payload", http.StatusFound)
	}))
	defer server.Close()
	u := New(Options{APIURL: server.URL + "/latest", AssetBaseURL: server.URL})
	if _, err := u.get(context.Background(), server.URL+"/latest", 1024, server.URL+"/latest"); err == nil {
		t.Fatal("followed cross-host redirect")
	}
}

func TestPlatformSelection(t *testing.T) {
	cases := []struct{ platform, arch, want string }{
		{"windows", "amd64", "Yoru-1.2.4-windows-x64-setup.exe"},
		{"darwin", "amd64", "Yoru-1.2.4-macos-x64.dmg"},
		{"darwin", "arm64", "Yoru-1.2.4-macos-arm64.dmg"},
	}
	for _, tc := range cases {
		got, err := assetName(tc.platform, tc.arch, "1.2.4")
		if err != nil || got != tc.want {
			t.Errorf("%s/%s: %s, %v", tc.platform, tc.arch, got, err)
		}
	}
	if _, err := assetName("linux", "amd64", "1.2.4"); err == nil {
		t.Fatal("accepted unsupported platform")
	}
}

// Opt-in end-to-end smoke against the official release; never executes the file.
func TestLiveGitHubDownload(t *testing.T) {
	if os.Getenv("YORU_UPDATER_SMOKE") != "1" {
		t.Skip("set YORU_UPDATER_SMOKE=1 for live release download")
	}
	u := New(Options{CacheDir: t.TempDir()})
	release, newer, err := u.Check(context.Background(), "0.0.0")
	if err != nil || !newer {
		t.Fatalf("live check: newer=%t err=%v", newer, err)
	}
	got, err := u.Download(context.Background(), release)
	if err != nil {
		t.Fatal(err)
	}
	valid, err := verifyFile(got.Path, release.SHA256)
	if err != nil || !valid {
		t.Fatalf("live installer verification: valid=%t err=%v", valid, err)
	}
}
