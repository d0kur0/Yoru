//go:build windows

package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"syscall"
	"testing"
	"time"
)

func TestWindowsInstallerParameters(t *testing.T) {
	got, err := windowsInstallerParameters(`C:\Program Files\Custom Yoru\Yoru.exe`, 4271)
	if err != nil {
		t.Fatal(err)
	}
	if want := `/S /UPDATEPID=4271 /D=C:\Program Files\Custom Yoru`; got != want {
		t.Fatalf("parameters = %q, want %q", got, want)
	}
	if !strings.HasSuffix(got, `/D=C:\Program Files\Custom Yoru`) {
		t.Fatal("/D must be the final NSIS argument")
	}
}

func TestWindowsInstallerParametersRejectsUnsafeTarget(t *testing.T) {
	for _, test := range []struct {
		name string
		exe  string
		pid  int
	}{
		{"relative path", `bin\Yoru.exe`, 42},
		{"development binary", `C:\dev\Yoru-next.exe`, 42},
		{"root directory", `C:\Yoru.exe`, 42},
		{"missing pid", `C:\Apps\Yoru.exe`, 0},
		{"newline", "C:\\Apps\nOther\\Yoru.exe", 42},
	} {
		t.Run(test.name, func(t *testing.T) {
			if _, err := windowsInstallerParameters(test.exe, test.pid); err == nil {
				t.Fatal("unsafe target accepted")
			}
		})
	}
}

// Opt-in integration check. YORU_UPDATE_SMOKE builds an installer that writes
// only to the test directory and launches a tiny marker executable, never Yoru.
func TestWindowsSilentUpdateInstallerSmoke(t *testing.T) {
	if os.Getenv("YORU_INSTALLER_SMOKE") != "1" {
		t.Skip("set YORU_INSTALLER_SMOKE=1 to run the isolated NSIS check")
	}
	makensis, err := exec.LookPath("makensis")
	if err != nil {
		makensis = filepath.Join(os.Getenv("ProgramFiles(x86)"), "NSIS", "makensis.exe")
		if _, statErr := os.Stat(makensis); statErr != nil {
			t.Fatalf("makensis is unavailable: %v", err)
		}
	}

	root := t.TempDir()
	source := filepath.Join(root, "marker.go")
	program := `package main
import ("os"; "time")
var variant = "new"
func main() {
    if variant == "old" {
        d, _ := time.ParseDuration(os.Getenv("YORU_SMOKE_OLD_DURATION"))
        time.Sleep(d)
        return
    }
    if marker := os.Getenv("YORU_SMOKE_MARKER"); marker != "" {
        _ = os.WriteFile(marker, []byte("relaunched"), 0600)
    }
}`
	if err := os.WriteFile(source, []byte(program), 0600); err != nil {
		t.Fatal(err)
	}
	oldBinary := filepath.Join(root, "old.exe")
	newBinary := filepath.Join(root, "new.exe")
	for _, build := range []struct{ variant, output string }{{"old", oldBinary}, {"new", newBinary}} {
		cmd := exec.Command("go", "build", "-ldflags=-H=windowsgui -X main.variant="+build.variant, "-o", build.output, source)
		if output, err := cmd.CombinedOutput(); err != nil {
			t.Fatalf("build %s marker: %v\n%s", build.variant, err, output)
		}
	}
	newContent, err := os.ReadFile(newBinary)
	if err != nil {
		t.Fatal(err)
	}
	oldContent, err := os.ReadFile(oldBinary)
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Equal(newContent, oldContent) {
		t.Fatal("smoke binaries must differ to prove file replacement")
	}

	nsisDir := filepath.Join("build", "windows", "nsis")
	cmd := exec.Command(makensis, "-DYORU_UPDATE_SMOKE="+root, "-DARG_WAILS_AMD64_BINARY="+newBinary, "project.nsi")
	cmd.Dir = nsisDir
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("compile smoke installer: %v\n%s", err, output)
	}
	installer := filepath.Join(root, "installer.exe")

	for _, test := range []struct {
		name       string
		oldFor     string
		passSilent bool
		wantExit   int
		wantUpdate bool
	}{
		{"wait succeeds", "150ms", true, 0, true},
		{"old updater arguments", "150ms", false, 0, true},
		{"wait times out", "3s", true, 67, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			installDir := filepath.Join(root, test.name, "Program Files", "Yoru")
			if err := os.MkdirAll(installDir, 0700); err != nil {
				t.Fatal(err)
			}
			installed := filepath.Join(installDir, "Yoru.exe")
			if err := os.WriteFile(installed, oldContent, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(root, test.name, "relaunched.txt")
			old := exec.Command(installed)
			old.Env = append(os.Environ(), "YORU_SMOKE_OLD_DURATION="+test.oldFor)
			if err := old.Start(); err != nil {
				t.Fatal(err)
			}
			defer old.Wait()

			// NSIS /D consumes the raw remainder of the command line, including
			// spaces. Go's normal argv quoting would change that syntax.
			args := ""
			if test.passSilent {
				args = " /S"
			}
			commandLine := fmt.Sprintf(`"%s"%s /UPDATEPID=%d /D=%s`, installer, args, old.Process.Pid, installDir)
			update := exec.Command(installer)
			update.SysProcAttr = &syscall.SysProcAttr{CmdLine: commandLine}
			update.Env = append(os.Environ(), "YORU_SMOKE_MARKER="+marker)
			output, err := update.CombinedOutput()
			exitCode := 0
			if err != nil {
				var exitErr *exec.ExitError
				if !errors.As(err, &exitErr) {
					t.Fatalf("run smoke installer: %v\n%s", err, output)
				}
				exitCode = exitErr.ExitCode()
			}
			if exitCode != test.wantExit {
				t.Fatalf("installer exit = %d, want %d; output: %s", exitCode, test.wantExit, output)
			}
			got, err := os.ReadFile(installed)
			if err != nil {
				t.Fatal(err)
			}
			want := oldContent
			if test.wantUpdate {
				want = newContent
			}
			if !bytes.Equal(got, want) {
				t.Fatal("installed executable does not match expected version")
			}
			if !test.wantUpdate {
				if _, err := os.Stat(marker); !os.IsNotExist(err) {
					t.Fatalf("failed install relaunched marker: %v", err)
				}
				return
			}
			deadline := time.Now().Add(3 * time.Second)
			for time.Now().Before(deadline) {
				if content, err := os.ReadFile(marker); err == nil && string(content) == "relaunched" {
					return
				}
				time.Sleep(20 * time.Millisecond)
			}
			t.Fatal("successful update did not relaunch installed executable")
		})
	}

	t.Run("file copy fails", func(t *testing.T) {
		installDir := filepath.Join(root, "copy fails", "Yoru")
		installed := filepath.Join(installDir, "Yoru.exe")
		if err := os.MkdirAll(installed, 0700); err != nil {
			t.Fatal(err)
		}
		marker := filepath.Join(root, "copy fails", "relaunched.txt")
		old := exec.Command(oldBinary)
		old.Env = append(os.Environ(), "YORU_SMOKE_OLD_DURATION=150ms")
		if err := old.Start(); err != nil {
			t.Fatal(err)
		}
		defer old.Wait()
		update := exec.Command(installer)
		update.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`"%s" /S /UPDATEPID=%d /D=%s`, installer, old.Process.Pid, installDir)}
		update.Env = append(os.Environ(), "YORU_SMOKE_MARKER="+marker)
		output, err := update.CombinedOutput()
		var exitErr *exec.ExitError
		if !errors.As(err, &exitErr) || exitErr.ExitCode() == 0 {
			t.Fatalf("file-copy failure was not reported: %v\n%s", err, output)
		}
		if _, err := os.Stat(marker); !os.IsNotExist(err) {
			t.Fatalf("failed file copy relaunched marker: %v", err)
		}
		info, err := os.Stat(installed)
		if err != nil || !info.IsDir() {
			t.Fatalf("failed file copy changed target: %v", err)
		}
	})

	for _, pid := range []string{"0", "-1", "abc"} {
		t.Run("invalid pid "+pid, func(t *testing.T) {
			installDir := filepath.Join(root, "invalid pid "+pid)
			if err := os.MkdirAll(installDir, 0700); err != nil {
				t.Fatal(err)
			}
			installed := filepath.Join(installDir, "Yoru.exe")
			if err := os.WriteFile(installed, oldContent, 0700); err != nil {
				t.Fatal(err)
			}
			marker := filepath.Join(installDir, "relaunched.txt")
			update := exec.Command(installer)
			update.SysProcAttr = &syscall.SysProcAttr{CmdLine: fmt.Sprintf(`"%s" /S /UPDATEPID=%s /D=%s`, installer, pid, installDir)}
			update.Env = append(os.Environ(), "YORU_SMOKE_MARKER="+marker)
			output, err := update.CombinedOutput()
			var exitErr *exec.ExitError
			if !errors.As(err, &exitErr) || exitErr.ExitCode() != 66 {
				t.Fatalf("invalid PID exit = %v, want 66; output: %s", err, output)
			}
			got, err := os.ReadFile(installed)
			if err != nil || !bytes.Equal(got, oldContent) {
				t.Fatalf("invalid PID replaced target: %v", err)
			}
			if _, err := os.Stat(marker); !os.IsNotExist(err) {
				t.Fatalf("invalid PID relaunched marker: %v", err)
			}
		})
	}
}
