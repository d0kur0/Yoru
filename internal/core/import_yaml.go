package core

import (
	"errors"
	"fmt"
	"gopkg.in/yaml.v3"
	"sort"
	"strings"
)

type YAMLImport struct {
	Config   Config   `json:"config"`
	Warnings []string `json:"warnings"`
}

func ImportYAML(data []byte) (YAMLImport, error) {
	out := YAMLImport{Config: DefaultConfig(), Warnings: []string{}}
	c := &out.Config
	if len(data) > 2<<20 {
		return out, errors.New("Конфигурация больше 2 МБ")
	}
	var doc map[string]any
	if yaml.Unmarshal(data, &doc) != nil || doc == nil {
		return out, errors.New("Некорректный YAML")
	}
	servers, e := ParseSubscription(data, "")
	if e != nil {
		return out, e
	}
	c.Servers = servers
	c.Selected = servers[0].ID
	groups, ok := doc["proxy-groups"].([]any)
	if !ok || len(groups) != 1 {
		return out, errors.New("Импорт поддерживает одну select-группу; сложные группы требуют отдельного переноса")
	}
	g, ok := groups[0].(map[string]any)
	if !ok || g["type"] != "select" {
		return out, errors.New("Нужна select-группа прокси")
	}
	group, _ := g["name"].(string)
	convert := func(a string) string {
		if a == group {
			return "PROXY"
		}
		return a
	}
	rawRules, ok := doc["rules"].([]any)
	if !ok {
		return out, errors.New("Отсутствуют rules")
	}
	match := false
	for i, raw := range rawRules {
		line, ok := raw.(string)
		if !ok {
			return out, errors.New("Некорректная строка rules")
		}
		parts := strings.Split(line, ",")
		for j := range parts {
			parts[j] = strings.TrimSpace(parts[j])
		}
		if parts[0] == "MATCH" {
			if len(parts) != 2 || i != len(rawRules)-1 {
				return out, errors.New("MATCH должен быть последним")
			}
			c.DefaultAction = convert(parts[1])
			match = true
			continue
		}
		if len(parts) < 3 || len(parts) > 4 {
			return out, fmt.Errorf("Правило %d: неподдерживаемый формат", i+1)
		}
		r := Rule{ID: fmt.Sprintf("import-%d", i), Type: parts[0], Value: parts[1], Action: convert(parts[2])}
		if len(parts) == 4 {
			if parts[3] != "no-resolve" {
				return out, fmt.Errorf("Правило %d: неизвестный параметр", i+1)
			}
			r.NoResolve = true
		}
		c.Rules = append(c.Rules, r)
	}
	if !match {
		return out, errors.New("Укажите финальное правило MATCH")
	}
	if v, ok := doc["mode"].(string); ok {
		c.Settings.Mode = v
	}
	if v, ok := doc["log-level"].(string); ok {
		c.Logging.Level = v
	}
	if v, ok := doc["ipv6"].(bool); ok {
		c.Settings.IPv6 = v
	}
	list := func(v any) ([]string, error) {
		switch x := v.(type) {
		case string:
			return []string{x}, nil
		case []any:
			r := []string{}
			for _, v := range x {
				s, ok := v.(string)
				if !ok {
					return nil, errors.New("Ожидался список строк")
				}
				r = append(r, s)
			}
			return r, nil
		case nil:
			return []string{}, nil
		}
		return nil, errors.New("Ожидался список строк")
	}
	if d, ok := doc["dns"].(map[string]any); ok {
		if v, ok := d["enable"].(bool); ok {
			c.DNS.Enabled = v
		}
		if v, ok := d["enhanced-mode"].(string); ok {
			c.DNS.Mode = v
		}
		if v, ok := d["ipv6"].(bool); ok {
			c.Settings.IPv6 = v
		}
		if v, ok := d["respect-rules"].(bool); ok {
			c.DNS.RespectRules = v
		}
		for key, target := range map[string]*[]string{"nameserver": &c.DNS.Servers, "fallback": &c.DNS.Fallback, "default-nameserver": &c.DNS.Bootstrap, "proxy-server-nameserver": &c.DNS.NodeResolvers, "direct-nameserver": &c.DNS.DirectResolvers, "fake-ip-filter": &c.DNS.Exclusions} {
			if v, ok := d[key]; ok {
				*target, e = list(v)
				if e != nil {
					return out, e
				}
			}
		}
		if policies, ok := d["nameserver-policy"].(map[string]any); ok {
			keys := []string{}
			for k := range policies {
				keys = append(keys, k)
			}
			sort.Strings(keys)
			for _, domain := range keys {
				resolvers, e := list(policies[domain])
				if e != nil {
					return out, e
				}
				for _, s := range resolvers {
					c.DNS.Policies = append(c.DNS.Policies, Policy{domain, s})
				}
			}
		}
		for _, key := range []string{"cache-algorithm", "use-hosts", "fake-ip-range"} {
			if _, ok := d[key]; ok {
				out.Warnings = append(out.Warnings, "DNS "+key+": используется значение по умолчанию приложения")
			}
		}
	}
	if tun, ok := doc["tun"].(map[string]any); ok {
		if v, ok := tun["enable"].(bool); ok {
			c.Settings.TUN = v
		}
		if v, ok := tun["stack"].(string); ok {
			c.TUNOptions.Stack = v
		}
		if v, ok := tun["mtu"].(int); ok {
			c.TUNOptions.MTU = v
		}
		if v, ok := tun["route-exclude-address"]; ok {
			c.Settings.RouteExclusions, e = list(v)
			if e != nil {
				return out, e
			}
		}
		if hijack, ok := tun["dns-hijack"]; ok {
			values, e := list(hijack)
			if e != nil {
				return out, e
			}
			c.DNS.Hijack = len(values) > 0
			for _, s := range values {
				if s != "any:53" && s != "tcp://any:53" {
					out.Warnings = append(out.Warnings, "Перехват DNS "+s+" заменён на UDP/TCP порт 53")
				}
			}
		}
		out.Warnings = append(out.Warnings, "TUN: автоматический выбор выходного интерфейса включён")
	}
	c.Settings.SystemProxy = false
	c.Settings.AutoConnect = false
	c.Settings.Autostart = false
	out.Warnings = append(out.Warnings, "Локальные порты и контроллер назначает приложение. Доступ из LAN отключён; системный прокси и автозапуск не включаются при импорте.")
	if e = c.Validate(); e != nil {
		return out, e
	}
	out.Warnings = append(out.Warnings, c.Warnings()...)
	return out, nil
}
