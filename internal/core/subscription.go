package core

import (
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"time"

	"gopkg.in/yaml.v3"
)

func decodeBase64(s string) ([]byte, error) {
	s = strings.TrimSpace(s)
	for _, encoding := range []*base64.Encoding{base64.RawStdEncoding, base64.StdEncoding, base64.RawURLEncoding, base64.URLEncoding} {
		b, e := encoding.DecodeString(s)
		if e == nil {
			return b, nil
		}
	}
	return nil, errors.New("Некорректный Base64")
}
func ParseLink(raw string) (Server, error) {
	raw = strings.TrimSpace(raw)
	if strings.HasPrefix(raw, "[Interface]") {
		return parseWireGuard(raw)
	}
	if strings.HasPrefix(raw, "proxies:") {
		items, e := ParseSubscription([]byte(raw), "")
		if e != nil {
			return Server{}, e
		}
		if len(items) != 1 {
			return Server{}, errors.New("Добавляйте один сервер за раз")
		}
		return items[0], nil
	}
	if strings.HasPrefix(raw, "vmess://") || strings.HasPrefix(raw, "tuic://") || strings.HasPrefix(raw, "anytls://") {
		return parseExtendedLink(raw)
	}
	// Legacy SIP002 puts userinfo, host and port in a single base64 payload.
	if strings.HasPrefix(raw, "ss://") && !strings.Contains(raw, "@") {
		body, fragment, _ := strings.Cut(strings.TrimPrefix(raw, "ss://"), "#")
		decoded, e := decodeBase64(body)
		if e != nil {
			return Server{}, e
		}
		raw = "ss://" + string(decoded)
		if fragment != "" {
			raw += "#" + fragment
		}
	}
	u, e := url.Parse(raw)
	if e != nil || u.Hostname() == "" || u.User == nil {
		return Server{}, errors.New("Некорректная ссылка сервера")
	}
	q := u.Query()
	s := Server{Name: u.Fragment, Host: u.Hostname(), Port: 443, Secret: u.User.Username(), SNI: q.Get("sni"), PublicKey: q.Get("pbk"), ShortID: q.Get("sid"), Flow: q.Get("flow"), Fingerprint: q.Get("fp"), Path: q.Get("path"), WSHost: q.Get("host"), ALPN: q.Get("alpn"), Country: "Свой сервер", Flag: "🌐", Transport: "TLS"}
	if s.SNI == "" {
		s.SNI = q.Get("peer")
	}
	if s.Name == "" {
		s.Name = s.Host
	}
	if u.Port() != "" {
		s.Port, e = strconv.Atoi(u.Port())
		if e != nil {
			return s, e
		}
	}
	switch u.Scheme {
	case "vless":
		s.Protocol = "VLESS"
		if q.Get("security") == "none" {
			s.Transport = "TCP"
		}
	case "trojan":
		s.Protocol = "Trojan"
	case "hysteria2", "hy2":
		s.Protocol = "Hysteria2"
	case "ss":
		s.Protocol = "Shadowsocks"
		s.Transport = "TCP"
		if password, ok := u.User.Password(); ok {
			s.Cipher = s.Secret
			s.Secret = password
		} else {
			decoded, e := decodeBase64(s.Secret)
			if e != nil {
				return s, e
			}
			cipher, password, ok := strings.Cut(string(decoded), ":")
			if !ok {
				return s, errors.New("Некорректные параметры Shadowsocks")
			}
			s.Cipher = cipher
			s.Secret = password
		}
	default:
		return s, errors.New("Поддерживаются VLESS, Trojan, Shadowsocks и Hysteria2")
	}
	if q.Get("type") == "ws" {
		s.Transport = "WebSocket"
		if s.Protocol == "VLESS" {
			enabled := q.Get("security") != "none"
			s.TLS = &enabled
		}
	}
	if q.Get("security") == "reality" {
		s.Transport = "Reality"
	}
	if q.Get("type") == "xhttp" || q.Get("type") == "grpc" {
		return extendedTransport(raw, s, q)
	}
	if transport := q.Get("type"); transport != "" && transport != "tcp" && transport != "ws" {
		return s, fmt.Errorf("Транспорт %s добавляйте через YAML-подписку", transport)
	}
	if s.Protocol == "Hysteria2" {
		s.Options = map[string]any{}
		if password, ok := u.User.Password(); ok {
			s.Secret += ":" + password
		}
		if obfs := q.Get("obfs"); obfs != "" {
			if obfs != "salamander" && obfs != "gecko" {
				return s, errors.New("Hysteria2: неизвестный тип обфускации")
			}
			if q.Get("obfs-password") == "" {
				return s, errors.New("Hysteria2: в ссылке отсутствует obfs-password")
			}
			s.Options["obfs"] = obfs
			s.Options["obfs-password"] = q.Get("obfs-password")
		} else if q.Get("obfs-password") != "" {
			return s, errors.New("Hysteria2: в ссылке отсутствует тип obfs")
		}
		if v := q.Get("insecure"); v != "" {
			b, err := strconv.ParseBool(v)
			if err != nil {
				return s, errors.New("Hysteria2: неверный параметр insecure")
			}
			s.Options["skip-cert-verify"] = b
		}
		if v := q.Get("pinSHA256"); v != "" {
			s.Options["fingerprint"] = v
		}
		if v := q.Get("ech"); v != "" {
			s.Options["ech-opts"] = map[string]any{"enable": true, "config": v}
		}
	}
	if q.Get("plugin") != "" || (q.Get("obfs") != "" && s.Protocol != "Hysteria2") {
		return s, errors.New("Параметры plugin/obfs импортируйте через YAML-подписку")
	}
	hash := sha256.Sum256([]byte(raw))
	s.ID = hex.EncodeToString(hash[:12])
	_, e = s.Proxy()
	return s, e
}
func ParseSubscription(data []byte, id string) ([]Server, error) {
	var doc struct {
		Proxies []map[string]any `yaml:"proxies"`
	}
	servers := []Server{}
	if yaml.Unmarshal(data, &doc) == nil && len(doc.Proxies) > 0 {
		for _, p := range doc.Proxies {
			str := func(k string) string { v, _ := p[k].(string); return v }
			port, err := strconv.Atoi(fmt.Sprint(p["port"]))
			if err != nil {
				return nil, errors.New("Подписка содержит некорректный порт")
			}
			s := Server{Name: str("name"), Host: str("server"), Port: port, Transport: "Imported", Country: "Подписка", Flag: "🌐", Options: p, Subscription: id, SNI: str("servername"), Flow: str("flow"), Cipher: str("cipher"), Fingerprint: str("client-fingerprint")}
			if s.SNI == "" {
				s.SNI = str("sni")
			}
			switch str("type") {
			case "vmess":
				s.Protocol = "VMess"
				s.Secret = str("uuid")
			case "tuic":
				s.Protocol = "TUIC"
				s.Secret = str("password")
			case "anytls":
				s.Protocol = "AnyTLS"
				s.Secret = str("password")
			case "wireguard":
				s.Protocol = "WireGuard"
				s.Secret = str("private-key")
			case "vless":
				s.Protocol = "VLESS"
				s.Secret = str("uuid")
			case "ss":
				s.Protocol = "Shadowsocks"
				s.Secret = str("password")
			case "trojan":
				s.Protocol = "Trojan"
				s.Secret = str("password")
			case "hysteria2":
				s.Protocol = "Hysteria2"
				s.Secret = str("password")
			default:
				return nil, fmt.Errorf("Протокол %s из подписки пока не поддерживается", str("type"))
			}
			if _, e := s.Proxy(); e != nil {
				return nil, e
			}
			servers = append(servers, s)
		}
	} else {
		text := strings.TrimSpace(string(data))
		if !strings.Contains(text, "://") {
			b, e := decodeBase64(text)
			if e != nil {
				return nil, errors.New("Подписка должна содержать YAML proxies или ссылки серверов")
			}
			text = string(b)
		}
		for lineNumber, line := range strings.Split(text, "\n") {
			line = strings.TrimSpace(line)
			if line == "" || strings.HasPrefix(line, "#") {
				continue
			}
			s, e := ParseLink(line)
			if e != nil {
				return nil, fmt.Errorf("Строка %d: %w", lineNumber+1, e)
			}
			s.Subscription = id
			servers = append(servers, s)
		}
	}
	if len(servers) == 0 || len(servers) > 10000 {
		return nil, errors.New("Подписка пустая или содержит слишком много серверов")
	}
	seen := map[string]bool{}
	for i := range servers {
		s := &servers[i]
		if s.Name == "" {
			s.Name = s.Host
		}
		hash := sha256.Sum256([]byte(id + "\x00" + s.Name + "\x00" + s.Host + "\x00" + strconv.Itoa(s.Port)))
		s.ID = hex.EncodeToString(hash[:12])
		if seen[s.ID] {
			return nil, errors.New("Повторяющиеся серверы в подписке")
		}
		seen[s.ID] = true
	}
	return servers, nil
}

// FetchSubscription validates and downloads a draft without mutating saved settings.
func (m *Manager) FetchSubscription(ctx context.Context, source Subscription) ([]Server, error) {
	u, err := url.Parse(source.URL)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || source.ID == "" {
		return nil, errors.New("Укажите корректный HTTPS URL подписки")
	}
	ctx, cancel := m.operation(ctx, 45*time.Second)
	defer cancel()
	data, err := get(ctx, m.downloadClient, source.URL, 2<<20)
	if err != nil {
		return nil, err
	}
	return ParseSubscription(data, source.ID)
}

func (m *Manager) UpdateSubscription(ctx context.Context, id string) (Config, error) {
	m.mu.Lock()
	var source Subscription
	for _, s := range m.config.Subscriptions {
		if s.ID == id {
			source = s
			break
		}
	}
	m.mu.Unlock()
	if source.ID == "" {
		return Config{}, errors.New("Подписка не найдена")
	}
	ctx, cancel := m.operation(ctx, 45*time.Second)
	defer cancel()
	b, e := get(ctx, m.downloadClient, source.URL, 2<<20)
	if e != nil {
		return Config{}, e
	}
	servers, e := ParseSubscription(b, id)
	if e != nil {
		return Config{}, e
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	c := m.config
	idx := -1
	for i, s := range c.Subscriptions {
		if s.ID == id && s.URL == source.URL {
			idx = i
			break
		}
	}
	if idx < 0 {
		return Config{}, errors.New("Подписка изменилась во время загрузки")
	}
	c.Servers = append([]Server{}, c.Servers...)
	kept := []Server{}
	for _, s := range c.Servers {
		if s.Subscription != id {
			kept = append(kept, s)
		}
	}
	c.Servers = append(kept, servers...)
	selected := false
	for _, s := range c.Servers {
		selected = selected || s.ID == c.Selected
	}
	if !selected {
		c.Selected = c.Servers[0].ID
	}
	c.Subscriptions = append([]Subscription{}, c.Subscriptions...)
	c.Subscriptions[idx].Updated = time.Now().UTC().Format(time.RFC3339)
	if e = m.save(c); e != nil {
		return Config{}, e
	}
	c.Revision = m.config.Revision
	return c, nil
}

// Runs only after the desktop application is explicitly started. No core is spawned here.
func (m *Manager) RefreshDue(ctx context.Context) {
	c := m.Config()
	for _, s := range c.Subscriptions {
		hours, _ := strconv.Atoi(s.Interval)
		last, e := time.Parse(time.RFC3339, s.Updated)
		if e != nil || time.Since(last) >= time.Duration(hours)*time.Hour {
			_, _ = m.UpdateSubscription(ctx, s.ID)
		}
		if ctx.Err() != nil {
			return
		}
	}
}
func (m *Manager) RecoverProxy() error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.process != nil {
		return errors.New("Сначала остановите ядро")
	}
	p, ok := m.platform.(interface{ RecoverProxy() error })
	if !ok {
		return nil
	}
	return p.RecoverProxy()
}
