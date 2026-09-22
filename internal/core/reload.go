package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"reflect"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

// ReloadConfig updates routing without restarting the core or its TUN listener.
func (m *Manager) ReloadConfig(parent context.Context) error {
	ctx, cancel := m.operation(parent, 30*time.Second)
	defer cancel()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process == nil {
		return errors.New("Сначала подключитесь к VPN")
	}
	return m.reloadConfig(ctx, m.config)
}

// Called under m.mu. Keep the exact TUN configuration until an explicit restart.
func (m *Manager) reloadConfig(ctx context.Context, c Config) error {
	var before liveSettings
	if err := m.api(ctx, http.MethodGet, "/configs", nil, &before); err != nil {
		return err
	}
	var applied Config
	if err := json.Unmarshal(m.applied, &applied); err != nil {
		return err
	}
	document := func(config Config) (map[string]any, error) {
		data, err := config.YAML(strings.TrimPrefix(m.endpoint, "http://"), m.secret, before.MixedPort)
		if err != nil {
			return nil, err
		}
		var doc map[string]any
		err = yaml.Unmarshal(data, &doc)
		return doc, err
	}
	old, err := document(applied)
	if err != nil {
		return err
	}
	next, err := document(c)
	if err != nil {
		return err
	}
	tun := m.retainedTUN
	if tun == nil {
		tun = old["tun"].(map[string]any)
	}
	pendingTUN := !reflect.DeepEqual(next["tun"], tun)
	old["tun"], next["tun"] = tun, tun
	proxyBefore := m.restoreProxy != nil
	var global struct {
		Now string `json:"now"`
	}
	if before.Mode == "global" {
		if err := m.api(ctx, http.MethodGet, "/proxies/GLOBAL", nil, &global); err != nil {
			return err
		}
	}
	apply := func(ctx context.Context, doc map[string]any) error {
		payload, err := yaml.Marshal(doc)
		if err != nil {
			return err
		}
		client := *m.client
		client.Timeout = 25 * time.Second
		return m.apiWithClient(ctx, &client, http.MethodPut, "/configs?force=false", map[string]string{"payload": string(payload)}, nil)
	}
	rollback := func(cause error) error {
		ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
		defer cancel()
		undoErr := apply(ctx, old)
		if global.Now != "" {
			undoErr = errors.Join(undoErr, m.api(ctx, http.MethodPut, "/proxies/GLOBAL", map[string]string{"name": global.Now}, nil))
		}
		undoErr = errors.Join(undoErr, m.setLiveProxy(proxyBefore, before.MixedPort))
		if err := undoErr; err != nil {
			m.lastError = "Не удалось восстановить конфигурацию ядра; повторите обновление"
			return errors.Join(cause, errors.New(m.lastError), err)
		}
		return cause
	}
	if err := apply(ctx, next); err != nil {
		return rollback(err)
	}
	if c.Settings.Mode == "global" {
		if err := m.api(ctx, http.MethodPut, "/proxies/GLOBAL", map[string]string{"name": "PROXY"}, nil); err != nil {
			return rollback(err)
		}
	}
	if err := m.setLiveProxy(c.Settings.SystemProxy, before.MixedPort); err != nil {
		return rollback(err)
	}
	if err := m.save(c); err != nil {
		return rollback(err)
	}
	m.applied = runtimeSnapshot(m.config)
	m.retainedTUN, m.pendingTUN = tun, pendingTUN
	m.lastError = ""
	return nil
}
