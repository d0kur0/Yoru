package main

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"os"
	"path/filepath"
	"testing"

	"github.com/d0kur0/Yoru/internal/updater"
)

func TestAppUpdateCannotInstallBeforeDownload(t *testing.T) {
	for _, phase := range []string{"idle", "checking", "available", "downloading", "error", "current"} {
		v := &VPN{updates: newAppUpdateManager()}
		v.updates.state.Phase = phase
		if err := v.InstallAppUpdate(); err == nil {
			t.Fatalf("phase %s allowed installation", phase)
		}
	}
}

func TestAppUpdateRejectsConcurrentOperations(t *testing.T) {
	v := &VPN{updates: newAppUpdateManager()}
	v.updates.busy = true
	if _, err := v.CheckAppUpdate(context.Background()); err == nil {
		t.Fatal("concurrent check allowed")
	}
	if _, err := v.DownloadAppUpdate(context.Background()); err == nil {
		t.Fatal("concurrent download allowed")
	}
}

func TestAppUpdateVerifiesInstallerAgainBeforeLaunch(t *testing.T) {
	path := filepath.Join(t.TempDir(), "installer.exe")
	original := []byte("verified installer")
	hash := sha256.Sum256(original)
	file := updater.Downloaded{Path: path, Version: "1.3.0", SHA256: hex.EncodeToString(hash[:])}
	if err := os.WriteFile(path, original, 0600); err != nil {
		t.Fatal(err)
	}
	if err := verifyAppInstaller(file); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte("changed after download"), 0600); err != nil {
		t.Fatal(err)
	}
	v := &VPN{updates: newAppUpdateManager()}
	v.updates.file = file
	v.updates.state.Phase = "ready"
	// No app or launch stub: the checksum must reject this before any handoff.
	if err := v.InstallAppUpdate(); err == nil {
		t.Fatal("modified installer accepted")
	}
	if v.updates.busy || v.updates.state.Phase != "error" || v.updates.file.Path != "" {
		t.Fatal("failed verification did not reset update state")
	}
}
