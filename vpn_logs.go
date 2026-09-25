package main

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"runtime"
)

// CopyLogText is a legacy clipboard API also used by the server editor to
// copy credentials. Keep its name and size limit for existing frontend calls.
func (v *VPN) CopyLogText(text string) bool {
	if len(text) > 128<<10 {
		return false
	}
	return v.app.Clipboard.SetText(text)
}

// LogsLocation exposes the log writer's own directory, never a caller path.
func (v *VPN) LogsLocation() string { return v.manager.LogsDir() }

// OpenLogsFolder opens the directory in Explorer or Finder. Creating it first
// makes the action useful before Mihomo has written its first log message.
func (v *VPN) OpenLogsFolder() error {
	return openLogsFolder(v.LogsLocation(), runtime.GOOS, startLogsFolderCommand)
}

func openLogsFolder(dir, goos string, start func(*exec.Cmd) error) error {
	if err := os.MkdirAll(dir, 0700); err != nil {
		return fmt.Errorf("Не удалось создать папку логов: %w", err)
	}
	cmd, err := logsFolderCommand(goos, dir)
	if err != nil {
		return err
	}
	return start(cmd)
}

func startLogsFolderCommand(cmd *exec.Cmd) error {
	if err := cmd.Start(); err != nil {
		return fmt.Errorf("Не удалось открыть папку логов: %w", err)
	}
	return cmd.Process.Release()
}

func logsFolderCommand(goos, dir string) (*exec.Cmd, error) {
	switch goos {
	case "windows":
		return exec.Command("explorer.exe", dir), nil
	case "darwin":
		return exec.Command("/usr/bin/open", dir), nil
	default:
		return nil, errors.New("Открытие папки логов не поддерживается на этой платформе")
	}
}
