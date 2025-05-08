package packages

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/system"
)

func TestSnapHandlerCommands(t *testing.T) {
	type test struct {
		testFunc func(s *SnapHandler)
		expected []string
	}

	tests := []test{
		{
			func(s *SnapHandler) { s.Prepare() },
			[]string{
				"snap refresh charmcraft --channel latest/stable --classic",
				"snap install jq --channel latest/stable",
				"snap install microk8s --channel 1.30-strict/stable",
				"snap install jhack --channel latest/edge",
				"snap connect jhack:dot-local-share-juju",
			},
		},
		{
			func(s *SnapHandler) { s.Restore() },
			[]string{
				"snap remove charmcraft --purge",
				"snap remove jq --purge",
				"snap remove microk8s --purge",
				"snap remove jhack --purge",
			},
		},
	}

	for _, tc := range tests {
		r := system.NewMockSystem()
		r.MockSnapStoreLookup("charmcraft", "latest/stable", true, true)

		snaps := []*system.Snap{
			system.NewSnap("charmcraft", "latest/stable", []string{}),
			system.NewSnap("jq", "latest/stable", []string{}),
			system.NewSnapFromString("microk8s/1.30-strict/stable"),
			system.NewSnap("jhack", "latest/edge", []string{"jhack:dot-local-share-juju"}),
		}

		tc.testFunc(NewSnapHandler(r, snaps))

		if !reflect.DeepEqual(tc.expected, r.ExecutedCommands) {
			t.Fatalf("expected: %v, got: %v", tc.expected, r.ExecutedCommands)
		}
	}

}
