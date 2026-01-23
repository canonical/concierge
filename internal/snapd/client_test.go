package snapd

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// createTestServer creates a test HTTP server with a Unix socket listener.
func createTestServer(t *testing.T, handler http.Handler) (*httptest.Server, string) {
	t.Helper()
	
	// Create temporary directory for socket
	tmpDir := t.TempDir()
	socketPath := filepath.Join(tmpDir, "snapd.socket")
	
	// Create Unix listener
	listener, err := net.Listen("unix", socketPath)
	if err != nil {
		t.Fatalf("Failed to create Unix listener: %v", err)
	}
	
	// Create test server with custom listener
	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	
	return server, socketPath
}

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
		json.NewEncoder(w).Encode(resp)
	})
	
	server, socketPath := createTestServer(t, handler)
	defer server.Close()
	
	client := NewClient(&Config{Socket: socketPath})
	snap, err := client.Snap("test-snap")
	
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
		json.NewEncoder(w).Encode(resp)
	})
	
	server, socketPath := createTestServer(t, handler)
	defer server.Close()
	
	client := NewClient(&Config{Socket: socketPath})
	_, err := client.Snap("nonexistent")
	
	if err == nil {
		t.Fatal("Expected error for non-existent snap")
	}
	if err.Error() != "snap not installed: nonexistent" {
		t.Errorf("Expected 'snap not installed' error, got: %v", err)
	}
}

func TestSnap_UnexpectedStatusCode(t *testing.T) {
	handler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
		resp := response{
			Type:   "error",
			Status: "Internal Server Error",
		}
		json.NewEncoder(w).Encode(resp)
	})
	
	server, socketPath := createTestServer(t, handler)
	defer server.Close()
	
	client := NewClient(&Config{Socket: socketPath})
	_, err := client.Snap("test-snap")
	
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
		json.NewEncoder(w).Encode(resp)
	})
	
	server, socketPath := createTestServer(t, handler)
	defer server.Close()
	
	client := NewClient(&Config{Socket: socketPath})
	snap, err := client.FindOne("test-snap")
	
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
		json.NewEncoder(w).Encode(resp)
	})
	
	server, socketPath := createTestServer(t, handler)
	defer server.Close()
	
	client := NewClient(&Config{Socket: socketPath})
	_, err := client.FindOne("nonexistent")
	
	if err == nil {
		t.Fatal("Expected error for non-existent snap")
	}
	if err.Error() != "snap not found: nonexistent" {
		t.Errorf("Expected 'snap not found' error, got: %v", err)
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
		json.NewEncoder(w).Encode(resp)
	})
	
	server, socketPath := createTestServer(t, handler)
	defer server.Close()
	
	client := NewClient(&Config{Socket: socketPath})
	_, err := client.FindOne("nonexistent")
	
	if err == nil {
		t.Fatal("Expected error for empty results")
	}
	if err.Error() != "snap not found: nonexistent" {
		t.Errorf("Expected 'snap not found' error, got: %v", err)
	}
}
