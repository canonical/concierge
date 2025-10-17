package providers

import (
    "fmt"
    "log/slog"
    "time"

    "github.com/canonical/concierge/internal/config"
    "github.com/canonical/concierge/internal/packages"
    "github.com/canonical/concierge/internal/system"
)

// default channel if none specified
const defaultMicroCephChannel = "latest/stable"

// NewMicroCeph constructs a new MicroCeph provider instance.
func NewMicroCeph(r system.Worker, config *config.Config) *MicroCeph {
    channel := config.Providers.MicroCeph.Channel
    if config.Overrides.MicroCephChannel != "" {
        channel = config.Overrides.MicroCephChannel
    }
    if channel == "" {
        channel = defaultMicroCephChannel
    }

    return &MicroCeph{
        Channel: channel,
        system:  r,
        snaps: []*system.Snap{
            {Name: "microceph", Channel: channel},
        },
    }
}

// MicroCeph represents a microceph installation used to provide RADOSGW/S3.
type MicroCeph struct {
    Channel string

    system system.Worker
    snaps  []*system.Snap
}

// Prepare installs and configures microceph and radosgw for S3 access.
func (m *MicroCeph) Prepare() error {
    // Install the snap
    snapHandler := packages.NewSnapHandler(m.system, m.snaps)
    if err := snapHandler.Prepare(); err != nil {
        return fmt.Errorf("failed to install microceph snap: %w", err)
    }

    // Bootstrap the cluster
    cmd := system.NewCommand("microceph", []string{"cluster", "bootstrap"})
    if _, err := m.system.RunWithRetries(cmd, 2*time.Minute); err != nil {
        return fmt.Errorf("failed to bootstrap microceph cluster: %w", err)
    }

    // Add a loop disk (4G, 3 devices) — best-effort
    cmd = system.NewCommand("microceph", []string{"disk", "add", "loop,4G,3"})
    if _, err := m.system.RunWithRetries(cmd, 1*time.Minute); err != nil {
        // Log and continue — disks may already be added or not required
        slog.Warn("microceph: failed to add loop disk, continuing", "error", err)
    }

    // Wait for ceph to settle
    cmd = system.NewCommand("microceph.ceph", []string{"-s"})
    if _, err := m.system.RunWithRetries(cmd, 30*time.Second); err != nil {
        slog.Warn("microceph: ceph status check failed", "error", err)
    }

    // Enable RADOS Gateway on alternate ports to avoid collisions with traefik
    cmd = system.NewCommand("microceph", []string{"enable", "rgw", "--port", "8080", "--ssl-port", "8443"})
    if _, err := m.system.RunWithRetries(cmd, 2*time.Minute); err != nil {
        return fmt.Errorf("failed to enable radosgw: %w", err)
    }

    // Ensure ceph is up after enabling rgw
    cmd = system.NewCommand("microceph.ceph", []string{"-s"})
    if _, err := m.system.RunWithRetries(cmd, 30*time.Second); err != nil {
        slog.Warn("microceph: post-enable ceph status check failed", "error", err)
    }

    // Create a default S3 user
    cmd = system.NewCommand("microceph.radosgw-admin", []string{"user", "create", "--uid=user", "--display-name=User"})
    if _, err := m.system.RunWithRetries(cmd, 30*time.Second); err != nil {
        // It may already exist; log and continue
        slog.Warn("microceph: failed to create radosgw user", "error", err)
    }

    // Create keys for the user. Use fixed keys to allow tests/users to connect easily.
    cmd = system.NewCommand("microceph.radosgw-admin", []string{"key", "create", "--uid=user", "--key-type=s3", "--access-key=access-key", "--secret-key=secret-key"})
    if _, err := m.system.RunWithRetries(cmd, 30*time.Second); err != nil {
        slog.Warn("microceph: failed to create radosgw keys", "error", err)
    }

    // Install s3cmd using apt (best-effort)
    cmd = system.NewCommand("apt", []string{"update"})
    if _, err := m.system.Run(cmd); err != nil {
        slog.Warn("microceph: apt update failed", "error", err)
    }
    cmd = system.NewCommand("apt", []string{"install", "-y", "s3cmd"})
    if _, err := m.system.Run(cmd); err != nil {
        slog.Warn("microceph: failed to install s3cmd", "error", err)
    }

    slog.Info("Prepared provider", "provider", m.Name())
    return nil
}

func (m *MicroCeph) Name() string { return "microceph" }

func (m *MicroCeph) Bootstrap() bool { return false }

func (m *MicroCeph) CloudName() string { return "microceph" }

func (m *MicroCeph) GroupName() string { return "microceph" }

func (m *MicroCeph) Credentials() map[string]interface{} { return nil }

func (m *MicroCeph) ModelDefaults() map[string]string { return nil }

func (m *MicroCeph) BootstrapConstraints() map[string]string { return nil }

// Restore uninstalls microceph snap and removes s3cmd package.
func (m *MicroCeph) Restore() error {
    snapHandler := packages.NewSnapHandler(m.system, m.snaps)
    if err := snapHandler.Restore(); err != nil {
        return err
    }

    // Attempt to remove s3cmd
    cmd := system.NewCommand("apt", []string{"remove", "-y", "s3cmd"})
    if _, err := m.system.Run(cmd); err != nil {
        slog.Warn("microceph: failed to remove s3cmd", "error", err)
    }

    slog.Info("Removed provider", "provider", m.Name())
    return nil
}
