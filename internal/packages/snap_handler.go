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
	Snaps []*system.Snap
	// SkipRefreshIfCurrent leaves an installed snap alone if it is already
	// tracking the requested channel, instead of running `snap refresh`. This
	// avoids contacting the snap store for snaps that are already in place,
	// for example on a machine image built with them pre-installed.
	SkipRefreshIfCurrent bool
	system               system.Worker
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
// If already installed, the snap is refreshed, unless SkipRefreshIfCurrent is
// set and the snap is already tracking the requested channel.
func (h *SnapHandler) installSnap(s *system.Snap) error {
	slog.Debug("Installing snap", "snap", s.Name)
	var action, logAction string

	// Decide whether to skip before SnapInfo, which queries the snap store (to
	// find the channel's confinement) and so would defeat the point of skipping.
	if h.SkipRefreshIfCurrent && s.Revision == "" {
		local, err := h.system.InstalledSnapInfo(s.Name)
		if err != nil {
			return fmt.Errorf("failed to lookup installed snap details: %w", err)
		}
		if local.Installed && ChannelSatisfied(s.Channel, local.TrackingChannel) {
			if err := h.enableIfDisabled(s.Name, local); err != nil {
				return err
			}
			slog.Info("Snap already installed on the requested channel, skipping refresh", "snap", s.Name, "tracking", local.TrackingChannel)
			return nil
		}
	}

	snapInfo, err := h.system.SnapInfo(s.Name, s.Channel)
	if err != nil {
		return fmt.Errorf("failed to lookup snap details: %w", err)
	}

	if snapInfo.Installed {
		// A disabled snap must be enabled before it can be refreshed.
		if err := h.enableIfDisabled(s.Name, snapInfo); err != nil {
			return err
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
	_, err = system.RunExclusive(h.system, cmd)
	if err != nil {
		return fmt.Errorf("command failed: %w", err)
	}

	slog.Info(fmt.Sprintf("%s snap", logAction), "snap", s.Name)
	return nil
}

// enableIfDisabled enables an installed snap that is disabled.
func (h *SnapHandler) enableIfDisabled(name string, info *system.SnapInfo) error {
	if info.Active {
		return nil
	}
	enableCmd := system.NewCommand("snap", []string{"enable", name})
	if _, err := system.RunExclusive(h.system, enableCmd); err != nil {
		return fmt.Errorf("failed to enable snap %q: %w", name, err)
	}
	slog.Info("Enabled disabled snap", "snap", name)
	return nil
}

// snapRisks are the snap channel risk levels, from most to least stable.
var snapRisks = map[string]bool{"stable": true, "candidate": true, "beta": true, "edge": true}

// normaliseChannel expands a snap channel to the track/risk[/branch] form
// that snapd reports as a snap's tracking channel: "stable" becomes
// "latest/stable", "3" becomes "3/stable", and "edge/fix" becomes
// "latest/edge/fix".
func normaliseChannel(channel string) string {
	parts := strings.Split(channel, "/")
	switch {
	case len(parts) == 1 && snapRisks[parts[0]]:
		return "latest/" + channel
	case len(parts) == 1:
		return channel + "/stable"
	case len(parts) == 2 && snapRisks[parts[0]]:
		return "latest/" + channel
	default:
		return channel
	}
}

// ChannelSatisfied reports whether a snap tracking the given channel already
// satisfies a request for the wanted channel. An empty wanted channel means
// any channel will do, matching what a bare `snap refresh` would keep.
func ChannelSatisfied(wanted, tracking string) bool {
	if wanted == "" {
		return true
	}
	if tracking == "" {
		return false
	}
	return normaliseChannel(wanted) == normaliseChannel(tracking)
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
