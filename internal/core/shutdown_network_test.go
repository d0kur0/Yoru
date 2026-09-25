package core

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"reflect"
	"sync"
	"testing"
	"time"
)

type observedStop struct {
	Process
	check func()
}

func (p observedStop) Stop() error { p.check(); return p.Process.Stop() }

func TestStopReleasesTUNBeforeTerminatingCore(t *testing.T) {
	m, _, _ := testManager(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	var mu sync.Mutex
	var calls []string
	enabled := true
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		defer mu.Unlock()
		calls = append(calls, r.Method)
		if r.Method == http.MethodPatch {
			var patch struct {
				TUN struct {
					Enable bool `json:"enable"`
				} `json:"tun"`
			}
			if err := json.NewDecoder(r.Body).Decode(&patch); err != nil {
				t.Error(err)
			}
			enabled = patch.TUN.Enable
			w.WriteHeader(204)
			return
		}
		_ = json.NewEncoder(w).Encode(map[string]any{"tun": map[string]bool{"enable": enabled}})
	}))
	defer server.Close()
	m.endpoint = server.URL
	config := m.Config()
	m.process = observedStop{m.process, func() {
		mu.Lock()
		defer mu.Unlock()
		if enabled || !reflect.DeepEqual(calls, []string{"GET", "PATCH", "GET"}) {
			t.Errorf("terminated before cleanup: %v, enabled=%v", calls, enabled)
		}
	}}
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(config, m.Config()) {
		t.Fatal("disconnect changed saved TUN preference")
	}
}

func TestStopStillTerminatesWhenControllerIsUnresponsive(t *testing.T) {
	m, _, _ := testManager(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { <-r.Context().Done() }))
	defer server.Close()
	m.endpoint = server.URL
	m.client.Timeout = 30 * time.Millisecond
	if err := m.Stop(); err != nil {
		t.Fatal(err)
	}
	if m.process != nil {
		t.Fatal("unresponsive core was left running")
	}
}
