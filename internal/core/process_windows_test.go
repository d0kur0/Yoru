package core

import (
	"io"
	"os/exec"
	"testing"

	"golang.org/x/sys/windows"
)

func TestCommandRunnerStartsCoreWithHighPriority(t *testing.T) {
	path, err := exec.LookPath("ping.exe")
	if err != nil {
		t.Skip("ping.exe is unavailable")
	}
	p, err := commandRunner{}.Start(path, []string{"-n", "30", "127.0.0.1"}, io.Discard)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = p.Stop(); _ = p.Wait() }()
	pid := uint32(p.(*commandProcess).cmd.Process.Pid)
	h, err := windows.OpenProcess(windows.PROCESS_QUERY_LIMITED_INFORMATION, false, pid)
	if err != nil {
		t.Fatal(err)
	}
	defer windows.CloseHandle(h)
	class, err := windows.GetPriorityClass(h)
	if err != nil {
		t.Fatal(err)
	}
	if class != windows.HIGH_PRIORITY_CLASS {
		t.Fatalf("priority class = %#x, want %#x", class, windows.HIGH_PRIORITY_CLASS)
	}
}
