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

func TestSnapHandlerSkipRefreshIfCurrent(t *testing.T) {
	type test struct {
		name     string
		tracking string // channel the installed snap is tracking; "" means not installed
		disabled bool
		snap     *system.Snap
		skip     bool
		expected []string
		store    bool // whether the snap store should be queried
	}

	tests := []test{
		{
			name:     "same channel is skipped",
			tracking: "3/stable",
			snap:     &system.Snap{Name: "juju", Channel: "3/stable"},
			skip:     true,
			expected: nil,
		},
		{
			name:     "equivalent channel spelling is skipped",
			tracking: "latest/stable",
			snap:     &system.Snap{Name: "jq", Channel: "stable"},
			skip:     true,
			expected: nil,
		},
		{
			name:     "no channel requested is skipped",
			tracking: "4.0/stable",
			snap:     &system.Snap{Name: "juju"},
			skip:     true,
			expected: nil,
		},
		{
			name:     "different channel is refreshed",
			tracking: "3/stable",
			snap:     &system.Snap{Name: "juju", Channel: "4.0/stable"},
			skip:     true,
			expected: []string{"snap refresh juju --channel 4.0/stable"},
			store:    true,
		},
		{
			name:     "pinned revision is refreshed",
			tracking: "3.6/stable",
			snap:     &system.Snap{Name: "juju", Channel: "3.6/stable", Revision: "30000"},
			skip:     true,
			expected: []string{"snap refresh juju --channel 3.6/stable --revision 30000"},
			store:    true,
		},
		{
			name:     "not installed is installed",
			snap:     &system.Snap{Name: "juju", Channel: "3/stable"},
			skip:     true,
			expected: []string{"snap install juju --channel 3/stable"},
			store:    true,
		},
		{
			name:     "disabled snap is enabled but not refreshed",
			tracking: "3/stable",
			disabled: true,
			snap:     &system.Snap{Name: "juju", Channel: "3/stable"},
			skip:     true,
			expected: []string{"snap enable juju"},
		},
		{
			name:     "same channel is refreshed without the option",
			tracking: "3/stable",
			snap:     &system.Snap{Name: "juju", Channel: "3/stable"},
			skip:     false,
			expected: []string{"snap refresh juju --channel 3/stable"},
			store:    true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			r := system.NewMockSystem()
			if tc.tracking != "" {
				r.MockSnapStoreLookup(tc.snap.Name, tc.tracking, false, true)
				if tc.disabled {
					info, _ := r.SnapInfo(tc.snap.Name, tc.tracking)
					info.Active = false
				}
			}
			r.StoreLookups = nil // ignore lookups made while setting up

			h := NewSnapHandler(r, []*system.Snap{tc.snap})
			h.SkipRefreshIfCurrent = tc.skip
			if err := h.Prepare(); err != nil {
				t.Fatal(err.Error())
			}

			if !reflect.DeepEqual(tc.expected, r.ExecutedCommands) {
				t.Fatalf("expected: %v, got: %v", tc.expected, r.ExecutedCommands)
			}
			if queried := len(r.StoreLookups) > 0; queried != tc.store {
				t.Fatalf("snap store queried: %v, want %v (lookups: %v)", queried, tc.store, r.StoreLookups)
			}
		})
	}
}

func TestChannelSatisfied(t *testing.T) {
	tests := []struct {
		wanted, tracking string
		want             bool
	}{
		{"3/stable", "3/stable", true},
		{"3", "3/stable", true},
		{"stable", "latest/stable", true},
		{"latest/stable", "latest/stable", true},
		{"edge", "latest/edge", true},
		{"edge/fix", "latest/edge/fix", true},
		{"1.32-classic/stable", "1.32-classic/stable", true},
		{"", "4.0/stable", true},
		{"", "", true},
		{"3/stable", "3/edge", false},
		{"3/stable", "4.0/stable", false},
		{"stable", "3/stable", false},
		{"3/stable", "", false},
	}

	for _, tc := range tests {
		if got := channelSatisfied(tc.wanted, tc.tracking); got != tc.want {
			t.Errorf("channelSatisfied(%q, %q) = %v, want %v", tc.wanted, tc.tracking, got, tc.want)
		}
	}
}
