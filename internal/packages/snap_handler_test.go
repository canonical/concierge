package packages

import (
	"bytes"
	"log/slog"
	"reflect"
	"strings"
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

func TestSnapHandlerLogsSnapDetails(t *testing.T) {
	type test struct {
		name     string
		before   *system.SnapInfo
		after    *system.SnapInfo
		expected []string
	}

	installed := &system.SnapInfo{Installed: true, Active: true, TrackingChannel: "3.6/stable", Revision: "31000", Version: "3.6.10"}

	tests := []test{
		{
			name:   "install",
			before: &system.SnapInfo{},
			after:  installed,
			expected: []string{
				`msg="Installed snap" snap=juju version=3.6.10 revision=31000 tracking=3.6/stable`,
			},
		},
		{
			name:   "enable and refresh",
			before: &system.SnapInfo{Installed: true, Active: false, TrackingChannel: "3.5/stable", Revision: "30000", Version: "3.5.7"},
			after:  installed,
			expected: []string{
				`msg="Enabled disabled snap" snap=juju version=3.5.7 revision=30000 tracking=3.5/stable`,
				`msg="Refreshed snap" snap=juju version=3.6.10 revision=31000 tracking=3.6/stable from.version=3.5.7 from.revision=30000 from.tracking=3.5/stable`,
			},
		},
		{
			name:   "not installed afterwards",
			before: &system.SnapInfo{},
			after:  &system.SnapInfo{},
			expected: []string{
				`msg="Installed snap" snap=juju`,
			},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			logs := captureLogs(t)

			r := system.NewMockSystem()
			r.MockSnapInfo("juju", tc.before)
			r.MockSnapInstalledInfo("juju", tc.after)

			if err := NewSnapHandler(r, []*system.Snap{system.NewSnap("juju", "3.6/stable", []string{})}).Prepare(); err != nil {
				t.Fatal(err.Error())
			}

			got := strings.Split(strings.TrimSpace(logs.String()), "\n")
			if !reflect.DeepEqual(tc.expected, got) {
				t.Fatalf("expected: %q, got: %q", tc.expected, got)
			}
		})
	}
}

// captureLogs redirects the default logger to the returned buffer for the rest
// of the test, omitting the time and level from each record.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	defaultLogger := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey) {
				return slog.Attr{}
			}
			return a
		},
	})))
	t.Cleanup(func() { slog.SetDefault(defaultLogger) })
	return &buf
}
