package core

import (
	"context"
	"io"
	"os/exec"
)

type Process interface {
	Wait() error
	Stop() error
}
type Runner interface {
	Validate(context.Context, string, []string) error
	Start(string, []string, io.Writer) (Process, error)
}
type commandRunner struct{}
type commandProcess struct {
	cmd     *exec.Cmd
	release func()
}

func (commandRunner) Validate(ctx context.Context, path string, args []string) error {
	c := exec.CommandContext(ctx, path, args...)
	hideConsole(c)
	return c.Run()
}
func (commandRunner) Start(path string, args []string, output io.Writer) (Process, error) {
	c := exec.Command(path, args...)
	hideConsole(c)
	c.Stdout = output
	c.Stderr = output
	if e := c.Start(); e != nil {
		return nil, e
	}
	release, e := protectChild(c)
	if e != nil {
		_ = c.Process.Kill()
		_ = c.Wait()
		return nil, e
	}
	return &commandProcess{c, release}, nil
}
func (p *commandProcess) Wait() error { defer p.release(); return p.cmd.Wait() }
