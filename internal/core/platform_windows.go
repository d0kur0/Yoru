package core

import (
	"encoding/binary"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"unicode/utf16"

	"golang.org/x/sys/windows"
	"golang.org/x/sys/windows/registry"
)

type desktopPlatform struct{ dir string }

func newPlatform(dir string) Platform { return &desktopPlatform{dir} }

// Autostart uses an elevated on-demand task; the Run entry is the opt-in trigger.
func (p *desktopPlatform) Autostart(enabled, minimized, tun bool) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if !enabled {
		e = k.DeleteValue("Yoru")
		if e != nil && !errors.Is(e, registry.ErrNotExist) {
			return e
		}
		cmd := exec.Command("schtasks.exe", "/delete", "/tn", "Yoru", "/f")
		cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
		_ = cmd.Run()
		return nil
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	user, e := windows.GetCurrentProcessToken().GetTokenUser()
	if e != nil {
		return e
	}
	file, e := os.CreateTemp("", "yoru-autostart-*.xml")
	if e != nil {
		return e
	}
	defer os.Remove(file.Name())
	_, writeErr := file.Write(autostartTaskFile(exe, user.User.Sid.String(), minimized))
	closeErr := file.Close()
	if e = errors.Join(writeErr, closeErr); e != nil {
		return e
	}
	// Replace existing task so its executable path and arguments stay current.
	cmd := exec.Command("schtasks.exe", "/create", "/tn", "Yoru", "/xml", file.Name(), "/f")
	cmd.SysProcAttr = &syscall.SysProcAttr{HideWindow: true}
	if out, e := cmd.CombinedOutput(); e != nil {
		return fmt.Errorf("настройка автозапуска: %s: %w", strings.TrimSpace(string(out)), e)
	}
	return k.SetStringValue("Yoru", `schtasks.exe /run /tn "Yoru"`)
}

// schtasks imports task files as Unicode. Match its UTF-16 declaration and BOM.
func autostartTaskFile(exe, sid string, minimized bool) []byte {
	xml := strings.Replace(autostartTaskXML(exe, sid, minimized), `encoding="UTF-8"`, `encoding="UTF-16"`, 1)
	units := utf16.Encode([]rune(xml))
	data := make([]byte, 2+len(units)*2)
	data[0], data[1] = 0xff, 0xfe
	for i, unit := range units {
		binary.LittleEndian.PutUint16(data[2+i*2:], unit)
	}
	return data
}

func autostartTaskXML(exe, sid string, minimized bool) string {
	escape := func(value string) string {
		var b strings.Builder
		_ = xml.EscapeText(&b, []byte(value))
		return b.String()
	}
	args := ""
	if minimized {
		args = "--minimized"
	}
	return `<?xml version="1.0" encoding="UTF-8"?>
<Task version="1.2" xmlns="http://schemas.microsoft.com/windows/2004/02/mit/task">
<Principals><Principal id="User"><UserId>` + escape(sid) + `</UserId><LogonType>InteractiveToken</LogonType><RunLevel>HighestAvailable</RunLevel></Principal></Principals>
<Settings><MultipleInstancesPolicy>IgnoreNew</MultipleInstancesPolicy><DisallowStartIfOnBatteries>false</DisallowStartIfOnBatteries><StopIfGoingOnBatteries>false</StopIfGoingOnBatteries><ExecutionTimeLimit>PT0S</ExecutionTimeLimit></Settings>
<Actions Context="User"><Exec><Command>` + escape(exe) + `</Command><Arguments>` + args + `</Arguments></Exec></Actions>
</Task>`
}

const internetKey = `Software\Microsoft\Windows\CurrentVersion\Internet Settings`

type proxySnapshot struct {
	Enabled                      uint32
	EnableExists                 bool
	Server, Override             string
	ServerExists, OverrideExists bool
	Owned                        string
}

func refreshProxy() {
	dll := syscall.NewLazyDLL("wininet.dll")
	f := dll.NewProc("InternetSetOptionW")
	_, _, _ = f.Call(0, 39, 0, 0)
	_, _, _ = f.Call(0, 37, 0, 0)
}
func (p *desktopPlatform) Proxy(port int) (func() error, error) {
	recovery := filepath.Join(p.dir, "proxy-recovery.json")
	if _, e := os.Stat(recovery); e == nil {
		return nil, errors.New("Есть незавершённое восстановление прокси. Нажмите «Восстановить прокси»")
	}
	k, e := registry.OpenKey(registry.CURRENT_USER, internetKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if e != nil {
		return nil, e
	}
	defer k.Close()
	// PAC can override manual proxy settings; do not silently overwrite it.
	if pac, _, _ := k.GetStringValue("AutoConfigURL"); strings.TrimSpace(pac) != "" {
		return nil, errors.New("В системе используется PAC. Отключите его вручную или используйте TUN")
	}
	var s proxySnapshot
	v, _, err := k.GetIntegerValue("ProxyEnable")
	s.Enabled = uint32(v)
	s.EnableExists = err == nil
	s.Server, _, err = k.GetStringValue("ProxyServer")
	s.ServerExists = err == nil
	s.Override, _, err = k.GetStringValue("ProxyOverride")
	s.OverrideExists = err == nil
	s.Owned = "127.0.0.1:" + strconv.Itoa(port)
	b, _ := json.Marshal(s)
	if e = atomicWrite(recovery, b, 0600); e != nil {
		return nil, e
	}
	e = k.SetStringValue("ProxyServer", s.Owned)
	if e == nil {
		e = k.SetStringValue("ProxyOverride", "localhost;127.*;[::1];<local>")
	}
	if e == nil {
		e = k.SetDWordValue("ProxyEnable", 1)
	}
	refreshProxy()
	restore := func() error { return p.RecoverProxy() }
	if e != nil {
		restoreErr := restore()
		return nil, errors.Join(e, restoreErr)
	}
	return restore, nil
}
func (p *desktopPlatform) RecoverProxy() error {
	file := filepath.Join(p.dir, "proxy-recovery.json")
	b, e := os.ReadFile(file)
	if errors.Is(e, os.ErrNotExist) {
		return nil
	}
	if e != nil {
		return e
	}
	var s proxySnapshot
	if e = json.Unmarshal(b, &s); e != nil {
		return e
	}
	k, e := registry.OpenKey(registry.CURRENT_USER, internetKey, registry.QUERY_VALUE|registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	current, _, _ := k.GetStringValue("ProxyServer")
	if current != s.Owned && current != s.Server {
		return errors.New("Прокси изменён другим приложением. Автоматическое восстановление отменено")
	}
	currentOverride, _, _ := k.GetStringValue("ProxyOverride")
	if currentOverride != s.Override && currentOverride != "localhost;127.*;[::1];<local>" {
		return errors.New("Исключения прокси изменены другим приложением")
	}
	set := func(name, value string, exists bool) error {
		if exists {
			return k.SetStringValue(name, value)
		}
		e := k.DeleteValue(name)
		if errors.Is(e, registry.ErrNotExist) {
			return nil
		}
		return e
	}
	if e = set("ProxyServer", s.Server, s.ServerExists); e != nil {
		return e
	}
	if e = set("ProxyOverride", s.Override, s.OverrideExists); e != nil {
		return e
	}
	if s.EnableExists {
		e = k.SetDWordValue("ProxyEnable", s.Enabled)
	} else {
		e = k.DeleteValue("ProxyEnable")
		if errors.Is(e, registry.ErrNotExist) {
			e = nil
		}
	}
	refreshProxy()
	if e != nil {
		return fmt.Errorf("Восстановление прокси: %w", e)
	}
	return os.Remove(file)
}
