package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"os"
	"runtime"
	"sync"

	"github.com/d0kur0/Yoru/internal/updater"
)

type appUpdateState struct {
	Phase      string `json:"phase"`
	Version    string `json:"version,omitempty"`
	ReleaseURL string `json:"releaseURL,omitempty"`
	Platform   string `json:"platform"`
	Error      string `json:"error,omitempty"`
	Downloaded int64  `json:"downloaded"`
	Total      int64  `json:"total"`
}

type appUpdateManager struct {
	mu      sync.Mutex
	client  *updater.Updater
	state   appUpdateState
	release updater.Release
	file    updater.Downloaded
	busy    bool
}

func newAppUpdateManager() *appUpdateManager {
	m := &appUpdateManager{state: appUpdateState{Phase: "idle", Platform: runtime.GOOS}}
	m.client = updater.New(updater.Options{Progress: func(done, total int64) {
		m.mu.Lock()
		defer m.mu.Unlock()
		m.state.Downloaded, m.state.Total = done, total
	}})
	return m
}

func (v *VPN) AppUpdateStatus() (string, error) {
	m := v.updates
	m.mu.Lock()
	defer m.mu.Unlock()
	return encoded(m.state)
}

func (v *VPN) CheckAppUpdate(ctx context.Context) (string, error) {
	m := v.updates
	m.mu.Lock()
	if m.busy {
		m.mu.Unlock()
		return "", errors.New("Обновление уже выполняется")
	}
	m.busy = true
	m.state = appUpdateState{Phase: "checking", Platform: runtime.GOOS}
	m.mu.Unlock()
	release, available, err := m.client.Check(ctx, product.Version)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.busy = false
	if err != nil {
		m.state.Phase = "error"
		m.state.Error = "Не удалось проверить обновления: " + err.Error()
		return encoded(m.state)
	}
	m.release = release
	m.state.Phase = "current"
	if available {
		m.state.Phase = "available"
		m.state.Version = release.Version
		m.state.ReleaseURL = "https://github.com/d0kur0/Yoru/releases/tag/v" + release.Version
		if m.file.Version == release.Version && m.file.SHA256 == release.SHA256 {
			m.state.Phase = "ready"
		}
	}
	return encoded(m.state)
}

func (v *VPN) DownloadAppUpdate(ctx context.Context) (string, error) {
	m := v.updates
	m.mu.Lock()
	if m.busy || m.state.Phase != "available" {
		m.mu.Unlock()
		return "", errors.New("Сначала проверьте наличие обновления")
	}
	m.busy = true
	m.state.Phase = "downloading"
	m.state.Error = ""
	release := m.release
	m.mu.Unlock()
	file, err := m.client.Download(ctx, release)
	m.mu.Lock()
	defer m.mu.Unlock()
	m.busy = false
	if err != nil {
		m.state.Phase = "error"
		m.state.Error = "Не удалось скачать обновление: " + err.Error()
		return encoded(m.state)
	}
	m.file = file
	m.state.Phase = "ready"
	return encoded(m.state)
}

// Verify again immediately before handing the file to the OS: a completed
// download may have been changed or removed while the user was deciding.
func verifyAppInstaller(file updater.Downloaded) error {
	f, err := os.Open(file.Path)
	if err != nil {
		return err
	}
	defer f.Close()
	info, err := f.Stat()
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() || info.Size() > 512<<20 {
		return errors.New("Недопустимый файл обновления")
	}
	h := sha256.New()
	if _, err = io.Copy(h, io.LimitReader(f, (512<<20)+1)); err != nil {
		return err
	}
	if hex.EncodeToString(h.Sum(nil)) != file.SHA256 {
		return errors.New("Контрольная сумма обновления не совпадает. Скачайте его заново")
	}
	return nil
}

func (v *VPN) InstallAppUpdate() error {
	m := v.updates
	m.mu.Lock()
	if m.busy || m.state.Phase != "ready" {
		m.mu.Unlock()
		return errors.New("Обновление ещё не загружено")
	}
	m.busy = true
	m.state.Phase = "installing"
	file := m.file
	m.mu.Unlock()
	err := verifyAppInstaller(file)
	if err == nil {
		err = launchAppInstaller(file.Path)
	}
	if err != nil {
		m.mu.Lock()
		m.busy = false
		m.state.Phase = "error"
		m.state.Error = fmt.Sprintf("Не удалось начать установку: %v", err)
		m.file = updater.Downloaded{}
		m.mu.Unlock()
		return err
	}
	// OnShutdown releases the proxy and TUN before the installer replaces Yoru.
	v.app.Quit()
	return nil
}
