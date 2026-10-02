package core

import (
	"context"
	"errors"
	"time"
)

// The platform runner, rather than the GUI, supplies the privileged core.
func (m *Manager) needsPrivilegedCore(c Config) bool {
	_, supported := m.runner.(PrivilegedRunner)
	return supported && c.Settings.TUN && !m.platform.IsElevated()
}

// Called with m.mu held. Turning on TUN after an ordinary core was started
// requires a new child; a cancelled authorization restores the prior session.
func (m *Manager) restartPrivilegedCore(ctx context.Context, next Config) error {
	previous := m.config
	previousRuntime, err := DecodeConfig(m.applied)
	if err != nil {
		return err
	}
	previousTUN, previousPending := m.retainedTUN, m.pendingTUN
	if err := m.stop(); err != nil {
		return err
	}
	restore := func(cause error) error {
		previous.Revision = m.config.Revision
		saveErr := m.save(previous)
		var startErr error
		if saveErr == nil {
			rollbackCtx, cancel := m.operation(context.Background(), 30*time.Second)
			defer cancel()
			// Saved settings may contain unapplied edits. Restart the configuration
			// that actually ran, while preserving those saved edits on disk.
			stored := m.config
			previousRuntime.Revision = stored.Revision
			m.config = previousRuntime
			startErr = m.startLocked(rollbackCtx)
			m.config = stored
			if startErr == nil {
				m.retainedTUN, m.pendingTUN = previousTUN, previousPending
			}
		}
		result := errors.Join(cause, saveErr, startErr)
		m.lastError = result.Error()
		return result
	}
	if err := m.save(next); err != nil {
		return restore(err)
	}
	if err := m.startLocked(ctx); err != nil {
		return restore(err)
	}
	return nil
}
