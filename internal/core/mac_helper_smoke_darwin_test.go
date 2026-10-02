package core

import (
	"context"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// CI runs this against the signed, packaged application. It verifies headless
// root dispatch and the real UID boundary without changing the runner's routes.
func TestPackagedMacPrivilegedHelperDispatch(t *testing.T) {
	app := os.Getenv("YORU_MAC_HELPER_APP")
	if app == "" {
		t.Skip("requires the packaged macOS application and passwordless CI sudo")
	}
	if !filepath.IsAbs(app) || os.Geteuid() == 0 {
		t.Fatal("smoke test requires an absolute packaged executable and an ordinary GUI user")
	}
	dir, err := os.MkdirTemp("/tmp", "yoru-session-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	socket := filepath.Join(dir, "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Net: "unix", Name: socket})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	if err := os.Chmod(socket, 0600); err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	cmd := exec.CommandContext(ctx, "/usr/bin/sudo", "-n", "/usr/bin/env", "-i", "PATH=/usr/bin:/bin:/usr/sbin:/sbin", app, privilegedCoreFlag, socket)
	cmd.Stderr = &privilegedPromptOutput{}
	if err := cmd.Start(); err != nil {
		t.Fatal(err)
	}
	done := make(chan struct{})
	go func() { _ = cmd.Wait(); close(done) }()
	defer func() {
		cancel()
		select {
		case <-done:
			return
		case <-time.After(2 * time.Second):
		}
		// sudo owns the child credentials, so a user-level Kill may be denied.
		killCtx, killCancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer killCancel()
		_ = exec.CommandContext(killCtx, "/usr/bin/sudo", "-n", "/bin/kill", "-TERM", strconv.Itoa(cmd.Process.Pid)).Run()
		select {
		case <-done:
		case <-time.After(3 * time.Second):
			t.Error("packaged helper did not exit after the session closed")
		}
	}()
	if err := listener.SetDeadline(time.Now().Add(15 * time.Second)); err != nil {
		t.Fatal(err)
	}
	conn, err := listener.AcceptUnix()
	if err != nil {
		t.Fatalf("packaged helper failed to connect: %v", err)
	}
	defer conn.Close()
	uid, err := privilegedPeerUID(conn)
	if err != nil || uid != 0 {
		t.Fatalf("packaged helper did not authenticate as root: uid=%d error=%v", uid, err)
	}
	c := DefaultConfig()
	c.Settings.TUN = false // A deliberate validation failure, never a real TUN.
	request := privilegedRequest{
		Installation: Installation{Version: BundledVersion, SHA256: strings.Repeat("0", 64)},
		Config:       c, Controller: "127.0.0.1:19090", Secret: strings.Repeat("0", 64), MixedPort: 17890,
	}
	wire := &privilegedWire{conn: conn}
	if err := wire.send(privilegedFrame{Kind: "start", Request: &request}); err != nil {
		t.Fatal(err)
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	frame, err := readPrivilegedFrame(conn)
	if err != nil || frame.Kind != "exit" || !strings.Contains(frame.Error, "только для режима TUN") {
		t.Fatalf("packaged helper did not enforce request validation: frame=%+v error=%v", frame, err)
	}
}
