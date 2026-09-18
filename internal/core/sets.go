package core

import (
	"errors"
	"fmt"
	"net"
	"net/url"
	"reflect"
	"strings"
)

type RouteSet struct {
	ID        string   `json:"id,omitempty"`
	Name      string   `json:"name"`
	Enabled   bool     `json:"enabled"`
	Domains   []string `json:"domains"`
	Keywords  []string `json:"keywords,omitempty"`
	Processes []string `json:"processes"`
	CIDRs     []string `json:"cidrs"`
	Action    string   `json:"action"`
	Resolvers []string `json:"resolvers"`
	RealIP    bool     `json:"realIP"`
	BypassTUN bool     `json:"bypassTUN"`
}
type LogSettings struct {
	Enabled   bool   `json:"enabled"`
	Level     string `json:"level"`
	MaxSizeMB int    `json:"maxSizeMB"`
	Files     int    `json:"files"`
	Days      int    `json:"days"`
}

func DefaultLogs() *LogSettings { return &LogSettings{true, "info", 5, 5, 7} }

type TUNOptions struct {
	Stack      string `json:"stack"`
	MTU        int    `json:"mtu"`
	Interface  string `json:"interface"`
	AutoDetect *bool  `json:"autoDetect,omitempty"`
}

func (c Config) validateExtensions() error {
	if c.Logging != nil {
		l := c.Logging
		switch l.Level {
		case "debug", "info", "warning", "error", "silent":
		default:
			return errors.New("Некорректный уровень журнала")
		}
		if l.MaxSizeMB < 1 || l.MaxSizeMB > 50 || l.Files < 1 || l.Files > 10 || l.Days < 1 || l.Days > 30 {
			return errors.New("Журнал: 1–50 МБ, 1–10 файлов, 1–30 дней")
		}
	}
	if c.TUNOptions.Stack != "" && c.TUNOptions.Stack != "mixed" && c.TUNOptions.Stack != "system" && c.TUNOptions.Stack != "gvisor" {
		return errors.New("Неизвестный стек TUN")
	}
	if c.TUNOptions.MTU != 0 && (c.TUNOptions.MTU < 1280 || c.TUNOptions.MTU > 9000) {
		return errors.New("MTU должен быть 1280–9000")
	}
	for _, list := range [][]string{c.DNS.Bootstrap, c.DNS.NodeResolvers, c.DNS.DirectResolvers} {
		for _, s := range list {
			if !resolver(s) {
				return errors.New("Некорректный дополнительный DNS")
			}
		}
	}
	claimed := map[string][]string{}
	for _, p := range c.DNS.Policies {
		claimed[p.Domain] = append(claimed[p.Domain], p.Server)
	}
	ids := map[string]bool{}
	for _, s := range c.Sets {
		if s.ID == "" || ids[s.ID] || s.Name == "" || !action(s.Action) {
			return errors.New("Некорректное имя, ID или маршрут набора")
		}
		ids[s.ID] = true
		if s.BypassTUN && s.Action != "DIRECT" {
			return errors.New("Обход TUN доступен только для направления «Напрямую»")
		}
		for _, domain := range s.Domains {
			d := strings.TrimPrefix(domain, "+.")
			if d == "" || strings.ContainsAny(d, "* /:,\t\r\n") {
				return errors.New("Домен набора: example.com или +.example.com")
			}
			if s.Enabled && len(s.Resolvers) > 0 {
				if old, ok := claimed[domain]; ok && !reflect.DeepEqual(old, s.Resolvers) {
					return fmt.Errorf("Для %s уже указан другой набор DNS", domain)
				}
				claimed[domain] = s.Resolvers
			}
		}
		for _, keyword := range s.Keywords {
			if keyword == "" || !clean(keyword) || strings.ContainsAny(keyword, " /:\t\r\n") {
				return errors.New("Часть домена: укажите текст без протокола, пробелов и пути")
			}
		}
		for _, p := range s.Processes {
			if p == "" || !clean(p) {
				return errors.New("Некорректное имя процесса")
			}
		}
		for _, cidr := range s.CIDRs {
			if _, _, e := net.ParseCIDR(cidr); e != nil {
				return errors.New("Некорректная подсеть набора")
			}
		}
		for _, dns := range s.Resolvers {
			if !resolver(dns) {
				return errors.New("Некорректный DNS набора")
			}
		}
	}
	return nil
}
func appendUnique(dst []string, values ...string) []string {
	for _, v := range values {
		found := false
		for _, x := range dst {
			if x == v {
				found = true
				break
			}
		}
		if !found {
			dst = append(dst, v)
		}
	}
	return dst
}
func (c Config) expanded() Config {
	c.Rules = append([]Rule{}, c.Rules...)
	c.DNS.Policies = append([]Policy{}, c.DNS.Policies...)
	c.DNS.Exclusions = append([]string{}, c.DNS.Exclusions...)
	c.Settings.RouteExclusions = append([]string{}, c.Settings.RouteExclusions...)
	generated := []Rule{}
	for _, s := range c.Sets {
		if !s.Enabled {
			continue
		}
		for _, d := range s.Domains {
			typ := "DOMAIN"
			value := d
			if strings.HasPrefix(d, "+.") {
				typ = "DOMAIN-SUFFIX"
				value = d[2:]
			}
			generated = append(generated, Rule{Type: typ, Value: value, Action: s.Action})
			for _, dns := range s.Resolvers {
				exists := false
				for _, p := range c.DNS.Policies {
					if p.Domain == d && p.Server == dns {
						exists = true
					}
				}
				if !exists {
					c.DNS.Policies = append(c.DNS.Policies, Policy{d, dns})
				}
			}
			if s.RealIP {
				c.DNS.Exclusions = appendUnique(c.DNS.Exclusions, d)
			}
		}
		for _, keyword := range s.Keywords {
			generated = append(generated, Rule{Type: "DOMAIN-KEYWORD", Value: keyword, Action: s.Action})
		}
		for _, p := range s.Processes {
			typ := "PROCESS-NAME"
			if strings.ContainsAny(p, "/\\") {
				typ = "PROCESS-PATH"
			}
			generated = append(generated, Rule{Type: typ, Value: p, Action: s.Action})
		}
		for _, cidr := range s.CIDRs {
			typ := "IP-CIDR"
			if strings.Contains(cidr, ":") {
				typ = "IP-CIDR6"
			}
			generated = append(generated, Rule{Type: typ, Value: cidr, Action: s.Action, NoResolve: true})
			if s.BypassTUN {
				c.Settings.RouteExclusions = appendUnique(c.Settings.RouteExclusions, cidr)
			}
		}
		if s.BypassTUN {
			for _, dns := range s.Resolvers {
				host := dns
				if u, e := url.Parse(dns); e == nil && u.Hostname() != "" {
					host = u.Hostname()
				} else if h, _, e := net.SplitHostPort(dns); e == nil {
					host = h
				}
				if ip := net.ParseIP(host); ip != nil && !ip.IsLoopback() {
					mask := "/32"
					if ip.To4() == nil {
						mask = "/128"
					}
					c.Settings.RouteExclusions = appendUnique(c.Settings.RouteExclusions, host+mask)
				}
			}
		}
	}
	c.Rules = append(generated, c.Rules...)
	return c
}

func WorkPreset() RouteSet {
	return RouteSet{ID: "work", Name: "Рабочие ресурсы", Enabled: false, Domains: []string{"+.company.example", "+.internal.example", "+.service.example", "+.local"}, Processes: []string{}, CIDRs: []string{"172.16.0.0/12", "10.20.0.53/32", "10.20.0.54/32"}, Action: "DIRECT", Resolvers: []string{"10.20.0.53", "10.20.0.54"}, RealIP: true, BypassTUN: true}
}

// Warnings concern static configuration, never claimed live connectivity.
func (c Config) Warnings() []string {
	warnings := []string{}
	if c.Settings.Mode == "global" {
		warnings = append(warnings, "Глобальный режим игнорирует правила. Для исключений используйте «По правилам» и финальный маршрут VPN.")
	}
	x := c.expanded()
	for _, p := range x.DNS.Policies {
		if strings.HasPrefix(p.Server, "127.0.0.1:") {
			warnings = appendUnique(warnings, "DNS "+p.Server+" требует отдельно работающего локального DNSProxy.")
		}
	}
	for _, r := range x.Rules {
		if r.Action != "PROXY" || !(r.Type == "IP-CIDR" || r.Type == "IP-CIDR6") {
			continue
		}
		_, network, e := net.ParseCIDR(r.Value)
		if e != nil {
			continue
		}
		for _, excluded := range x.Settings.RouteExclusions {
			_, bypass, e := net.ParseCIDR(excluded)
			if e == nil && (network.Contains(bypass.IP) || bypass.Contains(network.IP)) {
				warnings = appendUnique(warnings, "Правило VPN "+r.Value+" пересекается с обходом TUN "+excluded+".")
			}
		}
	}
	return warnings
}
