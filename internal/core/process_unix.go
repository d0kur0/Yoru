//go:build !windows

package core

import (
	"os/exec"
	"syscall"
	"time"
)

func hideConsole(c *exec.Cmd)                  {}
func protectChild(c *exec.Cmd) (func(), error) { return func() {}, nil }
func (p *commandProcess) Stop() error {
	// Signal only our child; a delayed kill bounds shutdown if it ignores SIGTERM.
	e := p.cmd.Process.Signal(syscall.SIGTERM)
	time.AfterFunc(3*time.Second, func() { _ = p.cmd.Process.Kill() })
	return e
}
