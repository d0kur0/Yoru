package core

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"golang.org/x/sys/windows/registry"
)

type desktopPlatform struct{ dir string }

func newPlatform(dir string) Platform { return &desktopPlatform{dir} }
func (p *desktopPlatform) Autostart(enabled, minimized bool) error {
	k, _, e := registry.CreateKey(registry.CURRENT_USER, `Software\Microsoft\Windows\CurrentVersion\Run`, registry.SET_VALUE)
	if e != nil {
		return e
	}
	defer k.Close()
	if !enabled {
		e = k.DeleteValue("MihomoDesktop")
		if errors.Is(e, registry.ErrNotExist) {
			return nil
		}
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	value := `"` + exe + `"`
	if minimized {
		value += " --minimized"
	}
	return k.SetStringValue("MihomoDesktop", value)
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
