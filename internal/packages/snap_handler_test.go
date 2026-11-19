package packages

import (
	"bytes"
	"errors"
	"log"
	"log/slog"
	"os"
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

		tc.testFunc(NewSnapHandler(r, snaps, false))

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

		if err := NewSnapHandler(r, []*system.Snap{tc.snap}, false).Prepare(); err != nil {
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

			if err := NewSnapHandler(r, []*system.Snap{tc.snap}, false).Prepare(); err != nil {
				t.Fatal(err.Error())
			}

			if !reflect.DeepEqual([]string{tc.expected}, r.ExecutedCommands) {
				t.Fatalf("expected: %v, got: %v", []string{tc.expected}, r.ExecutedCommands)
			}
		})
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
			// Any channel satisfies an empty request, so this needs the installed check.
			name:     "not installed with no channel requested is installed",
			snap:     &system.Snap{Name: "jq"},
			skip:     true,
			expected: []string{"snap install jq"},
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

			h := NewSnapHandler(r, []*system.Snap{tc.snap}, tc.skip)
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
		if got := ChannelSatisfied(tc.wanted, tc.tracking); got != tc.want {
			t.Errorf("ChannelSatisfied(%q, %q) = %v, want %v", tc.wanted, tc.tracking, got, tc.want)
		}
	}
}

func TestNormaliseChannel(t *testing.T) {
	tests := map[string]string{
		"":                    "",
		"stable":              "latest/stable",
		"3":                   "3/stable",
		"edge/fix":            "latest/edge/fix",
		"3/stable":            "3/stable",
		"latest/stable":       "latest/stable",
		"3/edge/fix":          "3/edge/fix",
		"1.32-classic/stable": "1.32-classic/stable",
	}

	for channel, want := range tests {
		if got := normaliseChannel(channel); got != want {
			t.Errorf("normaliseChannel(%q) = %q, want %q", channel, got, want)
		}
	}
}

func TestSnapHandlerLogsSnapDetails(t *testing.T) {
	type test struct {
		name     string
		before   *system.SnapInfo
		after    *system.InstalledSnapInfo
		afterErr error
		expected []string
	}

	before := system.InstalledSnapInfo{Installed: true, Active: true, TrackingChannel: "3.5/stable", Revision: "30000", Version: "3.5.7"}
	installed := &system.InstalledSnapInfo{Installed: true, Active: true, TrackingChannel: "3.6/stable", Revision: "31000", Version: "3.6.10"}

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
			before: &system.SnapInfo{InstalledSnapInfo: system.InstalledSnapInfo{Installed: true, Active: false, TrackingChannel: "3.5/stable", Revision: "30000", Version: "3.5.7"}},
			after:  installed,
			expected: []string{
				`msg="Enabled disabled snap" snap=juju version=3.5.7 revision=30000 tracking=3.5/stable`,
				`msg="Refreshed snap" snap=juju version=3.6.10 revision=31000 tracking=3.6/stable from.version=3.5.7 from.revision=30000 from.tracking=3.5/stable`,
			},
		},
		{
			name:     "refresh, lookup afterwards fails",
			before:   &system.SnapInfo{InstalledSnapInfo: before},
			afterErr: errors.New("snapd unavailable"),
			expected: []string{
				`msg="Refreshed snap" snap=juju from.version=3.5.7 from.revision=30000 from.tracking=3.5/stable`,
			},
		},
		{
			name:   "not installed afterwards",
			before: &system.SnapInfo{},
			after:  &system.InstalledSnapInfo{},
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
			if tc.afterErr != nil {
				r.MockSnapInstalledInfoError("juju", tc.afterErr)
			} else {
				r.MockSnapInstalledInfo("juju", tc.after)
			}

			if err := NewSnapHandler(r, []*system.Snap{system.NewSnap("juju", "3.6/stable", []string{})}, false).Prepare(); err != nil {
				t.Fatal(err.Error())
			}

			got := strings.Split(strings.TrimSpace(logs.String()), "\n")
			if !reflect.DeepEqual(tc.expected, got) {
				t.Fatalf("expected: %q, got: %q", tc.expected, got)
			}
		})
	}
}

func TestSnapHandlerDryRunLogsOnlyPreviousSnapDetails(t *testing.T) {
	logs := captureLogs(t)

	r := system.NewMockSystem()
	r.MockSnapInfo("juju", &system.SnapInfo{InstalledSnapInfo: system.InstalledSnapInfo{Installed: true, Active: true, TrackingChannel: "3.5/stable", Revision: "30000", Version: "3.5.7"}})
	// In a real dry run this would still report the old state, as nothing was refreshed.
	r.MockSnapInstalledInfo("juju", &system.InstalledSnapInfo{Installed: true, Active: true, TrackingChannel: "3.5/stable", Revision: "30000", Version: "3.5.7"})

	// The dry-run worker prints the commands it skips to stdout.
	devNull, err := os.Open(os.DevNull)
	if err != nil {
		t.Fatal(err)
	}
	defer devNull.Close()
	stdout := os.Stdout
	os.Stdout = devNull
	w := system.NewDryRunWorker(r)
	os.Stdout = stdout

	if err := NewSnapHandler(w, []*system.Snap{system.NewSnap("juju", "3.6/stable", []string{})}, false).Prepare(); err != nil {
		t.Fatal(err.Error())
	}

	expected := []string{
		`msg="Refreshed snap" snap=juju from.version=3.5.7 from.revision=30000 from.tracking=3.5/stable`,
	}
	got := strings.Split(strings.TrimSpace(logs.String()), "\n")
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected: %q, got: %q", expected, got)
	}
}

// captureLogs redirects the default logger to the returned buffer for the rest
// of the test, omitting the time and level from each record.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	defaultLogger := slog.Default()
	// slog.SetDefault also redirects the log package's output to the new handler
	// (and clears its flags), and restoring the old default doesn't undo that, so
	// restore those separately.
	logWriter, logFlags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(defaultLogger)
		log.SetOutput(logWriter)
		log.SetFlags(logFlags)
	})
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, &slog.HandlerOptions{
		ReplaceAttr: func(groups []string, a slog.Attr) slog.Attr {
			if len(groups) == 0 && (a.Key == slog.TimeKey || a.Key == slog.LevelKey) {
				return slog.Attr{}
			}
			return a
		},
	})))
	return &buf
}
