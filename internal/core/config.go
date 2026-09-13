package core

import (
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/url"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

type Server struct {
	ID           string `json:"id"`
	Name         string `json:"name"`
	Country      string `json:"country"`
	Flag         string `json:"flag"`
	Host         string `json:"host"`
	Port         int    `json:"port"`
	Protocol     string `json:"protocol"`
	Transport    string `json:"transport"`
	Secret       string `json:"secret"`
	Cipher       string `json:"cipher"`
	SNI          string `json:"sni"`
	PublicKey    string `json:"publicKey"`
	ShortID      string `json:"shortId"`
	Flow         string `json:"flow"`
	Fingerprint  string `json:"fingerprint"`
	Path         string `json:"path"`
	WSHost       string `json:"wsHost"`
	ALPN         string `json:"alpn"`
	TLS          *bool  `json:"tls,omitempty"`
	Latency      int    `json:"latency"`
	Subscription string `json:"subscription,omitempty"`
	// Imported proxy options are preserved, but cannot override application routing.
	Options map[string]any `json:"options,omitempty"`
}
type Rule struct {
	Target    string `json:"target,omitempty"`
	NoResolve bool   `json:"noResolve,omitempty"`
	ID        string `json:"id"`
	Type      string `json:"type"`
	Value     string `json:"value"`
	Action    string `json:"action"`
}
type Subscription struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	URL      string `json:"url"`
	Interval string `json:"interval"`
	Updated  string `json:"updated"`
}
type Policy struct {
	Domain string `json:"domain"`
	Server string `json:"server"`
}
type DNS struct {
	Bootstrap       []string `json:"bootstrap"`
	NodeResolvers   []string `json:"nodeResolvers"`
	DirectResolvers []string `json:"directResolvers"`
	RespectRules    bool     `json:"respectRules"`
	Enabled         bool     `json:"enabled"`
	Mode            string   `json:"mode"`
	Servers         []string `json:"servers"`
	Fallback        []string `json:"fallback"`
	Exclusions      []string `json:"exclusions"`
	Policies        []Policy `json:"policies"`
	Hijack          bool     `json:"hijack"`
	IPv6            bool     `json:"ipv6"`
}
type Settings struct {
	GroupMode       string `json:"groupMode,omitempty"`
	HealthInterval  int    `json:"healthInterval,omitempty"`
	HealthTimeout   int    `json:"healthTimeout,omitempty"`
	HealthTolerance *int   `json:"healthTolerance,omitempty"`
	HealthLazy      bool   `json:"healthLazy,omitempty"`
	BalanceStrategy string `json:"balanceStrategy,omitempty"`

	Failover        *bool    `json:"failover,omitempty"`
	ServerOrder     []string `json:"serverOrder,omitempty"`
	LatencyURL      string   `json:"latencyURL,omitempty"`
	TUN             bool     `json:"tun"`
	SystemProxy     bool     `json:"systemProxy"`
	Mode            string   `json:"mode"`
	Autostart       bool     `json:"autostart"`
	Minimized       bool     `json:"minimized"`
	AutoConnect     bool     `json:"autoConnect"`
	Tray            bool     `json:"tray"`
	Sniffer         bool     `json:"sniffer"`
	IPv6            bool     `json:"ipv6"`
	RouteExclusions []string `json:"routeExclusions"`
}
type Config struct {
	Sets          []RouteSet     `json:"sets"`
	Logging       *LogSettings   `json:"logging"`
	TUNOptions    TUNOptions     `json:"tunOptions"`
	Revision      uint64         `json:"revision"`
	Servers       []Server       `json:"servers"`
	Selected      string         `json:"selected"`
	Rules         []Rule         `json:"rules"`
	Subscriptions []Subscription `json:"subscriptions"`
	DNS           DNS            `json:"dns"`
	Settings      Settings       `json:"settings"`
	DefaultAction string         `json:"defaultAction"`
}

func DefaultConfig() Config {
	c := Config{Servers: []Server{}, Rules: []Rule{}, Subscriptions: []Subscription{},
		DNS:      DNS{Enabled: true, Mode: "fake-ip", Servers: []string{"https://dns.quad9.net/dns-query"}, Fallback: []string{}, Exclusions: []string{"+.lan", "+.local", "localhost"}, Policies: []Policy{}, Hijack: true},
		Settings: Settings{TUN: true, Mode: "rule", Tray: true, Sniffer: true, RouteExclusions: []string{"192.168.0.0/16", "10.0.0.0/8", "172.16.0.0/12"}}, DefaultAction: "DIRECT"}
	c.normalize()
	return c
}
func DecodeConfig(data []byte) (Config, error) {
	var c Config
	if len(data) > 2<<20 {
		return c, errors.New("Конфигурация больше 2 МБ")
	}
	if err := json.Unmarshal(data, &c); err != nil {
		return c, errors.New("Некорректный JSON конфигурации")
	}
	c.normalize()
	return c, c.Validate()
}

func (c *Config) normalize() {
	if c.Sets == nil {
		c.Sets = []RouteSet{}
	}
	if c.Logging == nil {
		c.Logging = DefaultLogs()
	}
	if c.DNS.Bootstrap == nil {
		c.DNS.Bootstrap = []string{"1.1.1.1", "9.9.9.9"}
	}
	if c.DNS.NodeResolvers == nil {
		c.DNS.NodeResolvers = []string{}
	}
	if c.DNS.DirectResolvers == nil {
		c.DNS.DirectResolvers = []string{}
	}
	if c.Servers == nil {
		c.Servers = []Server{}
	}
	if c.Rules == nil {
		c.Rules = []Rule{}
	}
	if c.Subscriptions == nil {
		c.Subscriptions = []Subscription{}
	}
	if c.DNS.Servers == nil {
		c.DNS.Servers = []string{}
	}
	if c.DNS.Fallback == nil {
		c.DNS.Fallback = []string{}
	}
	if c.DNS.Exclusions == nil {
		c.DNS.Exclusions = []string{}
	}
	if c.DNS.Policies == nil {
		c.DNS.Policies = []Policy{}
	}
	if c.Settings.RouteExclusions == nil {
		c.Settings.RouteExclusions = []string{}
	}
}
func action(s string) bool { return s == "PROXY" || s == "DIRECT" || s == "REJECT" }
func clean(s string) bool  { return !strings.ContainsAny(s, ",\r\n\x00") }
func resolver(s string) bool {
	if net.ParseIP(s) != nil {
		return true
	}
	if h, p, err := net.SplitHostPort(s); err == nil && net.ParseIP(h) != nil {
		port, e := strconv.Atoi(p)
		return e == nil && port > 0 && port <= 65535
	}
	u, e := url.Parse(s)
	if e != nil || u.Hostname() == "" || u.User != nil || strings.ContainsAny(s, "\r\n\t ") {
		return false
	}
	if u.Port() != "" {
		p, e := strconv.Atoi(u.Port())
		if e != nil || p < 1 || p > 65535 {
			return false
		}
	}
	return u.Scheme == "https" || u.Scheme == "tls" || u.Scheme == "quic" || u.Scheme == "udp" || u.Scheme == "tcp"
}
func (c Config) Validate() error {
	if err := c.validateProxyGroups(); err != nil {
		return err
	}
	if c.Settings.LatencyURL != "" {
		u, err := url.Parse(c.Settings.LatencyURL)
		if err != nil || (u.Scheme != "https" && u.Scheme != "http") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
			return errors.New("Адрес проверки задержки: укажите HTTP/HTTPS URL без логина и фрагмента")
		}
	}

	if e := c.validateExtensions(); e != nil {
		return e
	}
	if !action(c.DefaultAction) || !(c.Settings.Mode == "rule" || c.Settings.Mode == "global" || c.Settings.Mode == "direct") {
		return errors.New("Некорректный режим маршрутизации")
	}
	ids := map[string]bool{}
	selected := c.Selected == ""
	if len(c.Servers) > 10000 || len(c.Rules) > 10000 {
		return errors.New("Слишком много серверов или правил")
	}
	for _, s := range c.Servers {
		if s.ID == "" || ids[s.ID] || s.Name == "" || !clean(s.ID) {
			return errors.New("Некорректный или повторяющийся ID сервера")
		}
		ids[s.ID] = true
		selected = selected || s.ID == c.Selected
		if _, e := s.Proxy(); e != nil {
			return fmt.Errorf("Сервер %s: %w", s.Name, e)
		}
	}
	if !selected {
		return errors.New("Выбранный сервер отсутствует")
	}
	for _, r := range c.Rules {
		if r.NoResolve && r.Type != "IP-CIDR" && r.Type != "IP-CIDR6" {
			return errors.New("no-resolve доступен только для IP-правил")
		}
		if !action(r.Action) || r.Value == "" || !clean(r.Value) {
			return errors.New("Некорректное правило")
		}
		switch r.Type {
		case "DOMAIN", "DOMAIN-SUFFIX", "DOMAIN-KEYWORD":
			if strings.ContainsAny(r.Value, " /:\t") {
				return errors.New("Некорректный домен")
			}
		case "PROCESS-NAME", "PROCESS-PATH", "PROCESS-NAME-WILDCARD", "PROCESS-PATH-WILDCARD":
		case "IP-CIDR", "IP-CIDR6":
			if _, _, e := net.ParseCIDR(r.Value); e != nil {
				return errors.New("Некорректная подсеть правила")
			}
		default:
			return errors.New("Неподдерживаемый тип правила")
		}
	}
	if c.DNS.Mode != "fake-ip" && c.DNS.Mode != "redir-host" {
		return errors.New("Некорректный режим DNS")
	}
	if c.DNS.Enabled && len(c.DNS.Servers) == 0 {
		return errors.New("Нужен основной DNS-сервер")
	}
	for _, v := range append(append([]string{}, c.DNS.Servers...), c.DNS.Fallback...) {
		if !resolver(v) {
			return errors.New("Некорректный адрес DNS")
		}
	}
	for _, p := range c.DNS.Policies {
		if p.Domain == "" || strings.ContainsAny(p.Domain, " /:\r\n\t") || !resolver(p.Server) {
			return errors.New("Некорректное правило DNS")
		}
	}
	for _, p := range c.Settings.RouteExclusions {
		if _, _, e := net.ParseCIDR(p); e != nil {
			return errors.New("Некорректное исключение TUN")
		}
	}
	ids = map[string]bool{}
	for _, s := range c.Subscriptions {
		u, e := url.Parse(s.URL)
		if e != nil || u.Scheme != "https" || u.Hostname() == "" || u.User != nil || s.ID == "" || ids[s.ID] {
			return errors.New("Некорректная HTTPS-подписка")
		}
		ids[s.ID] = true
		switch s.Interval {
		case "1", "6", "24", "168":
		default:
			return errors.New("Некорректный интервал подписки")
		}
	}
	return nil
}

func (s Server) Proxy() (map[string]any, error) {
	if s.Host == "" || strings.ContainsAny(s.Host, " /\r\n\t") || s.Port < 1 || s.Port > 65535 {
		return nil, errors.New("Некорректный адрес или порт")
	}
	p := map[string]any{}
	for k, v := range s.Options {
		p[k] = v
	}
	if s.Transport != "Imported" {
		for _, k := range []string{"network", "ws-opts", "grpc-opts", "h2-opts", "http-opts", "reality-opts", "tls", "flow", "servername", "sni", "alpn", "client-fingerprint"} {
			delete(p, k)
		}
	}
	// Reject options capable of referencing external files or chaining to arbitrary local listeners.
	for _, k := range []string{"dialer-proxy", "interface-name", "routing-mark", "certificate", "private-key"} {
		if _, ok := p[k]; ok && !(k == "private-key" && s.Protocol == "WireGuard") {
			return nil, fmt.Errorf("Параметр %s не поддерживается", k)
		}
	}
	p["name"] = proxyName(s.ID)
	p["server"] = s.Host
	p["port"] = s.Port
	p["udp"] = true
	switch s.Protocol {
	case "VMess":
		p["type"] = "vmess"
		p["uuid"] = s.Secret
	case "TUIC":
		p["type"] = "tuic"
		p["password"] = s.Secret
		if p["uuid"] == nil {
			return nil, errors.New("TUIC требует UUID")
		}
	case "AnyTLS":
		p["type"] = "anytls"
		p["password"] = s.Secret
	case "WireGuard":
		p["type"] = "wireguard"
		p["private-key"] = s.Secret
	case "VLESS":
		p["type"] = "vless"
		p["uuid"] = s.Secret
	case "Shadowsocks":
		p["type"] = "ss"
		p["password"] = s.Secret
		p["cipher"] = s.Cipher
		if s.Cipher == "" {
			return nil, errors.New("Укажите шифр Shadowsocks")
		}
	case "Trojan":
		p["type"] = "trojan"
		p["password"] = s.Secret
	case "Hysteria2":
		p["type"] = "hysteria2"
		p["password"] = s.Secret
	default:
		return nil, errors.New("Протокол не поддерживается")
	}
	if s.Protocol == "WireGuard" {
		for _, key := range []string{"private-key", "public-key"} {
			v, _ := p[key].(string)
			decoded, e := base64.StdEncoding.DecodeString(v)
			if e != nil || len(decoded) != 32 {
				return nil, errors.New("WireGuard: ключ должен содержать 32 байта Base64")
			}
		}
		if p["ip"] == nil && p["ipv6"] == nil {
			return nil, errors.New("WireGuard: укажите адрес интерфейса")
		}
	}
	if strings.TrimSpace(s.Secret) == "" {
		return nil, errors.New("Укажите UUID или пароль")
	}
	if s.Protocol == "VLESS" {
		if len(s.Secret) != 36 || s.Secret[8] != '-' || s.Secret[13] != '-' || s.Secret[18] != '-' || s.Secret[23] != '-' {
			return nil, errors.New("Некорректный UUID VLESS")
		}
		if _, err := hex.DecodeString(strings.ReplaceAll(s.Secret, "-", "")); err != nil {
			return nil, errors.New("Некорректный UUID VLESS")
		}
		if s.SNI != "" {
			p["servername"] = s.SNI
		}
		if s.Flow != "" {
			p["flow"] = s.Flow
		}
	} else if s.SNI != "" {
		p["sni"] = s.SNI
	}
	switch s.Transport {
	case "Reality":
		if s.Protocol != "VLESS" || s.PublicKey == "" {
			return nil, errors.New("Reality требует VLESS и публичный ключ")
		}
		p["tls"] = true
		p["reality-opts"] = map[string]any{"public-key": s.PublicKey, "short-id": s.ShortID}
	case "TLS":
		if s.Protocol == "VLESS" {
			p["tls"] = true
		}
	case "WebSocket":
		if s.Protocol != "VLESS" && s.Protocol != "Trojan" {
			return nil, errors.New("WebSocket доступен для VLESS и Trojan")
		}
		p["network"] = "ws"
		p["tls"] = true
		path := s.Path
		if path == "" {
			path = "/"
		}
		opts := map[string]any{"path": path}
		if s.WSHost != "" {
			opts["headers"] = map[string]string{"Host": s.WSHost}
		}
		p["ws-opts"] = opts
	case "TCP":
	case "Imported":
	default:
		return nil, errors.New("Транспорт не поддерживается")
	}
	if s.Protocol == "VLESS" {
		fp := s.Fingerprint
		if fp == "" {
			fp = "chrome"
		}
		p["client-fingerprint"] = fp
	}
	if s.TLS != nil && s.Transport == "WebSocket" {
		p["tls"] = *s.TLS
	}
	if s.ALPN != "" {
		p["alpn"] = strings.Split(s.ALPN, ",")
	}
	return p, nil
}
func proxyName(id string) string { return "server-" + id }

func (c Config) YAML(controller, secret string, mixedPort int) ([]byte, error) {
	if e := c.Validate(); e != nil {
		return nil, e
	}
	c.normalize()
	c = c.expanded()
	proxies := []map[string]any{}
	names := []string{}
	for _, s := range c.Servers {
		p, e := s.Proxy()
		if e != nil {
			return nil, e
		}
		proxies = append(proxies, p)
	}
	for _, id := range c.OrderedServerIDs() {
		names = append(names, proxyName(id))
	}

	if len(names) == 0 {
		names = []string{"DIRECT"}
	}
	rules := []string{}
	for _, r := range c.Rules {
		rule := r.Type + "," + r.Value + "," + c.ruleOutbound(r)
		if r.NoResolve && (r.Type == "IP-CIDR" || r.Type == "IP-CIDR6") {
			rule += ",no-resolve"
		}
		rules = append(rules, rule)
	}
	rules = append(rules, "MATCH,"+c.DefaultAction)
	policies := map[string][]string{}
	for _, p := range c.DNS.Policies {
		policies[p.Domain] = append(policies[p.Domain], p.Server)
	}
	tun := map[string]any{"enable": c.Settings.TUN, "stack": "mixed", "auto-route": true, "auto-detect-interface": true, "strict-route": false, "route-exclude-address": c.Settings.RouteExclusions}
	if c.TUNOptions.Stack != "" {
		tun["stack"] = c.TUNOptions.Stack
	}
	if c.TUNOptions.MTU != 0 {
		tun["mtu"] = c.TUNOptions.MTU
	}
	if c.TUNOptions.AutoDetect != nil {
		tun["auto-detect-interface"] = *c.TUNOptions.AutoDetect
	}
	if c.DNS.Hijack && c.DNS.Enabled {
		tun["dns-hijack"] = []string{"any:53", "tcp://any:53"}
	}
	dns := map[string]any{"enable": c.DNS.Enabled, "ipv6": c.Settings.IPv6, "enhanced-mode": c.DNS.Mode, "fake-ip-range": "198.18.0.1/16", "nameserver": c.DNS.Servers, "default-nameserver": []string{"1.1.1.1", "9.9.9.9"}, "proxy-server-nameserver": c.DNS.Servers, "fake-ip-filter": c.DNS.Exclusions, "nameserver-policy": policies}
	dns["default-nameserver"] = c.DNS.Bootstrap
	dns["respect-rules"] = c.DNS.RespectRules
	if len(c.DNS.NodeResolvers) > 0 {
		dns["proxy-server-nameserver"] = c.DNS.NodeResolvers
	}
	if len(c.DNS.DirectResolvers) > 0 {
		dns["direct-nameserver"] = c.DNS.DirectResolvers
		dns["direct-nameserver-follow-policy"] = true
	}
	if len(c.DNS.Fallback) > 0 {
		dns["fallback"] = c.DNS.Fallback
		dns["fallback-filter"] = map[string]any{"geoip": false, "ipcidr": []string{"0.0.0.0/8", "240.0.0.0/4"}}
	}
	groups := c.proxyGroups(names)
	document := map[string]any{"mixed-port": mixedPort, "allow-lan": false, "bind-address": "127.0.0.1", "external-controller": controller, "secret": secret, "mode": c.Settings.Mode, "ipv6": c.Settings.IPv6, "log-level": c.Logging.Level, "find-process-mode": "always", "profile": map[string]bool{"store-selected": false}, "tun": tun, "dns": dns, "sniffer": map[string]any{"enable": c.Settings.Sniffer, "sniff": map[string]any{"HTTP": map[string]any{"ports": []int{80, 8080}}, "TLS": map[string]any{"ports": []int{443}}, "QUIC": map[string]any{"ports": []int{443}}}}, "proxies": proxies, "proxy-groups": groups, "rules": rules}
	if c.TUNOptions.Interface != "" {
		document["interface-name"] = c.TUNOptions.Interface
	}
	return yaml.Marshal(document)
}

func (c Config) LatencyTestURL() string {
	if c.Settings.LatencyURL != "" {
		return c.Settings.LatencyURL
	}
	return "https://www.apple.com/library/test/success.html"
}
func runtimeSnapshot(c Config) []byte {
	b, _ := json.Marshal(c)
	return b
}

func (c Config) FailoverEnabled() bool { return c.Settings.Failover == nil || *c.Settings.Failover }
func (c Config) OrderedServerIDs() []string {
	available := map[string]bool{}
	for _, s := range c.Servers {
		available[s.ID] = true
	}
	result := []string{}
	add := func(id string) {
		if available[id] {
			result = append(result, id)
			delete(available, id)
		}
	}
	add(c.Selected)
	for _, id := range c.Settings.ServerOrder {
		add(id)
	}
	for _, s := range c.Servers {
		add(s.ID)
	}
	return result
}
