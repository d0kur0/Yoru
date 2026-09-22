package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"reflect"
	"time"
)

type liveSettings struct {
	Mode      string `json:"mode"`
	MixedPort int    `json:"mixed-port"`
	TUN       struct {
		Enable bool `json:"enable"`
	} `json:"tun"`
}

// SaveLive patches connection controls or reloads routing in the current core.
// A failed runtime change does not persist a false state.
func (m *Manager) SaveLive(parent context.Context, c Config) error {
	ctx, cancel := m.operation(parent, 30*time.Second)
	defer cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	c.normalize()
	if c.Revision != m.config.Revision {
		return errors.New("Настройки обновились в фоне. Повторите изменение")
	}
	if err := c.Validate(); err != nil {
		return err
	}
	tunChanged := c.Settings.TUN != m.config.Settings.TUN
	modeChanged := c.Settings.Mode != m.config.Settings.Mode
	proxyChanged := c.Settings.SystemProxy != m.config.Settings.SystemProxy
	routingChanged := !reflect.DeepEqual(c.Settings.RouteExclusions, m.config.Settings.RouteExclusions) || !reflect.DeepEqual(c.TUNOptions, m.config.TUNOptions) || !reflect.DeepEqual(c.Rules, m.config.Rules) || !reflect.DeepEqual(c.Sets, m.config.Sets) || !reflect.DeepEqual(c.DNS, m.config.DNS) || c.DefaultAction != m.config.DefaultAction || c.Settings.Sniffer != m.config.Settings.Sniffer || c.Settings.IPv6 != m.config.Settings.IPv6
	if m.process != nil && routingChanged && !tunChanged && !modeChanged && !proxyChanged {
		return m.reloadConfig(ctx, c)
	}
	if m.process == nil || (!tunChanged && !modeChanged && !proxyChanged) {
		return m.save(c)
	}
	var applied Config
	if err := json.Unmarshal(m.applied, &applied); err != nil {
		return errors.New("Не удалось прочитать настройки работающего ядра")
	}
	var before liveSettings
	if err := m.api(ctx, http.MethodGet, "/configs", nil, &before); err != nil {
		return err
	}
	patch, rollback := map[string]any{}, map[string]any{}
	if tunChanged {
		patch["tun"] = map[string]bool{"enable": c.Settings.TUN}
		rollback["tun"] = map[string]bool{"enable": before.TUN.Enable}
	}
	if modeChanged {
		patch["mode"] = c.Settings.Mode
		rollback["mode"] = before.Mode
	}
	var global struct {
		Now string `json:"now"`
	}
	changeGlobal := modeChanged && c.Settings.Mode == "global"
	if changeGlobal {
		if err := m.api(ctx, http.MethodGet, "/proxies/GLOBAL", nil, &global); err != nil {
			return err
		}
		if global.Now == "" {
			return errors.New("Не удалось определить текущий маршрут GLOBAL")
		}
	}
	proxyBefore := m.restoreProxy != nil
	rollbackChanges := func(cause error) error {
		undoCtx, undoCancel := context.WithTimeout(context.Background(), 8*time.Second)
		defer undoCancel()
		var undoError error
		if len(rollback) > 0 {
			undoError = m.api(undoCtx, http.MethodPatch, "/configs", rollback, nil)
		}
		if changeGlobal {
			undoError = errors.Join(undoError, m.api(undoCtx, http.MethodPut, "/proxies/GLOBAL", map[string]string{"name": global.Now}, nil))
		}
		if proxyChanged {
			undoError = errors.Join(undoError, m.setLiveProxy(proxyBefore, before.MixedPort))
		}
		if undoError != nil {
			m.lastError = "Не удалось восстановить предыдущие настройки подключения"
			return errors.Join(cause, errors.New(m.lastError), undoError)
		}
		return cause
	}
	if changeGlobal {
		if err := m.api(ctx, http.MethodPut, "/proxies/GLOBAL", map[string]string{"name": "PROXY"}, nil); err != nil {
			return rollbackChanges(err)
		}
	}
	if len(patch) > 0 {
		if err := m.api(ctx, http.MethodPatch, "/configs", patch, nil); err != nil {
			return rollbackChanges(err)
		}
		var actual liveSettings
		if err := m.api(ctx, http.MethodGet, "/configs", nil, &actual); err != nil {
			return rollbackChanges(err)
		}
		if tunChanged && actual.TUN.Enable != c.Settings.TUN {
			return rollbackChanges(errors.New("Mihomo не смог переключить TUN. Проверьте права администратора и журнал ядра"))
		}
		if modeChanged && actual.Mode != c.Settings.Mode {
			return rollbackChanges(errors.New("Mihomo не применил режим маршрутизации"))
		}
	}
	if proxyChanged {
		if err := m.setLiveProxy(c.Settings.SystemProxy, before.MixedPort); err != nil {
			return rollbackChanges(fmt.Errorf("Системный прокси: %w", err))
		}
	}
	if err := m.save(c); err != nil {
		return rollbackChanges(err)
	}
	if tunChanged {
		applied.Settings.TUN = c.Settings.TUN
		if m.retainedTUN != nil {
			m.retainedTUN["enable"] = c.Settings.TUN
		}
	}
	if modeChanged {
		applied.Settings.Mode = c.Settings.Mode
	}
	if proxyChanged {
		applied.Settings.SystemProxy = c.Settings.SystemProxy
	}
	applied.Revision = m.config.Revision
	m.applied = runtimeSnapshot(applied)
	m.lastError = ""
	return nil
}

// Called under m.mu. Reuses the owned runtime port and proxy recovery mechanism.
func (m *Manager) setLiveProxy(enabled bool, port int) error {
	if enabled == (m.restoreProxy != nil) {
		return nil
	}
	if enabled {
		if port <= 0 || port > 65535 {
			return errors.New("Порт работающего прокси недоступен")
		}
		restore, err := m.platform.Proxy(port)
		if err != nil {
			return err
		}
		m.restoreProxy = restore
		return nil
	}
	if err := m.restoreProxy(); err != nil {
		return err
	}
	m.restoreProxy = nil
	return nil
}
