package snapd

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
	"os"
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
		result, _ := json.Marshal(snap)
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
		result, _ := json.Marshal(snaps)
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
		result, _ := json.Marshal(snaps)
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

func TestNewClient_DefaultSocket(t *testing.T) {
	client := NewClient(nil)
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.socketPath != "/run/snapd.socket" {
		t.Errorf("Expected default socket path, got: %s", client.socketPath)
	}
}

func TestNewClient_CustomSocket(t *testing.T) {
	customPath := "/custom/snapd.socket"
	client := NewClient(&Config{Socket: customPath})
	if client.socketPath != customPath {
		t.Errorf("Expected custom socket path %s, got: %s", customPath, client.socketPath)
	}
}

// Integration test - only runs if snapd socket is available
func TestSnap_Integration(t *testing.T) {
	if _, err := os.Stat("/run/snapd.socket"); os.IsNotExist(err) {
		t.Skip("Skipping integration test: snapd socket not available")
	}
	
	client := NewClient(nil)
	
	// Try to query for a snap that definitely doesn't exist
	_, err := client.Snap("this-snap-definitely-does-not-exist-12345")
	if err == nil {
		t.Error("Expected error for non-existent snap")
	}
}

// Integration test for FindOne
func TestFindOne_Integration(t *testing.T) {
	if _, err := os.Stat("/run/snapd.socket"); os.IsNotExist(err) {
		t.Skip("Skipping integration test: snapd socket not available")
	}
	
	client := NewClient(nil)
	
	// Try to find a common snap that's likely in the store
	snap, err := client.FindOne("core")
	if err != nil {
		t.Skipf("Could not find snap in store: %v", err)
	}
	
	if snap.Name != "core" {
		t.Errorf("Expected snap name 'core', got: %s", snap.Name)
	}
	if len(snap.Channels) == 0 {
		t.Error("Expected snap to have channels")
	}
}
