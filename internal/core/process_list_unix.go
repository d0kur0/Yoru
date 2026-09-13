//go:build !windows

package core

import (
	"context"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

func ListProcesses(ctx context.Context) ([]ProcessInfo, error) {
	ctx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	// comm asks for the executable, not args. Paths may contain spaces.
	data, e := exec.CommandContext(ctx, "/bin/ps", "-axo", "pid=,comm=").Output()
	if e != nil {
		return nil, e
	}
	items := []ProcessInfo{}
	for _, line := range strings.Split(string(data), "\n") {
		line = strings.TrimSpace(line)
		i := strings.IndexAny(line, " \t")
		if i < 1 {
			continue
		}
		pid, e := strconv.Atoi(line[:i])
		if e != nil {
			continue
		}
		path := strings.TrimSpace(line[i:])
		name := filepath.Base(path)
		if !filepath.IsAbs(path) {
			path = ""
		}
		items = append(items, ProcessInfo{PID: pid, Name: name, Path: path})
	}
	return sortProcesses(items), nil
}
