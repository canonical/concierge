package providers

import (
	"cmp"
	"fmt"
	"log/slog"
	"path"
	"strings"
	"time"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/system"
)

// Default channel from which MicroK8s is installed, if it is not already
// installed and the latest strict version cannot be determined.
const defaultMicroK8sChannel = "1.32-strict/stable"

// fallbackMetalLBIPRange is the range MetalLB is configured with when the
// addons list has a bare "metallb" entry and no range is configured. It is
// the example range from MicroK8s' own metallb addon prompt. Clients on the
// same machine (and pods) reach a LoadBalancer address in it without it
// existing on any real network, which is all CI needs. Reaching one from
// another machine needs a free range on that machine's LAN instead.
const fallbackMetalLBIPRange = "10.64.140.43-10.64.140.49"

// metalLBCheckService is the throwaway LoadBalancer Service created to check
// that MetalLB hands out addresses.
const metalLBCheckService = "concierge-metallb-check"

// metalLBCheckTimeout and metalLBCheckInterval bound how long to wait for
// MetalLB to give the check Service an address. Variables so tests don't wait.
var (
	metalLBCheckTimeout  = 3 * time.Minute
	metalLBCheckInterval = 5 * time.Second
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
		MetalLBIPRange:       cmp.Or(config.Overrides.MetalLBIPRange, config.Providers.MicroK8s.MetalLBIPRange),
		metalLBIPRangeForced: config.Overrides.MetalLBIPRange != "",
		ImageRegistry:        config.Providers.MicroK8s.ImageRegistry,
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
	Channel string
	Addons  []string
	// MetalLBIPRange is the range used for a bare "metallb" addon entry: the
	// override if one was given, otherwise the configured range. Empty means
	// fallbackMetalLBIPRange.
	MetalLBIPRange string
	ImageRegistry  config.ImageRegistryConfig

	// metalLBIPRangeForced is set when the range came from the
	// --metallb-ip-range flag or its environment variable, which also
	// replaces a range given in the addons list ("metallb:<range>"), the
	// way the channel flags replace a configured channel.
	metalLBIPRangeForced bool

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
	metalLB := false

	for _, addon := range m.Addons {
		enableArg := addon

		// MetalLB needs a range to hand out. A bare "metallb" gets the
		// configured one; an override replaces any range given here.
		isMetalLB := addon == "metallb" || strings.HasPrefix(addon, "metallb:")
		if addon == "metallb" || (isMetalLB && m.metalLBIPRangeForced) {
			enableArg = "metallb:" + cmp.Or(m.MetalLBIPRange, fallbackMetalLBIPRange)
		}
		metalLB = metalLB || isMetalLB

		cmd := system.NewCommand("microk8s", []string{"enable", enableArg})
		_, err := system.RunWithRetries(m.system, cmd, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to enable MicroK8s addon '%s': %w", addon, err)
		}
	}

	if !metalLB {
		return nil
	}

	err := m.patchMetalLBRBAC()
	if err != nil {
		return fmt.Errorf("failed to work around the MetalLB RBAC bug: %w", err)
	}

	return m.checkMetalLB()
}

// patchMetalLBRBAC adds the permissions the MicroK8s 1.36 metallb addon's
// ClusterRoles are missing. The addon deploys MetalLB 0.15.3, which watches a
// ConfigurationState CRD, with RBAC generated from an older MetalLB manifest,
// so once the rbac addon is enabled the controller can't sync and never
// assigns addresses, and the speaker crash-loops. It does nothing when the
// CRD isn't there (older addons) or the controller can already use it (RBAC
// off, or a fixed addon).
//
// Remove once https://github.com/canonical/microk8s-core-addons/issues/405
// is fixed in every MicroK8s channel concierge installs.
func (m *MicroK8s) patchMetalLBRBAC() error {
	crd := m.kubectl("get", "crd", "configurationstates.metallb.io", "--ignore-not-found", "-o", "name")
	crd.ReadOnly = true
	output, err := m.system.Run(crd)
	if err != nil || strings.TrimSpace(string(output)) == "" {
		// No CRD (or, in a dry run, no cluster to ask): nothing to patch.
		return nil
	}

	// Ask as the controller rather than reading its ClusterRole: the answer
	// is also "yes" when the rbac addon is off and nothing is enforced.
	canI := m.kubectl("auth", "can-i", "list", "configurationstates.metallb.io",
		"--as=system:serviceaccount:metallb-system:controller", "-n", "metallb-system")
	canI.ReadOnly = true
	canI.ExpectedError = "^no"
	output, err = m.system.Run(canI)
	switch strings.TrimSpace(string(output)) {
	case "yes":
		return nil
	case "no":
	default:
		return fmt.Errorf("failed to check the MetalLB controller's permissions: %w", err)
	}

	slog.Info("Patching the MetalLB ClusterRoles for microk8s-core-addons#405")

	patches := []*system.Command{
		m.kubectl("patch", "clusterrole", "metallb-system:controller", "--type=json", "-p", `[`+
			`{"op":"add","path":"/rules/-","value":{"apiGroups":["metallb.io"],"resources":["configurationstates"],"verbs":["create","delete","get","list","patch","update","watch"]}},`+
			`{"op":"add","path":"/rules/-","value":{"apiGroups":["metallb.io"],"resources":["configurationstates/status"],"verbs":["get","patch","update"]}}`+
			`]`),
		m.kubectl("patch", "clusterrole", "metallb-system:speaker", "--type=json", "-p", `[`+
			`{"op":"add","path":"/rules/-","value":{"apiGroups":["metallb.io"],"resources":["configurationstates","configurationstates/status"],"verbs":["*"]}},`+
			`{"op":"add","path":"/rules/-","value":{"apiGroups":["metallb.io"],"resources":["servicebgpstatuses"],"verbs":["get","list","watch"]}}`+
			`]`),
		m.kubectl("-n", "metallb-system", "rollout", "restart", "deployment/controller"),
		m.kubectl("-n", "metallb-system", "rollout", "restart", "daemonset/speaker"),
		m.kubectl("-n", "metallb-system", "rollout", "status", "deployment/controller", "--timeout=180s"),
		m.kubectl("-n", "metallb-system", "rollout", "status", "daemonset/speaker", "--timeout=180s"),
	}
	for _, cmd := range patches {
		_, err := m.system.Run(cmd)
		if err != nil {
			return err
		}
	}

	return nil
}

// checkMetalLB creates a throwaway LoadBalancer Service and waits for MetalLB
// to give it an address. Checking the address pool exists isn't enough: the
// addon already retries until it does, and a controller that can't run
// leaves the pool in place but never assigns from it.
func (m *MicroK8s) checkMetalLB() (err error) {
	create := m.kubectl("-n", "default", "create", "service", "loadbalancer", metalLBCheckService, "--tcp=80:80")
	_, err = m.system.Run(create)
	if err != nil {
		return fmt.Errorf("failed to create a LoadBalancer Service to check MetalLB: %w", err)
	}
	defer func() {
		del := m.kubectl("-n", "default", "delete", "service", metalLBCheckService, "--ignore-not-found")
		_, delErr := m.system.Run(del)
		if delErr != nil && err == nil {
			err = fmt.Errorf("failed to delete the MetalLB check Service: %w", delErr)
		}
	}()

	// Nothing was created in a dry run, so there is no address to wait for.
	if system.IsDryRun(m.system) {
		return nil
	}

	get := m.kubectl("-n", "default", "get", "service", metalLBCheckService,
		"-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
	get.ReadOnly = true

	deadline := time.Now().Add(metalLBCheckTimeout)
	for {
		output, err := m.system.Run(get)
		if ip := strings.TrimSpace(string(output)); err == nil && ip != "" {
			slog.Info("MetalLB assigned a LoadBalancer address", "address", ip)
			return nil
		}
		if time.Now().After(deadline) {
			break
		}
		time.Sleep(metalLBCheckInterval)
	}

	logs := m.kubectl("-n", "metallb-system", "logs", "deployment/controller", "--tail=20")
	logs.ReadOnly = true
	output, _ := m.system.Run(logs)

	return fmt.Errorf("MetalLB did not assign an address to a LoadBalancer Service within %s; "+
		"controller log:\n%s", metalLBCheckTimeout, strings.TrimSpace(string(output)))
}

// kubectl returns a command running kubectl against MicroK8s. It uses
// "microk8s kubectl" because the user's kubeconfig isn't written until after
// the addons are enabled.
func (m *MicroK8s) kubectl(args ...string) *system.Command {
	return system.NewCommand("microk8s", append([]string{"kubectl"}, args...))
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
