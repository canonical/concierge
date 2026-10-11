package providers

import (
	"errors"
	"fmt"
	"log/slog"
	"net"
	"os"
	"path"
	"slices"
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
// addons list contains a bare "metallb" entry and no range is given in the
// config. It is the example range from MicroK8s' own metallb addon prompt,
// which is where concierge got it, and it is the default because it is a
// range no one is otherwise using: MetalLB hands the addresses out to
// Services, so they have to be addresses nothing else answers on.
const fallbackMetalLBIPRange = "10.64.140.43-10.64.140.49"

// metalLBIPRangeAuto is the metallb-ip-range value that asks for the host's
// own address instead of a fixed range. Opt-in: see detectMetalLBIPRange
// for why it is not the default.
const metalLBIPRangeAuto = "auto"

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
// the addon is enabled without an explicit range: the configured range, the
// host's own address if the configuration asks for "auto", and otherwise
// the example range MicroK8s itself suggests.
func (m *MicroK8s) resolveMetalLBIPRange() string {
	if m.MetalLBIPRange == metalLBIPRangeAuto {
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
			return fallbackMetalLBIPRange
		}
	}

	if m.MetalLBIPRange != "" {
		slog.Debug("Using configured MetalLB IP range", "range", m.MetalLBIPRange)
		return m.MetalLBIPRange
	}

	return fallbackMetalLBIPRange
}

// detectMetalLBIPRange returns the host's own primary IPv4 address as a
// one-address MetalLB pool ("ip-ip").
//
// MetalLB's L2 mode answers ARP for the pool addresses, so they have to be
// on a segment where that answer is believed, and on the large shared
// subnet of a cloud CI runner a slice of the surrounding subnet is a guess
// about what is free. The host's own address is the one address that is
// certainly reachable, which is why this is offered at all.
//
// It is opt-in rather than the default because the address is not free: it
// is the host's. MetalLB gives it to a Service, and the cluster's own
// datapath then answers on it, so a LoadBalancer on a port the host also
// serves takes that port over on the host's address - measured on microk8s
// 1.35, where a Service on :8080 shadowed a process still listening on
// 0.0.0.0:8080, with only 127.0.0.1 still reaching the host. A single
// address also serves only one LoadBalancer Service.
func detectMetalLBIPRange() (string, error) {
	// Prefer the interface carrying the default route. Without this the
	// choice is whatever net.InterfaceAddrs happens to return first, which
	// on a host with several bridges (a CI runner, say) is arbitrary. It
	// looks the interface up itself, so it can still answer when the
	// general scan can't -- hence trying it first, and treating the scan's
	// failure as fatal only if this came back with nothing.
	primary, primaryErr := primaryInterfaceAddrs()
	candidates := slices.Clone(primary)

	addrs, err := interfaceAddrs()
	if err != nil {
		if len(candidates) == 0 {
			return "", fmt.Errorf(
				"failed to list host interface addresses: %w",
				errors.Join(err, primaryErr),
			)
		}
		slog.Debug(
			"Could not list every host interface address; using the default-route interface alone",
			"error", err,
		)
	}
	candidates = append(candidates, addrs...)

	for _, addr := range candidates {
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
//
// Issue #251 suggests `ip -4 -j route get 2.2.2.2 | jq -r '.[] | .prefsrc'`
// instead, which asks the kernel the question directly and gets back the
// source address it would actually use. Reading /proc/net/route keeps this
// to a file read rather than a shell-out to two tools, at the cost of two
// cases it gets wrong:
//
//   - A host whose default route lives in a table other than main -- an
//     `ip rule` setup, or a multi-homed runner -- has no 00000000 row here
//     at all, so detection falls through to the general interface scan.
//   - Where the chosen interface carries more than one IPv4 address, the
//     order iface.Addrs() returns them in decides which one is used;
//     prefsrc would name one.
//
// Both are acceptable while this path is opt-in and its consumer is a
// single-homed CI runner. If either starts to matter, `ip route get` is
// the answer rather than more parsing.
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
