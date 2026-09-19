package core

import (
	"bytes"
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"sync"
	"time"
)

type Manager struct {
	lifetime         context.Context
	cancel           context.CancelFunc
	mu               sync.Mutex
	probeMu          sync.Mutex
	dir              string
	config           Config
	runner           Runner
	client           *http.Client
	downloadClient   *http.Client
	releaseURL       string
	process          Process
	done             chan struct{}
	endpoint, secret string
	started          time.Time
	lastError        string
	applied          []byte
	restoreProxy     func() error
	platform         Platform
	log              *rotatingLog
}
type Platform interface {
	Autostart(enabled, minimized, tun bool) error
	Proxy(int) (func() error, error)
	IsElevated() bool
}
type Status struct {
	ActiveServer   string       `json:"activeServer"`
	UsingReserve   bool         `json:"usingReserve"`
	Bundled        bool         `json:"bundled"`
	Revision       uint64       `json:"revision"`
	Installed      bool         `json:"installed"`
	Version        string       `json:"version"`
	Running        bool         `json:"running"`
	Started        int64        `json:"started"`
	Error          string       `json:"error"`
	Pending        bool         `json:"pending"`
	DownloadTotal  uint64       `json:"downloadTotal"`
	UploadTotal    uint64       `json:"uploadTotal"`
	Connections    []Connection `json:"connections"`
	Elevated       bool         `json:"elevated"`
	NeedsElevation bool         `json:"needsElevation"`
}
type Connection struct {
	ProcessPath string `json:"processPath"`
	ID          string `json:"id"`
	App         string `json:"app"`
	Host        string `json:"host"`
	IP          string `json:"ip"`
	Type        string `json:"type"`
	Rule        string `json:"rule"`
	Action      string `json:"action"`
	Down        uint64 `json:"down"`
	Up          uint64 `json:"up"`
}

func New(dir string) (*Manager, error) {
	m := &Manager{dir: dir, config: DefaultConfig(), runner: commandRunner{}, client: &http.Client{Timeout: 3 * time.Second, Transport: &http.Transport{Proxy: nil}}, downloadClient: &http.Client{Timeout: 4 * time.Minute}, releaseURL: "https://api.github.com/repos/MetaCubeX/mihomo/releases/latest", platform: newPlatform(dir), log: &rotatingLog{dir: filepath.Join(dir, "logs"), settings: *DefaultLogs()}}
	m.lifetime, m.cancel = context.WithCancel(context.Background())
	m.downloadClient.CheckRedirect = func(req *http.Request, via []*http.Request) error {
		if len(via) >= 8 {
			return errors.New("Слишком много перенаправлений")
		}
		if req.URL.Scheme != "https" {
			return errors.New("Перенаправление с HTTPS запрещено")
		}
		return nil
	}
	data, e := os.ReadFile(filepath.Join(dir, "config.json"))
	if e == nil {
		m.config, e = DecodeConfig(data)
		if e != nil {
			return nil, e
		}
	} else if !errors.Is(e, os.ErrNotExist) {
		return nil, e
	}
	m.log.Configure(*m.config.Logging)
	return m, nil
}
func (m *Manager) Config() Config {
	m.mu.Lock()
	defer m.mu.Unlock()
	b, _ := json.Marshal(m.config)
	var c Config
	_ = json.Unmarshal(b, &c)
	return c
}
func (m *Manager) Save(c Config) error { m.mu.Lock(); defer m.mu.Unlock(); return m.save(c) }
func (m *Manager) save(c Config) error {
	c.normalize()
	if c.Revision != m.config.Revision {
		return errors.New("Настройки обновились в фоне. Повторите изменение после обновления страницы")
	}
	if e := c.Validate(); e != nil {
		return e
	}
	c.Revision++
	b, e := json.MarshalIndent(c, "", "  ")
	if e != nil {
		return e
	}
	old := m.config.Settings
	changed := old.Autostart != c.Settings.Autostart ||
		(c.Settings.Autostart && (old.Minimized != c.Settings.Minimized || old.TUN != c.Settings.TUN))
	if changed {
		if e = m.platform.Autostart(c.Settings.Autostart, c.Settings.Minimized, c.Settings.TUN); e != nil {
			return e
		}
	}
	if e = atomicWrite(filepath.Join(m.dir, "config.json"), b, 0600); e != nil {
		if changed {
			_ = m.platform.Autostart(old.Autostart, old.Minimized, old.TUN)
		}
		return e
	}
	// Decode to detach all slices/maps from the caller.
	_ = json.Unmarshal(b, &m.config)
	m.log.Configure(*m.config.Logging)
	return nil
}

func freePort() (int, error) {
	l, e := net.Listen("tcp", "127.0.0.1:0")
	if e != nil {
		return 0, e
	}
	p := l.Addr().(*net.TCPAddr).Port
	e = l.Close()
	return p, e
}
func (m *Manager) Start(ctx context.Context) (err error) {
	ctx, cancelAll := m.operation(ctx, 45*time.Second)
	defer cancelAll()
	m.mu.Lock()
	defer m.mu.Unlock()
	defer func() {
		if err != nil {
			m.lastError = err.Error()
		}
	}()
	if m.process != nil {
		return errors.New("Ядро уже запущено. Для применения изменений переподключитесь")
	}
	if len(m.config.Servers) == 0 {
		return errors.New("Добавьте сервер перед подключением")
	}
	i, e := m.installed(true)
	if e != nil {
		if _, statErr := os.Stat(filepath.Join(m.dir, "core.json")); os.IsNotExist(statErr) {
			i, e = m.installBundled()
		}
		if e != nil {
			return e
		}
	}
	controller, e := freePort()
	if e != nil {
		return e
	}
	port, e := freePort()
	if e != nil {
		return e
	}
	for port == controller {
		port, e = freePort()
		if e != nil {
			return e
		}
	}
	secret := make([]byte, 32)
	if _, e = rand.Read(secret); e != nil {
		return e
	}
	m.secret = hex.EncodeToString(secret)
	address := fmt.Sprintf("127.0.0.1:%d", controller)
	config, e := m.config.YAML(address, m.secret, port)
	if e != nil {
		return e
	}
	runDir := filepath.Join(m.dir, "runtime")
	configPath := filepath.Join(runDir, "config.yaml")
	if e = atomicWrite(configPath, config, 0600); e != nil {
		return e
	}
	validateCtx, cancel := context.WithTimeout(ctx, 20*time.Second)
	defer cancel()
	if e = m.runner.Validate(validateCtx, i.Path, []string{"-t", "-d", runDir, "-f", configPath}); e != nil {
		return errors.New("Mihomo отклонил конфигурацию. Проверьте параметры сервера и DNS")
	}
	if e = ctx.Err(); e != nil {
		return e
	}
	m.log.Reset()
	p, e := m.runner.Start(i.Path, []string{"-d", runDir, "-f", configPath}, m.log)
	if e != nil {
		return fmt.Errorf("Не удалось запустить Mihomo: %w", e)
	}
	m.process = p
	m.done = make(chan struct{})
	m.endpoint = "http://" + address
	m.lastError = ""
	done := m.done
	go func() {
		err := p.Wait()
		close(done)
		m.mu.Lock()
		defer m.mu.Unlock()
		if m.process != p {
			return
		}
		m.process = nil
		m.endpoint = ""
		m.started = time.Time{}
		if err != nil {
			m.lastError = "Процесс Mihomo завершился с ошибкой. Проверьте журнал и права TUN"
		}
		if m.restoreProxy != nil {
			if e := m.restoreProxy(); e != nil {
				m.lastError = "Не удалось восстановить системный прокси: " + e.Error()
			}
			m.restoreProxy = nil
		}
	}()
	readyCtx, readyCancel := context.WithTimeout(ctx, 15*time.Second)
	defer readyCancel()
	ticker := time.NewTicker(150 * time.Millisecond)
	defer ticker.Stop()
	for {
		var version struct {
			Version string `json:"version"`
		}
		if m.api(readyCtx, http.MethodGet, "/version", nil, &version) == nil && version.Version != "" {
			break
		}
		select {
		case <-done:
			m.process = nil
			m.endpoint = ""
			return errors.New("Mihomo завершился при запуске. Для TUN нужны права администратора/root; подробности в журнале")
		case <-readyCtx.Done():
			_ = m.stop()
			return errors.New("Ядро не ответило за 15 секунд; процесс остановлен")
		case <-ticker.C:
		}
	}
	// A valid token proves this is our controller, never an existing VPN instance.
	if m.config.Settings.Mode == "global" {
		if e = m.api(ctx, http.MethodPut, "/proxies/GLOBAL", map[string]string{"name": "PROXY"}, nil); e != nil {
			_ = m.stop()
			return e
		}
	}
	if m.config.Settings.SystemProxy {
		m.restoreProxy, e = m.platform.Proxy(port)
		if e != nil {
			_ = m.stop()
			return e
		}
	}
	m.started = time.Now()
	m.applied = runtimeSnapshot(m.config)
	return nil
}
func (m *Manager) stop() error {
	var result error
	if m.process != nil {
		p := m.process
		done := m.done
		if e := p.Stop(); e != nil {
			select {
			case <-done:
			default:
				result = e
			}
		}
		select {
		case <-done:
			m.process = nil
			m.endpoint = ""
			m.started = time.Time{}
		case <-time.After(5 * time.Second):
			return errors.New("Mihomo не завершился; повторите остановку")
		}
	}
	if m.restoreProxy != nil {
		result = errors.Join(result, m.restoreProxy())
		m.restoreProxy = nil
	}
	return result
}
func (m *Manager) Stop() error     { m.mu.Lock(); defer m.mu.Unlock(); return m.stop() }
func (m *Manager) Shutdown() error { m.cancel(); return m.Stop() }
func (m *Manager) operation(parent context.Context, timeout time.Duration) (context.Context, context.CancelFunc) {
	ctx, cancel := context.WithTimeout(parent, timeout)
	stop := context.AfterFunc(m.lifetime, cancel)
	return ctx, func() { stop(); cancel() }
}
func (m *Manager) Preview() (string, error) {
	c := m.Config()
	b, e := c.YAML("127.0.0.1:9090", "<generated-at-start>", 7890)
	return string(b), e
}
func (m *Manager) api(ctx context.Context, method, path string, body, result any) error {
	return m.apiWithClient(ctx, m.client, method, path, body, result)
}
func (m *Manager) apiWithClient(ctx context.Context, client *http.Client, method, path string, body, result any) error {
	return controllerAPI(ctx, client, m.endpoint, m.secret, method, path, body, result)
}
func controllerAPI(ctx context.Context, client *http.Client, endpoint, secret, method, path string, body, result any) error {
	if endpoint == "" {
		return errors.New("Ядро не запущено")
	}
	var data []byte
	if body != nil {
		data, _ = json.Marshal(body)
	}
	req, e := http.NewRequestWithContext(ctx, method, endpoint+path, bytes.NewReader(data))
	if e != nil {
		return e
	}
	req.Header.Set("Authorization", "Bearer "+secret)
	req.Header.Set("Content-Type", "application/json")
	res, e := client.Do(req)
	if e != nil {
		return errors.New("Контроллер Mihomo недоступен")
	}
	defer res.Body.Close()
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return fmt.Errorf("Mihomo API: HTTP %d", res.StatusCode)
	}
	if result != nil {
		b, e := readLimit(res.Body, 8<<20)
		if e != nil {
			return e
		}
		return json.Unmarshal(b, result)
	}
	return nil
}
func (m *Manager) Status(ctx context.Context) Status {
	m.mu.Lock()
	defer m.mu.Unlock()
	s := Status{Bundled: HasBundledCore(), Error: m.lastError, Connections: []Connection{}, Revision: m.config.Revision}
	s.Elevated = m.platform.IsElevated()
	s.NeedsElevation = m.config.Settings.TUN && !s.Elevated
	if i, e := m.installed(false); e == nil {
		s.Installed = true
		s.Version = i.Version
	}
	if m.process == nil {
		return s
	}
	select {
	case <-m.done:
		return s
	default:
	}
	s.Running = true
	if !m.started.IsZero() {
		s.Started = m.started.UnixMilli()
	}
	b := runtimeSnapshot(m.config)
	s.Pending = !bytes.Equal(b, m.applied)
	var group struct {
		Now string `json:"now"`
	}
	if m.api(ctx, http.MethodGet, "/proxies/PROXY", nil, &group) == nil {
		var applied Config
		_ = json.Unmarshal(m.applied, &applied)
		for _, node := range applied.Servers {
			if proxyName(node.ID) == group.Now {
				s.ActiveServer = node.ID
				s.UsingReserve = applied.GroupMode() == "fallback" && node.ID != applied.Selected
				break
			}
		}
	}

	var raw struct {
		Download    uint64 `json:"downloadTotal"`
		Upload      uint64 `json:"uploadTotal"`
		Connections []struct {
			ID       string `json:"id"`
			Metadata struct {
				Process       string `json:"process"`
				ProcessPath   string `json:"processPath"`
				Host          string `json:"host"`
				DestinationIP string `json:"destinationIP"`
				Network       string `json:"network"`
			} `json:"metadata"`
			Rule        string   `json:"rule"`
			RulePayload string   `json:"rulePayload"`
			Chains      []string `json:"chains"`
			Upload      uint64   `json:"upload"`
			Download    uint64   `json:"download"`
		} `json:"connections"`
	}
	if e := m.api(ctx, http.MethodGet, "/connections", nil, &raw); e != nil {
		s.Error = e.Error()
		return s
	}
	s.DownloadTotal = raw.Download
	s.UploadTotal = raw.Upload
	for _, c := range raw.Connections {
		a := "PROXY"
		for _, chain := range c.Chains {
			if chain == "DIRECT" || chain == "REJECT" {
				a = chain
			}
		}
		app := c.Metadata.Process
		if app == "" {
			app = c.Metadata.ProcessPath
		}
		host := c.Metadata.Host
		if host == "" {
			host = c.Metadata.DestinationIP
		}
		s.Connections = append(s.Connections, Connection{c.Metadata.ProcessPath, c.ID, app, host, c.Metadata.DestinationIP, c.Metadata.Network, c.Rule + " " + c.RulePayload, a, c.Download, c.Upload})
	}
	return s
}
func (m *Manager) latencyAPI(ctx context.Context, path string, result any) error {
	client := *m.client
	client.Timeout = time.Duration(m.config.HealthTimeout()+1000) * time.Millisecond
	return m.apiWithClient(ctx, &client, http.MethodGet, path, nil, result)
}

// Called with m.mu held. Saved nodes may not exist in the running core yet.
func (m *Manager) nodeApplied(id string) bool {
	var applied Config
	if json.Unmarshal(m.applied, &applied) != nil {
		return false
	}
	for _, old := range applied.Servers {
		if old.ID != id {
			continue
		}
		for _, current := range m.config.Servers {
			if current.ID != id {
				continue
			}
			a, ea := old.Proxy()
			b, eb := current.Proxy()
			if ea != nil || eb != nil {
				return false
			}
			av, _ := json.Marshal(a)
			bv, _ := json.Marshal(b)
			return bytes.Equal(av, bv)
		}
	}
	return false
}
func (m *Manager) TestLatency(ctx context.Context) (map[string]int, error) {
	c := m.Config()
	results := make(map[string]int, len(c.Servers))
	for _, s := range c.Servers {
		delay, err := m.TestServerLatency(ctx, s.ID)
		if err != nil {
			return nil, err
		}
		results[s.ID] = delay
	}
	return results, nil
}

// TestServerLatency measures saved settings without restarting the active VPN.
func (m *Manager) TestServerLatency(ctx context.Context, id string) (int, error) {
	return m.testServerLatency(ctx, id, false)
}

// Status always measures the node actually loaded in the active core.
func (m *Manager) TestRunningServerLatency(ctx context.Context, id string) (int, error) {
	return m.testServerLatency(ctx, id, true)
}
func (m *Manager) testServerLatency(ctx context.Context, id string, running bool) (int, error) {
	m.mu.Lock()
	if m.process == nil {
		m.mu.Unlock()
		return 0, errors.New("Ядро не запущено")
	}
	timeout, target := m.config.HealthTimeout(), m.config.LatencyTestURL()
	endpoint, secret, client := m.endpoint, m.secret, *m.client
	if running {
		var applied Config
		found := false
		if json.Unmarshal(m.applied, &applied) == nil {
			for _, s := range applied.Servers {
				if s.ID == id {
					found = true
					break
				}
			}
		}
		m.mu.Unlock()
		if !found {
			return -1, errors.New("Сервер отсутствует в работающем ядре")
		}
	} else if !m.nodeApplied(id) {
		var proxy map[string]any
		var err error
		for _, s := range m.config.Servers {
			if s.ID == id {
				proxy, err = s.Proxy()
				break
			}
		}
		if err != nil || proxy == nil {
			m.mu.Unlock()
			return -1, errors.New("Сервер недоступен для проверки")
		}
		installation, err := m.installed(true)
		ipv6 := m.config.Settings.IPv6
		m.mu.Unlock()
		if err != nil {
			return -1, err
		}
		return m.probeSavedServer(ctx, installation.Path, proxy, ipv6, timeout, target)
	} else {
		m.mu.Unlock()
	}
	ctx, cancel := m.operation(ctx, time.Duration(timeout+1000)*time.Millisecond)
	defer cancel()
	client.Timeout = time.Duration(timeout+1000) * time.Millisecond
	return probeDelay(ctx, &client, endpoint, secret, id, timeout, target)
}

func (m *Manager) CloseConnection(ctx context.Context, id string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.api(ctx, http.MethodDelete, "/connections/"+url.PathEscape(id), nil, nil)
}
func (m *Manager) Logs() string { return m.log.String() }
