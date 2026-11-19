package core

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"sync"
	"time"
)

// Parallel delay requests per batch. Mihomo health checks use a similar limit.
const latencyConcurrency = 10

// LatencySample is one delay measurement. Delay -1 means no response.
type LatencySample struct {
	Delay int   `json:"delay"`
	Time  int64 `json:"time"`
}

type delayRecord struct {
	Time  time.Time `json:"time"`
	Delay int       `json:"delay"`
}

// Called with m.mu held. Returns IDs whose running definition matches the saved one.
func (m *Manager) appliedNodeIDs() map[string]bool {
	ids := map[string]bool{}
	var applied Config
	if json.Unmarshal(m.applied, &applied) != nil {
		return ids
	}
	current := make(map[string]Server, len(m.config.Servers))
	for _, s := range m.config.Servers {
		current[s.ID] = s
	}
	for _, old := range applied.Servers {
		saved, ok := current[old.ID]
		if !ok {
			continue
		}
		a, ea := old.Proxy()
		b, eb := saved.Proxy()
		if ea != nil || eb != nil {
			continue
		}
		av, _ := json.Marshal(a)
		bv, _ := json.Marshal(b)
		if string(av) == string(bv) {
			ids[old.ID] = true
		}
	}
	return ids
}

// ServerLatencies reads the core's own health-check history for the configured
// URL. It sends no probes, so the UI can poll it cheaply.
func (m *Manager) ServerLatencies(ctx context.Context) (map[string]LatencySample, error) {
	result := map[string]LatencySample{}
	m.mu.Lock()
	if m.process == nil {
		m.mu.Unlock()
		return result, nil
	}
	endpoint, secret, client := m.endpoint, m.secret, *m.client
	target := m.config.LatencyTestURL()
	ids := m.appliedNodeIDs()
	m.mu.Unlock()
	var raw struct {
		Proxies map[string]struct {
			Extra map[string]struct {
				History []delayRecord `json:"history"`
			} `json:"extra"`
		} `json:"proxies"`
	}
	if err := controllerAPI(ctx, &client, endpoint, secret, http.MethodGet, "/proxies", nil, &raw); err != nil {
		return nil, err
	}
	for id := range ids {
		history := raw.Proxies[proxyName(id)].Extra[target].History
		if len(history) == 0 {
			continue
		}
		last := history[len(history)-1]
		delay := last.Delay
		if delay <= 0 {
			delay = -1
		}
		result[id] = LatencySample{Delay: delay, Time: last.Time.UnixMilli()}
	}
	return result, nil
}

// MeasureServers checks the given servers concurrently and returns delays in
// one call. Nodes loaded in the core are measured through it; saved but not
// yet applied nodes share one isolated controller.
func (m *Manager) MeasureServers(ctx context.Context, ids []string) (map[string]int, error) {
	m.mu.Lock()
	if m.process == nil {
		m.mu.Unlock()
		return nil, errors.New("Ядро не запущено")
	}
	timeout, target := m.config.HealthTimeout(), m.config.LatencyTestURL()
	endpoint, secret, client := m.endpoint, m.secret, *m.client
	ipv6 := m.config.Settings.IPv6
	appliedIDs := m.appliedNodeIDs()
	servers := make(map[string]Server, len(m.config.Servers))
	for _, s := range m.config.Servers {
		servers[s.ID] = s
	}
	var live []string
	pending := map[string]map[string]any{}
	result := make(map[string]int, len(ids))
	for _, id := range ids {
		if _, seen := result[id]; seen {
			continue
		}
		result[id] = -1
		s, ok := servers[id]
		if !ok {
			continue
		}
		if appliedIDs[id] {
			live = append(live, id)
			continue
		}
		if proxy, err := s.Proxy(); err == nil {
			pending[id] = proxy
		}
	}
	var binary string
	if len(pending) > 0 {
		installation, err := m.installed(true)
		if err != nil {
			pending = nil
		} else {
			binary = installation.Path
		}
	}
	m.mu.Unlock()

	var mu sync.Mutex
	set := func(id string, delay int) { mu.Lock(); result[id] = delay; mu.Unlock() }
	var wg sync.WaitGroup
	if len(live) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			ctx, cancel := m.operation(ctx, time.Duration(timeout+2000)*time.Millisecond*time.Duration((len(live)+latencyConcurrency-1)/latencyConcurrency))
			defer cancel()
			client.Timeout = time.Duration(timeout+1000) * time.Millisecond
			parallel(live, func(id string) {
				delay, _ := probeDelay(ctx, &client, endpoint, secret, id, timeout, target)
				set(id, normalDelay(delay))
			})
		}()
	}
	if len(pending) > 0 {
		wg.Add(1)
		go func() {
			defer wg.Done()
			delays, err := m.probeSavedServers(ctx, binary, pending, ipv6, timeout, target)
			if err == nil {
				for id, delay := range delays {
					set(id, normalDelay(delay))
				}
				return
			}
			// One rejected node fails the shared controller; isolate them instead.
			for id, proxy := range pending {
				delay, _ := m.probeSavedServer(ctx, binary, proxy, ipv6, timeout, target)
				set(id, normalDelay(delay))
			}
		}()
	}
	wg.Wait()
	return result, nil
}

func normalDelay(delay int) int {
	if delay <= 0 {
		return -1
	}
	return delay
}

func parallel(ids []string, run func(string)) {
	work := make(chan string)
	var wg sync.WaitGroup
	for i := 0; i < latencyConcurrency && i < len(ids); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for id := range work {
				run(id)
			}
		}()
	}
	for _, id := range ids {
		work <- id
	}
	close(work)
	wg.Wait()
}

func delayPath(name string, timeout int, target string) string {
	return "/proxies/" + url.PathEscape(name) + "/delay?timeout=" + strconv.Itoa(timeout) + "&url=" + url.QueryEscape(target)
}
