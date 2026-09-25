//go:build darwin

package main

import (
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// launchAppInstaller opens the downloaded disk image in Finder. The user
// replaces Yoru.app by dragging it to Applications after this app exits.
func launchAppInstaller(path string) error {
	if !strings.EqualFold(filepath.Ext(path), ".dmg") {
		return errors.New("macOS update must be a .dmg image")
	}
	image, err := filepath.Abs(path)
	if err != nil {
		return err
	}
	info, err := os.Stat(image)
	if err != nil {
		return err
	}
	if !info.Mode().IsRegular() {
		return errors.New("disk image is not a regular file")
	}
	return exec.Command("/usr/bin/open", image).Run()
}
