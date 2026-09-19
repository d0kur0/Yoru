package core

import (
	"bytes"
	"context"
	"encoding/json"
	"encoding/xml"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

type desktopPlatform struct{ dir string }

func newPlatform(dir string) Platform { return &desktopPlatform{dir} }

// IsElevated always reports true here: macOS doesn't participate in the
// Windows admin-elevation flow (see elevate_windows.go), so this just keeps
// Status().NeedsElevation from spuriously showing an admin-rights prompt.
func (p *desktopPlatform) IsElevated() bool { return true }

// tun is unused on macOS: TUN doesn't change how autostart is registered here.
func (p *desktopPlatform) Autostart(enabled, minimized, tun bool) error {
	home, e := os.UserHomeDir()
	if e != nil {
		return e
	}
	path := filepath.Join(home, "Library", "LaunchAgents", "local.yoru.desktop.plist")
	if !enabled {
		e = os.Remove(path)
		if errors.Is(e, os.ErrNotExist) {
			return nil
		}
		return e
	}
	exe, e := os.Executable()
	if e != nil {
		return e
	}
	var escaped bytes.Buffer
	_ = xml.EscapeText(&escaped, []byte(exe))
	args := "<string>" + escaped.String() + "</string>"
	if minimized {
		args += "<string>--minimized</string>"
	}
	return atomicWrite(path, []byte(`<?xml version="1.0" encoding="UTF-8"?><!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd"><plist version="1.0"><dict><key>Label</key><string>local.yoru.desktop</string><key>ProgramArguments</key><array>`+args+`</array><key>RunAtLoad</key><true/></dict></plist>`), 0600)
}
func network(args ...string) (string, error) {
	ctx, cancel := context.WithTimeout(context.Background(), 8*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/sbin/networksetup", args...)
	b, e := cmd.CombinedOutput()
	if e != nil {
		return "", errors.New("networksetup: нужны права на изменение сетевых настроек")
	}
	if strings.Contains(string(b), "Error:") {
		return "", errors.New("networksetup отклонил изменение настроек")
	}
	return string(b), nil
}

type macProxy struct {
	Service, Kind, Server, Port string
	Enabled                     bool
}
type macSnapshot struct {
	Port  string
	Items []macProxy
}

func readProxy(service, kind string) (macProxy, error) {
	s := macProxy{Service: service, Kind: kind}
	b, e := network("-get"+kind+"proxy", service)
	if e != nil {
		return s, e
	}
	for _, line := range strings.Split(b, "\n") {
		key, val, ok := strings.Cut(line, ": ")
		if !ok {
			continue
		}
		switch key {
		case "Enabled":
			s.Enabled = val == "Yes"
		case "Server":
			s.Server = val
		case "Port":
			s.Port = val
		case "Authenticated Proxy Enabled":
			if val == "1" {
				return s, errors.New("Прокси с аутентификацией нельзя заменить автоматически")
			}
		}
	}
	return s, nil
}
func setProxy(s macProxy) error {
	host := s.Server
	if host == "" {
		host = "127.0.0.1"
	}
	port := s.Port
	if port == "" || port == "0" {
		port = "1"
	}
	if _, e := network("-set"+s.Kind+"proxy", s.Service, host, port); e != nil {
		return e
	}
	state := "off"
	if s.Enabled {
		state = "on"
	}
	_, e := network("-set"+s.Kind+"proxystate", s.Service, state)
	return e
}
func (p *desktopPlatform) Proxy(port int) (func() error, error) {
	file := filepath.Join(p.dir, "proxy-recovery.json")
	if _, e := os.Stat(file); e == nil {
		return nil, errors.New("Сначала восстановите предыдущие настройки прокси")
	}
	list, e := network("-listallnetworkservices")
	if e != nil {
		return nil, e
	}
	snapshot := macSnapshot{Port: strconv.Itoa(port)}
	for i, line := range strings.Split(strings.TrimSpace(list), "\n") {
		if i == 0 || line == "" || strings.HasPrefix(line, "*") {
			continue
		}
		pac, e := network("-getautoproxyurl", line)
		if e != nil {
			return nil, e
		}
		if strings.Contains(pac, "Enabled: Yes") {
			return nil, errors.New("В системе используется PAC. Отключите его вручную или используйте TUN")
		}
		for _, kind := range []string{"web", "secureweb", "socksfirewall"} {
			s, e := readProxy(line, kind)
			if e != nil {
				return nil, e
			}
			snapshot.Items = append(snapshot.Items, s)
		}
	}
	b, _ := json.Marshal(snapshot)
	if e = atomicWrite(file, b, 0600); e != nil {
		return nil, e
	}
	for _, s := range snapshot.Items {
		s.Server = "127.0.0.1"
		s.Port = snapshot.Port
		s.Enabled = true
		if e = setProxy(s); e != nil {
			return nil, errors.Join(e, p.RecoverProxy())
		}
	}
	return p.RecoverProxy, nil
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
	var s macSnapshot
	if e = json.Unmarshal(b, &s); e != nil {
		return e
	}
	var result error
	for _, old := range s.Items {
		cur, e := readProxy(old.Service, old.Kind)
		if e != nil {
			result = errors.Join(result, e)
			continue
		}
		if cur.Server == old.Server && cur.Port == old.Port && cur.Enabled == old.Enabled {
			continue
		}
		if cur.Server != "127.0.0.1" || cur.Port != s.Port {
			result = errors.Join(result, fmt.Errorf("Прокси %s изменён другим приложением", old.Service))
			continue
		}
		result = errors.Join(result, setProxy(old))
	}
	if result != nil {
		return result
	}
	return os.Remove(file)
}
