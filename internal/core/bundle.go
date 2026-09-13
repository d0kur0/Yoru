package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/d0kur0/Yoru/internal/bundled"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"time"
)

const BundledVersion = "v1.19.30"

func HasBundledCore() bool {
	_, e := bundled.Files.ReadFile("payload/" + runtime.GOOS + "-" + runtime.GOARCH + ".json")
	return e == nil
}

type BundleManifest struct {
	Version string `json:"version"`
	SHA256  string `json:"sha256"`
	OS      string `json:"os"`
	Arch    string `json:"arch"`
}

func (m *Manager) installBundled() (Installation, error) {
	prefix := "payload/" + runtime.GOOS + "-" + runtime.GOARCH
	metadata, e := bundled.Files.ReadFile(prefix + ".json")
	if e != nil {
		return Installation{}, fmt.Errorf("В этой сборке нет встроенного ядра. Нажмите «Скачать ядро»")
	}
	var info BundleManifest
	if json.Unmarshal(metadata, &info) != nil || info.OS != runtime.GOOS || info.Arch != runtime.GOARCH {
		return Installation{}, fmt.Errorf("Неверная платформа встроенного ядра")
	}
	compressed, e := bundled.Files.ReadFile(prefix + ".gz")
	if e != nil {
		return Installation{}, e
	}
	return m.extractBundle(info, compressed)
}
func (m *Manager) extractBundle(info BundleManifest, compressed []byte) (Installation, error) {
	if !versionPattern.MatchString(info.Version) {
		return Installation{}, fmt.Errorf("Некорректная версия встроенного ядра")
	}
	r, e := gzip.NewReader(bytes.NewReader(compressed))
	if e != nil {
		return Installation{}, e
	}
	defer r.Close()
	bin, e := readLimit(r, maxBinary)
	if e != nil {
		return Installation{}, e
	}
	sum := sha256.Sum256(bin)
	hash := hex.EncodeToString(sum[:])
	if hash != info.SHA256 {
		return Installation{}, fmt.Errorf("Повреждено встроенное ядро")
	}
	filename := "mihomo"
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	target := filepath.Join(m.dir, "core", info.Version+"-"+hash[:12], filename)
	if e = atomicWrite(target, bin, 0700); e != nil {
		return Installation{}, e
	}
	i := Installation{info.Version, target, hash}
	metadata, _ := json.Marshal(i)
	if e = atomicWrite(filepath.Join(m.dir, "core.json"), metadata, 0600); e != nil {
		return Installation{}, e
	}
	return i, nil
}

// PrepareBundle runs at build time, never during an application startup.
func PrepareBundle(ctx context.Context, dir, platform, arch string) error {
	name, e := assetName(platform, arch, BundledVersion)
	if e != nil {
		return e
	}
	prefix := filepath.Join(dir, platform+"-"+arch)
	if metadata, e := os.ReadFile(prefix + ".json"); e == nil {
		var info BundleManifest
		if json.Unmarshal(metadata, &info) == nil && info.Version == BundledVersion && info.OS == platform && info.Arch == arch {
			if compressed, e := os.ReadFile(prefix + ".gz"); e == nil {
				r, e := gzip.NewReader(bytes.NewReader(compressed))
				if e == nil {
					b, e := readLimit(r, maxBinary)
					r.Close()
					sum := sha256.Sum256(b)
					if e == nil && hex.EncodeToString(sum[:]) == info.SHA256 {
						return nil
					}
				}
			}
		}
	}
	client := &http.Client{Timeout: 4 * time.Minute}
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if req.URL.Scheme != "https" || len(via) > 8 {
			return fmt.Errorf("Недопустимый redirect")
		}
		return nil
	}
	data, e := get(ctx, client, "https://api.github.com/repos/MetaCubeX/mihomo/releases/tags/"+BundledVersion, 2<<20)
	if e != nil {
		return e
	}
	var release Release
	if e = json.Unmarshal(data, &release); e != nil {
		return e
	}
	var asset Asset
	for _, a := range release.Assets {
		if a.Name == name {
			asset = a
			break
		}
	}
	expected := "https://github.com/MetaCubeX/mihomo/releases/download/" + BundledVersion + "/" + name
	if asset.URL != expected {
		return fmt.Errorf("Официальный asset не найден")
	}
	data, e = get(ctx, client, asset.URL, maxBinary)
	if e != nil {
		return e
	}
	bin, e := unpack(asset, data)
	if e != nil {
		return e
	}
	sum := sha256.Sum256(bin)
	info := BundleManifest{BundledVersion, hex.EncodeToString(sum[:]), platform, arch}
	var compressed bytes.Buffer
	z := gzip.NewWriter(&compressed)
	if _, e = z.Write(bin); e != nil {
		return e
	}
	if e = z.Close(); e != nil {
		return e
	}
	license, e := get(ctx, client, "https://raw.githubusercontent.com/MetaCubeX/mihomo/"+BundledVersion+"/LICENSE", 1<<20)
	if e != nil {
		return e
	}
	if e = atomicWrite(filepath.Join(dir, "LICENSE-mihomo.txt"), license, 0644); e != nil {
		return e
	}
	if e = atomicWrite(prefix+".gz", compressed.Bytes(), 0644); e != nil {
		return e
	}
	metadata, _ := json.Marshal(info)
	return atomicWrite(prefix+".json", metadata, 0644)
}
