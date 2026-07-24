package packages

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/system"
)

func TestDebHandlerCommands(t *testing.T) {
	type test struct {
		testFunc func(d *DebHandler)
		expected []string
	}

	tests := []test{
		{
			func(d *DebHandler) { _ = d.Prepare() },
			[]string{
				"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update",
				"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y install -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold cowsay",
				"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y install -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold python3-venv",
			},
		},
		{
			func(d *DebHandler) { _ = d.Restore() },
			[]string{
				"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y remove cowsay",
				"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y remove python3-venv",
				"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y autoremove",
			},
		},
	}

	debs := []*Deb{
		NewDeb("cowsay"),
		NewDeb("python3-venv"),
	}

	for _, tc := range tests {
		system := system.NewMockSystem()
		tc.testFunc(NewDebHandler(system, debs))

		if !reflect.DeepEqual(tc.expected, system.ExecutedCommands) {
			t.Fatalf("expected: %v, got: %v", tc.expected, system.ExecutedCommands)
		}
	}
}
