package main

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/d0kur0/Yoru/internal/core"
)

func TestLogsLocationMatchesWriterDirectory(t *testing.T) {
	profile := filepath.Join(t.TempDir(), "Yoru")
	manager, err := core.New(profile)
	if err != nil {
		t.Fatal(err)
	}
	vpn := &VPN{manager: manager}
	want := filepath.Join(profile, "logs")
	if got := vpn.LogsLocation(); got != want {
		t.Fatalf("LogsLocation() = %q, want %q", got, want)
	}
	if _, err := os.Stat(want); !os.IsNotExist(err) {
		t.Fatalf("log directory should not be created by lookup: %v", err)
	}
}

func TestLogsFolderCommandsPassOnePathArgument(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "folder with spaces")
	for _, goos := range []string{"windows", "darwin"} {
		cmd, err := logsFolderCommand(goos, dir)
		if err != nil {
			t.Fatal(err)
		}
		if len(cmd.Args) != 2 || cmd.Args[1] != dir {
			t.Fatalf("%s command arguments = %q", goos, cmd.Args)
		}
	}
	if _, err := logsFolderCommand("linux", dir); err == nil {
		t.Fatal("unsupported platform accepted")
	}
}

func TestOpenLogsFolderCreatesDirectoryBeforeOpening(t *testing.T) {
	dir := filepath.Join(t.TempDir(), "Yoru", "logs")
	called := false
	err := openLogsFolder(dir, "windows", func(cmd *exec.Cmd) error {
		called = true
		info, err := os.Stat(dir)
		if err != nil || !info.IsDir() {
			t.Fatalf("log directory not ready before OS command: %v", err)
		}
		if len(cmd.Args) != 2 || cmd.Args[1] != dir {
			t.Fatalf("folder command arguments = %q", cmd.Args)
		}
		return nil
	})
	if err != nil || !called {
		t.Fatalf("openLogsFolder() = %v, callback called = %t", err, called)
	}
}

func TestOpenLogsFolderDoesNotLaunchWhenPathIsFile(t *testing.T) {
	path := filepath.Join(t.TempDir(), "logs")
	if err := os.WriteFile(path, []byte("occupied"), 0600); err != nil {
		t.Fatal(err)
	}
	called := false
	err := openLogsFolder(path, "windows", func(*exec.Cmd) error { called = true; return nil })
	if err == nil || called {
		t.Fatalf("openLogsFolder() = %v, callback called = %t", err, called)
	}
}
