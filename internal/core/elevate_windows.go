package core

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
	"syscall"

	"golang.org/x/sys/windows"
)

// elevationTaskName is the Windows Task Scheduler entry that lets Yoru relaunch
// itself elevated without a UAC prompt on every run: the trust decision is made
// once, at creation time (see ensureElevationTask), and every later
// "schtasks /run" against it is silent.
const elevationTaskName = "Yoru"

func elevationTaskExists() bool {
	return exec.Command("schtasks", "/query", "/tn", elevationTaskName).Run() == nil
}

func isAdministrator() (bool, error) {
	var sid *windows.SID
	e := windows.AllocateAndInitializeSid(&windows.SECURITY_NT_AUTHORITY, 2,
		windows.SECURITY_BUILTIN_DOMAIN_RID, windows.DOMAIN_ALIAS_RID_ADMINS,
		0, 0, 0, 0, 0, 0, &sid)
	if e != nil {
		return false, e
	}
	defer func() { _ = windows.FreeSid(sid) }()
	return windows.Token(0).IsMember(sid)
}

// ensureElevationTask creates elevationTaskName if it doesn't exist yet. The
// task's own trigger is a one-time schedule already far in the past, so it
// never fires on its own - it exists purely as an on-demand "run me elevated"
// target for RequestElevatedRelaunch and for the TUN-autostart Run key entry.
// Creating it needs admin rights; schtasks itself has no elevation manifest,
// so a plain "schtasks /create ... /rl highest" from a standard token just
// fails - the UAC consent has to be requested explicitly, which is what the
// one-shot elevated PowerShell Start-Process below does.
func ensureElevationTask() error {
	if elevationTaskExists() {
		return nil
	}
	admin, e := isAdministrator()
	if e != nil {
		return fmt.Errorf("не удалось проверить права учётной записи: %w", e)
	}
	if !admin {
		return errors.New("учётная запись не входит в группу администраторов Windows; запуск TUN с правами администратора недоступен")
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	// 1223 (ERROR_CANCELLED) is our own sentinel, returned from the catch
	// block below - not something PowerShell guarantees on decline, but since
	// we control both sides we can rely on it to distinguish "user clicked
	// No" from any other failure.
	script := "try {\n" +
		"  $p = Start-Process -FilePath 'schtasks.exe' -ArgumentList @('/create','/tn','" + elevationTaskName + "','/tr','\"" + exe + "\"','/sc','once','/sd','01/01/2000','/st','00:00','/rl','highest','/f') -Verb RunAs -Wait -PassThru -WindowStyle Hidden\n" +
		"  exit $p.ExitCode\n" +
		"} catch {\n" +
		"  exit 1223\n" +
		"}"
	cmd := exec.Command("powershell.exe", "-NoProfile", "-NonInteractive", "-Command", script)
	e = cmd.Run()
	if e == nil {
		return nil
	}
	var exitErr *exec.ExitError
	if errors.As(e, &exitErr) && exitErr.ExitCode() == 1223 {
		return errors.New("запрос прав администратора отменён")
	}
	return fmt.Errorf("не удалось настроить запуск с правами администратора: %w", e)
}

func (p *desktopPlatform) IsElevated() bool {
	return windows.GetCurrentProcessToken().IsElevated()
}

// RequestElevatedRelaunch ensures the elevation task exists (may show one UAC
// prompt, only the first time) and spawns a detached helper that will trigger
// it a couple seconds from now. The delay - and the separate process - give
// the caller time to quit and release Wails' single-instance lock before the
// elevated replacement tries to start; see the helper flag handling in
// main.go for the other half of this dance.
func (p *desktopPlatform) RequestElevatedRelaunch() error {
	if e := ensureElevationTask(); e != nil {
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	cmd := exec.Command(exe, "--elevate-relaunch-helper")
	// CREATE_NEW_PROCESS_GROUP | CREATE_NO_WINDOW: outlives us regardless of
	// how we exit, and never flashes a console even if run from one.
	cmd.SysProcAttr = &syscall.SysProcAttr{CreationFlags: 0x00000208}
	return cmd.Start()
}

// RelaunchElevated runs the pre-approved scheduled task now. Once
// ensureElevationTask has created it, this never shows a UAC prompt.
func RelaunchElevated() error {
	out, e := exec.Command("schtasks", "/run", "/tn", elevationTaskName).CombinedOutput()
	if e != nil {
		return fmt.Errorf("schtasks /run: %s: %w", strings.TrimSpace(string(out)), e)
	}
	return nil
}
