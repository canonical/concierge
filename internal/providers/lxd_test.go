package providers

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestNewLXD(t *testing.T) {
	type test struct {
		config   *config.Config
		expected *LXD
	}

	noOverrides := &config.Config{}

	channelInConfig := &config.Config{}
	channelInConfig.Providers.LXD.Channel = "latest/edge"

	overrides := &config.Config{}
	overrides.Overrides.LXDChannel = "5.20/stable"

	system := system.NewMockSystem()

	tests := []test{
		{config: noOverrides, expected: &LXD{Channel: "", system: system}},
		{config: channelInConfig, expected: &LXD{Channel: "latest/edge", system: system}},
		{config: overrides, expected: &LXD{Channel: "5.20/stable", system: system}},
	}

	for _, tc := range tests {
		lxd := NewLXD(system, tc.config)

		// Check the constructed snaps are correct
		if lxd.snaps[0].Channel != tc.expected.Channel {
			t.Fatalf("expected: %v, got: %v", lxd.snaps[0].Channel, tc.expected.Channel)
		}

		// Remove the snaps so the rest of the object can be compared
		lxd.snaps = nil
		if !reflect.DeepEqual(tc.expected, lxd) {
			t.Fatalf("expected: %v, got: %v", tc.expected, lxd)
		}
	}
}

func TestLXDPrepareCommands(t *testing.T) {
	config := &config.Config{}

	expected := []string{
		"snap install lxd",
		"lxd waitready",
		"lxd init --minimal",
		"lxc network set lxdbr0 ipv6.address none",
		"chmod a+wr /var/snap/lxd/common/lxd/unix.socket",
		"usermod -a -G lxd test-user",
		"iptables -F FORWARD",
		"iptables -P FORWARD ACCEPT",
	}

	system := system.NewMockSystem()
	lxd := NewLXD(system, config)
	lxd.Prepare()

	if !reflect.DeepEqual(expected, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, system.ExecutedCommands)
	}
}

func TestLXDPrepareCommandsLXDAlreadyInstalled(t *testing.T) {
	config := &config.Config{}

	expected := []string{
		"snap stop lxd",
		"snap refresh lxd",
		"snap start lxd",
		"lxd waitready",
		"lxd init --minimal",
		"lxc network set lxdbr0 ipv6.address none",
		"chmod a+wr /var/snap/lxd/common/lxd/unix.socket",
		"usermod -a -G lxd test-user",
		"iptables -F FORWARD",
		"iptables -P FORWARD ACCEPT",
	}

	system := system.NewMockSystem()
	system.MockSnapStoreLookup("lxd", "", false, true)

	lxd := NewLXD(system, config)
	lxd.Prepare()

	if !reflect.DeepEqual(expected, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, system.ExecutedCommands)
	}
}

func TestLXDRestore(t *testing.T) {
	config := &config.Config{}

	system := system.NewMockSystem()
	lxd := NewLXD(system, config)
	lxd.Restore()

	expectedCommands := []string{"snap remove lxd --purge"}

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}
