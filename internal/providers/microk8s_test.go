package providers

import (
	"os"
	"path"
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
	uk8s.Prepare()

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
	uk8s.Restore()

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
	uk8s.Prepare()

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
	uk8s.Prepare()

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
