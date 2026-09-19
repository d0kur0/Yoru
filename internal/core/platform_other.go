//go:build !windows && !darwin

package core

import "errors"

type desktopPlatform struct{}

func newPlatform(string) Platform { return &desktopPlatform{} }
func (*desktopPlatform) Autostart(bool, bool, bool) error {
	return errors.New("Автозапуск пока доступен только в Windows и macOS")
}
func (*desktopPlatform) IsElevated() bool { return true }
func (*desktopPlatform) Proxy(int) (func() error, error) {
	return nil, errors.New("Системный прокси пока доступен только в Windows и macOS")
}
func (*desktopPlatform) RecoverProxy() error { return nil }
