package core

import (
	"bytes"
	"context"
	"path/filepath"
	"testing"
)

func TestPendingNodesMeasuredWithoutChangingLiveVPN(t *testing.T) {
	m, runner, platform := testManager(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	live, applied, endpoint := m.process, append([]byte(nil), m.applied...), m.endpoint
	c := m.Config()
	c.Servers[0].Port++
	added := c.Servers[0]
	added.ID = "new-node"
	c.Servers = append(c.Servers, added)
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	for _, id := range []string{"a", "new-node"} {
		if delay, err := m.TestServerLatency(context.Background(), id); err != nil || delay != 42 {
			t.Fatalf("probe %s: %d %v", id, delay, err)
		}
		config := runner.lastConfig
		if config["mixed-port"] != 0 || config["tun"].(map[string]any)["enable"] != false || config["dns"].(map[string]any)["enable"] != false {
			t.Fatal("probe changes network settings")
		}
		proxy := config["proxies"].([]any)[0].(map[string]any)
		if proxy["name"] != proxyName(id) || proxy["port"] != added.Port {
			t.Fatal("probe did not use saved node settings")
		}
		select {
		case <-runner.p.done:
		default:
			t.Fatal("temporary process left running")
		}
	}
	if m.process != live || m.endpoint != endpoint || !bytes.Equal(m.applied, applied) {
		t.Fatal("live VPN replaced during probe")
	}
	if platform.enabled != 0 || platform.restored != 0 {
		t.Fatal("probe touched system proxy")
	}
	if delay, err := m.TestRunningServerLatency(context.Background(), "a"); err != nil || delay != 42 {
		t.Fatal("live controller disrupted", delay, err)
	}
	dirs, err := filepath.Glob(filepath.Join(m.dir, "latency-*"))
	if err != nil || len(dirs) != 0 {
		t.Fatal("temporary probe data not removed", dirs, err)
	}
}
