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

// NewSnapHandler constructs a new instance of a SnapHandler. When
// skipRefreshIfCurrent is set, an installed snap that is already tracking the
// requested channel is left alone instead of being refreshed.
func NewSnapHandler(system system.Worker, snaps []*system.Snap, skipRefreshIfCurrent bool) *SnapHandler {
	return &SnapHandler{
		Snaps:                snaps,
		skipRefreshIfCurrent: skipRefreshIfCurrent,
		system:               system,
	}
}

// SnapHandler can install or remove a set of snaps.
type SnapHandler struct {
	Snaps []*system.Snap
	// skipRefreshIfCurrent leaves an installed snap alone if it is already
	// tracking the requested channel, instead of running `snap refresh`. This
	// avoids contacting the snap store for snaps that are already in place,
	// for example on a machine image built with them pre-installed.
	skipRefreshIfCurrent bool
	system               system.Worker
}

// Prepare installs a set of snaps on the machine.
func (h *SnapHandler) Prepare() error {
	for _, snap := range h.Snaps {
		done, err := h.PrepareIfCurrent(snap)
		if err != nil {
			return err
		}
		if done {
			continue
		}

		snapInfo, err := h.system.SnapInfo(snap.Name, lookupChannel(snap))
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

// PrepareIfCurrent finishes preparing a snap without refreshing it, if
// skipRefreshIfCurrent is set and the snap is installed and already tracking the
// requested channel: it enables the snap if it is disabled and makes its
// connections. It reports whether it did so; if not, the caller should go on to
// look the snap up with SnapInfo and call PrepareSnap. It asks only the local
// snapd, because SnapInfo queries the snap store (to find the channel's
// confinement), which would defeat the point of skipping.
func (h *SnapHandler) PrepareIfCurrent(s *system.Snap) (bool, error) {
	if !h.skipRefreshIfCurrent || s.Revision != "" {
		return false, nil
	}

	local, err := h.system.SnapInstalledInfo(context.Background(), s.Name)
	if err != nil {
		return false, fmt.Errorf("failed to install snap: failed to lookup installed snap details: %w", err)
	}
	if !local.Installed || !ChannelSatisfied(s.Channel, local.TrackingChannel) {
		return false, nil
	}

	if err := h.enableIfDisabled(s.Name, local); err != nil {
		return false, fmt.Errorf("failed to install snap: %w", err)
	}
	slog.Info("Snap already installed on the requested channel, skipping refresh", "snap", s.Name, "version", local.Version, "revision", local.Revision, "tracking", local.TrackingChannel)

	if err := h.connectSnap(s); err != nil {
		return false, fmt.Errorf("failed to create snap connections: %w", err)
	}
	return true, nil
}

// lookupChannel returns the channel to look a snap up on: the requested channel,
// or the default channel if none was requested. The default is only installed
// from if the snap is not already installed, but the snap store must still be
// asked about a channel to find its confinement.
func lookupChannel(s *system.Snap) string {
	if s.Channel != "" {
		return s.Channel
	}
	return s.DefaultChannel
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
// If already installed, the snap is refreshed.
func (h *SnapHandler) installSnap(s *system.Snap, snapInfo *system.SnapInfo) error {
	slog.Debug("Installing snap", "snap", s.Name)
	var action, logAction string

	// Without an explicit channel, an installed snap is refreshed on the channel
	// it is already tracking: the default might be a different version, and
	// moving to it could be a downgrade or an unsupported upgrade.
	channel := s.Channel

	if snapInfo.Installed {
		// A disabled snap must be enabled before it can be refreshed.
		if err := h.enableIfDisabled(s.Name, &snapInfo.InstalledSnapInfo); err != nil {
			return err
		}
		action = "refresh"
		logAction = "Refreshed"
	} else {
		channel = lookupChannel(s)
		action = "install"
		logAction = "Installed"
	}

	args := []string{action, s.Name}

	if channel != "" {
		args = append(args, "--channel", channel)
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

// enableIfDisabled enables an installed snap that is disabled.
func (h *SnapHandler) enableIfDisabled(name string, info *system.InstalledSnapInfo) error {
	if info.Active {
		return nil
	}
	enableCmd := system.NewCommand("snap", []string{"enable", name})
	if _, err := system.RunExclusive(h.system, enableCmd); err != nil {
		return fmt.Errorf("failed to enable snap %q: %w", name, err)
	}
	slog.Info("Enabled disabled snap", "snap", name, "version", info.Version, "revision", info.Revision, "tracking", info.TrackingChannel)
	return nil
}

// snapRisks are the snap channel risk levels, from most to least stable.
var snapRisks = map[string]bool{"stable": true, "candidate": true, "beta": true, "edge": true}

// normaliseChannel expands a snap channel to the track/risk[/branch] form
// that snapd reports as a snap's tracking channel: "stable" becomes
// "latest/stable", "3" becomes "3/stable", and "edge/fix" becomes
// "latest/edge/fix". An empty channel stays empty.
func normaliseChannel(channel string) string {
	if channel == "" {
		return ""
	}
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
