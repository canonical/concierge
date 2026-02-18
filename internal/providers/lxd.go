package providers

import (
	"fmt"
	"log/slog"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/system"
)

// NewLXD constructs a new LXD provider instance.
func NewLXD(r system.Worker, config *config.Config) *LXD {
	var channel string
	if config.Overrides.LXDChannel != "" {
		channel = config.Overrides.LXDChannel
	} else {
		channel = config.Providers.LXD.Channel
	}

	return &LXD{
		Channel:              channel,
		system:               r,
		bootstrap:            config.Providers.LXD.Bootstrap,
		modelDefaults:        config.Providers.LXD.ModelDefaults,
		bootstrapConstraints: config.Providers.LXD.BootstrapConstraints,
		snaps:                []*system.Snap{{Name: "lxd", Channel: channel}},
	}
}

// LXD represents a LXD install on a given machine.
type LXD struct {
	Channel string

	bootstrap            bool
	modelDefaults        map[string]string
	bootstrapConstraints map[string]string

	system system.Worker
	snaps  []*system.Snap
}

// Prepare installs and configures LXD such that it can work in testing environments.
// This includes installing the snap, enabling the user who ran concierge to interact
// with LXD without sudo, and deconflicting the firewall rules with docker.
func (l *LXD) Prepare() error {
	err := l.install()
	if err != nil {
		return fmt.Errorf("failed to install LXD: %w", err)
	}

	err = l.init()
	if err != nil {
		return fmt.Errorf("failed to initialise LXD: %w", err)
	}

	err = l.enableNonRootUserControl()
	if err != nil {
		return fmt.Errorf("failed to enable non-root LXD access: %w", err)
	}

	err = l.deconflictFirewall()
	if err != nil {
		return fmt.Errorf("failed to adjust firewall rules for LXD: %w", err)
	}

	slog.Info("Prepared provider", "provider", l.Name())
	return nil
}

// Name reports the name of the provider for Concierge's purposes.
func (l *LXD) Name() string { return "lxd" }

// Bootstrap reports whether a Juju controller should be bootstrapped on LXD.
func (l *LXD) Bootstrap() bool { return l.bootstrap }

// CloudName reports the name of the provider as Juju sees it.
func (l *LXD) CloudName() string { return "localhost" }

// GroupName reports the name of the POSIX group with permissions over the LXD socket.
func (l *LXD) GroupName() string { return "lxd" }

// Credentials reports the section of Juju's credentials.yaml for the provider
func (l *LXD) Credentials() map[string]any { return nil }

// ModelDefaults reports the Juju model-defaults specific to the provider.
func (l *LXD) ModelDefaults() map[string]string { return l.modelDefaults }

// BootstrapConstraints reports the Juju bootstrap-constraints specific to the provider.
func (l *LXD) BootstrapConstraints() map[string]string { return l.bootstrapConstraints }

// Remove uninstalls LXD.
func (l *LXD) Restore() error {
	snapHandler := packages.NewSnapHandler(l.system, l.snaps)

	err := snapHandler.Restore()
	if err != nil {
		return err
	}

	slog.Info("Restored provider", "provider", l.Name())
	return nil
}

// install ensures that LXD is installed.
func (l *LXD) install() error {
	// Check if LXD is already installed, and stop the snap if it is.
	restart, err := l.workaroundRefresh()
	if err != nil {
		return err
	}

	snapHandler := packages.NewSnapHandler(l.system, l.snaps)

	err = snapHandler.Prepare()
	if err != nil {
		return err
	}

	// If we stopped the LXD snap, make sure we start it again now the refresh
	// has happened.
	if restart {
		args := []string{"start", l.Name()}
		cmd := system.NewCommand("snap", args)
		_, err = system.RunExclusive(l.system, cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// init ensures that LXD is minimally configured, and ready.
func (l *LXD) init() error {
	return system.RunMany(l.system,
		system.NewCommand("lxd", []string{"waitready", "--timeout", "270"}),
		system.NewCommand("lxd", []string{"init", "--minimal"}),
		system.NewCommand("lxc", []string{"network", "set", "lxdbr0", "ipv6.address", "none"}),
	)
}

// enableNonRootUserControl ensures the current user is in the `lxd` group.
func (l *LXD) enableNonRootUserControl() error {
	username := l.system.User().Username

	return system.RunMany(l.system,
		system.NewCommand("chmod", []string{"a+wr", "/var/snap/lxd/common/lxd/unix.socket"}),
		system.NewCommand("usermod", []string{"-a", "-G", "lxd", username}),
	)
}

// deconflictFirewall ensures that LXD containers can talk out to the internet.
// This is to avoid a conflict with the default iptables rules that ship with
// docker on Ubuntu.
func (l *LXD) deconflictFirewall() error {
	return system.RunMany(l.system,
		system.NewCommand("iptables", []string{"-F", "FORWARD"}),
		system.NewCommand("iptables", []string{"-P", "FORWARD", "ACCEPT"}),
	)
}

// workaroundRefresh checks if LXD will be refreshed and stops it first.
// This is a workaround for an issue in the LXD snap sometimes failing
// on refresh because of a missing snap socket file.
func (l *LXD) workaroundRefresh() (bool, error) {
	snapInfo, err := l.system.SnapInfo(l.Name(), l.Channel)
	if err != nil {
		return false, fmt.Errorf("failed to lookup snap details: %w", err)
	}

	// Only stop LXD if it's installed AND needs to be refreshed (channel mismatch).
	if snapInfo.Installed {
		// If no channel is specified, snapd will refresh on the current channel without changing it.
		// If the tracking channel matches the target channel, the refresh won't change channels.
		// In both cases, no stop is needed since the channel isn't changing.
		if l.Channel == "" || snapInfo.TrackingChannel == l.Channel {
			slog.Debug("Skipping LXD stop - no channel change required",
				"tracking", snapInfo.TrackingChannel, "target", l.Channel)
			return false, nil
		}

		// Channel mismatch detected - LXD will be refreshed, so stop it first
		// to work around a snap refresh issue with missing socket files.
		slog.Debug("LXD channel mismatch, stopping for refresh",
			"tracking", snapInfo.TrackingChannel, "target", l.Channel)
		args := []string{"stop", l.Name()}
		cmd := system.NewCommand("snap", args)
		_, err = system.RunExclusive(l.system, cmd)
		if err != nil {
			return false, fmt.Errorf("command failed: %w", err)
		}
		return true, nil
	}

	return false, nil
}
