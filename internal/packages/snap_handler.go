package packages

import (
	"fmt"
	"log/slog"
	"strings"

	"github.com/canonical/concierge/internal/system"
)

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
		slog.Debug("Installing snap", "snap", snap.Name)
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

	attrs := []any{"snap", s.Name}
	if newInfo, err := h.system.SnapInstalledInfo(s.Name); err != nil {
		slog.Debug("Could not look up snap details after operation", "snap", s.Name, "error", err)
	} else if newInfo.Installed {
		attrs = append(attrs, "version", newInfo.Version, "revision", newInfo.Revision, "tracking", newInfo.TrackingChannel)
	}
	// The "from" details are already known, so log them even if the lookup above failed.
	if snapInfo.Installed {
		attrs = append(attrs, slog.Group("from", "version", snapInfo.Version, "revision", snapInfo.Revision, "tracking", snapInfo.TrackingChannel))
	}
	slog.Info(fmt.Sprintf("%s snap", logAction), attrs...)
	return nil
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
