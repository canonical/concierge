package providers

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"os"
	"path"
	"strings"
	"time"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/system"
)

// defaultMicroK8sChannel is the fallback for when the latest strict channel
// cannot be determined: computeDefaultChannel asks snapd first and reaches
// this only when there is no snapd to ask. On a host with snapd it decides
// nothing, so what matters here is the floor, not the exact version.
//
// Do not move it below 1.34. On a kernel with AppArmor network_v9
// mediation, strict channels up to and including 1.33 cannot load their
// containerd profile ("apparmor_parser: Unable to replace
// cri-containerd.apparmor.d. Profile doesn't conform to protocol"),
// daemon-containerd gives up after five restarts, and everything behind it
// crashloops. Measured on 26.04 with kernel 7.0.0-30: active on 1.34 and
// 1.36, not on 1.31, 1.32 or 1.33. On 24.04 with 6.8.0-138, which has
// network_v8 and no v9, 1.31 is healthy -- so the trigger is the mediation
// generation, not the MicroK8s version alone. Upstream:
// canonical/microk8s#5394, still open.
//
// 1.36 rather than 1.34 is newest-available at the time of writing, not a
// tested floor; 1.34 and 1.35 were not tried beyond the check above.
const defaultMicroK8sChannel = "1.36-strict/stable"

// microk8sContainerdTemplate is the template MicroK8s renders containerd's
// config from each time containerd starts.
const microk8sContainerdTemplate = "/var/snap/microk8s/current/args/containerd-template.toml"

// Markers around the block concierge appends to the containerd template, so
// that re-running prepare replaces it rather than adding another, and restore
// can remove it.
const (
	microk8sRegistriesBegin = "# BEGIN concierge image-registries (managed by concierge, do not edit)"
	microk8sRegistriesEnd   = "# END concierge image-registries"
)

// NewMicroK8s constructs a new MicroK8s provider instance.
func NewMicroK8s(r system.Worker, config *config.Config) *MicroK8s {
	var channel, defaultChannel string

	if config.Overrides.MicroK8sChannel != "" {
		channel = config.Overrides.MicroK8sChannel
	} else if config.Providers.MicroK8s.Channel == "" {
		defaultChannel = computeDefaultChannel(r)
	} else {
		channel = config.Providers.MicroK8s.Channel
	}

	return &MicroK8s{
		Channel:              cmp.Or(channel, defaultChannel),
		Addons:               config.Providers.MicroK8s.Addons,
		ImageRegistry:        config.Providers.MicroK8s.ImageRegistry,
		ImageRegistries:      config.Providers.MicroK8s.ImageRegistries,
		bootstrap:            config.Providers.MicroK8s.Bootstrap,
		controllerName:       config.Providers.MicroK8s.ControllerName,
		modelDefaults:        config.Providers.MicroK8s.ModelDefaults,
		bootstrapConstraints: config.Providers.MicroK8s.BootstrapConstraints,
		skipSnapRefresh:      config.Overrides.SkipSnapRefresh,
		system:               r,
		snaps: []*system.Snap{
			{Name: "microk8s", Channel: channel, DefaultChannel: defaultChannel},
			{Name: "kubectl", Channel: "stable"},
		},
	}
}

// MicroK8s represents a MicroK8s install on a given machine.
type MicroK8s struct {
	// Channel is the channel MicroK8s is installed from: the one requested in
	// the config or overrides, otherwise the channel an existing install is
	// tracking, otherwise the latest strict stable channel.
	Channel       string
	Addons        []string
	ImageRegistry config.ImageRegistryConfig
	// ImageRegistries holds the image-registries entries. Mirrors are written
	// to containerd's certs.d directory; credentials are added to the
	// containerd template.
	ImageRegistries []config.RegistryConfig

	bootstrap            bool
	controllerName       string
	modelDefaults        map[string]string
	bootstrapConstraints map[string]string

	system system.Worker
	snaps  []*system.Snap

	skipSnapRefresh bool
}

// Prepare installs and configures MicroK8s such that it can work in testing environments.
// This includes installing the snap, enabling the user who ran concierge to interact
// with MicroK8s without sudo, and sets up the user's kubeconfig file.
func (m *MicroK8s) Prepare() error {
	err := m.install()
	if err != nil {
		return fmt.Errorf("failed to install MicroK8s: %w", err)
	}

	// Wait for MicroK8s to be ready before configuring the image registry:
	// `microk8s stop` fails with "service-control change in progress" if
	// snapd is still bringing the snap's services up after install.
	err = m.init()
	if err != nil {
		return fmt.Errorf("failed to configure MicroK8s: %w", err)
	}

	err = m.configureImageRegistry()
	if err != nil {
		return fmt.Errorf("failed to configure image registry: %w", err)
	}

	err = m.configureImageRegistries()
	if err != nil {
		return fmt.Errorf("failed to configure image registries: %w", err)
	}

	err = m.enableAddons()
	if err != nil {
		return fmt.Errorf("failed to enable MicroK8s addons: %w", err)
	}

	err = m.enableNonRootUserControl()
	if err != nil {
		return fmt.Errorf("failed to enable non-root MicroK8s access: %w", err)
	}

	err = m.setupKubectl()
	if err != nil {
		return fmt.Errorf("failed to setup kubectl for MicroK8s: %w", err)
	}

	slog.Info("Prepared provider", "provider", m.Name())

	return nil
}

// Name reports the name of the provider for Concierge's purposes.
func (m *MicroK8s) Name() string { return "microk8s" }

// Bootstrap reports whether a Juju controller should be bootstrapped onto the provider.
func (m *MicroK8s) Bootstrap() bool { return m.bootstrap }

// CloudName reports the name of the provider as Juju sees it.
func (m *MicroK8s) CloudName() string { return "microk8s" }

// GroupName reports the name of the POSIX group with permission to use MicroK8s.
func (m *MicroK8s) GroupName() string {
	if strings.Contains(m.Channel, "strict") {
		return "snap_microk8s"
	} else {
		return "microk8s"
	}
}

// Credentials reports the section of Juju's credentials.yaml for the provider
func (m MicroK8s) Credentials() map[string]any { return nil }

// ModelDefaults reports the Juju model-defaults specific to the provider.
func (m *MicroK8s) ModelDefaults() map[string]string { return m.modelDefaults }

// BootstrapConstraints reports the Juju bootstrap-constraints specific to the provider.
func (m *MicroK8s) BootstrapConstraints() map[string]string { return m.bootstrapConstraints }

// ControllerName reports the name of the Juju controller bootstrapped on the provider.
func (m *MicroK8s) ControllerName() string { return controllerName(m.controllerName, m.Name()) }

// Remove uninstalls MicroK8s and kubectl.
func (m *MicroK8s) Restore() error {
	// Removing the snap with --purge also removes these files, but undo them
	// explicitly first so credentials don't outlive a failed snap removal.
	m.restoreImageRegistries()

	snapHandler := packages.NewSnapHandler(m.system, m.snaps, false)

	err := snapHandler.Restore()
	if err != nil {
		return err
	}

	err = m.system.RemovePath(path.Join(m.system.User().HomeDir, ".kube"))
	if err != nil {
		return fmt.Errorf("failed to remove '.kube' from user's home directory: %w", err)
	}

	slog.Info("Removed provider", "provider", m.Name())

	return nil
}

// install ensures that MicroK8s is installed.
func (m *MicroK8s) install() error {
	// GroupName depends on the channel, so it must reflect an existing install.
	channel, err := installedChannel(m.system, m.snaps[0])
	if err != nil {
		return err
	}
	if channel != "" {
		m.Channel = channel
	}

	snapHandler := packages.NewSnapHandler(m.system, m.snaps, m.skipSnapRefresh)

	err = snapHandler.Prepare()
	if err != nil {
		return err
	}

	return nil
}

// configureImageRegistry configures an image registry mirror for MicroK8s.
// This allows using alternative registries like internal mirrors for docker.io.
func (m *MicroK8s) configureImageRegistry() error {
	if m.ImageRegistry.URL == "" {
		return nil
	}

	slog.Info("Configuring image registry", "url", m.ImageRegistry.URL)

	// Create the certs.d directory for docker.io registry configuration
	certsDir := "/var/snap/microk8s/current/args/certs.d/docker.io"
	err := m.system.MkdirAll(certsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create certs directory: %w", err)
	}

	// Build the hosts.toml content and write it to the file
	hostsConfig := m.buildHostsToml()
	hostsPath := path.Join(certsDir, "hosts.toml")

	err = m.system.WriteFile(hostsPath, []byte(hostsConfig), 0600)
	if err != nil {
		return fmt.Errorf("failed to write hosts.toml: %w", err)
	}

	// Restart MicroK8s to apply the registry configuration
	stopCmd := system.NewCommand("microk8s", []string{"stop"})
	_, err = m.system.Run(stopCmd)
	if err != nil {
		return fmt.Errorf("failed to stop MicroK8s: %w", err)
	}

	startCmd := system.NewCommand("microk8s", []string{"start"})
	_, err = m.system.Run(startCmd)
	if err != nil {
		return fmt.Errorf("failed to start MicroK8s: %w", err)
	}

	// Wait for services to come back up before downstream steps run
	// commands that assume a ready cluster.
	return m.init()
}

// configureImageRegistries writes a hosts.toml for each image-registries mirror
// and adds the credentials to the containerd template, restarting containerd
// if the template changed, because containerd only reads it at startup.
func (m *MicroK8s) configureImageRegistries() error {
	if len(m.ImageRegistries) == 0 {
		return nil
	}

	if err := validateRegistries(m.ImageRegistries, m.ImageRegistry); err != nil {
		return err
	}

	if err := writeRegistryMirrors(m.system, "/var/snap/microk8s/current/args/certs.d", m.ImageRegistries); err != nil {
		return err
	}

	if !hasRegistryAuth(m.ImageRegistries) {
		return nil
	}

	slog.Info("Configuring image registry credentials")

	template, err := m.system.ReadFile(microk8sContainerdTemplate)
	if err != nil {
		return fmt.Errorf("failed to read containerd template: %w", err)
	}

	base, err := stripMicroK8sRegistries(string(template))
	if err != nil {
		return err
	}
	updated := appendMicroK8sRegistries(base, buildRegistryAuthToml(m.ImageRegistries))
	if updated == string(template) {
		slog.Debug("Containerd template already has the image registry credentials")
		return nil
	}

	if err := m.system.WriteFile(microk8sContainerdTemplate, []byte(updated), 0600); err != nil {
		return fmt.Errorf("failed to write containerd template: %w", err)
	}

	restartCmd := system.NewCommand("systemctl", []string{"restart", "snap.microk8s.daemon-containerd.service"})
	if _, err := m.system.Run(restartCmd); err != nil {
		return fmt.Errorf("failed to restart containerd: %w", err)
	}

	return m.init()
}

// restoreImageRegistries removes the hosts.toml directories written for
// image-registries mirrors, and the credentials block from the containerd
// template. Failures are logged as warnings rather than failing restore.
func (m *MicroK8s) restoreImageRegistries() {
	if len(m.ImageRegistries) == 0 {
		return
	}

	removeRegistryMirrors(m.system, "/var/snap/microk8s/current/args/certs.d", m.ImageRegistries)

	template, err := m.system.ReadFile(microk8sContainerdTemplate)
	if err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			slog.Warn("Failed to read containerd template", "path", microk8sContainerdTemplate, "error", err)
		}
		return
	}

	stripped, err := stripMicroK8sRegistries(string(template))
	if err != nil {
		slog.Warn("Failed to remove image registry credentials from containerd template", "error", err)
		return
	}
	if stripped == string(template) {
		return
	}

	slog.Debug("Removing image registry credentials", "path", microk8sContainerdTemplate)
	if err := m.system.WriteFile(microk8sContainerdTemplate, []byte(stripped), 0600); err != nil {
		slog.Warn("Failed to remove image registry credentials from containerd template", "error", err)
	}
}

// appendMicroK8sRegistries appends a marked block holding auth to template.
func appendMicroK8sRegistries(template, auth string) string {
	if template != "" {
		if !strings.HasSuffix(template, "\n") {
			template += "\n"
		}
		template += "\n"
	}
	return template + microk8sRegistriesBegin + "\n" + auth + microk8sRegistriesEnd + "\n"
}

// stripMicroK8sRegistries removes the block that appendMicroK8sRegistries
// added, if there is one.
func stripMicroK8sRegistries(template string) (string, error) {
	begin := strings.Index(template, microk8sRegistriesBegin)
	if begin < 0 {
		return template, nil
	}
	length := strings.Index(template[begin:], microk8sRegistriesEnd)
	if length < 0 {
		return "", fmt.Errorf("containerd template %s has a %q line with no matching %q line", microk8sContainerdTemplate, microk8sRegistriesBegin, microk8sRegistriesEnd)
	}
	end := begin + length + len(microk8sRegistriesEnd)
	if strings.HasPrefix(template[end:], "\n") {
		end++
	}
	// Also remove the blank line that appendMicroK8sRegistries added before the block.
	if strings.HasSuffix(template[:begin], "\n\n") {
		begin--
	}
	return template[:begin] + template[end:], nil
}

// buildHostsToml generates the hosts.toml configuration for containerd using
// the MicroK8s provider's image registry configuration.
func (m *MicroK8s) buildHostsToml() string {
	return buildHostsTomlFromConfig(m.ImageRegistry)
}

// init waits for MicroK8s to be ready (via `microk8s status --wait-ready`).
// Named for parity with the other providers' init() methods, even though
// MicroK8s has nothing to do here beyond waiting; callers may invoke it more
// than once to re-synchronise after operations like stop/start.
func (m *MicroK8s) init() error {
	cmd := system.NewCommand("microk8s", []string{"status", "--wait-ready", "--timeout", "270"})
	_, err := system.RunWithRetries(m.system, cmd, 5*time.Minute)

	return err
}

// enableAddons iterates over the specified addons, enabling and configuring them.
func (m *MicroK8s) enableAddons() error {
	for _, addon := range m.Addons {
		enableArg := addon

		// If the addon is MetalLB, add the predefined IP range
		if addon == "metallb" {
			enableArg = "metallb:10.64.140.43-10.64.140.49"
		}

		cmd := system.NewCommand("microk8s", []string{"enable", enableArg})
		_, err := system.RunWithRetries(m.system, cmd, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to enable MicroK8s addon '%s': %w", addon, err)
		}
	}

	return nil
}

// enableNonRootUserControl ensures the current user is in the correct POSIX group
// that allows them to interact with MicroK8s.
func (m *MicroK8s) enableNonRootUserControl() error {
	username := m.system.User().Username

	cmd := system.NewCommand("usermod", []string{"-a", "-G", m.GroupName(), username})

	_, err := m.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to add user '%s' to group 'microk8s': %w", username, err)
	}

	return nil
}

// setupKubectl both installs the kubectl snap, and writes the relevant kubeconfig
// file to the user's home directory such that kubectl works with MicroK8s.
func (m *MicroK8s) setupKubectl() error {
	cmd := system.NewCommand("microk8s", []string{"config"})
	result, err := m.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to fetch MicroK8s configuration: %w", err)
	}

	return system.WriteHomeDirFile(m.system, path.Join(".kube", "config"), result)
}

// Try to compute the "correct" default channel. Concierge prefers that the 'strict'
// variants are installed, so we filter available channels and sort descending by
// version. If the list cannot be retrieved, default to a know good version.
func computeDefaultChannel(s system.Worker) string {
	channels, err := s.SnapChannels("microk8s")
	if err != nil {
		return defaultMicroK8sChannel
	}

	for _, c := range channels {
		if strings.Contains(c, "strict") && strings.Contains(c, "stable") {
			return c
		}
	}

	return defaultMicroK8sChannel
}
