//go:build windows

package main

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"golang.org/x/sys/windows"
)

// launchAppInstaller starts the verified installer. The caller must quit the
// application only after this returns nil, so OnShutdown can stop Mihomo.
func launchAppInstaller(path string) error {
	installer, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	if !strings.EqualFold(filepath.Ext(installer), ".exe") {
		return errors.New("Windows update must be an .exe installer")
	}
	info, err := os.Stat(installer)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("installer is not a regular file")
	}

	executable, err := os.Executable()
	if err != nil {
		return err
	}
	parameters, err := windowsInstallerParameters(executable, os.Getpid())
	if err != nil {
		return err
	}
	file, err := windows.UTF16PtrFromString(installer)
	if err != nil {
		return err
	}
	args, err := windows.UTF16PtrFromString(parameters)
	if err != nil {
		return err
	}
	verb, _ := windows.UTF16PtrFromString("open")
	// ShellExecute honors the installer's elevation manifest and reports UAC
	// cancellation. CreateProcess would fail with ERROR_ELEVATION_REQUIRED.
	return windows.ShellExecute(0, verb, file, args, nil, 1)
}

func windowsInstallerParameters(executable string, pid int) (string, error) {
	if pid <= 0 || !filepath.IsAbs(executable) ||
		!strings.EqualFold(filepath.Base(executable), product.Name+".exe") {
		return "", errors.New("Обновление доступно из установленного Yoru.exe. Откройте установленную версию приложения")
	}
	dir := filepath.Dir(executable)
	if strings.ContainsAny(dir, "\x00\r\n\"") || dir == filepath.VolumeName(dir)+`\` {
		return "", errors.New("invalid installation directory")
	}
	// NSIS requires /D to be the final, unquoted argument. It consumes the
	// remainder of the command line, including spaces in the directory name.
	return fmt.Sprintf("/UPDATEPID=%d /D=%s", pid, dir), nil
}
