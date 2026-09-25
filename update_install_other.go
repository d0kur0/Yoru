//go:build !windows && !darwin

package main

import "errors"

func launchAppInstaller(string) error {
	return errors.New("app update is unsupported on this platform")
}
