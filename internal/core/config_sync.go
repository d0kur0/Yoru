package core

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"strconv"
	"strings"
	"time"
)

const configSyncBackup = "config-before-url-sync.json"

func validateConfigURL(value string) error {
	u, err := url.Parse(value)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || u.Fragment != "" || strings.ContainsAny(value, " \t\r\n") {
		return errors.New("Укажите HTTPS URL без логина и фрагмента")
	}
	if port := u.Port(); port != "" {
		if n, err := strconv.Atoi(port); err != nil || n < 1 || n > 65535 {
			return errors.New("Некорректный порт URL")
		}
	}
	return nil
}

// DecodeRemoteConfig accepts a complete Yoru configuration, never a partial
// settings object or a provider subscription. Validate order before normalize
// can repair a corrupt reference and inadvertently change routing priority.
func DecodeRemoteConfig(data []byte) (Config, error) {
	invalid := errors.New("Получена некорректная полная конфигурация Yoru")
	var fields map[string]json.RawMessage
	if len(data) > 2<<20 || json.Unmarshal(data, &fields) != nil {
		return Config{}, invalid
	}
	for _, key := range []string{"servers", "selected", "rules", "sets", "routeOrder", "subscriptions", "dns", "settings", "defaultAction"} {
		if raw, ok := fields[key]; !ok || string(raw) == "null" {
			return Config{}, invalid
		}
	}
	var raw Config
	if json.Unmarshal(data, &raw) != nil {
		return Config{}, invalid
	}
	available := map[RouteRef]bool{}
	for _, set := range raw.Sets {
		ref := RouteRef{"set", set.ID}
		if set.ID == "" || available[ref] {
			return Config{}, invalid
		}
		available[ref] = true
	}
	for _, rule := range raw.Rules {
		ref := RouteRef{"rule", rule.ID}
		if rule.ID == "" || available[ref] {
			return Config{}, invalid
		}
		available[ref] = true
	}
	for _, ref := range raw.RouteOrder {
		if !available[ref] {
			return Config{}, invalid
		}
		delete(available, ref)
	}
	if len(available) != 0 {
		return Config{}, invalid
	}
	// The source of this import always owns the remembered URL.
	raw.ConfigSyncURL = ""
	raw.normalize()
	if raw.Validate() != nil {
		return Config{}, invalid
	}
	return raw, nil
}

// SyncConfigURL fetches without holding the manager lock. The revision protects
// edits and background subscription refreshes made while the request is pending.
// A private, restorable backup is written before SaveLive's transactional apply.
func (m *Manager) SyncConfigURL(parent context.Context, source string, revision uint64) (Config, error) {
	if err := validateConfigURL(source); err != nil {
		return Config{}, err
	}
	ctx, cancel := m.operation(parent, 30*time.Second)
	defer cancel()
	client := *m.downloadClient
	client.Timeout = 30 * time.Second
	origin, _ := url.Parse(source)
	client.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 5 || validateConfigURL(req.URL.String()) != nil || !strings.EqualFold(req.URL.Host, origin.Host) {
			return errors.New("Перенаправление URL синхронизации запрещено")
		}
		return nil
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, source, nil)
	if err != nil {
		return Config{}, errors.New("Не удалось создать запрос конфигурации")
	}
	req.Header.Set("Accept", "application/json")
	resp, err := client.Do(req)
	if err != nil {
		// net/url errors include the personal URL. Never expose those to logs/UI.
		return Config{}, errors.New("Не удалось загрузить конфигурацию. Проверьте ссылку и подключение")
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return Config{}, fmt.Errorf("Сервер конфигурации ответил HTTP %d", resp.StatusCode)
	}
	data, err := readLimit(resp.Body, 2<<20)
	if err != nil {
		return Config{}, errors.New("Не удалось прочитать конфигурацию размером до 2 МБ")
	}
	next, err := DecodeRemoteConfig(data)
	if err != nil {
		return Config{}, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	if ctx.Err() != nil {
		return Config{}, errors.New("Загрузка конфигурации отменена или превышено время ожидания")
	}
	if revision != m.config.Revision {
		return Config{}, errors.New("Настройки изменились во время загрузки. Повторите синхронизацию")
	}
	local := m.config
	// These settings belong to this device and can require local OS permission.
	next.Settings.TUN, next.Settings.SystemProxy = local.Settings.TUN, local.Settings.SystemProxy
	next.Settings.Autostart, next.Settings.Minimized = local.Settings.Autostart, local.Settings.Minimized
	next.Settings.AutoConnect, next.Settings.Tray = local.Settings.AutoConnect, local.Settings.Tray
	next.TUNOptions.Interface, next.TUNOptions.AutoDetect = local.TUNOptions.Interface, local.TUNOptions.AutoDetect
	next.Revision, next.ConfigSyncURL = local.Revision, source
	backup, err := json.MarshalIndent(Backup{"mihomo-desktop", 1, local}, "", "  ")
	if err != nil {
		return Config{}, errors.New("Не удалось подготовить резервную копию")
	}
	if err := atomicWrite(filepath.Join(m.dir, configSyncBackup), backup, 0600); err != nil {
		return Config{}, errors.New("Не удалось сохранить резервную копию. Конфигурация не заменена")
	}
	if err := m.saveLive(ctx, ctx, next); err != nil {
		return Config{}, err
	}
	// A repeat import can change only local bookkeeping. Keep those fields in
	// the runtime snapshot current without hiding any real pending differences.
	if m.process != nil {
		var applied Config
		if json.Unmarshal(m.applied, &applied) == nil {
			applied.Revision, applied.ConfigSyncURL = m.config.Revision, source
			m.applied = runtimeSnapshot(applied)
		}
	}
	// Return a detached snapshot without reacquiring the held lock.
	data, _ = json.Marshal(m.config)
	var result Config
	_ = json.Unmarshal(data, &result)
	return result, nil
}
