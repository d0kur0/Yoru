// core-check is an explicit, bounded live smoke test. It never changes the system proxy.
package main

import (
	"context"
	"flag"
	"fmt"
	"github.com/d0kur0/Yoru/internal/core"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"time"
)

func main() {
	if e := run(); e != nil {
		fmt.Fprintln(os.Stderr, e)
		os.Exit(1)
	}
}
func run() error {
	source := flag.String("config", "", "YAML to import")
	dir := flag.String("dir", "", "Isolated application directory")
	tun := flag.Bool("tun", false, "Explicitly exercise TUN")
	validateOnly := flag.Bool("validate-only", false, "Validate imported YAML without starting core")
	flag.Parse()
	if *source == "" || *dir == "" {
		return fmt.Errorf("config and dir required")
	}
	data, e := os.ReadFile(*source)
	if e != nil {
		return e
	}
	result, e := core.ImportYAML(data)
	if e != nil {
		return e
	}
	fmt.Printf("import: servers=%d rules=%d warnings=%d default=%s\n", len(result.Config.Servers), len(result.Config.Rules), len(result.Warnings), result.Config.DefaultAction)
	m, e := core.New(*dir)
	if e != nil {
		return e
	}
	defer m.Shutdown()
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Minute)
	defer cancel()
	c := result.Config
	c.Revision = m.Config().Revision
	c.Settings.SystemProxy = false
	c.Settings.TUN = *tun
	c.DefaultAction = "DIRECT"
	c.Rules = nil
	c.Sets = nil
	// Exercise wildcard parsing while keeping other VPN processes on direct egress.
	c.Rules = []core.Rule{{ID: "smoke", Type: "PROCESS-NAME-WILDCARD", Value: "__mihomo_smoke_no_process__*", Action: "DIRECT"}}
	if e = m.Save(c); e != nil {
		return e
	}
	info, e := m.Install(ctx)
	if e != nil {
		return e
	}
	fmt.Printf("installed: %v\n", info.Version)
	reference, e := result.Config.YAML("127.0.0.1:19091", "smoke-validation", 19092)
	if e != nil {
		return e
	}
	referencePath := filepath.Join(*dir, "reference.yaml")
	if e = os.WriteFile(referencePath, reference, 0600); e != nil {
		return e
	}
	validationCtx, validationCancel := context.WithTimeout(ctx, 20*time.Second)
	defer validationCancel()
	if e = exec.CommandContext(validationCtx, info.Path, "-t", "-d", *dir, "-f", referencePath).Run(); e != nil {
		return fmt.Errorf("reference YAML validation: %w", e)
	}
	fmt.Println("full imported YAML: validated by Mihomo")
	if *validateOnly {
		return nil
	}
	if e = m.Start(ctx); e != nil {
		return e
	}
	defer m.Stop()
	fmt.Printf("started: tun=%t running=%t\n", *tun, m.Status(ctx).Running)
	client := &http.Client{Timeout: 15 * time.Second, Transport: &http.Transport{Proxy: nil}}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, "https://example.com", nil)
	resp, e := client.Do(req)
	if e != nil {
		return e
	}
	resp.Body.Close()
	fmt.Printf("direct HTTPS: %d\n", resp.StatusCode)
	delays, e := m.TestLatency(ctx)
	if e != nil {
		return e
	}
	ok := 0
	for _, d := range delays {
		if d >= 0 {
			ok++
		}
	}
	fmt.Printf("proxy latency: %d/%d responding\n", ok, len(delays))
	if e = m.Stop(); e != nil {
		return e
	}
	fmt.Printf("stopped: %t\n", !m.Status(ctx).Running)
	return m.ExportLogs(filepath.Join(*dir, "smoke-logs.zip"))
}
