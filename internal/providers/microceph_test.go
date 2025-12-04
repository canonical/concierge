package providers

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestNewMicroCeph(t *testing.T) {
	type test struct {
		config   *config.Config
		expected *MicroCeph
	}

	noOverrides := &config.Config{}

	channelInConfig := &config.Config{}
	channelInConfig.Providers.MicroCeph.Channel = "latest/candidate"

	overrides := &config.Config{}
	overrides.Overrides.MicroCephChannel = "latest/edge"

	system := system.NewMockSystem()

	tests := []test{
		{
			config:   noOverrides,
			expected: &MicroCeph{Channel: defaultMicroCephChannel, system: system},
		},
		{
			config:   channelInConfig,
			expected: &MicroCeph{Channel: "latest/candidate", system: system},
		},
		{
			config:   overrides,
			expected: &MicroCeph{Channel: "latest/edge", system: system},
		},
	}

	for _, tc := range tests {
		mceph := NewMicroCeph(system, tc.config)

		// Check the constructed snaps are correct
		if mceph.snaps[0].Channel != tc.expected.Channel {
			t.Fatalf("expected: %v, got: %v", mceph.snaps[0].Channel, tc.expected.Channel)
		}

		// Remove the snaps so the rest of the object can be compared
		mceph.snaps = nil
		if !reflect.DeepEqual(tc.expected, mceph) {
			t.Fatalf("expected: %v, got: %v", tc.expected, mceph)
		}
	}
}

func TestMicroCephPrepareCommands(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroCeph.Channel = "latest/stable"

	expectedCommands := []string{
		"snap install microceph --channel latest/stable",
		"microceph cluster bootstrap",
		"microceph disk add loop,4G,3",
		"microceph.ceph -s",
		"microceph enable rgw --port 8080 --ssl-port 8443",
		"microceph.ceph -s",
		"microceph.radosgw-admin user create --uid=user --display-name=User",
		"microceph.radosgw-admin key create --uid=user --key-type=s3 --access-key=access-key --secret-key=secret-key",
		"apt update",
		"apt install -y s3cmd",
	}

	system := system.NewMockSystem()
	mceph := NewMicroCeph(system, config)
	mceph.Prepare()

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected commands:\n%v\ngot:\n%v", expectedCommands, system.ExecutedCommands)
	}
}

func TestMicroCephRestore(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroCeph.Channel = "latest/stable"

	system := system.NewMockSystem()
	mceph := NewMicroCeph(system, config)
	mceph.Restore()

	expectedCommands := []string{
		"snap remove microceph --purge",
		"apt remove -y s3cmd",
	}

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected commands:\n%v\ngot:\n%v", expectedCommands, system.ExecutedCommands)
	}
}