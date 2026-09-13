package core

import "errors"

func (c Config) GroupMode() string {
	if c.Settings.GroupMode != "" {
		return c.Settings.GroupMode
	}
	if c.FailoverEnabled() {
		return "fallback"
	}
	return "select"
}
func (c Config) HealthInterval() int {
	if c.Settings.HealthInterval == 0 {
		return 30
	}
	return c.Settings.HealthInterval
}
func (c Config) HealthTimeout() int {
	if c.Settings.HealthTimeout == 0 {
		return 5000
	}
	return c.Settings.HealthTimeout
}
func (c Config) HealthTolerance() int {
	if c.Settings.HealthTolerance == nil {
		return 50
	}
	return *c.Settings.HealthTolerance
}
func (c Config) BalanceStrategy() string {
	if c.Settings.BalanceStrategy == "" {
		return "consistent-hashing"
	}
	return c.Settings.BalanceStrategy
}

func (c Config) validateProxyGroups() error {
	switch c.GroupMode() {
	case "select", "fallback", "url-test", "load-balance":
	default:
		return errors.New("Неизвестный режим группы VPN")
	}
	if c.HealthInterval() < 10 || c.HealthInterval() > 3600 {
		return errors.New("Интервал проверки: от 10 до 3600 секунд")
	}
	if c.HealthTimeout() < 1000 || c.HealthTimeout() > 30000 {
		return errors.New("Тайм-аут проверки: от 1000 до 30000 мс")
	}
	if c.HealthTolerance() < 0 || c.HealthTolerance() > 1000 {
		return errors.New("Допуск задержки: от 0 до 1000 мс")
	}
	switch c.BalanceStrategy() {
	case "consistent-hashing", "round-robin", "sticky-sessions":
	default:
		return errors.New("Неизвестная стратегия балансировки")
	}
	for _, r := range c.Rules {
		if r.Target != "" && (r.Action != "PROXY" || !clean(r.Target)) {
			return errors.New("Сервер или группа доступны только для маршрута через VPN")
		}
	}
	return nil
}

// An absent pinned server must never silently route the application through another exit.
func (c Config) ruleOutbound(r Rule) string {
	if r.Action != "PROXY" || r.Target == "" {
		return r.Action
	}
	if r.Target == "@fastest" {
		return "AUTO"
	}
	for _, s := range c.Servers {
		if s.ID == r.Target {
			return proxyName(s.ID)
		}
	}
	return "REJECT"
}

func (c Config) proxyGroups(names []string) []map[string]any {
	makeGroup := func(name, kind string) map[string]any {
		g := map[string]any{"name": name, "type": kind, "proxies": names}
		if kind != "select" {
			g["url"] = c.LatencyTestURL()
			g["interval"] = c.HealthInterval()
			g["timeout"] = c.HealthTimeout()
			g["lazy"] = c.Settings.HealthLazy
		}
		if kind == "url-test" {
			g["tolerance"] = c.HealthTolerance()
		}
		if kind == "load-balance" {
			g["strategy"] = c.BalanceStrategy()
		}
		return g
	}
	groups := []map[string]any{makeGroup("PROXY", c.GroupMode())}
	for _, r := range c.Rules {
		if r.Action == "PROXY" && r.Target == "@fastest" {
			groups = append(groups, makeGroup("AUTO", "url-test"))
			break
		}
	}
	return groups
}
