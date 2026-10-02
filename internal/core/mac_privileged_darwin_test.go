package core

import (
	"bytes"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"testing"
)

func TestPrivilegedMacQuotingPreservesUntrustedPath(t *testing.T) {
	for _, input := range []string{"/Applications/Yoru.app/Contents/MacOS/Yoru", "/tmp/a ' quoted \\ \" path", "/tmp/$(echo injected); `echo injected`\nend"} {
		got, err := exec.Command("/bin/sh", "-c", "printf '%s' "+shellPrivilegedQuote(input)).Output()
		if err != nil || string(got) != input {
			t.Fatalf("shell quote %q: %q %v", input, got, err)
		}
		got, err = exec.Command("/usr/bin/osascript", "-e", "return "+applePrivilegedQuote(input)).Output()
		if err != nil || !bytes.Equal(got, []byte(input+"\n")) {
			t.Fatalf("AppleScript quote %q: %q %v", input, got, err)
		}
	}
}

func TestPrivilegedMacSocketPeerUID(t *testing.T) {
	// A short path avoids Darwin's Unix socket path-length limit in CI temp dirs.
	dir, err := os.MkdirTemp("/tmp", "yoru-peer-test-")
	if err != nil {
		t.Fatal(err)
	}
	defer os.RemoveAll(dir)
	listener, err := net.ListenUnix("unix", &net.UnixAddr{Name: filepath.Join(dir, "socket"), Net: "unix"})
	if err != nil {
		t.Fatal(err)
	}
	defer listener.Close()
	accepted := make(chan *net.UnixConn, 1)
	failure := make(chan error, 1)
	go func() {
		c, e := listener.AcceptUnix()
		if e != nil {
			failure <- e
		} else {
			accepted <- c
		}
	}()
	client, err := net.DialUnix("unix", nil, listener.Addr().(*net.UnixAddr))
	if err != nil {
		t.Fatal(err)
	}
	defer client.Close()
	var server *net.UnixConn
	select {
	case server = <-accepted:
	case err = <-failure:
		t.Fatal(err)
	}
	defer server.Close()
	for _, c := range []*net.UnixConn{client, server} {
		uid, err := privilegedPeerUID(c)
		if err != nil || uid != uint32(os.Geteuid()) {
			t.Fatalf("peer UID %d expected %d: %v", uid, os.Geteuid(), err)
		}
	}
}

func TestPrivilegedMacPromptOutputIsBounded(t *testing.T) {
	buffer := &privilegedPromptOutput{}
	data := bytes.Repeat([]byte("x"), 32<<10)
	n, err := buffer.Write(data)
	if err != nil || n != len(data) || len(buffer.data) != 16<<10 {
		t.Fatal("prompt output was not bounded")
	}
}
