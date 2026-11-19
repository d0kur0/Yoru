package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"sync"
	"time"
)

func probeDelay(ctx context.Context, client *http.Client, endpoint, secret, id string, timeout int, target string) (int, error) {
	var response struct {
		Delay int `json:"delay"`
	}
	if err := controllerAPI(ctx, client, endpoint, secret, http.MethodGet, delayPath(proxyName(id), timeout, target), nil, &response); err != nil {
		return -1, nil
	}
	return response.Delay, nil
}

// An isolated controller checks pending nodes without applying saved routing or
// replacing the live core. No inbound proxy, DNS listener or TUN is created.
func (m *Manager) probeSavedServer(parent context.Context, binary string, proxy map[string]any, ipv6 bool, timeout int, target string) (int, error) {
	delays, err := m.probeSavedServers(parent, binary, map[string]map[string]any{"": proxy}, ipv6, timeout, target)
	if err != nil {
		return -1, err
	}
	return delays[""], nil
}

// probeSavedServers starts one isolated controller for all given nodes (keyed
// by server ID) and measures them concurrently.
func (m *Manager) probeSavedServers(parent context.Context, binary string, proxies map[string]map[string]any, ipv6 bool, timeout int, target string) (map[string]int, error) {
	m.probeMu.Lock()
	defer m.probeMu.Unlock()
	rounds := (len(proxies) + latencyConcurrency - 1) / latencyConcurrency
	ctx, cancel := m.operation(parent, time.Duration(6000+(timeout+1000)*rounds)*time.Millisecond)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	names := make(map[string]string, len(proxies))
	list := make([]any, 0, len(proxies))
	for id, proxy := range proxies {
		// Proxy() names are generated from the stable node ID.
		name, _ := proxy["name"].(string)
		names[id] = name
		list = append(list, proxy)
	}
	dir, err := os.MkdirTemp(m.dir, "latency-")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(dir)
	port, err := freePort()
	if err != nil {
		return nil, err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return nil, err
	}
	secret := hex.EncodeToString(token)
	address := fmt.Sprintf("127.0.0.1:%d", port)
	data, err := marshalMihomoYAML(map[string]any{
		"external-controller": address, "secret": secret,
		"mixed-port": 0, "allow-lan": false, "ipv6": ipv6,
		"tun":       map[string]any{"enable": false},
		"dns":       map[string]any{"enable": false},
		"profile":   map[string]any{"store-selected": false, "store-fake-ip": false},
		"log-level": "silent", "mode": "rule",
		"proxies": list, "rules": []string{"MATCH,DIRECT"},
	})
	if err != nil {
		return nil, err
	}
	path := filepath.Join(dir, "config.yaml")
	if err = atomicWrite(path, data, 0600); err != nil {
		return nil, err
	}
	process, err := m.runner.Start(binary, []string{"-d", dir, "-f", path}, io.Discard)
	if err != nil {
		return nil, errors.New("Не удалось запустить проверку сервера")
	}
	done := make(chan struct{})
	go func() { _ = process.Wait(); close(done) }()
	defer func() {
		_ = process.Stop()
		select {
		case <-done:
		case <-time.After(5 * time.Second):
		}
	}()
	client := &http.Client{Timeout: time.Second, Transport: &http.Transport{Proxy: nil}}
	defer client.CloseIdleConnections()
	ready, stopReady := context.WithTimeout(ctx, 5*time.Second)
	defer stopReady()
	ticker := time.NewTicker(100 * time.Millisecond)
	defer ticker.Stop()
	for {
		var version struct {
			Version string `json:"version"`
		}
		if controllerAPI(ready, client, "http://"+address, secret, http.MethodGet, "/version", nil, &version) == nil && version.Version != "" {
			break
		}
		select {
		case <-done:
			return nil, errors.New("Mihomo отклонил параметры проверяемого сервера")
		case <-ready.Done():
			return nil, errors.New("Проверка сервера не запустилась вовремя")
		case <-ticker.C:
		}
	}
	client.Timeout = time.Duration(timeout+1000) * time.Millisecond
	ids := make([]string, 0, len(names))
	for id := range names {
		ids = append(ids, id)
	}
	var mu sync.Mutex
	delays := make(map[string]int, len(ids))
	parallel(ids, func(id string) {
		var response struct {
			Delay int `json:"delay"`
		}
		delay := -1
		if controllerAPI(ctx, client, "http://"+address, secret, http.MethodGet, delayPath(names[id], timeout, target), nil, &response) == nil {
			delay = response.Delay
		}
		mu.Lock()
		delays[id] = delay
		mu.Unlock()
	})
	return delays, nil
}
