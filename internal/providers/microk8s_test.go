package providers

import (
	"errors"
	"fmt"
	"net"
	"os"
	"path"
	"path/filepath"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

var defaultAddons []string = []string{
	"hostpath-storage",
	"dns",
	"rbac",
	"metallb:10.64.140.43-10.64.140.49",
}

// stubInterfaceAddrs replaces the interfaceAddrs package var for the
// duration of a test, restoring it via t.Cleanup. It also neutralises
// primaryInterfaceAddrs, so a test that stubs the address list is not
// quietly reading the real host's default route as well; a test that cares
// about the default route stubs it afterwards.
func stubInterfaceAddrs(t *testing.T, addrs []net.Addr, err error) {
	t.Helper()
	prev := interfaceAddrs
	interfaceAddrs = func() ([]net.Addr, error) { return addrs, err }
	t.Cleanup(func() { interfaceAddrs = prev })
	stubPrimaryInterfaceAddrs(t, nil, errors.New("no default route in tests"))
}

// mustIPNet parses a CIDR and fails the test if it does not.
func mustIPNet(t *testing.T, cidr string) *net.IPNet {
	t.Helper()
	_, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("failed to parse CIDR %q: %v", cidr, err)
	}
	return ipNet
}

// hostAddr builds a *net.IPNet whose IP is the host address (not the
// masked network address), matching what net.InterfaceAddrs returns.
func hostAddr(t *testing.T, ip string, cidr string) *net.IPNet {
	t.Helper()
	parsed := net.ParseIP(ip)
	if parsed == nil {
		t.Fatalf("failed to parse IP %q", ip)
	}
	return &net.IPNet{IP: parsed, Mask: mustIPNet(t, cidr).Mask}
}

func TestNewMicroK8s(t *testing.T) {
	type test struct {
		config   *config.Config
		expected *MicroK8s
	}

	noOverrides := &config.Config{}

	channelInConfig := &config.Config{}
	channelInConfig.Providers.MicroK8s.Channel = "1.29-strict/stable"

	overrides := &config.Config{}
	overrides.Overrides.MicroK8sChannel = "1.30/edge"
	overrides.Providers.MicroK8s.Addons = defaultAddons

	system := system.NewMockSystem()

	tests := []test{
		{
			config:   noOverrides,
			expected: &MicroK8s{Channel: defaultMicroK8sChannel, system: system},
		},
		{
			config:   channelInConfig,
			expected: &MicroK8s{Channel: "1.29-strict/stable", system: system},
		},
		{
			config:   overrides,
			expected: &MicroK8s{Channel: "1.30/edge", Addons: defaultAddons, system: system},
		},
	}

	for _, tc := range tests {
		uk8s := NewMicroK8s(system, tc.config)

		// Check the constructed snaps are correct
		if uk8s.snaps[0].Channel != tc.expected.Channel {
			t.Fatalf("expected: %v, got: %v", uk8s.snaps[0].Channel, tc.expected.Channel)
		}

		// Remove the snaps so the rest of the object can be compared
		uk8s.snaps = nil
		if !reflect.DeepEqual(tc.expected, uk8s) {
			t.Fatalf("expected: %v, got: %v", tc.expected, uk8s)
		}
	}
}

func TestMicroK8sGroupName(t *testing.T) {
	type test struct {
		channel  string
		expected string
	}

	tests := []test{
		{channel: "1.30-strict/stable", expected: "snap_microk8s"},
		{channel: "1.30/stable", expected: "microk8s"},
	}

	for _, tc := range tests {
		config := &config.Config{}
		config.Providers.MicroK8s.Channel = tc.channel
		uk8s := NewMicroK8s(system.NewMockSystem(), config)

		if !reflect.DeepEqual(tc.expected, uk8s.GroupName()) {
			t.Fatalf("expected: %v, got: %v", tc.expected, uk8s.GroupName())
		}
	}
}

func TestMicroK8sPrepareCommands(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroK8s.Channel = "1.31-strict/stable"
	config.Providers.MicroK8s.Addons = defaultAddons

	expectedCommands := []string{
		"snap install microk8s --channel 1.31-strict/stable",
		"snap install kubectl --channel stable",
		"microk8s status --wait-ready --timeout 270",
		"microk8s enable hostpath-storage",
		"microk8s enable dns",
		"microk8s enable rbac",
		"microk8s enable metallb:10.64.140.43-10.64.140.49",
		"usermod -a -G snap_microk8s test-user",
		"microk8s config",
	}

	expectedFiles := map[string]string{
		path.Join(os.TempDir(), ".kube", "config"): "",
	}

	system := system.NewMockSystem()
	uk8s := NewMicroK8s(system, config)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}

	if !reflect.DeepEqual(expectedFiles, system.CreatedFiles) {
		t.Fatalf("expected: %v, got: %v", expectedFiles, system.CreatedFiles)
	}
}

func TestMicroK8sRestore(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroK8s.Channel = "1.31-strict/stable"
	config.Providers.MicroK8s.Addons = defaultAddons

	system := system.NewMockSystem()
	uk8s := NewMicroK8s(system, config)
	if err := uk8s.Restore(); err != nil {
		t.Fatal(err)
	}

	expectedRemovedPaths := []string{path.Join(os.TempDir(), ".kube")}

	if !slices.Equal(expectedRemovedPaths, system.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, system.RemovedPaths)
	}

	expectedCommands := []string{
		"snap remove microk8s --purge",
		"snap remove kubectl --purge",
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestMicroK8sImageRegistryConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = defaultAddons
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)

	// Check that ImageRegistry was set correctly
	if uk8s.ImageRegistry.URL != "https://mirror.example.com" {
		t.Fatalf("expected ImageRegistry URL to be 'https://mirror.example.com', got: %v", uk8s.ImageRegistry.URL)
	}
}

func TestMicroK8sPrepareWithImageRegistry(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = defaultAddons
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"

	expectedCommands := []string{
		"snap install microk8s --channel 1.31-strict/stable",
		"snap install kubectl --channel stable",
		"microk8s status --wait-ready --timeout 270",
		"microk8s stop",
		"microk8s start",
		"microk8s status --wait-ready --timeout 270",
		"microk8s enable hostpath-storage",
		"microk8s enable dns",
		"microk8s enable rbac",
		"microk8s enable metallb:10.64.140.43-10.64.140.49",
		"usermod -a -G snap_microk8s test-user",
		"microk8s config",
	}

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	kubeConfigPath := path.Join(sys.User().HomeDir, ".kube", "config")
	kubeDir := path.Join(sys.User().HomeDir, ".kube")
	expectedFiles := map[string]string{
		kubeConfigPath: "",
		"/var/snap/microk8s/current/args/certs.d/docker.io/hosts.toml": "server = \"https://mirror.example.com\"\n\n[host.\"https://mirror.example.com\"]\ncapabilities = [\"pull\", \"resolve\"]\n",
	}

	expectedDirs := []string{
		"/var/snap/microk8s/current/args/certs.d/docker.io",
		kubeDir,
	}

	if !slices.Equal(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected commands: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}

	if !reflect.DeepEqual(expectedFiles, sys.CreatedFiles) {
		t.Fatalf("expected files: %v, got: %v", expectedFiles, sys.CreatedFiles)
	}

	if !slices.Equal(expectedDirs, sys.CreatedDirectories) {
		t.Fatalf("expected directories: %v, got: %v", expectedDirs, sys.CreatedDirectories)
	}
}

func TestMicroK8sPrepareWithImageRegistryAndAuth(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{}
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"
	cfg.Providers.MicroK8s.ImageRegistry.Username = "testuser"
	cfg.Providers.MicroK8s.ImageRegistry.Password = "testpass"

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	hostsToml := sys.CreatedFiles["/var/snap/microk8s/current/args/certs.d/docker.io/hosts.toml"]

	// Check that the auth header is present (base64 of "testuser:testpass")
	expectedAuth := "dGVzdHVzZXI6dGVzdHBhc3M=" // base64("testuser:testpass")
	if !strings.Contains(hostsToml, expectedAuth) {
		t.Fatalf("expected hosts.toml to contain base64-encoded credentials, got: %v", hostsToml)
	}

	if !strings.Contains(hostsToml, "Authorization = [\"Basic") {
		t.Fatalf("expected hosts.toml to contain authorization header, got: %v", hostsToml)
	}
}

// TestMicroK8sBareMetalLBUsesConfiguredRange verifies that a bare "metallb"
// entry in the addons list is expanded using the configured
// metallb-ip-range, in preference to auto-detection or the fallback.
func TestMicroK8sBareMetalLBUsesConfiguredRange(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}
	cfg.Providers.MicroK8s.MetalLBIPRange = "192.168.99.240-192.168.99.245"

	// Stub the detector to a value that would clearly change the command
	// if the configured range were ignored.
	stubInterfaceAddrs(t, []net.Addr{hostAddr(t, "10.0.0.2", "10.0.0.0/24")}, nil)

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	want := "microk8s enable metallb:192.168.99.240-192.168.99.245"
	if !slices.Contains(sys.ExecutedCommands, want) {
		t.Fatalf("expected commands to contain %q, got: %v", want, sys.ExecutedCommands)
	}
}

// TestMicroK8sBareMetalLBAutoDetectsRange verifies that a bare "metallb"
// entry falls back to auto-detection when no explicit range is configured.
func TestMicroK8sBareMetalLBAutoDetectsRange(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	stubInterfaceAddrs(t, []net.Addr{
		&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)},
		hostAddr(t, "192.168.1.42", "192.168.1.0/24"),
	}, nil)

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	// The host's own address, as a one-address pool.
	want := "microk8s enable metallb:192.168.1.42-192.168.1.42"
	if !slices.Contains(sys.ExecutedCommands, want) {
		t.Fatalf("expected commands to contain %q, got: %v", want, sys.ExecutedCommands)
	}
}

// TestMicroK8sBareMetalLBFallsBackWhenDetectionFails covers the last-resort
// path: bare "metallb", no config, and detection returns nothing usable.
func TestMicroK8sBareMetalLBFallsBackWhenDetectionFails(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	stubInterfaceAddrs(t, nil, fmt.Errorf("mock: no interfaces"))

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	want := "microk8s enable metallb:" + fallbackMetalLBIPRange
	if !slices.Contains(sys.ExecutedCommands, want) {
		t.Fatalf("expected commands to contain %q, got: %v", want, sys.ExecutedCommands)
	}
}

// TestDetectMetalLBIPRange exercises the range-derivation heuristic across
// a few subnet shapes and interface layouts.
func TestDetectMetalLBIPRange(t *testing.T) {
	tests := []struct {
		name    string
		addrs   []net.Addr
		want    string
		wantErr bool
	}{
		{
			name: "skips loopback and uses the host's own address",
			addrs: []net.Addr{
				&net.IPNet{IP: net.IPv4(127, 0, 0, 1), Mask: net.CIDRMask(8, 32)},
				hostAddr(t, "10.0.0.5", "10.0.0.0/24"),
			},
			want: "10.0.0.5-10.0.0.5",
		},
		{
			name: "a tiny subnet is still fine, the pool is one address",
			addrs: []net.Addr{
				hostAddr(t, "10.0.0.1", "10.0.0.0/30"),
			},
			want: "10.0.0.1-10.0.0.1",
		},
		{
			name: "skips link-local",
			addrs: []net.Addr{
				hostAddr(t, "169.254.3.4", "169.254.0.0/16"),
				hostAddr(t, "192.168.7.20", "192.168.7.0/24"),
			},
			want: "192.168.7.20-192.168.7.20",
		},
		{
			name:    "no interfaces yields an error",
			addrs:   nil,
			wantErr: true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			stubInterfaceAddrs(t, tc.addrs, nil)
			got, err := detectMetalLBIPRange()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("expected error, got range %q", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tc.want {
				t.Fatalf("expected %q, got %q", tc.want, got)
			}
		})
	}
}

func TestMicroK8sBuildHostsToml(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)

	hostsToml := uk8s.buildHostsToml()

	expectedContent := `server = "https://mirror.example.com"

[host."https://mirror.example.com"]
capabilities = ["pull", "resolve"]
`

	if hostsToml != expectedContent {
		t.Fatalf("expected:\n%v\ngot:\n%v", expectedContent, hostsToml)
	}
}

// stubPrimaryInterfaceAddrs replaces the primaryInterfaceAddrs package var
// for the duration of a test.
func stubPrimaryInterfaceAddrs(t *testing.T, addrs []net.Addr, err error) {
	t.Helper()
	prev := primaryInterfaceAddrs
	primaryInterfaceAddrs = func() ([]net.Addr, error) { return addrs, err }
	t.Cleanup(func() { primaryInterfaceAddrs = prev })
}

func mustCIDR(t *testing.T, cidr string) net.Addr {
	t.Helper()
	ip, ipNet, err := net.ParseCIDR(cidr)
	if err != nil {
		t.Fatalf("bad test CIDR %q: %v", cidr, err)
	}
	ipNet.IP = ip
	return ipNet
}

// The default-route interface wins even when it is not first in the list
// that net.InterfaceAddrs returns. Without this, which bridge is picked on a
// multi-interface host is down to enumeration order.
func TestDetectMetalLBIPRangePrefersDefaultRoute(t *testing.T) {
	bridges := []net.Addr{
		mustCIDR(t, "10.205.224.1/24"),
		mustCIDR(t, "10.153.100.1/24"),
	}
	primary := []net.Addr{mustCIDR(t, "192.168.132.147/24")}
	stubInterfaceAddrs(t, bridges, nil)
	stubPrimaryInterfaceAddrs(t, primary, nil)

	got, err := detectMetalLBIPRange()
	if err != nil {
		t.Fatalf("detectMetalLBIPRange() error: %v", err)
	}
	if want := "192.168.132.147-192.168.132.147"; got != want {
		t.Errorf("detectMetalLBIPRange() = %q, want %q", got, want)
	}
}

// When the default route can't be determined we fall back to scanning every
// interface, which is the behaviour this had before.
func TestDetectMetalLBIPRangeFallsBackWithoutDefaultRoute(t *testing.T) {
	stubInterfaceAddrs(t, []net.Addr{mustCIDR(t, "10.205.224.1/24")}, nil)
	stubPrimaryInterfaceAddrs(t, nil, errors.New("no default route"))

	got, err := detectMetalLBIPRange()
	if err != nil {
		t.Fatalf("detectMetalLBIPRange() error: %v", err)
	}
	if want := "10.205.224.1-10.205.224.1"; got != want {
		t.Errorf("detectMetalLBIPRange() = %q, want %q", got, want)
	}
}

func TestDefaultRouteInterface(t *testing.T) {
	for name, tc := range map[string]struct {
		contents string
		want     string
		wantErr  bool
	}{
		"default route present": {
			contents: "Iface\tDestination\tGateway\tFlags\tRefCnt\tUse\tMetric\tMask\n" +
				"enp5s0\t00000000\t0102A8C0\t0003\t0\t0\t100\t00000000\n" +
				"lxdbr0\t00E0A80A\t00000000\t0001\t0\t0\t0\t00FFFFFF\n",
			want: "enp5s0",
		},
		"default route not up is skipped": {
			contents: "Iface\tDestination\tGateway\tFlags\n" +
				"enp5s0\t00000000\t0102A8C0\t0002\t\n",
			wantErr: true,
		},
		"no default route": {
			contents: "Iface\tDestination\tGateway\tFlags\n" +
				"lxdbr0\t00E0A80A\t00000000\t0001\t\n",
			wantErr: true,
		},
	} {
		t.Run(name, func(t *testing.T) {
			path := filepath.Join(t.TempDir(), "route")
			if err := os.WriteFile(path, []byte(tc.contents), 0o644); err != nil {
				t.Fatal(err)
			}
			prev := procNetRoute
			procNetRoute = path
			t.Cleanup(func() { procNetRoute = prev })

			got, err := defaultRouteInterface()
			if tc.wantErr {
				if err == nil {
					t.Fatalf("defaultRouteInterface() = %q, want an error", got)
				}
				return
			}
			if err != nil {
				t.Fatalf("defaultRouteInterface() error: %v", err)
			}
			if got != tc.want {
				t.Errorf("defaultRouteInterface() = %q, want %q", got, tc.want)
			}
		})
	}
}
