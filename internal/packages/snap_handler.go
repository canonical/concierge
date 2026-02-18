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
		err := h.installSnap(snap)
		if err != nil {
			return fmt.Errorf("failed to install snap: %w", err)
		}

		err = h.connectSnap(snap)
		if err != nil {
			return fmt.Errorf("failed to create snap connections: %w", err)
		}
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
func (h *SnapHandler) installSnap(s *system.Snap) error {
	slog.Debug("Installing snap", "snap", s.Name)
	var action, logAction string

	snapInfo, err := h.system.SnapInfo(s.Name, s.Channel)
	if err != nil {
		return fmt.Errorf("failed to lookup snap details: %w", err)
	}

	if snapInfo.Installed {
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

	if snapInfo.Classic {
		args = append(args, "--classic")
	}

	cmd := system.NewCommand("snap", args)
	_, err = system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	slog.Info(fmt.Sprintf("%s snap", logAction), "snap", s.Name)
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
