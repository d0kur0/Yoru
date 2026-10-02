package core

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net"
	"os"
	"os/exec"
	"os/signal"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"golang.org/x/sys/unix"
)

type macDesktopRunner struct{ commandRunner }

type privilegedPromptOutput struct{ data []byte }

func (b *privilegedPromptOutput) Write(data []byte) (int, error) {
	n := len(data)
	remaining := (16 << 10) - len(b.data)
	if remaining > 0 {
		b.data = append(b.data, data[:min(remaining, n)]...)
	}
	return n, nil
}

func NewDesktopRunner() Runner { return macDesktopRunner{} }

func shellPrivilegedQuote(s string) string { return "'" + strings.ReplaceAll(s, "'", "'\"'\"'") + "'" }
func applePrivilegedQuote(s string) string {
	return "\"" + strings.ReplaceAll(strings.ReplaceAll(s, "\\", "\\\\"), "\"", "\\\"") + "\""
}

func privilegedPeerUID(conn *net.UnixConn) (uint32, error) {
	raw, err := conn.SyscallConn()
	if err != nil {
		return 0, err
	}
	var uid uint32
	var sockErr error
	err = raw.Control(func(fd uintptr) {
		cred, e := unix.GetsockoptXucred(int(fd), unix.SOL_LOCAL, unix.LOCAL_PEERCRED)
		sockErr = e
		if e == nil {
			uid = cred.Uid
		}
	})
	if err != nil {
		return 0, err
	}
	return uid, sockErr
}

func (macDesktopRunner) StartPrivileged(ctx context.Context, i Installation, c Config, controller, secret string, mixedPort int, output io.Writer) (Process, error) {
	r := privilegedRequest{Installation: i, Config: c, Controller: controller, Secret: secret, MixedPort: mixedPort}
	// The helper must never receive a caller-selected executable path.
	r.Installation.Path = ""
	if err := r.validate(); err != nil {
		return nil, err
	}
	dir, err := os.MkdirTemp("/tmp", "yoru-session-")
	if err != nil {
		return nil, err
	}
	keep := false
	defer func() {
		if !keep {
			_ = os.RemoveAll(dir)
		}
	}()
	socket := filepath.Join(dir, "control.sock")
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return nil, err
	}
	defer listener.Close()
	if err = os.Chmod(socket, 0600); err != nil {
		return nil, err
	}
	exe, err := os.Executable()
	if err != nil {
		return nil, err
	}
	command := "/usr/bin/env -i PATH=/usr/bin:/bin:/usr/sbin:/sbin " + shellPrivilegedQuote(exe) + " " + privilegedCoreFlag + " " + shellPrivilegedQuote(socket)
	// Apple events normally time out after two minutes. This command represents
	// the entire connected session, so allow a year; IPC remains its lifetime lease.
	script := "with timeout of 31536000 seconds\ndo shell script " + applePrivilegedQuote(command) + " with administrator privileges\nend timeout"
	prompt := exec.Command("/usr/bin/osascript", "-e", script)
	promptOutput := &privilegedPromptOutput{}
	prompt.Stderr = promptOutput
	if err = prompt.Start(); err != nil {
		return nil, fmt.Errorf("Не удалось открыть запрос прав macOS: %w", err)
	}
	promptDone := make(chan error, 1)
	go func() { promptDone <- prompt.Wait() }()
	success := false
	defer func() {
		if !success {
			_ = prompt.Process.Kill()
		}
	}()
	type accepted struct {
		conn *net.UnixConn
		err  error
	}
	connections := make(chan accepted, 1)
	acceptDone := make(chan struct{})
	go func() { defer close(acceptDone); conn, e := listener.AcceptUnix(); connections <- accepted{conn, e} }()
	defer func() {
		_ = listener.Close()
		<-acceptDone
		select {
		case a := <-connections:
			if a.conn != nil {
				_ = a.conn.Close()
			}
		default:
		}
	}()
	var conn *net.UnixConn
	select {
	case a := <-connections:
		if a.err != nil {
			return nil, a.err
		}
		conn = a.conn
	case e := <-promptDone:
		if e != nil {
			message := strings.TrimSpace(string(promptOutput.data))
			if strings.Contains(message, "(-128)") {
				return nil, errors.New("Запрос прав администратора macOS отменён")
			}
			if message != "" {
				return nil, fmt.Errorf("Не удалось запустить помощник TUN macOS: %s", message)
			}
			return nil, fmt.Errorf("Не удалось получить права администратора macOS: %w", e)
		}
		return nil, errors.New("Помощник TUN завершился до подключения")
	case <-ctx.Done():
		return nil, errors.New("Время ожидания прав администратора macOS истекло")
	}
	connected := false
	defer func() {
		if !connected {
			_ = conn.Close()
		}
	}()
	uid, err := privilegedPeerUID(conn)
	if err != nil || uid != 0 {
		return nil, errors.New("Помощник TUN не подтвердил права root")
	}
	wire := &privilegedWire{conn: conn}
	if err = wire.send(privilegedFrame{Kind: "start", Request: &r}); err != nil {
		return nil, err
	}
	// Startup cancellation closes the lease, but after success the manager's
	// operation context is intentionally detached from the connected session.
	ready := make(chan error, 1)
	go func() {
		for {
			f, e := readPrivilegedFrame(conn)
			if e != nil {
				ready <- e
				return
			}
			switch f.Kind {
			case "log":
				if output != nil {
					_, _ = output.Write(f.Data)
				}
			case "started":
				ready <- nil
				return
			case "exit":
				ready <- errors.New(f.Error)
				return
			default:
				ready <- errors.New("Некорректный ответ помощника TUN")
				return
			}
		}
	}()
	select {
	case err = <-ready:
		if err != nil {
			return nil, err
		}
	case <-ctx.Done():
		return nil, errors.New("Запуск помощника TUN отменён или превысил время ожидания")
	}
	process := &privilegedProcess{wire: wire, done: make(chan struct{}), cleanup: func() { _ = os.RemoveAll(dir) }}
	connected, keep, success = true, true, true
	go process.read(output)
	return process, nil
}

// HandlePrivilegedCore must run before GUI creation or reading user settings.
func HandlePrivilegedCore(args []string) (bool, error) {
	if len(args) == 0 || args[0] != privilegedCoreFlag {
		return false, nil
	}
	if len(args) != 2 || os.Geteuid() != 0 {
		return true, errors.New("Помощник TUN требует отдельный root-сеанс")
	}
	socket := args[1]
	dir := filepath.Dir(socket)
	if !filepath.IsAbs(socket) || filepath.Base(socket) != "control.sock" || !strings.HasPrefix(filepath.Base(dir), "yoru-session-") || (filepath.Dir(dir) != "/tmp" && filepath.Dir(dir) != "/private/tmp") {
		return true, errors.New("Недопустимый адрес сеанса TUN")
	}
	dirInfo, err := os.Lstat(dir)
	if err != nil {
		return true, err
	}
	socketInfo, err := os.Lstat(socket)
	if err != nil {
		return true, err
	}
	dirStat, ok := dirInfo.Sys().(*syscall.Stat_t)
	if !ok {
		return true, errors.New("Не удалось проверить владельца сеанса TUN")
	}
	socketStat, ok := socketInfo.Sys().(*syscall.Stat_t)
	if !ok || !dirInfo.IsDir() || dirInfo.Mode().Perm() != 0700 || socketInfo.Mode()&os.ModeSocket == 0 || socketInfo.Mode().Perm() != 0600 || dirStat.Uid == 0 || socketStat.Uid != dirStat.Uid {
		return true, errors.New("Недопустимые права каталога сеанса TUN")
	}
	conn, err := net.DialUnix("unix", nil, &net.UnixAddr{Name: socket, Net: "unix"})
	if err != nil {
		return true, err
	}
	defer conn.Close()
	uid, err := privilegedPeerUID(conn)
	if err != nil || uid != dirStat.Uid {
		return true, errors.New("Владелец GUI не совпадает с владельцем сеанса TUN")
	}
	_ = conn.SetReadDeadline(time.Now().Add(10 * time.Second))
	f, err := readPrivilegedFrame(conn)
	if err != nil {
		return true, err
	}
	_ = conn.SetReadDeadline(time.Time{})
	if f.Kind != "start" || f.Request == nil || f.Request.Installation.Path != "" {
		return true, errors.New("Некорректный запрос запуска TUN")
	}
	// Only preparation has a deadline; the root process stays alive until the
	// GUI lease closes. Its child lifecycle is bounded by runPrivilegedSession.
	resolver := func(session context.Context, i Installation) ([]byte, error) {
		ctx, cancel := context.WithTimeout(session, 5*time.Minute)
		defer cancel()
		return privilegedCoreBinary(ctx, i)
	}
	session, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM, syscall.SIGHUP)
	defer cancel()
	err = runPrivilegedSession(session, conn, *f.Request, "/private/var/tmp", resolver, commandRunner{})
	if err != nil {
		_ = (&privilegedWire{conn: conn}).send(privilegedFrame{Kind: "exit", Error: err.Error()})
	}
	return true, err
}
