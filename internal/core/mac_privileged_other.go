//go:build !darwin

package core

import "errors"

func NewDesktopRunner() Runner { return commandRunner{} }
func HandlePrivilegedCore(args []string) (bool, error) {
	if len(args) > 0 && args[0] == privilegedCoreFlag {
		return true, errors.New("Помощник TUN поддерживается только на macOS")
	}
	return false, nil
}
