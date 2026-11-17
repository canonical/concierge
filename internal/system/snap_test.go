package system

import (
	"context"
	"encoding/json"
	"errors"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

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
		expected *InstalledSnapInfo
	}

	tests := []test{
		{
			name:   "active",
			status: http.StatusOK,
			snap: snapd.Snap{
				Name: "juju", Status: snapd.StatusActive, Revision: "30000", Version: "3.6.9",
				Channel: "3.6/stable", TrackingChannel: "3.6/stable",
			},
			expected: &InstalledSnapInfo{Installed: true, Active: true, TrackingChannel: "3.6/stable", Revision: "30000", Version: "3.6.9"},
		},
		{
			name:   "disabled, no tracking channel",
			status: http.StatusOK,
			snap: snapd.Snap{
				Name: "lxd", Status: snapd.StatusInstalled, Revision: "33110", Version: "5.21.3",
				Channel: "5.21/stable",
			},
			expected: &InstalledSnapInfo{Installed: true, Active: false, TrackingChannel: "5.21/stable", Revision: "33110", Version: "5.21.3"},
		},
		{
			name:     "not installed",
			status:   http.StatusNotFound,
			expected: &InstalledSnapInfo{},
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			// Marshal up front: t.Fatalf in the handler's goroutine wouldn't stop the test.
			result, err := json.Marshal(tc.snap)
			if err != nil {
				t.Fatalf("failed to marshal snap: %v", err)
			}
			handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
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

			// Bound the retries, so that an unexpected failure doesn't hang the test.
			ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
			defer cancel()

			info, err := s.SnapInstalledInfo(ctx, "juju")
			if err != nil {
				t.Fatal(err)
			}
			if !reflect.DeepEqual(tc.expected, info) {
				t.Fatalf("expected: %+v, got: %+v", tc.expected, info)
			}
		})
	}
}

func TestSnapInstalledInfoErrorRespectsContext(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "snapd is broken", http.StatusInternalServerError)
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

	// Without the context, the failed query would be retried for many minutes.
	ctx, cancel := context.WithTimeout(context.Background(), 100*time.Millisecond)
	defer cancel()

	info, err := s.SnapInstalledInfo(ctx, "juju")
	if err == nil {
		t.Fatalf("expected an error, got: %+v", info)
	}
	if !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("expected the context deadline to stop the retries, got: %v", err)
	}
}
