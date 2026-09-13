package core

import (
	"archive/zip"
	"bytes"
	"compress/gzip"
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
	"strings"
	"time"
)

const maxBinary = 200 << 20

type Release struct {
	Tag    string  `json:"tag_name"`
	Assets []Asset `json:"assets"`
}
type Asset struct {
	Name   string `json:"name"`
	URL    string `json:"browser_download_url"`
	Digest string `json:"digest"`
}
type Installation struct {
	Version string `json:"version"`
	Path    string `json:"path"`
	SHA256  string `json:"sha256"`
}

var versionPattern = regexp.MustCompile(`^v[0-9]+\.[0-9]+\.[0-9]+(?:[a-zA-Z0-9.-]*)$`)

func assetName(platform, arch, version string) (string, error) {
	if !versionPattern.MatchString(version) {
		return "", errors.New("Некорректная версия релиза")
	}
	if platform != "windows" && platform != "darwin" && platform != "linux" {
		return "", errors.New("ОС не поддерживается")
	}
	if arch != "amd64" && arch != "arm64" {
		return "", errors.New("Архитектура не поддерживается")
	}
	if arch == "amd64" {
		arch += "-compatible"
	}
	ext := ".gz"
	if platform == "windows" {
		ext = ".zip"
	}
	return "mihomo-" + platform + "-" + arch + "-" + version + ext, nil
}
func readLimit(r io.Reader, limit int64) ([]byte, error) {
	b, e := io.ReadAll(io.LimitReader(r, limit+1))
	if e != nil {
		return nil, e
	}
	if int64(len(b)) > limit {
		return nil, errors.New("Превышен допустимый размер загрузки")
	}
	return b, nil
}
func get(ctx context.Context, client *http.Client, address string, limit int64) ([]byte, error) {
	req, e := http.NewRequestWithContext(ctx, http.MethodGet, address, nil)
	if e != nil {
		return nil, e
	}
	req.Header.Set("User-Agent", "Mihomo-Desktop")
	res, e := client.Do(req)
	if e != nil {
		return nil, errors.New("Не удалось загрузить данные: проверьте соединение")
	}
	defer res.Body.Close()
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("Сервер загрузки вернул HTTP %d", res.StatusCode)
	}
	return readLimit(res.Body, limit)
}
func unpack(asset Asset, data []byte) ([]byte, error) {
	sum := sha256.Sum256(data)
	if asset.Digest != "sha256:"+hex.EncodeToString(sum[:]) {
		return nil, errors.New("SHA-256 архива не совпадает с официальным релизом")
	}
	if strings.HasSuffix(asset.Name, ".gz") {
		r, e := gzip.NewReader(bytes.NewReader(data))
		if e != nil {
			return nil, e
		}
		defer r.Close()
		return readLimit(r, maxBinary)
	}
	z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if e != nil {
		return nil, e
	}
	var found *zip.File
	for _, f := range z.File {
		if !f.FileInfo().IsDir() && strings.HasPrefix(filepath.Base(f.Name), "mihomo") && strings.HasSuffix(f.Name, ".exe") {
			if found != nil {
				return nil, errors.New("В архиве несколько исполняемых файлов")
			}
			found = f
		}
	}
	if found == nil {
		return nil, errors.New("Исполняемый файл не найден в архиве")
	}
	r, e := found.Open()
	if e != nil {
		return nil, e
	}
	defer r.Close()
	return readLimit(r, maxBinary)
}
func atomicWrite(path string, data []byte, mode os.FileMode) error {
	if e := os.MkdirAll(filepath.Dir(path), 0700); e != nil {
		return e
	}
	f, e := os.CreateTemp(filepath.Dir(path), ".write-*")
	if e != nil {
		return e
	}
	defer os.Remove(f.Name())
	if e = f.Chmod(mode); e == nil {
		_, e = f.Write(data)
	}
	if e == nil {
		e = f.Sync()
	}
	closeErr := f.Close()
	if e != nil {
		return e
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
func (m *Manager) Install(ctx context.Context) (Installation, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != nil {
		return Installation{}, errors.New("Остановите ядро перед обновлением")
	}
	ctx, cancel := m.operation(ctx, 5*time.Minute)
	defer cancel()
	b, e := get(ctx, m.downloadClient, m.releaseURL, 2<<20)
	if e != nil {
		return Installation{}, e
	}
	var release Release
	if e = json.Unmarshal(b, &release); e != nil {
		return Installation{}, errors.New("Некорректный ответ GitHub")
	}
	name, e := assetName(runtime.GOOS, runtime.GOARCH, release.Tag)
	if e != nil {
		return Installation{}, e
	}
	var asset Asset
	for _, a := range release.Assets {
		if a.Name == name {
			asset = a
			break
		}
	}
	u, e := url.Parse(asset.URL)
	if e != nil || u.Scheme != "https" || u.Host != "github.com" || !strings.HasPrefix(u.Path, "/MetaCubeX/mihomo/releases/download/"+release.Tag+"/") {
		return Installation{}, errors.New("Официальный файл ядра не найден")
	}
	if !strings.HasPrefix(asset.Digest, "sha256:") || len(asset.Digest) != 71 {
		return Installation{}, errors.New("GitHub не предоставил SHA-256 релиза")
	}
	b, e = get(ctx, m.downloadClient, asset.URL, maxBinary)
	if e != nil {
		return Installation{}, e
	}
	bin, e := unpack(asset, b)
	if e != nil {
		return Installation{}, e
	}
	if len(bin) < 4 {
		return Installation{}, errors.New("Пустое ядро")
	}
	sum := sha256.Sum256(bin)
	hash := hex.EncodeToString(sum[:])
	filename := "mihomo"
	if runtime.GOOS == "windows" {
		filename += ".exe"
	}
	target := filepath.Join(m.dir, "core", release.Tag+"-"+hash[:12], filename)
	if e = atomicWrite(target, bin, 0700); e != nil {
		return Installation{}, e
	}
	installed := Installation{release.Tag, target, hash}
	manifest, _ := json.Marshal(installed)
	if e = atomicWrite(filepath.Join(m.dir, "core.json"), manifest, 0600); e != nil {
		return Installation{}, e
	}
	return installed, nil
}
func (m *Manager) installed(verify bool) (Installation, error) {
	var i Installation
	b, e := os.ReadFile(filepath.Join(m.dir, "core.json"))
	if e != nil {
		return i, errors.New("Ядро не установлено. Скачайте его в настройках")
	}
	if json.Unmarshal(b, &i) != nil {
		return i, errors.New("Повреждена информация об установленном ядре")
	}
	rel, e := filepath.Rel(filepath.Join(m.dir, "core"), i.Path)
	if e != nil || filepath.IsAbs(rel) || strings.HasPrefix(rel, "..") {
		return i, errors.New("Ядро находится вне каталога приложения")
	}
	if verify {
		b, e = os.ReadFile(i.Path)
		if e != nil {
			return i, e
		}
		sum := sha256.Sum256(b)
		if hex.EncodeToString(sum[:]) != i.SHA256 {
			return i, errors.New("Файл ядра изменён. Установите ядро заново")
		}
	}
	return i, nil
}
