package providers

import (
	"cmp"
	"errors"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/system"
)

// Default channel from which K8s is installed, if it is not already installed.
// This is the latest Canonical Kubernetes LTS release.
const defaultK8sChannel = "1.32-classic/stable"

// k8sBootstrapConfigPath is where concierge writes the `k8s bootstrap --file`
// config when registry credentials are configured. It is under the snap's
// common directory so that strictly confined k8s can read it too, and it is
// removed as soon as bootstrap has run, because it holds the credentials.
const k8sBootstrapConfigPath = "/var/snap/k8s/common/concierge-bootstrap.yaml"

// NewK8s constructs a new K8s provider instance.
func NewK8s(r system.Worker, config *config.Config) *K8s {
	var channel, defaultChannel string

	if config.Overrides.K8sChannel != "" {
		channel = config.Overrides.K8sChannel
	} else {
		channel = config.Providers.K8s.Channel
	}
	// Only set when it will be used, as for MicroK8s.
	if channel == "" {
		defaultChannel = defaultK8sChannel
	}

	return &K8s{
		Channel:              cmp.Or(channel, defaultK8sChannel),
		Features:             config.Providers.K8s.Features,
		ImageRegistry:        config.Providers.K8s.ImageRegistry,
		ImageRegistries:      config.Providers.K8s.ImageRegistries,
		bootstrap:            config.Providers.K8s.Bootstrap,
		controllerName:       config.Providers.K8s.ControllerName,
		modelDefaults:        config.Providers.K8s.ModelDefaults,
		bootstrapConstraints: config.Providers.K8s.BootstrapConstraints,
		skipSnapRefresh:      config.Overrides.SkipSnapRefresh,
		system:               r,
		debs: []*packages.Deb{
			{Name: "iptables"},
		},
		snaps: []*system.Snap{
			{Name: "k8s", Channel: channel, DefaultChannel: defaultChannel},
			{Name: "kubectl", Channel: "stable"},
		},
	}
}

// K8s represents a K8s install on a given machine.
type K8s struct {
	// Channel is the channel requested in the config or overrides, otherwise
	// defaultK8sChannel. An existing install that wasn't given a channel
	// stays on the one it's tracking, whatever this says.
	Channel       string
	Features      map[string]map[string]string
	ImageRegistry config.ImageRegistryConfig
	// ImageRegistries holds the image-registries entries. Mirrors are written
	// to containerd's hosts directory before bootstrap; credentials are passed
	// to `k8s bootstrap` as extra containerd config.
	ImageRegistries []config.RegistryConfig

	bootstrap            bool
	controllerName       string
	modelDefaults        map[string]string
	bootstrapConstraints map[string]string

	system system.Worker
	debs   []*packages.Deb
	snaps  []*system.Snap

	skipSnapRefresh bool
}

// Prepare installs and configures K8s such that it can work in testing environments.
// This includes installing the snap, enabling the user who ran concierge to interact
// with K8s without sudo, and sets up the user's kubeconfig file.
func (k *K8s) Prepare() error {
	err := k.install()
	if err != nil {
		return fmt.Errorf("failed to install K8s: %w", err)
	}

	err = k.deconflictFirewall()
	if err != nil {
		return fmt.Errorf("failed to deconflict firewall for K8s: %w", err)
	}

	// Configure image registry before bootstrapping K8s
	err = k.configureImageRegistry()
	if err != nil {
		return fmt.Errorf("failed to configure image registry: %w", err)
	}

	err = k.configureImageRegistries()
	if err != nil {
		return fmt.Errorf("failed to configure image registries: %w", err)
	}

	err = k.init()
	if err != nil {
		return fmt.Errorf("failed to install K8s: %w", err)
	}

	err = k.configureFeatures()
	if err != nil {
		return fmt.Errorf("failed to enable K8s features: %w", err)
	}

	err = k.setupKubectl()
	if err != nil {
		return fmt.Errorf("failed to setup kubectl for K8s: %w", err)
	}

	slog.Info("Prepared provider", "provider", k.Name())

	return nil
}

// deconflictFirewall ensures that K8s pods can reach the network through the
// host. Docker sets the iptables FORWARD chain's default policy to DROP, which
// blocks pod traffic (including DNS lookups from CoreDNS) leaving the host.
// Only the policy is changed: unlike the LXD provider, the chain is not
// flushed, so any rules K8s or Docker have added are kept.
func (k *K8s) deconflictFirewall() error {
	_, err := k.system.Run(system.NewCommand("iptables", []string{"-P", "FORWARD", "ACCEPT"}))
	return err
}

// Name reports the name of the provider for Concierge's purposes.
func (k *K8s) Name() string { return "k8s" }

// Bootstrap reports whether a Juju controller should be bootstrapped onto the provider.
func (k *K8s) Bootstrap() bool { return k.bootstrap }

// CloudName reports the name of the provider as Juju sees it.
func (k *K8s) CloudName() string { return "k8s" }

// GroupName reports the name of the POSIX group with permission to use K8s.
func (k *K8s) GroupName() string { return "" }

// Credentials reports the section of Juju's credentials.yaml for the provider
func (m K8s) Credentials() map[string]any { return nil }

// ModelDefaults reports the Juju model-defaults specific to the provider.
func (m *K8s) ModelDefaults() map[string]string { return m.modelDefaults }

// BootstrapConstraints reports the Juju bootstrap-constraints specific to the provider.
func (m *K8s) BootstrapConstraints() map[string]string { return m.bootstrapConstraints }

// ControllerName reports the name of the Juju controller bootstrapped on the provider.
func (m *K8s) ControllerName() string { return controllerName(m.controllerName, m.Name()) }

// Remove uninstalls K8s and kubectl.
func (k *K8s) Restore() error {
	snapHandler := packages.NewSnapHandler(k.system, k.snaps, false)

	err := snapHandler.Restore()
	if err != nil {
		return err
	}

	err = k.system.RemovePath(path.Join(k.system.User().HomeDir, ".kube"))
	if err != nil {
		return fmt.Errorf("failed to remove '.kube' from user's home directory: %w", err)
	}

	k.restoreImageRegistry()

	k.restoreImageRegistries()

	k.restoreContainerd()

	slog.Info("Removed provider", "provider", k.Name())

	return nil
}

// restoreImageRegistry removes the containerd hosts.d configuration that
// configureImageRegistry wrote. The k8s snap uses host paths under
// /etc/containerd/hosts.d/ that are not cleared when the snap is removed, so
// credentials embedded in hosts.toml would otherwise persist on disk after
// `concierge restore`. Failures are logged as warnings — restore has already
// done the heavy lifting and we should not fail it over a stray file.
func (k *K8s) restoreImageRegistry() {
	if k.ImageRegistry.URL == "" {
		return
	}

	hostsDir := "/etc/containerd/hosts.d/docker.io"
	slog.Debug("Removing image registry configuration", "path", hostsDir)
	if err := k.system.RemovePath(hostsDir); err != nil {
		slog.Warn("Failed to remove image registry configuration", "path", hostsDir, "error", err)
	}
}

// install ensures that K8s is installed.
func (k *K8s) install() error {
	var eg errgroup.Group

	// Prepare/restore package handlers concurrently
	debHandler := packages.NewDebHandler(k.system, k.debs)
	snapHandler := packages.NewSnapHandler(k.system, k.snaps, k.skipSnapRefresh)

	eg.Go(func() error {
		// In some cases, iptables is not present on the system. In those cases,
		// make sure it's installed.
		cmd := system.NewCommand("which", []string{"iptables"})
		cmd.ReadOnly = true
		cmd.ExpectedError = `.*`
		_, err := k.system.Run(cmd)
		if err != nil {
			err := debHandler.Prepare()
			if err != nil {
				return err
			}
		}
		return nil
	})

	eg.Go(func() error {
		err := snapHandler.Prepare()
		if err != nil {
			return err
		}
		return nil
	})

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// init ensures that K8s is installed, minimally configured, and ready.
func (k *K8s) init() error {
	if k.needsBootstrap() {
		k.handleExistingContainerd()
		err := k.bootstrapCluster()
		if err != nil {
			return err
		}
	} else if hasRegistryAuth(k.ImageRegistries) {
		// k8s only takes extra containerd config at bootstrap (or join), so
		// credentials can't be changed on a cluster that already exists.
		slog.Warn("K8s is already bootstrapped; image-registries credentials are only applied at bootstrap, so any changes since then are not applied")
	}

	cmd := system.NewCommand("k8s", []string{"status", "--wait-ready", "--timeout", "270s"})
	_, err := system.RunWithRetries(k.system, cmd, 5*time.Minute)

	return err
}

// bootstrapCluster runs `k8s bootstrap`, passing any image-registries
// credentials in a bootstrap config file. A file replaces k8s's default
// bootstrap config, so it also enables the features a plain bootstrap does.
func (k *K8s) bootstrapCluster() error {
	args := []string{"bootstrap"}

	if hasRegistryAuth(k.ImageRegistries) {
		contents, err := buildK8sBootstrapConfig(k.ImageRegistries)
		if err != nil {
			return fmt.Errorf("failed to build bootstrap config: %w", err)
		}
		err = k.system.WriteFile(k8sBootstrapConfigPath, contents, 0600)
		if err != nil {
			return fmt.Errorf("failed to write bootstrap config: %w", err)
		}
		defer k.removeBootstrapConfig()
		args = append(args, "--file", k8sBootstrapConfigPath)
	}

	cmd := system.NewCommand("k8s", args)
	_, err := system.RunWithRetries(k.system, cmd, 5*time.Minute)
	return err
}

// removeBootstrapConfig removes the bootstrap config file, which holds
// registry credentials.
func (k *K8s) removeBootstrapConfig() {
	if err := k.system.RemovePath(k8sBootstrapConfigPath); err != nil {
		slog.Warn("Failed to remove bootstrap config", "path", k8sBootstrapConfigPath, "error", err)
	}
}

// buildK8sBootstrapConfig generates a `k8s bootstrap --file` config that
// enables the same features as a plain `k8s bootstrap` and adds containerd
// registry credentials.
func buildK8sBootstrapConfig(registries []config.RegistryConfig) ([]byte, error) {
	enabled := map[string]any{"enabled": true}
	bootstrapConfig := map[string]any{
		"cluster-config": map[string]any{
			"network":       enabled,
			"dns":           enabled,
			"gateway":       enabled,
			"local-storage": enabled,
		},
		"extra-node-containerd-config": map[string]any{
			"plugins": map[string]any{
				"io.containerd.grpc.v1.cri": map[string]any{
					"registry": map[string]any{
						"configs": registryAuthConfigs(registries),
					},
				},
			},
		},
	}
	return yaml.Marshal(bootstrapConfig)
}

// configureImageRegistries validates the image-registries entries and writes a
// containerd hosts.toml for each entry with a mirror. Credentials are applied
// at bootstrap; see bootstrapCluster.
func (k *K8s) configureImageRegistries() error {
	if len(k.ImageRegistries) == 0 {
		return nil
	}

	if err := validateRegistries(k.ImageRegistries, k.ImageRegistry); err != nil {
		return err
	}

	if err := writeRegistryMirrors(k.system, "/etc/containerd/hosts.d", k.ImageRegistries); err != nil {
		return err
	}

	return nil
}

// restoreImageRegistries removes the image-registries mirror directories, and
// the bootstrap config in case an interrupted run left it behind. The k8s
// snap's remove hook deletes the credentials in containerd's config.toml.
func (k *K8s) restoreImageRegistries() {
	removeRegistryMirrors(k.system, "/etc/containerd/hosts.d", k.ImageRegistries)

	if hasRegistryAuth(k.ImageRegistries) {
		k.removeBootstrapConfig()
	}
}

// configureFeatures iterates over the specified features, enabling and configuring them.
func (k *K8s) configureFeatures() error {
	for featureName, conf := range k.Features {
		for key, value := range conf {
			featureConfig := fmt.Sprintf("%s.%s=%s", featureName, key, value)

			cmd := system.NewCommand("k8s", []string{"set", featureConfig})
			_, err := k.system.Run(cmd)
			if err != nil {
				return fmt.Errorf("failed to set K8s feature config '%s': %w", featureConfig, err)
			}
		}

		cmd := system.NewCommand("k8s", []string{"enable", featureName})
		_, err := system.RunWithRetries(k.system, cmd, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to enable K8s addon '%s': %w", featureName, err)
		}
	}

	return nil
}

// setupKubectl both installs the kubectl snap, and writes the relevant kubeconfig
// file to the user's home directory such that kubectl works with K8s.
func (k *K8s) setupKubectl() error {
	cmd := system.NewCommand("k8s", []string{"kubectl", "config", "view", "--raw"})
	result, err := k.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to fetch K8s configuration: %w", err)
	}

	return system.WriteHomeDirFile(k.system, path.Join(".kube", "config"), result)
}

func (k *K8s) needsBootstrap() bool {
	cmd := system.NewCommand("k8s", []string{"status"})
	cmd.ReadOnly = true
	cmd.ExpectedError = `not part of a Kubernetes cluster`
	output, err := k.system.Run(cmd)

	if err != nil {
		// If k8s is not installed, it needs bootstrapping.
		if errors.Is(err, system.ErrNotInstalled) {
			return true
		}
		if strings.Contains(string(output), "Error: The node is not part of a Kubernetes cluster.") {
			return true
		}
	}

	return false
}

// handleExistingContainerd checks for and handles pre-existing containerd installations
// that would conflict with the k8s snap's bootstrap process. It stops the containerd
// service (if running) and removes the directory to allow k8s to bootstrap successfully.
func (k *K8s) handleExistingContainerd() {
	cmd := system.NewCommand("systemctl", []string{"is-active", "containerd.service"})
	cmd.ReadOnly = true
	cmd.ExpectedError = `inactive|unknown`
	output, err := k.system.Run(cmd)

	if err == nil && strings.TrimSpace(string(output)) == "active" {
		slog.Debug("Containerd service is active, stopping it")
		stopCmd := system.NewCommand("systemctl", []string{"stop", "containerd.service"})
		_, err := k.system.Run(stopCmd)
		if err != nil {
			slog.Warn("Failed to stop containerd service", "error", err)
		} else {
			slog.Debug("Successfully stopped containerd service")
		}
	} else {
		slog.Debug("Containerd service is not active or does not exist")
	}

	slog.Debug("Removing /run/containerd directory")
	err = k.system.RemovePath("/run/containerd")
	if err != nil {
		slog.Warn("Failed to remove /run/containerd directory", "error", err)
	} else {
		slog.Debug("Successfully removed /run/containerd directory")
	}
}

// restoreContainerd attempts to restore the containerd service that may have been
// stopped during k8s preparation. This checks if containerd.service exists on the
// system and starts it if present, which will create /run/containerd if needed.
func (k *K8s) restoreContainerd() {
	cmd := system.NewCommand("systemctl", []string{"list-unit-files", "containerd.service"})
	cmd.ReadOnly = true
	cmd.ExpectedError = `.*`
	output, err := k.system.Run(cmd)

	if err != nil || !strings.Contains(string(output), "containerd.service") {
		slog.Debug("Containerd service does not exist on system, skipping restore")
		return
	}

	slog.Debug("Containerd service exists, attempting to start it")
	startCmd := system.NewCommand("systemctl", []string{"start", "containerd.service"})
	_, err = k.system.Run(startCmd)
	if err != nil {
		slog.Warn("Failed to start containerd service", "error", err)
		return
	}
	slog.Debug("Successfully started containerd service")
}

// configureImageRegistry configures an image registry mirror for K8s.
// This allows using alternative registries like internal mirrors for docker.io.
func (k *K8s) configureImageRegistry() error {
	if k.ImageRegistry.URL == "" {
		return nil
	}

	slog.Info("Configuring image registry", "url", k.ImageRegistry.URL)

	// Create the hosts.d directory for docker.io registry configuration
	// The k8s snap uses containerd with hosts.d configuration at /etc/containerd/hosts.d/
	hostsDir := "/etc/containerd/hosts.d/docker.io"
	err := k.system.MkdirAll(hostsDir, 0755)
	if err != nil {
		return fmt.Errorf("failed to create hosts directory: %w", err)
	}

	// Build the hosts.toml content and write it to the file
	hostsConfig := k.buildHostsToml()
	hostsPath := path.Join(hostsDir, "hosts.toml")

	err = k.system.WriteFile(hostsPath, []byte(hostsConfig), 0600)
	if err != nil {
		return fmt.Errorf("failed to write hosts.toml: %w", err)
	}

	return nil
}

// buildHostsToml generates the hosts.toml configuration for containerd using
// the K8s provider's image registry configuration.
func (k *K8s) buildHostsToml() string {
	return buildHostsTomlFromConfig(k.ImageRegistry)
}
