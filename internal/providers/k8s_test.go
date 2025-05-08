package providers

import (
	"fmt"
	"reflect"
	"slices"
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
		"k8s bootstrap",
		"k8s status --wait-ready",
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

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
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
		"which iptables",
		fmt.Sprintf("snap install k8s --channel %s", defaultK8sChannel),
		"snap install kubectl --channel stable",
		"k8s status --wait-ready",
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
	system.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)
	ck8s := NewK8s(system, config)
	ck8s.Prepare()

	slices.Sort(expectedCommands)
	slices.Sort(system.ExecutedCommands)

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
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
	ck8s := NewK8s(system, config)
	ck8s.Restore()

	expectedDeleted := []string{".kube"}

	if !reflect.DeepEqual(expectedDeleted, system.Deleted) {
		t.Fatalf("expected: %v, got: %v", expectedDeleted, system.Deleted)
	}

	expectedCommands := []string{
		"snap remove k8s --purge",
		"snap remove kubectl --purge",
	}

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}
