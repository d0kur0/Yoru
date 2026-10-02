package core

import (
	"bytes"
	"compress/gzip"
	"context"
	"crypto/sha256"
	"encoding/base64"
	"encoding/binary"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/d0kur0/Yoru/internal/bundled"
)

// PrivilegedRunner starts only the networking core, leaving the desktop process
// and all persistent settings under the logged-in user's ownership.
type PrivilegedRunner interface {
	StartPrivileged(context.Context, Installation, Config, string, string, int, io.Writer) (Process, error)
}

const privilegedCoreFlag = "--privileged-core-session"
const maxPrivilegedFrame = 2 << 20

type privilegedRequest struct {
	Installation Installation `json:"installation"`
	Config       Config       `json:"config"`
	Controller   string       `json:"controller"`
	Secret       string       `json:"secret"`
	MixedPort    int          `json:"mixedPort"`
}

type privilegedFrame struct {
	Kind    string             `json:"kind"`
	Request *privilegedRequest `json:"request,omitempty"`
	Data    []byte             `json:"data,omitempty"`
	Error   string             `json:"error,omitempty"`
}

type privilegedWire struct {
	conn net.Conn
	mu   sync.Mutex
}

func (w *privilegedWire) send(f privilegedFrame) error {
	b, err := json.Marshal(f)
	if err != nil {
		return err
	}
	if len(b) == 0 || len(b) > maxPrivilegedFrame {
		return errors.New("Слишком большое сообщение помощника TUN")
	}
	w.mu.Lock()
	defer w.mu.Unlock()
	_ = w.conn.SetWriteDeadline(time.Now().Add(5 * time.Second))
	defer w.conn.SetWriteDeadline(time.Time{})
	var size [4]byte
	binary.BigEndian.PutUint32(size[:], uint32(len(b)))
	if err = writePrivilegedBytes(w.conn, size[:]); err != nil {
		return err
	}
	return writePrivilegedBytes(w.conn, b)
}

func writePrivilegedBytes(w io.Writer, b []byte) error {
	for len(b) > 0 {
		n, err := w.Write(b)
		if err != nil {
			return err
		}
		if n == 0 {
			return io.ErrShortWrite
		}
		b = b[n:]
	}
	return nil
}

func readPrivilegedFrame(r io.Reader) (privilegedFrame, error) {
	var f privilegedFrame
	var size [4]byte
	if _, err := io.ReadFull(r, size[:]); err != nil {
		return f, err
	}
	n := binary.BigEndian.Uint32(size[:])
	if n == 0 || n > maxPrivilegedFrame {
		return f, errors.New("Недопустимый размер сообщения помощника TUN")
	}
	b := make([]byte, int(n))
	if _, err := io.ReadFull(r, b); err != nil {
		return f, err
	}
	d := json.NewDecoder(bytes.NewReader(b))
	d.DisallowUnknownFields()
	if err := d.Decode(&f); err != nil {
		return f, err
	}
	if d.Decode(new(any)) != io.EOF {
		return f, errors.New("Лишние данные сообщения помощника TUN")
	}
	return f, nil
}

func (r privilegedRequest) validate() error {
	if !privilegedVersionAllowed(r.Installation.Version) {
		return fmt.Errorf("Для безопасного запуска TUN нужны стабильная версия Mihomo %s или новее. Обновите ядро в настройках", BundledVersion)
	}
	hash, err := hex.DecodeString(r.Installation.SHA256)
	if err != nil || len(hash) != sha256.Size {
		return errors.New("Некорректный SHA-256 ядра TUN")
	}
	secret, err := hex.DecodeString(r.Secret)
	if err != nil || len(secret) != 32 {
		return errors.New("Некорректный секрет контроллера TUN")
	}
	host, port, err := net.SplitHostPort(r.Controller)
	if err != nil || host != "127.0.0.1" {
		return errors.New("Контроллер TUN должен слушать localhost")
	}
	p, err := strconv.Atoi(port)
	if err != nil || p < 1 || p > 65535 || r.MixedPort < 1 || r.MixedPort > 65535 || p == r.MixedPort {
		return errors.New("Некорректный порт ядра TUN")
	}
	if !r.Config.Settings.TUN {
		return errors.New("Помощник предназначен только для режима TUN")
	}
	return validatePrivilegedConfig(r.Config)
}

// Old controller implementations permit arbitrary root file paths in reload
// requests. Only the bundled security baseline and newer stable releases may
// run with privilege; prereleases have no defined compatibility guarantee.
func privilegedVersionAllowed(version string) bool {
	parse := func(value string) ([3]int, bool) {
		var result [3]int
		if !strings.HasPrefix(value, "v") {
			return result, false
		}
		parts := strings.Split(value[1:], ".")
		if len(parts) != 3 {
			return result, false
		}
		for n, p := range parts {
			if p == "" {
				return result, false
			}
			for _, digit := range p {
				if digit < '0' || digit > '9' {
					return result, false
				}
			}
			value, err := strconv.Atoi(p)
			if err != nil {
				return result, false
			}
			result[n] = value
		}
		return result, true
	}
	v, ok := parse(version)
	if !ok {
		return false
	}
	baseline, ok := parse(BundledVersion)
	if !ok {
		return false
	}
	for n := range v {
		if v[n] != baseline[n] {
			return v[n] > baseline[n]
		}
	}
	return true
}

func validatePrivilegedConfig(c Config) error {
	for _, server := range c.Servers {
		if err := validatePrivilegedOptions(server.Options, server.Protocol == "WireGuard", 0); err != nil {
			return err
		}
	}
	return c.Validate()
}

// Imported protocols contain nested plugin, transport, and peers dictionaries.
// The GUI's usual top-level validation is insufficient at the root boundary.
func validatePrivilegedOptions(value any, allowWireGuardKey bool, depth int) error {
	if depth > 32 {
		return errors.New("Слишком глубокие параметры сервера TUN")
	}
	switch v := value.(type) {
	case map[string]any:
		for key, item := range v {
			switch strings.ToLower(key) {
			case "private-key":
				if !(allowWireGuardKey && depth == 0) {
					return errors.New("Вложенные или файловые ключи не разрешены в режиме TUN")
				}
				key, ok := item.(string)
				decoded, err := base64.StdEncoding.DecodeString(key)
				if !ok || err != nil || len(decoded) != 32 {
					return errors.New("WireGuard TUN требует встроенный ключ Base64 из 32 байт")
				}
			case "certificate", "certificate-path", "private-key-path", "ca", "ca-path", "client-cert", "client-key", "dialer-proxy", "interface-name", "routing-mark":
				return errors.New("Файловые параметры и локальные цепочки сервера не разрешены в режиме TUN")
			}
			if err := validatePrivilegedOptions(item, false, depth+1); err != nil {
				return err
			}
		}
	case []any:
		for _, item := range v {
			if err := validatePrivilegedOptions(item, false, depth+1); err != nil {
				return err
			}
		}
	}
	return nil
}

// No path supplied by the GUI is ever opened or executed by the helper.
func privilegedCoreBinary(ctx context.Context, i Installation) ([]byte, error) {
	prefix := "payload/" + runtime.GOOS + "-" + runtime.GOARCH
	if metadata, err := bundled.Files.ReadFile(prefix + ".json"); err == nil {
		var m BundleManifest
		if json.Unmarshal(metadata, &m) == nil && m.Version == i.Version && m.OS == runtime.GOOS && m.Arch == runtime.GOARCH && m.SHA256 == i.SHA256 {
			data, err := bundled.Files.ReadFile(prefix + ".gz")
			if err != nil {
				return nil, err
			}
			r, err := gzip.NewReader(bytes.NewReader(data))
			if err != nil {
				return nil, err
			}
			b, err := readLimit(r, maxBinary)
			_ = r.Close()
			if err != nil {
				return nil, err
			}
			return verifyPrivilegedBinary(b, i.SHA256)
		}
	}
	name, err := assetName(runtime.GOOS, runtime.GOARCH, i.Version)
	if err != nil {
		return nil, err
	}
	client := &http.Client{Timeout: 4 * time.Minute, Transport: &http.Transport{Proxy: nil}, CheckRedirect: func(req *http.Request, via []*http.Request) error {
		if len(via) > 8 || req.URL.Scheme != "https" || !(req.URL.Hostname() == "github.com" || req.URL.Hostname() == "api.github.com" || strings.HasSuffix(req.URL.Hostname(), ".githubusercontent.com")) {
			return errors.New("Недопустимое перенаправление загрузки ядра")
		}
		return nil
	}}
	defer client.CloseIdleConnections()
	metadata, err := get(ctx, client, "https://api.github.com/repos/MetaCubeX/mihomo/releases/tags/"+i.Version, 2<<20)
	if err != nil {
		return nil, err
	}
	var release Release
	if json.Unmarshal(metadata, &release) != nil || release.Tag != i.Version {
		return nil, errors.New("Некорректный официальный релиз Mihomo")
	}
	expected := "https://github.com/MetaCubeX/mihomo/releases/download/" + i.Version + "/" + name
	for _, asset := range release.Assets {
		if asset.Name != name || asset.URL != expected {
			continue
		}
		data, err := get(ctx, client, expected, maxBinary)
		if err != nil {
			return nil, err
		}
		b, err := unpack(asset, data)
		if err != nil {
			return nil, err
		}
		return verifyPrivilegedBinary(b, i.SHA256)
	}
	return nil, errors.New("Официальный файл выбранной версии Mihomo не найден")
}

func verifyPrivilegedBinary(b []byte, expected string) ([]byte, error) {
	sum := sha256.Sum256(b)
	if len(b) < 4 || hex.EncodeToString(sum[:]) != expected {
		return nil, errors.New("SHA-256 выбранного ядра TUN не совпадает")
	}
	return b, nil
}

type privilegedLogWriter struct {
	wire   *privilegedWire
	cancel context.CancelFunc
}

func (w privilegedLogWriter) Write(b []byte) (int, error) {
	total := len(b)
	for len(b) > 0 {
		n := min(len(b), 32<<10)
		if err := w.wire.send(privilegedFrame{Kind: "log", Data: b[:n]}); err != nil {
			w.cancel()
			return total - len(b), err
		}
		b = b[n:]
	}
	return total, nil
}

type privilegedBinaryResolver func(context.Context, Installation) ([]byte, error)

// The socket is also the lifetime lease: disconnect cancels preparation and
// stops the child before removal of the root-owned temporary runtime.
func runPrivilegedSession(parent context.Context, conn net.Conn, request privilegedRequest, tempRoot string, resolve privilegedBinaryResolver, runner Runner) error {
	if err := request.validate(); err != nil {
		return err
	}
	ctx, cancel := context.WithCancel(parent)
	defer cancel()
	wire := &privilegedWire{conn: conn}
	go func() { _, _ = readPrivilegedFrame(conn); cancel() }()
	bin, err := resolve(ctx, request.Installation)
	if err != nil {
		return err
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	dir, err := os.MkdirTemp(tempRoot, "yoru-core-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	// The controller may update files inside its safe home. Keep executable
	// bytes outside that home so a runtime download can never replace them.
	corePath := filepath.Join(dir, "bin", "mihomo")
	runDir := filepath.Join(dir, "runtime")
	configPath := filepath.Join(runDir, "config.yaml")
	if err = atomicWrite(corePath, bin, 0700); err != nil {
		return err
	}
	yaml, err := request.Config.YAML(request.Controller, request.Secret, request.MixedPort)
	if err != nil {
		return err
	}
	if err = atomicWrite(configPath, yaml, 0600); err != nil {
		return err
	}
	if err = runner.Validate(ctx, corePath, []string{"-t", "-d", runDir, "-f", configPath}); err != nil {
		return errors.New("Mihomo отклонил конфигурацию TUN")
	}
	if err = ctx.Err(); err != nil {
		return err
	}
	child, err := runner.Start(corePath, []string{"-d", runDir, "-f", configPath}, privilegedLogWriter{wire, cancel})
	if err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- child.Wait() }()
	if err = wire.send(privilegedFrame{Kind: "started"}); err != nil {
		cancel()
	}
	select {
	case err = <-done:
	case <-ctx.Done():
		_ = child.Stop()
		select {
		case err = <-done:
		case <-time.After(6 * time.Second):
			return errors.New("Помощник TUN не смог остановить ядро")
		}
	}
	result := privilegedFrame{Kind: "exit"}
	if err != nil && ctx.Err() == nil {
		result.Error = "Mihomo завершился с ошибкой. Подробности в журнале"
	}
	_ = wire.send(result)
	return nil
}

type privilegedProcess struct {
	wire      *privilegedWire
	done      chan struct{}
	mu        sync.Mutex
	err       error
	closeOnce sync.Once
	cleanup   func()
}

func (p *privilegedProcess) finish(err error) {
	p.mu.Lock()
	p.err = err
	p.mu.Unlock()
	p.closeOnce.Do(func() {
		_ = p.wire.conn.Close()
		if p.cleanup != nil {
			p.cleanup()
		}
		close(p.done)
	})
}
func (p *privilegedProcess) Wait() error { <-p.done; p.mu.Lock(); defer p.mu.Unlock(); return p.err }
func (p *privilegedProcess) Stop() error {
	select {
	case <-p.done:
		return nil
	default:
	}
	if err := p.wire.send(privilegedFrame{Kind: "stop"}); err != nil {
		_ = p.wire.conn.Close()
	}
	select {
	case <-p.done:
		return nil
	case <-time.After(8 * time.Second):
		_ = p.wire.conn.Close()
		return errors.New("Помощник TUN не ответил на остановку")
	}
}
func (p *privilegedProcess) read(output io.Writer) {
	for {
		f, err := readPrivilegedFrame(p.wire.conn)
		if err != nil {
			p.finish(fmt.Errorf("Связь с помощником TUN прервана: %w", err))
			return
		}
		switch f.Kind {
		case "log":
			if output != nil {
				_, _ = output.Write(f.Data)
			}
		case "exit":
			if f.Error != "" {
				p.finish(errors.New(f.Error))
			} else {
				p.finish(nil)
			}
			return
		default:
			p.finish(errors.New("Некорректный ответ помощника TUN"))
			return
		}
	}
}
