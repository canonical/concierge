package packages

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/canonical/concierge/internal/system"
)

// snapLogLookupTimeout bounds the snapd lookup made after an install or refresh,
// which is only used for logging.
const snapLogLookupTimeout = 5 * time.Second

// NewSnapHandler constructs a new instance of a SnapHandler.
func NewSnapHandler(system system.Worker, snaps []*system.Snap) *SnapHandler {
	return &SnapHandler{
		Snaps:  snaps,
		system: system,
	}
}

// SnapHandler can install or remove a set of snaps.
type SnapHandler struct {
	Snaps  []*system.Snap
	system system.Worker
}

// Prepare installs a set of snaps on the machine.
func (h *SnapHandler) Prepare() error {
	for _, snap := range h.Snaps {
		snapInfo, err := h.system.SnapInfo(snap.Name, snap.Channel)
		if err != nil {
			return fmt.Errorf("failed to install snap: failed to lookup snap details: %w", err)
		}

		err = h.PrepareSnap(snap, snapInfo)
		if err != nil {
			return err
		}
	}
	return nil
}

// PrepareSnap installs (or refreshes) and connects a single snap, using snap
// details that the caller has already looked up with SnapInfo. This lets callers
// that need the details before installing avoid a second snap store query.
func (h *SnapHandler) PrepareSnap(s *system.Snap, snapInfo *system.SnapInfo) error {
	err := h.installSnap(s, snapInfo)
	if err != nil {
		return fmt.Errorf("failed to install snap: %w", err)
	}

	err = h.connectSnap(s)
	if err != nil {
		return fmt.Errorf("failed to create snap connections: %w", err)
	}
	return nil
}

// Restore removes a set of snaps from the machine.
func (h *SnapHandler) Restore() error {
	for _, snap := range h.Snaps {
		err := h.removeSnap(snap)
		if err != nil {
			return fmt.Errorf("failed to remove snap: %w", err)
		}
	}
	return nil
}

// installSnap ensures that the specified snap is installed at the specified channel.
// If already installed, but on the wrong channel, the snap is refreshed.
func (h *SnapHandler) installSnap(s *system.Snap, snapInfo *system.SnapInfo) error {
	slog.Debug("Installing snap", "snap", s.Name)
	var action, logAction string

	if snapInfo.Installed {
		// A disabled snap must be enabled before it can be refreshed.
		if !snapInfo.Active {
			enableCmd := system.NewCommand("snap", []string{"enable", s.Name})
			if _, err := system.RunExclusive(h.system, enableCmd); err != nil {
				return fmt.Errorf("failed to enable snap %q: %w", s.Name, err)
			}
			slog.Info("Enabled disabled snap", "snap", s.Name, "version", snapInfo.Version, "revision", snapInfo.Revision, "tracking", snapInfo.TrackingChannel)
		}
		action = "refresh"
		logAction = "Refreshed"
	} else {
		action = "install"
		logAction = "Installed"
	}

	args := []string{action, s.Name}

	if s.Channel != "" {
		args = append(args, "--channel", s.Channel)
	}

	if s.Revision != "" {
		args = append(args, "--revision", s.Revision)
	}

	if snapInfo.Classic {
		args = append(args, "--classic")
	}

	cmd := system.NewCommand("snap", args)
	_, err := system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	h.logSnapOperation(logAction, s, snapInfo)
	return nil
}

// logSnapOperation logs that a snap was installed or refreshed, including its
// new version, revision and tracking channel, and those it was refreshed from.
func (h *SnapHandler) logSnapOperation(logAction string, s *system.Snap, before *system.SnapInfo) {
	ctx := context.Background()
	// The lookup below is only needed for this log line, so skip it if it
	// wouldn't be shown.
	if !slog.Default().Enabled(ctx, slog.LevelInfo) {
		return
	}

	attrs := []any{"snap", s.Name}

	// In a dry run nothing was installed or refreshed, so looking the snap up
	// again would just report its unchanged state as the new one.
	if !system.IsDryRun(h.system) {
		// Bound the lookup so that a snapd hiccup can't stall provisioning just
		// to improve a log line.
		lookupCtx, cancel := context.WithTimeout(ctx, snapLogLookupTimeout)
		defer cancel()
		if after, err := h.system.SnapInstalledInfo(lookupCtx, s.Name); err != nil {
			slog.Debug("Could not look up snap details after operation", "snap", s.Name, "error", err)
		} else if after.Installed {
			attrs = append(attrs, "version", after.Version, "revision", after.Revision, "tracking", after.TrackingChannel)
		}
	}

	// The "from" details are already known, so log them even if the lookup above failed.
	if before.Installed {
		attrs = append(attrs, slog.Group("from", "version", before.Version, "revision", before.Revision, "tracking", before.TrackingChannel))
	}

	slog.Info(fmt.Sprintf("%s snap", logAction), attrs...)
}

// connectSnap ensures that the specified snap interfaces are connected.
func (h *SnapHandler) connectSnap(s *system.Snap) error {
	for _, connection := range s.Connections {
		parts := strings.Split(connection, " ")
		if len(parts) > 2 {
			return fmt.Errorf("too many arguments in snap connection string '%s'", connection)
		}

		args := append([]string{"connect"}, parts...)

		cmd := system.NewCommand("snap", args)
		_, err := system.RunExclusive(h.system, cmd)
		if err != nil {
			return fmt.Errorf("command failed: %w", err)
		}
	}
	return nil
}

// removeSnap uninstalls the specified snap from the system, optionally purging its data.
func (h *SnapHandler) removeSnap(s *system.Snap) error {
	slog.Debug("Removing snap", "snap", s.Name)
	args := []string{"remove", s.Name, "--purge"}

	cmd := system.NewCommand("snap", args)
	_, err := system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("failed to remove snap '%s': %w", s.Name, err)
	}

	slog.Info("Removed snap", "snap", s.Name)
	return nil
}
