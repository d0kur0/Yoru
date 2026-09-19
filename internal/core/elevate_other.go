//go:build !windows

package core

import "errors"

// RelaunchElevated backs the --elevate-relaunch-helper flag in main.go, which
// is compiled on every platform. The elevated-relaunch flow itself is
// Windows-only (see elevate_windows.go); on other platforms nothing ever
// spawns the helper that would call this, so it's unreachable in practice.
func RelaunchElevated() error {
	return errors.New("elevated relaunch is only supported on Windows")
}
