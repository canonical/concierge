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
			func(d *DebHandler) { d.Prepare() },
			[]string{
				"apt-get update",
				"apt-get install -y cowsay",
				"apt-get install -y python3-venv",
			},
		},
		{
			func(d *DebHandler) { d.Restore() },
			[]string{
				"apt-get remove -y cowsay",
				"apt-get remove -y python3-venv",
				"apt-get autoremove -y",
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
