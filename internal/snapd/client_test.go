package snapd

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"testing"

	"github.com/canonical/concierge/internal/snapd/snapdtest"
)

func TestSnap_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/snaps/test-snap" {
			t.Errorf("Expected path '/v2/snaps/test-snap', got: %s", r.URL.Path)
		}

		resp := response{
			Type:   "sync",
			Status: "OK",
		}
		snap := Snap{
			ID:              "test-id",
			Name:            "test-snap",
			Status:          "active",
			Version:         "1.0",
			Channel:         "stable",
			TrackingChannel: "latest/stable",
			Confinement:     "strict",
		}
		result, err := json.Marshal(snap)
		if err != nil {
			t.Fatalf("failed to marshal snap: %v", err)
		}
		resp.Result = result

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	socketPath := snapdtest.NewServer(t, handler)

	client := NewClient(&Config{Socket: socketPath})
	snap, err := client.Snap(context.Background(), "test-snap")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if snap.Name != "test-snap" {
		t.Errorf("Expected snap name 'test-snap', got: %s", snap.Name)
	}
	if snap.TrackingChannel != "latest/stable" {
		t.Errorf("Expected tracking channel 'latest/stable', got: %s", snap.TrackingChannel)
	}
	if snap.Status != "active" {
		t.Errorf("Expected status 'active', got: %s", snap.Status)
	}
}

func TestSnap_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := response{
			Type:   "error",
			Status: "Not Found",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	socketPath := snapdtest.NewServer(t, handler)

	client := NewClient(&Config{Socket: socketPath})
	_, err := client.Snap(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("Expected error for non-existent snap")
	}
	if !errors.Is(err, ErrNotInstalled) {
		t.Errorf("Expected ErrNotInstalled, got: %v", err)
	}
}

func TestSnap_UnexpectedStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		resp := response{
			Type:   "error",
			Status: "Internal Server Error",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	socketPath := snapdtest.NewServer(t, handler)

	client := NewClient(&Config{Socket: socketPath})
	_, err := client.Snap(context.Background(), "test-snap")

	if err == nil {
		t.Fatal("Expected error for 500 status code")
	}
}

func TestFindOne_Success(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/find" {
			t.Errorf("Expected path '/v2/find', got: %s", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "test-snap" {
			t.Errorf("Expected query param name=test-snap, got: %s", r.URL.Query().Get("name"))
		}

		resp := response{
			Type:   "sync",
			Status: "OK",
		}
		snaps := []Snap{
			{
				ID:          "test-id",
				Name:        "test-snap",
				Version:     "1.0",
				Confinement: "strict",
				Channels: map[string]ChannelInfo{
					"latest/stable": {
						Confinement: "strict",
						Version:     "1.0",
					},
					"latest/edge": {
						Confinement: "strict",
						Version:     "1.1",
					},
				},
			},
		}
		result, err := json.Marshal(snaps)
		if err != nil {
			t.Fatalf("failed to marshal snaps response: %v", err)
		}
		resp.Result = result

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	socketPath := snapdtest.NewServer(t, handler)

	client := NewClient(&Config{Socket: socketPath})
	snap, err := client.FindOne(context.Background(), "test-snap")

	if err != nil {
		t.Fatalf("Expected no error, got: %v", err)
	}
	if snap.Name != "test-snap" {
		t.Errorf("Expected snap name 'test-snap', got: %s", snap.Name)
	}
	if len(snap.Channels) != 2 {
		t.Errorf("Expected 2 channels, got: %d", len(snap.Channels))
	}
}

func TestFindOne_NotFound(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusNotFound)
		resp := response{
			Type:   "error",
			Status: "Not Found",
		}
		_ = json.NewEncoder(w).Encode(resp)
	})

	socketPath := snapdtest.NewServer(t, handler)

	client := NewClient(&Config{Socket: socketPath})
	_, err := client.FindOne(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("Expected error for non-existent snap")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}

func TestFindOne_EmptyResults(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		resp := response{
			Type:   "sync",
			Status: "OK",
		}
		snaps := []Snap{}
		result, err := json.Marshal(snaps)
		if err != nil {
			t.Fatalf("failed to marshal snaps: %v", err)
		}
		resp.Result = result

		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusOK)
		_ = json.NewEncoder(w).Encode(resp)
	})

	socketPath := snapdtest.NewServer(t, handler)

	client := NewClient(&Config{Socket: socketPath})
	_, err := client.FindOne(context.Background(), "nonexistent")

	if err == nil {
		t.Fatal("Expected error for empty results")
	}
	if !errors.Is(err, ErrNotFound) {
		t.Errorf("Expected ErrNotFound, got: %v", err)
	}
}
