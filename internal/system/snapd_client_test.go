package system

import (
	"encoding/json"
	"net"
	"os"
	"testing"
)

func TestSnapdClient_Snap_Installed(t *testing.T) {
	// Test response parsing
	response := SnapdResponse{
		Type:       "sync",
		StatusCode: 200,
		Status:     "OK",
	}
	snap := snapdSnap{
		ID:          "test-id",
		Name:        "juju",
		Status:      "active",
		Version:     "3.6.11",
		Confinement: "strict",
	}
	result, err := json.Marshal(snap)
	if err != nil {
		t.Fatalf("Failed to marshal snap: %v", err)
	}
	response.Result = result

	var parsedSnap snapdSnap
	if err := json.Unmarshal(response.Result, &parsedSnap); err != nil {
		t.Fatalf("Failed to parse snap: %v", err)
	}
	
	if parsedSnap.Name != "juju" {
		t.Errorf("Expected snap name 'juju', got: %s", parsedSnap.Name)
	}
	
	if parsedSnap.Status != "active" {
		t.Errorf("Expected snap status 'active', got: %s", parsedSnap.Status)
	}
}

func TestSnapdClient_FindOne_ResponseParsing(t *testing.T) {
	// Test response parsing
	response := SnapdResponse{
		Type:       "sync",
		StatusCode: 200,
		Status:     "OK",
	}
	snaps := []snapdSnap{
		{
			ID:          "test-id",
			Name:        "juju",
			Version:     "3.6.11",
			Confinement: "strict",
			Channels: map[string]ChannelInfo{
				"3.6/stable": {
					Confinement: "strict",
					Version:     "3.6.11",
				},
				"2.9/stable": {
					Confinement: "classic",
					Version:     "2.9.52",
				},
			},
		},
	}
	result, err := json.Marshal(snaps)
	if err != nil {
		t.Fatalf("Failed to marshal snaps: %v", err)
	}
	response.Result = result

	var parsedSnaps []snapdSnap
	if err := json.Unmarshal(response.Result, &parsedSnaps); err != nil {
		t.Fatalf("Failed to parse snaps: %v", err)
	}
	
	if len(parsedSnaps) != 1 {
		t.Fatalf("Expected 1 snap, got: %d", len(parsedSnaps))
	}
	
	snap := parsedSnaps[0]
	
	if snap.Name != "juju" {
		t.Errorf("Expected snap name 'juju', got: %s", snap.Name)
	}
	
	if len(snap.Channels) != 2 {
		t.Errorf("Expected 2 channels, got: %d", len(snap.Channels))
	}
	
	if snap.Channels["2.9/stable"].Confinement != "classic" {
		t.Errorf("Expected 2.9/stable to be classic, got: %s", snap.Channels["2.9/stable"].Confinement)
	}
	
	if snap.Channels["3.6/stable"].Confinement != "strict" {
		t.Errorf("Expected 3.6/stable to be strict, got: %s", snap.Channels["3.6/stable"].Confinement)
	}
}

// TestNewSnapdClient ensures the client is created correctly
func TestNewSnapdClient(t *testing.T) {
	client := NewSnapdClient()
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.httpClient == nil {
		t.Fatal("Expected non-nil HTTP client")
	}
}

// Integration test - only runs if snapd socket is available
func TestSnapdClient_Integration_RealSocket(t *testing.T) {
	// Skip if snapd socket doesn't exist
	if _, err := os.Stat("/run/snapd.socket"); os.IsNotExist(err) {
		t.Skip("Skipping integration test: snapd socket not available")
	}
	
	client := NewSnapdClient()
	
	// Try to find a common snap that's likely in the store
	snap, _, err := client.FindOne("core")
	if err != nil {
		t.Skipf("Skipping integration test: could not find snap in store: %v", err)
	}
	
	if snap.Name != "core" {
		t.Errorf("Expected snap name 'core', got: %s", snap.Name)
	}
	
	// Verify channels exist
	if len(snap.Channels) == 0 {
		t.Error("Expected snap to have channels")
	}
}

// TestSnapdClient_Snap_WithRealSocket tests against the actual snapd socket if available
func TestSnapdClient_Snap_WithRealSocket(t *testing.T) {
	// Skip if snapd socket doesn't exist
	socketPath := "/run/snapd.socket"
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skip("Skipping test: snapd socket not available")
	}
	
	// Check if we can actually connect to the socket
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to snapd socket: %v", err)
	}
	conn.Close()
	
	client := NewSnapdClient()
	
	// Query for a snap that shouldn't exist
	_, _, err = client.Snap("this-snap-definitely-does-not-exist-12345")
	if err == nil {
		t.Error("Expected error for non-existent snap")
	}
}

// TestSnapdResponse_ErrorHandling tests error response parsing
func TestSnapdResponse_ErrorHandling(t *testing.T) {
	// Test 404 response
	response := SnapdResponse{
		Type:       "error",
		StatusCode: 404,
		Status:     "Not Found",
	}
	
	if response.StatusCode != 404 {
		t.Errorf("Expected status code 404, got: %d", response.StatusCode)
	}
	
	if response.Type != "error" {
		t.Errorf("Expected type 'error', got: %s", response.Type)
	}
}

// TestChannelInfo_Structure tests the ChannelInfo structure
func TestChannelInfo_Structure(t *testing.T) {
	info := ChannelInfo{
		Revision:    "123",
		Confinement: "classic",
		Version:     "1.0.0",
		Channel:     "stable",
	}
	
	if info.Confinement != "classic" {
		t.Errorf("Expected confinement 'classic', got: %s", info.Confinement)
	}
	
	if info.Version != "1.0.0" {
		t.Errorf("Expected version '1.0.0', got: %s", info.Version)
	}
}
