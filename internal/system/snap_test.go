package system

import (
	"context"
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/snapd"
)

func TestNewSnapFromString(t *testing.T) {
	type test struct {
		input    string
		expected *Snap
	}

	tests := []test{
		{input: "juju", expected: &Snap{Name: "juju"}},
		{input: "juju/latest/edge", expected: &Snap{Name: "juju", Channel: "latest/edge"}},
		{input: "juju/stable", expected: &Snap{Name: "juju", Channel: "stable"}},
	}

	for _, tc := range tests {
		snap := NewSnapFromString(tc.input)

		if tc.expected.Channel != snap.Channel {
			t.Fatalf("incorrect snap channel; expected: %v, got: %v", tc.expected, snap)
		}
		if tc.expected.Name != snap.Name {
			t.Fatalf("incorrect snap name; expected: %v, got: %v", tc.expected, snap)
		}
	}
}

func TestSnapInstalledInfo(t *testing.T) {
	type test struct {
		name     string
		status   int
		snap     snapd.Snap
		expected *SnapInfo
	}

	tests := []test{
		{
			name:   "active",
			status: http.StatusOK,
			snap: snapd.Snap{
				Name: "juju", Status: snapd.StatusActive, Revision: "30000", Version: "3.6.9",
				Channel: "3.6/stable", TrackingChannel: "3.6/stable",
			},
			expected: &SnapInfo{Installed: true, Active: true, TrackingChannel: "3.6/stable", Revision: "30000", Version: "3.6.9"},
		},
		{
			name:   "disabled, no tracking channel",
			status: http.StatusOK,
			snap: snapd.Snap{
				Name: "lxd", Status: snapd.StatusInstalled, Revision: "33110", Version: "5.21.3",
				Channel: "5.21/stable",
			},
			expected: &SnapInfo{Installed: true, Active: false, TrackingChannel: "5.21/stable", Revision: "33110", Version: "5.21.3"},
		},
		{
			name:     "not installed",
			status:   http.StatusNotFound,
			expected: &SnapInfo{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				result, err := json.Marshal(tc.snap)
				if err != nil {
					t.Fatalf("failed to marshal snap: %v", err)
				}
				w.Header().Set("Content-Type", "application/json")
				w.WriteHeader(tc.status)
				_ = json.NewEncoder(w).Encode(map[string]any{"type": "sync", "result": json.RawMessage(result)})
			})

			socketPath := filepath.Join(t.TempDir(), "snapd.socket")
			lc := net.ListenConfig{}
			listener, err := lc.Listen(context.Background(), "unix", socketPath)
			if err != nil {
				t.Fatalf("failed to create Unix listener: %v", err)
			}
			server := httptest.NewUnstartedServer(handler)
			server.Listener = listener
			server.Start()
			defer server.Close()

			s := &System{snapd: snapd.NewClient(&snapd.Config{Socket: socketPath})}

			info := s.SnapInstalledInfo("juju")
			if !reflect.DeepEqual(tc.expected, info) {
				t.Fatalf("expected: %+v, got: %+v", tc.expected, info)
			}
		})
	}
}
