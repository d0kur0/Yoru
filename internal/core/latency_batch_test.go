package core

import (
	"context"
	"testing"
	"time"
)

func TestServerLatenciesReadCoreHistoryForConfiguredURL(t *testing.T) {
	m, runner, _ := testManager(t)
	if got, err := m.ServerLatencies(context.Background()); err != nil || len(got) != 0 {
		t.Fatalf("stopped core must return no samples: %v %v", got, err)
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	starts := runner.starts
	got, err := m.ServerLatencies(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 11, 19, 20, 0, 30, 0, time.UTC).UnixMilli()
	if len(got) != 1 || got["a"].Delay != 37 || got["a"].Time != want {
		t.Fatalf("history sample: %+v", got)
	}
	if runner.starts != starts {
		t.Fatal("reading history must not start probes")
	}
}

func TestServerLatenciesSkipNodesChangedSinceStart(t *testing.T) {
	m, _, _ := testManager(t)
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	c := m.Config()
	c.Servers[0].Port++
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	got, err := m.ServerLatencies(context.Background())
	if err != nil || len(got) != 0 {
		t.Fatalf("stale history of an edited node must not be shown: %v %v", got, err)
	}
}

func TestMeasureServersBatchesLiveAndPendingNodes(t *testing.T) {
	m, runner, _ := testManager(t)
	if _, err := m.MeasureServers(context.Background(), []string{"a"}); err == nil {
		t.Fatal("stopped core must be reported")
	}
	if err := m.Start(context.Background()); err != nil {
		t.Fatal(err)
	}
	live := runner.p
	c := m.Config()
	for _, id := range []string{"p1", "p2", "p3"} {
		s := c.Servers[0]
		s.ID, s.Name = id, id
		c.Servers = append(c.Servers, s)
	}
	if err := m.Save(c); err != nil {
		t.Fatal(err)
	}
	starts := runner.starts
	got, err := m.MeasureServers(context.Background(), []string{"a", "p1", "p2", "p3", "missing", "a"})
	if err != nil {
		t.Fatal(err)
	}
	if len(got) != 5 || got["a"] != 42 || got["p1"] != 42 || got["p3"] != 42 || got["missing"] != -1 {
		t.Fatalf("delays: %v", got)
	}
	if runner.starts != starts+1 {
		t.Fatalf("pending nodes must share one isolated controller, starts %d", runner.starts-starts)
	}
	if proxies := runner.lastConfig["proxies"].([]any); len(proxies) != 3 {
		t.Fatalf("isolated controller proxies: %d", len(proxies))
	}
	if m.process == nil || live == nil {
		t.Fatal("live core stopped during batch probe")
	}
	if delay, err := m.TestRunningServerLatency(context.Background(), "a"); err != nil || delay != 42 {
		t.Fatal("live controller disrupted", delay, err)
	}
}
