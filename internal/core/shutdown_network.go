package core

import (
	"context"
	"errors"
	"net/http"
	"time"
)

// Called before terminating the process. In particular, TerminateProcess on
// Windows does not run Mihomo's listener cleanup and network restoration.
func (m *Manager) releaseTUN() error {
	ctx, cancel := context.WithTimeout(context.Background(), 4*time.Second)
	defer cancel()
	var before liveSettings
	if err := m.api(ctx, http.MethodGet, "/configs", nil, &before); err != nil {
		return err
	}
	if !before.TUN.Enable {
		return nil
	}
	if err := m.api(ctx, http.MethodPatch, "/configs", map[string]any{"tun": map[string]bool{"enable": false}}, nil); err != nil {
		return err
	}
	var after liveSettings
	if err := m.api(ctx, http.MethodGet, "/configs", nil, &after); err != nil {
		return err
	}
	if after.TUN.Enable {
		return errors.New("ядро не освободило адаптер TUN")
	}
	return nil
}
