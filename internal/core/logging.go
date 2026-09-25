package core

import (
	"archive/zip"
	"bytes"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"sync"
	"time"
)

type rotatingLog struct {
	mu        sync.Mutex
	dir       string
	settings  LogSettings
	b         []byte
	lastError string
}

func (l *rotatingLog) Configure(s LogSettings) {
	l.mu.Lock()
	defer l.mu.Unlock()
	l.settings = s
	if !s.Enabled {
		l.b = nil
	}
	l.prune()
}
func (l *rotatingLog) name(i int) string {
	if i == 0 {
		return filepath.Join(l.dir, "mihomo.log")
	}
	return filepath.Join(l.dir, "mihomo."+strconv.Itoa(i)+".log")
}
func (l *rotatingLog) prune() {
	for i := 0; i < 10; i++ {
		p := l.name(i)
		info, e := os.Stat(p)
		if e == nil && (i >= l.settings.Files || time.Since(info.ModTime()) > time.Duration(l.settings.Days)*24*time.Hour) {
			if e = os.Remove(p); e != nil {
				l.lastError = e.Error()
			}
		}
	}
}
func (l *rotatingLog) Write(p []byte) (int, error) {
	l.mu.Lock()
	defer l.mu.Unlock()
	n := len(p)
	if !l.settings.Enabled {
		return n, nil
	}
	l.prune()
	if e := os.MkdirAll(l.dir, 0700); e != nil {
		l.lastError = e.Error()
		return n, nil
	}
	limit := int64(l.settings.MaxSizeMB) << 20
	l.b = append(l.b, p...)
	if len(l.b) > 64<<10 {
		l.b = bytes.Clone(l.b[len(l.b)-(64<<10):])
	}
	for len(p) > 0 {
		info, _ := os.Stat(l.name(0))
		var size int64
		if info != nil {
			size = info.Size()
		}
		if size >= limit {
			_ = os.Remove(l.name(l.settings.Files - 1))
			for i := l.settings.Files - 2; i >= 0; i-- {
				if e := os.Rename(l.name(i), l.name(i+1)); e != nil && !os.IsNotExist(e) {
					l.lastError = e.Error()
					return n, nil
				}
			}
			size = 0
		}
		count := min(int64(len(p)), limit-size)
		f, e := os.OpenFile(l.name(0), os.O_CREATE|os.O_APPEND|os.O_WRONLY, 0600)
		if e != nil {
			l.lastError = e.Error()
			return n, nil
		}
		_, e = f.Write(p[:count])
		ce := f.Close()
		if e == nil {
			e = ce
		}
		if e != nil {
			l.lastError = e.Error()
			return n, nil
		}
		p = p[count:]
	}
	return n, nil
}
func (l *rotatingLog) String() string {
	l.mu.Lock()
	defer l.mu.Unlock()
	s := string(l.b)
	if l.lastError != "" {
		s += "\nОшибка записи журнала: " + l.lastError
	}
	return s
}
func (l *rotatingLog) Reset() { l.mu.Lock(); defer l.mu.Unlock(); l.b = nil }

// LogsDir is the fixed directory used by the Mihomo log writer.
func (m *Manager) LogsDir() string { return m.log.dir }

func (m *Manager) ClearLogs() error {
	l := m.log
	l.mu.Lock()
	defer l.mu.Unlock()
	for i := 0; i < 10; i++ {
		if e := os.Remove(l.name(i)); e != nil && !os.IsNotExist(e) {
			return e
		}
	}
	l.b = nil
	l.lastError = ""
	return nil
}
func (m *Manager) ExportLogs(path string) error {
	l := m.log
	l.mu.Lock()
	defer l.mu.Unlock()
	l.prune()
	var buf bytes.Buffer
	z := zip.NewWriter(&buf)
	for i := 0; i < l.settings.Files; i++ {
		data, e := os.ReadFile(l.name(i))
		if os.IsNotExist(e) {
			continue
		}
		if e != nil {
			return e
		}
		w, e := z.Create(filepath.Base(l.name(i)))
		if e != nil {
			return e
		}
		if _, e = w.Write(data); e != nil {
			return e
		}
	}
	if e := z.Close(); e != nil {
		return e
	}
	if buf.Len() > 500<<20 {
		return fmt.Errorf("Журнал слишком большой")
	}
	return atomicWrite(path, buf.Bytes(), 0600)
}
