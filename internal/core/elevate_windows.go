package core

import "golang.org/x/sys/windows"

func (p *desktopPlatform) IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}
