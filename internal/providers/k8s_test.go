package providers

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

var defaultFeatureConfig = map[string]map[string]string{
	"load-balancer": {
		"l2-mode": "true",
		"cidrs":   "10.43.45.1/32",
	},
	"local-storage": {},
	"network":       {},
}

func TestNewK8s(t *testing.T) {
	type test struct {
		config   *config.Config
		expected *K8s
	}

	noOverrides := &config.Config{}

	channelInConfig := &config.Config{}
	channelInConfig.Providers.K8s.Channel = "1.32/candidate"

	overrides := &config.Config{}
	overrides.Overrides.K8sChannel = "1.32/edge"
	overrides.Providers.K8s.Features = defaultFeatureConfig

	system := system.NewMockSystem()

	tests := []test{
		{
			config:   noOverrides,
			expected: &K8s{Channel: defaultK8sChannel, system: system},
		},
		{
			config:   channelInConfig,
			expected: &K8s{Channel: "1.32/candidate", system: system},
		},
		{
			config:   overrides,
			expected: &K8s{Channel: "1.32/edge", Features: defaultFeatureConfig, system: system},
		},
	}

	for _, tc := range tests {
		ck8s := NewK8s(system, tc.config)

		// Check the constructed snaps are correct
		if ck8s.snaps[0].Channel != tc.expected.Channel {
			t.Fatalf("expected: %v, got: %v", ck8s.snaps[0].Channel, tc.expected.Channel)
		}

		// Remove the snaps so the rest of the object can be compared
		ck8s.snaps = nil
		ck8s.debs = nil
		if !reflect.DeepEqual(tc.expected, ck8s) {
			t.Fatalf("expected: %v, got: %v", tc.expected, ck8s)
		}
	}
}

func TestK8sPrepareCommands(t *testing.T) {
	config := &config.Config{}
	config.Providers.K8s.Channel = ""
	config.Providers.K8s.Features = defaultFeatureConfig

	expectedCommands := []string{
		"which iptables",
		"apt-get update",
		"apt-get install -y iptables",
		fmt.Sprintf("snap install k8s --channel %s", defaultK8sChannel),
		"snap install kubectl --channel stable",
		"systemctl is-active containerd.service",
		"k8s bootstrap",
		"k8s status --wait-ready --timeout 270s",
		"k8s set load-balancer.l2-mode=true",
		"k8s status",
		"k8s set load-balancer.cidrs=10.43.45.1/32",
		"k8s enable load-balancer",
		"k8s enable local-storage",
		"k8s enable network",
		"k8s kubectl config view --raw",
	}

	expectedFiles := map[string]string{
		".kube/config": "",
	}

	system := system.NewMockSystem()
	system.MockCommandReturn("k8s status", []byte("Error: The node is not part of a Kubernetes cluster."), fmt.Errorf("command error"))
	system.MockCommandReturn("which iptables", []byte(""), fmt.Errorf("command error"))

	ck8s := NewK8s(system, config)
	ck8s.Prepare()

	slices.Sort(expectedCommands)
	slices.Sort(system.ExecutedCommands)

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}

	if !reflect.DeepEqual(expectedFiles, system.CreatedFiles) {
		t.Fatalf("expected: %v, got: %v", expectedFiles, system.CreatedFiles)
	}
}

func TestK8sPrepareCommandsAlreadyBootstrappedIptablesInstalled(t *testing.T) {
	config := &config.Config{}
	config.Providers.K8s.Channel = ""
	config.Providers.K8s.Features = defaultFeatureConfig

	expectedCommands := []string{
		fmt.Sprintf("snap install k8s --channel %s", defaultK8sChannel),
		"snap install kubectl --channel stable",
		"which iptables",
		"systemctl is-active containerd.service",
		"k8s status",
		"k8s status --wait-ready --timeout 270s",
		"k8s set load-balancer.l2-mode=true",
		"k8s set load-balancer.cidrs=10.43.45.1/32",
		"k8s enable load-balancer",
		"k8s enable local-storage",
		"k8s enable network",
		"k8s kubectl config view --raw",
	}

	expectedFiles := map[string]string{
		".kube/config": "",
	}

	system := system.NewMockSystem()
	system.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)
	ck8s := NewK8s(system, config)
	ck8s.Prepare()

	slices.Sort(expectedCommands)
	slices.Sort(system.ExecutedCommands)

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}

	if !reflect.DeepEqual(expectedFiles, system.CreatedFiles) {
		t.Fatalf("expected: %v, got: %v", expectedFiles, system.CreatedFiles)
	}
}

func TestK8sRestore(t *testing.T) {
	config := &config.Config{}
	config.Providers.K8s.Channel = ""
	config.Providers.K8s.Features = defaultFeatureConfig

	system := system.NewMockSystem()
	// Mock that containerd service does not exist (typical case after k8s-only install)
	system.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("0 unit files listed."), nil)

	ck8s := NewK8s(system, config)
	ck8s.Restore()

	expectedRemovedPaths := []string{path.Join(os.TempDir(), ".kube")}

	if !slices.Equal(expectedRemovedPaths, system.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, system.RemovedPaths)
	}

	expectedCommands := []string{
		"snap remove k8s --purge",
		"snap remove kubectl --purge",
		"systemctl list-unit-files containerd.service",
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestK8sRestoreWithContainerdService(t *testing.T) {
	config := &config.Config{}
	config.Providers.K8s.Channel = ""
	config.Providers.K8s.Features = defaultFeatureConfig

	system := system.NewMockSystem()
	// Mock that containerd service exists on the system
	system.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("containerd.service enabled"), nil)
	system.MockCommandReturn("systemctl start containerd.service", []byte(""), nil)

	ck8s := NewK8s(system, config)
	ck8s.Restore()

	expectedRemovedPaths := []string{path.Join(os.TempDir(), ".kube")}

	if !slices.Equal(expectedRemovedPaths, system.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, system.RemovedPaths)
	}

	expectedCommands := []string{
		"snap remove k8s --purge",
		"snap remove kubectl --purge",
		"systemctl list-unit-files containerd.service",
		"systemctl start containerd.service",
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

// TestRestoreContainerdServiceExists tests that containerd service is started during restore
func TestRestoreContainerdServiceExists(t *testing.T) {
	config := &config.Config{}
	system := system.NewMockSystem()

	// Mock that containerd service exists
	system.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("containerd.service enabled"), nil)
	system.MockCommandReturn("systemctl start containerd.service", []byte(""), nil)

	ck8s := NewK8s(system, config)
	ck8s.restoreContainerd()

	expectedCommands := []string{
		"systemctl list-unit-files containerd.service",
		"systemctl start containerd.service",
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

// TestRestoreContainerdServiceNotExists tests that we skip restoration when service doesn't exist
func TestRestoreContainerdServiceNotExists(t *testing.T) {
	config := &config.Config{}
	system := system.NewMockSystem()

	// Mock that containerd service does not exist
	system.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("0 unit files listed."), nil)

	ck8s := NewK8s(system, config)
	ck8s.restoreContainerd()

	expectedCommands := []string{
		"systemctl list-unit-files containerd.service",
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestK8sImageRegistryConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistry.URL = "https://mirror.example.com"

	sys := system.NewMockSystem()
	ck8s := NewK8s(sys, cfg)

	// Check that ImageRegistry was set correctly
	if ck8s.ImageRegistry.URL != "https://mirror.example.com" {
		t.Fatalf("expected ImageRegistry URL to be 'https://mirror.example.com', got: %v", ck8s.ImageRegistry.URL)
	}
}

func TestK8sPrepareWithImageRegistry(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.Features = map[string]map[string]string{}
	cfg.Providers.K8s.ImageRegistry.URL = "https://mirror.example.com"

	expectedFiles := map[string]string{
		".kube/config": "",
		"/etc/containerd/hosts.d/docker.io/hosts.toml": "server = \"https://mirror.example.com\"\n\n[host.\"https://mirror.example.com\"]\ncapabilities = [\"pull\", \"resolve\"]\n",
	}

	expectedDirs := []string{
		"/etc/containerd/hosts.d/docker.io",
	}

	sys := system.NewMockSystem()
	sys.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)
	ck8s := NewK8s(sys, cfg)
	ck8s.Prepare()

	if !reflect.DeepEqual(expectedFiles, sys.CreatedFiles) {
		t.Fatalf("expected files: %v, got: %v", expectedFiles, sys.CreatedFiles)
	}

	if !slices.Equal(expectedDirs, sys.CreatedDirectories) {
		t.Fatalf("expected directories: %v, got: %v", expectedDirs, sys.CreatedDirectories)
	}
}

func TestK8sBuildHostsToml(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistry.URL = "https://mirror.example.com"

	sys := system.NewMockSystem()
	ck8s := NewK8s(sys, cfg)

	hostsToml := ck8s.buildHostsToml()

	expectedContent := `server = "https://mirror.example.com"

[host."https://mirror.example.com"]
capabilities = ["pull", "resolve"]
`

	if hostsToml != expectedContent {
		t.Fatalf("expected:\n%v\ngot:\n%v", expectedContent, hostsToml)
	}
}

func TestK8sBuildHostsTomlWithAuth(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistry.URL = "https://mirror.example.com"
	cfg.Providers.K8s.ImageRegistry.Username = "testuser"
	cfg.Providers.K8s.ImageRegistry.Password = "testpass"

	sys := system.NewMockSystem()
	ck8s := NewK8s(sys, cfg)

	hostsToml := ck8s.buildHostsToml()

	// Check that the auth header is present (base64 of "testuser:testpass")
	expectedAuth := "dGVzdHVzZXI6dGVzdHBhc3M=" // base64("testuser:testpass")
	if !strings.Contains(hostsToml, expectedAuth) {
		t.Fatalf("expected hosts.toml to contain base64-encoded credentials, got: %v", hostsToml)
	}

	if !strings.Contains(hostsToml, "authorization = \"Basic") {
		t.Fatalf("expected hosts.toml to contain authorization header, got: %v", hostsToml)
	}
}
