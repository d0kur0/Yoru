package core

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"time"

	"gopkg.in/yaml.v3"
)

func probeDelay(ctx context.Context, client *http.Client, endpoint, secret, id string, timeout int, target string) (int, error) {
	var response struct {
		Delay int `json:"delay"`
	}
	path := "/proxies/" + url.PathEscape(proxyName(id)) + "/delay?timeout=" + strconv.Itoa(timeout) + "&url=" + url.QueryEscape(target)
	if err := controllerAPI(ctx, client, endpoint, secret, http.MethodGet, path, nil, &response); err != nil {
		return -1, nil
	}
	return response.Delay, nil
}

// An isolated controller checks pending nodes without applying saved routing or
// replacing the live core. No inbound proxy, DNS listener or TUN is created.
func (m *Manager) probeSavedServer(parent context.Context, binary string, proxy map[string]any, ipv6 bool, timeout int, target string) (int, error) {
	m.probeMu.Lock()
	defer m.probeMu.Unlock()
	ctx, cancel := m.operation(parent, time.Duration(timeout+7000)*time.Millisecond)
	defer cancel()
	if err := ctx.Err(); err != nil {
		return -1, err
	}
	dir, err := os.MkdirTemp(m.dir, "latency-")
	if err != nil {
		return -1, err
	}
	defer os.RemoveAll(dir)
	port, err := freePort()
	if err != nil {
		return -1, err
	}
	token := make([]byte, 32)
	if _, err = rand.Read(token); err != nil {
		return -1, err
	}
	secret := hex.EncodeToString(token)
	address := fmt.Sprintf("127.0.0.1:%d", port)
	data, err := yaml.Marshal(map[string]any{
		"external-controller": address, "secret": secret,
		"mixed-port": 0, "allow-lan": false, "ipv6": ipv6,
		"tun":       map[string]any{"enable": false},
		"dns":       map[string]any{"enable": false},
		"profile":   map[string]any{"store-selected": false, "store-fake-ip": false},
		"log-level": "silent", "mode": "rule",
		"proxies": []any{proxy}, "rules": []string{"MATCH,DIRECT"},
	})
	if err != nil {
		return -1, err
	}
	path := filepath.Join(dir, "config.yaml")
	if err = atomicWrite(path, data, 0600); err != nil {
		return -1, err
	}
	process, err := m.runner.Start(binary, []string{"-d", dir, "-f", path}, io.Discard)
	if err != nil {
		return -1, errors.New("Не удалось запустить проверку сервера")
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
			return -1, errors.New("Mihomo отклонил параметры проверяемого сервера")
		case <-ready.Done():
			return -1, errors.New("Проверка сервера не запустилась вовремя")
		case <-ticker.C:
		}
	}
	client.Timeout = time.Duration(timeout+1000) * time.Millisecond
	// Proxy() names are generated from the stable node ID.
	name, _ := proxy["name"].(string)
	var response struct {
		Delay int `json:"delay"`
	}
	requestPath := "/proxies/" + url.PathEscape(name) + "/delay?timeout=" + strconv.Itoa(timeout) + "&url=" + url.QueryEscape(target)
	if err = controllerAPI(ctx, client, "http://"+address, secret, http.MethodGet, requestPath, nil, &response); err != nil {
		return -1, nil
	}
	return response.Delay, nil
}
