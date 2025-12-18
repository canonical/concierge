package providers

import (
	"os"
	"path"
	"reflect"
	"slices"
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
		".kube/config": "",
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
