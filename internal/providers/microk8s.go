package providers

import (
	"fmt"
	"log/slog"
	"net"
	"os"
	"path"
	"strconv"
	"strings"
	"time"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/system"
)

// Default channel from which MicroK8s is installed when the latest strict
// version cannot be determined.
const defaultMicroK8sChannel = "1.32-strict/stable"

// fallbackMetalLBIPRange is the range MetalLB is configured with when the
// addons list contains a bare "metallb" entry, no explicit range is given
// in the config, and auto-detection also fails. It is the example range
// from MicroK8s' own metallb addon prompt, which is where concierge got
// it, and is kept only so behaviour does not change for anyone who was
// relying on the previously hardcoded value.
const fallbackMetalLBIPRange = "10.64.140.43-10.64.140.49"

// routeFlagUp is RTF_UP from the kernel routing table flags.
const routeFlagUp = 0x0001

// interfaceAddrs is stubbed in tests to make MetalLB range auto-detection
// deterministic without touching the host's actual network configuration.
var interfaceAddrs = net.InterfaceAddrs

// primaryInterfaceAddrs returns the addresses of the interface carrying the
// default route, or nil if it cannot be determined. Stubbed in tests.
var primaryInterfaceAddrs = defaultRouteAddrs

// procNetRoute is the kernel routing table, read to find the interface that
// carries the default route. Overridden in tests.
var procNetRoute = "/proc/net/route"

// NewMicroK8s constructs a new MicroK8s provider instance.
func NewMicroK8s(r system.Worker, config *config.Config) *MicroK8s {
	var channel string

	if config.Overrides.MicroK8sChannel != "" {
		channel = config.Overrides.MicroK8sChannel
	} else if config.Providers.MicroK8s.Channel == "" {
		channel = computeDefaultChannel(r)
	} else {
		channel = config.Providers.MicroK8s.Channel
	}

	return &MicroK8s{
		Channel:              channel,
		Addons:               config.Providers.MicroK8s.Addons,
		MetalLBIPRange:       config.Providers.MicroK8s.MetalLBIPRange,
		ImageRegistry:        config.Providers.MicroK8s.ImageRegistry,
		bootstrap:            config.Providers.MicroK8s.Bootstrap,
		modelDefaults:        config.Providers.MicroK8s.ModelDefaults,
		bootstrapConstraints: config.Providers.MicroK8s.BootstrapConstraints,
		system:               r,
		snaps: []*system.Snap{
			{Name: "microk8s", Channel: channel},
			{Name: "kubectl", Channel: "stable"},
		},
	}
}

// MicroK8s represents a MicroK8s install on a given machine.
type MicroK8s struct {
	Channel        string
	Addons         []string
	MetalLBIPRange string
	ImageRegistry  config.ImageRegistryConfig

	bootstrap            bool
	modelDefaults        map[string]string
	bootstrapConstraints map[string]string

	system system.Worker
	snaps  []*system.Snap
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

// Remove uninstalls MicroK8s and kubectl.
func (m *MicroK8s) Restore() error {
	snapHandler := packages.NewSnapHandler(m.system, m.snaps)

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
	snapHandler := packages.NewSnapHandler(m.system, m.snaps)

	err := snapHandler.Prepare()
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
	for _, addon := range m.Addons {
		enableArg := addon

		// A bare "metallb" needs an IP range appended for the addon to be
		// usable; users may pass "metallb:<range>" directly to bypass this.
		if addon == "metallb" {
			enableArg = "metallb:" + m.resolveMetalLBIPRange()
		}

		cmd := system.NewCommand("microk8s", []string{"enable", enableArg})
		_, err := system.RunWithRetries(m.system, cmd, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to enable MicroK8s addon '%s': %w", addon, err)
		}
	}

	return nil
}

// resolveMetalLBIPRange returns the IP range to advertise via MetalLB when
// the addon is enabled without an explicit range. Preference order:
// (1) explicit configuration, (2) auto-detection from the host's primary
// interface, (3) a hardcoded Canonical-internal fallback range.
func (m *MicroK8s) resolveMetalLBIPRange() string {
	if m.MetalLBIPRange != "" {
		slog.Debug("Using configured MetalLB IP range", "range", m.MetalLBIPRange)
		return m.MetalLBIPRange
	}

	if detected, err := detectMetalLBIPRange(); err == nil {
		slog.Info("Using the host's own address as the MetalLB IP range", "range", detected)
		return detected
	} else {
		slog.Warn(
			"Could not auto-detect a MetalLB IP range; falling back to the MicroK8s example range. "+
				"Set providers.microk8s.metallb-ip-range in your concierge.yaml to override.",
			"fallback", fallbackMetalLBIPRange,
			"detection_error", err,
		)
	}

	return fallbackMetalLBIPRange
}

// detectMetalLBIPRange returns the host's own primary IPv4 address as a
// one-address MetalLB pool ("ip-ip").
//
// MetalLB's L2 mode answers ARP for the pool addresses, so they have to be
// on a segment where that answer is believed. Handing it an address the
// host already owns is the only choice that cannot collide with anything
// else on the network, and it is what we tell users to do in the Traefik
// "Gateway Address Unavailable" how-to. Taking a slice of the surrounding
// subnet instead is a guess about what is free, and on the large shared
// subnet of a cloud CI runner it is a bad one.
//
// The cost is that a single address only serves one LoadBalancer service.
// Set metallb-ip-range to hand over a wider range when that is not enough.
func detectMetalLBIPRange() (string, error) {
	addrs, err := interfaceAddrs()
	if err != nil {
		return "", fmt.Errorf("failed to list host interface addresses: %w", err)
	}

	// Prefer the interface carrying the default route. Without this the
	// choice is whatever net.InterfaceAddrs happens to return first, which
	// on a host with several bridges (a CI runner, say) is arbitrary.
	if primary, err := primaryInterfaceAddrs(); err == nil && len(primary) > 0 {
		addrs = append(primary, addrs...)
	}

	for _, addr := range addrs {
		ipNet, ok := addr.(*net.IPNet)
		if !ok {
			continue
		}
		ip4 := ipNet.IP.To4()
		if ip4 == nil {
			continue
		}
		if ip4.IsLoopback() || ip4.IsLinkLocalUnicast() || ip4.IsUnspecified() {
			continue
		}
		return fmt.Sprintf("%s-%s", ip4, ip4), nil
	}

	return "", fmt.Errorf("no suitable IPv4 interface found for MetalLB auto-detection")
}

func decIP(ip net.IP) net.IP {
	out := make(net.IP, 4)
	copy(out, ip.To4())
	for i := 3; i >= 0; i-- {
		if out[i] > 0 {
			out[i]--
			return out
		}
		out[i] = 0xff
	}
	return out
}

// bytesLE reports whether a <= b as unsigned 4-byte integers.
func bytesLE(a, b net.IP) bool {
	a4 := a.To4()
	b4 := b.To4()
	for i := 0; i < 4; i++ {
		if a4[i] < b4[i] {
			return true
		}
		if a4[i] > b4[i] {
			return false
		}
	}
	return true
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

// defaultRouteAddrs returns the addresses of the interface that carries the
// default route. This is what "the primary interface" means on a host with
// more than one candidate: the one packets leave by.
func defaultRouteAddrs() ([]net.Addr, error) {
	name, err := defaultRouteInterface()
	if err != nil {
		return nil, err
	}
	iface, err := net.InterfaceByName(name)
	if err != nil {
		return nil, fmt.Errorf("failed to look up interface %q: %w", name, err)
	}
	// A tunnel is a common default route (a VPN, a tailnet) and is exactly
	// the wrong answer here: MetalLB advertises over L2, so it needs a
	// broadcast segment. Leave those to the general scan.
	if iface.Flags&net.FlagPointToPoint != 0 || iface.Flags&net.FlagBroadcast == 0 {
		return nil, fmt.Errorf("default route interface %q is not a broadcast segment", name)
	}
	return iface.Addrs()
}

// defaultRouteInterface returns the name of the interface carrying the IPv4
// default route, read from the kernel routing table.
func defaultRouteInterface() (string, error) {
	contents, err := os.ReadFile(procNetRoute)
	if err != nil {
		return "", fmt.Errorf("failed to read %s: %w", procNetRoute, err)
	}
	// Columns are Iface, Destination, Gateway, Flags, ... The default route
	// is the entry whose destination is 0.0.0.0, written as eight zeroes.
	for line := range strings.SplitSeq(strings.TrimSpace(string(contents)), "\n") {
		fields := strings.Fields(line)
		if len(fields) < 4 || fields[0] == "Iface" {
			continue
		}
		if fields[1] != "00000000" {
			continue
		}
		flags, err := strconv.ParseUint(fields[3], 16, 32)
		if err != nil || flags&routeFlagUp == 0 {
			continue
		}
		return fields[0], nil
	}
	return "", fmt.Errorf("no default route found in %s", procNetRoute)
}
