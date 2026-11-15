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
			func(s *SnapHandler) { _ = s.Prepare() },
			[]string{
				"snap refresh charmcraft --channel latest/stable --classic",
				"snap install jq --channel latest/stable",
				"snap install microk8s --channel 1.30-strict/stable",
				"snap install jhack --channel latest/edge",
				"snap connect jhack:dot-local-share-juju",
			},
		},
		{
			func(s *SnapHandler) { _ = s.Restore() },
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

func TestSnapHandlerWithRevision(t *testing.T) {
	type test struct {
		snap     *system.Snap
		expected string
	}

	tests := []test{
		{
			snap:     &system.Snap{Name: "juju", Channel: "3.6/stable", Revision: "30000"},
			expected: "snap install juju --channel 3.6/stable --revision 30000",
		},
		{
			snap:     &system.Snap{Name: "juju", Revision: "30000"},
			expected: "snap install juju --revision 30000",
		},
	}

	for _, tc := range tests {
		r := system.NewMockSystem()

		if err := NewSnapHandler(r, []*system.Snap{tc.snap}).Prepare(); err != nil {
			t.Fatal(err.Error())
		}

		if !reflect.DeepEqual([]string{tc.expected}, r.ExecutedCommands) {
			t.Fatalf("expected: %v, got: %v", []string{tc.expected}, r.ExecutedCommands)
		}
	}
}

func TestSnapHandlerDefaultChannel(t *testing.T) {
	type test struct {
		name      string
		snap      *system.Snap
		installed bool
		expected  string
	}

	tests := []test{
		{
			name:     "fresh install uses the default channel",
			snap:     &system.Snap{Name: "k8s", DefaultChannel: "1.32-classic/stable"},
			expected: "snap install k8s --channel 1.32-classic/stable --classic",
		},
		{
			name:      "installed snap stays on its tracking channel",
			snap:      &system.Snap{Name: "k8s", DefaultChannel: "1.32-classic/stable"},
			installed: true,
			expected:  "snap refresh k8s --classic",
		},
		{
			name:     "explicit channel wins over the default on install",
			snap:     &system.Snap{Name: "k8s", Channel: "1.34-classic/stable", DefaultChannel: "1.32-classic/stable"},
			expected: "snap install k8s --channel 1.34-classic/stable --classic",
		},
		{
			name:      "explicit channel wins over the default on refresh",
			snap:      &system.Snap{Name: "k8s", Channel: "1.34-classic/stable", DefaultChannel: "1.32-classic/stable"},
			installed: true,
			expected:  "snap refresh k8s --channel 1.34-classic/stable --classic",
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := system.NewMockSystem()
			r.MockSnapStoreLookup(tc.snap.Name, "1.33-classic/stable", true, tc.installed)

			if err := NewSnapHandler(r, []*system.Snap{tc.snap}).Prepare(); err != nil {
				t.Fatal(err.Error())
			}

			if !reflect.DeepEqual([]string{tc.expected}, r.ExecutedCommands) {
				t.Fatalf("expected: %v, got: %v", []string{tc.expected}, r.ExecutedCommands)
			}
		})
	}
}
