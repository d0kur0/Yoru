package core

import (
	"bytes"
	"context"
	"encoding/binary"
	"encoding/hex"
	"errors"
	"io"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"testing"
	"time"
)

func privilegedTestRequest() privilegedRequest {
	return privilegedRequest{Installation: Installation{Version: BundledVersion, SHA256: strings.Repeat("0", 64)}, Config: validConfig(), Controller: "127.0.0.1:45001", Secret: strings.Repeat("1", 64), MixedPort: 45002}
}

func TestPrivilegedCoreVersionSecurityBaseline(t *testing.T) {
	for _, version := range []string{"v1.19.0", "v1.19.29", "v1.19.30-rc.1", "v1.20.0-rc1", "v2.0.0-alpha", "v1.20", "1.20.0", "v99999999999999999999999.0.0"} {
		if privilegedVersionAllowed(version) {
			t.Fatalf("accepted unsafe version %s", version)
		}
	}
	for _, version := range []string{"v1.19.30", "v1.19.31", "v1.20.0", "v2.0.0"} {
		if !privilegedVersionAllowed(version) {
			t.Fatalf("rejected stable version %s", version)
		}
	}
}

func TestPrivilegedFrameRejectsBoundsAndUnknownFields(t *testing.T) {
	for _, size := range []uint32{0, maxPrivilegedFrame + 1, ^uint32(0)} {
		var b [4]byte
		binary.BigEndian.PutUint32(b[:], size)
		if _, err := readPrivilegedFrame(bytes.NewReader(b[:])); err == nil {
			t.Fatalf("accepted frame size %d", size)
		}
	}
	for _, payload := range []string{`{"kind":"stop","command":"rm"}`, `{"kind":"stop"} {"kind":"stop"}`} {
		var b bytes.Buffer
		_ = binary.Write(&b, binary.BigEndian, uint32(len(payload)))
		b.WriteString(payload)
		if _, err := readPrivilegedFrame(&b); err == nil {
			t.Fatalf("accepted %s", payload)
		}
	}
	left, right := net.Pipe()
	defer left.Close()
	defer right.Close()
	done := make(chan error, 1)
	go func() {
		done <- (&privilegedWire{conn: left}).send(privilegedFrame{Kind: "log", Data: []byte("test\n")})
	}()
	f, err := readPrivilegedFrame(right)
	if err != nil || f.Kind != "log" || string(f.Data) != "test\n" {
		t.Fatalf("round trip: %#v %v", f, err)
	}
	if err = <-done; err != nil {
		t.Fatal(err)
	}
}

func TestPrivilegedRequestRejectsUnsafeConfiguration(t *testing.T) {
	cases := map[string]func(*privilegedRequest){
		"version traversal": func(r *privilegedRequest) { r.Installation.Version = "../../tmp" },
		"bad hash":          func(r *privilegedRequest) { r.Installation.SHA256 = "bad" },
		"remote controller": func(r *privilegedRequest) { r.Controller = "0.0.0.0:45001" },
		"no secret":         func(r *privilegedRequest) { r.Secret = "" },
		"port collision":    func(r *privilegedRequest) { r.MixedPort = 45001 },
		"named port":        func(r *privilegedRequest) { r.Controller = "127.0.0.1:http" },
		"external certificate": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"certificate": "/etc/private"}
		},
		"external key": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"private-key": "/etc/private"}
		},
		"nested plugin cert": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"plugin-opts": map[string]any{"certificate": "/etc/private"}}
		},
		"nested xhttp key": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"xhttp-opts": map[string]any{"download-settings": map[string]any{"private-key": "/etc/private"}}}
		},
		"peer array cert": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"peers": []any{map[string]any{"certificate": "/etc/private"}}}
		},
		"no TUN": func(r *privilegedRequest) { r.Config.Settings.TUN = false },
		"mixed case nested cert": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"plugin-opts": map[string]any{"Certificate": "/etc/private"}}
		},
		"mixed case nested key": func(r *privilegedRequest) {
			r.Config.Servers[0].Options = map[string]any{"peers": []any{map[string]any{"PRIVATE-KEY": "/etc/private"}}}
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			r := privilegedTestRequest()
			mutate(&r)
			if err := r.validate(); err == nil {
				t.Fatal("unsafe request accepted")
			}
		})
	}
	if err := privilegedTestRequest().validate(); err != nil {
		t.Fatal(err)
	}
	if _, err := verifyPrivilegedBinary([]byte("fake binary"), hex.EncodeToString(make([]byte, 32))); err == nil {
		t.Fatal("tampered binary accepted")
	}
}

type privilegedTestChild struct {
	done    chan struct{}
	once    sync.Once
	stopped chan struct{}
}

func (c *privilegedTestChild) Wait() error { <-c.done; return nil }
func (c *privilegedTestChild) Stop() error {
	c.once.Do(func() { close(c.stopped); close(c.done) })
	return nil
}

type privilegedTestRunner struct {
	child       *privilegedTestChild
	path        string
	args        []string
	validateErr error
}

func (r *privilegedTestRunner) Validate(context.Context, string, []string) error {
	return r.validateErr
}
func (r *privilegedTestRunner) Start(path string, args []string, w io.Writer) (Process, error) {
	r.path = path
	r.args = args
	_, err := w.Write([]byte("core log\n"))
	if err != nil {
		return nil, err
	}
	return r.child, nil
}

func TestPrivilegedSessionStopsOnDisconnectAndRemovesPrivateRuntime(t *testing.T) {
	for _, mode := range []string{"disconnect", "stop", "cancel"} {
		t.Run(mode, func(t *testing.T) {
			gui, helper := net.Pipe()
			defer helper.Close()
			runner := &privilegedTestRunner{child: &privilegedTestChild{done: make(chan struct{}), stopped: make(chan struct{})}}
			r := privilegedTestRequest()
			r.Installation.Path = filepath.Join(t.TempDir(), "untrusted-binary")
			root := t.TempDir()
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			done := make(chan error, 1)
			go func() {
				done <- runPrivilegedSession(ctx, helper, r, root, func(context.Context, Installation) ([]byte, error) { return []byte("safe core"), nil }, runner)
			}()
			for {
				f, err := readPrivilegedFrame(gui)
				if err != nil {
					t.Fatal(err)
				}
				if f.Kind == "started" {
					break
				}
			}
			if runner.path == r.Installation.Path || filepath.Dir(filepath.Dir(filepath.Dir(runner.path))) != root {
				t.Fatalf("executed untrusted path %q", runner.path)
			}
			runDir := runner.args[1]
			rel, err := filepath.Rel(runDir, runner.path)
			if err != nil || !strings.HasPrefix(rel, ".."+string(filepath.Separator)) || filepath.Base(filepath.Dir(runner.path)) != "bin" || filepath.Base(runDir) != "runtime" {
				t.Fatalf("controller safe home can access executable: binary=%q home=%q relative=%q err=%v", runner.path, runDir, rel, err)
			}
			config, err := os.ReadFile(filepath.Join(runDir, "config.yaml"))
			if err != nil || !bytes.Contains(config, []byte("external-controller: 127.0.0.1:45001")) {
				t.Fatalf("safe config missing: %v", err)
			}
			if mode == "cancel" {
				cancel()
				go io.Copy(io.Discard, gui)
			} else if mode == "stop" {
				if err = (&privilegedWire{conn: gui}).send(privilegedFrame{Kind: "stop"}); err != nil {
					t.Fatal(err)
				}
				go io.Copy(io.Discard, gui)
			} else {
				_ = gui.Close()
			}
			select {
			case err = <-done:
				if err != nil {
					t.Fatal(err)
				}
			case <-time.After(2 * time.Second):
				t.Fatal("helper did not stop")
			}
			select {
			case <-runner.child.stopped:
			default:
				t.Fatal("child not stopped")
			}
			entries, err := os.ReadDir(root)
			if err != nil || len(entries) != 0 {
				t.Fatalf("private runtime not removed: %v %v", entries, err)
			}
			_ = gui.Close()
		})
	}
}

func TestPrivilegedDisconnectCancelsBinaryPreparation(t *testing.T) {
	gui, helper := net.Pipe()
	defer helper.Close()
	resolving := make(chan struct{})
	done := make(chan error, 1)
	go func() {
		done <- runPrivilegedSession(context.Background(), helper, privilegedTestRequest(), t.TempDir(), func(ctx context.Context, _ Installation) ([]byte, error) {
			close(resolving)
			<-ctx.Done()
			return nil, ctx.Err()
		}, &privilegedTestRunner{})
	}()
	<-resolving
	_ = gui.Close()
	select {
	case err := <-done:
		if !errors.Is(err, context.Canceled) {
			t.Fatalf("preparation cancellation: %v", err)
		}
	case <-time.After(2 * time.Second):
		t.Fatal("download not cancelled on disconnect")
	}
}

func TestPrivilegedValidationFailureCleansRuntime(t *testing.T) {
	gui, helper := net.Pipe()
	defer gui.Close()
	defer helper.Close()
	root := t.TempDir()
	runner := &privilegedTestRunner{validateErr: errors.New("invalid")}
	err := runPrivilegedSession(context.Background(), helper, privilegedTestRequest(), root, func(context.Context, Installation) ([]byte, error) { return []byte("safe core"), nil }, runner)
	if err == nil || runner.path != "" {
		t.Fatal("invalid configuration started child")
	}
	entries, err := os.ReadDir(root)
	if err != nil || len(entries) != 0 {
		t.Fatal("validation failure left private files")
	}
}
