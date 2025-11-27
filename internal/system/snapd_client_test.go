package system

import (
	"encoding/json"
	"net"
	"os"
	"testing"
)

func TestSnapdSnapParsing(t *testing.T) {
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

	if parsedSnap.ID != "test-id" {
		t.Errorf("Expected snap ID 'test-id', got: %s", parsedSnap.ID)
	}
	if parsedSnap.Name != "juju" {
		t.Errorf("Expected snap name 'juju', got: %s", parsedSnap.Name)
	}
	if parsedSnap.Status != "active" {
		t.Errorf("Expected snap status 'active', got: %s", parsedSnap.Status)
	}
	if parsedSnap.Version != "3.6.11" {
		t.Errorf("Expected snap version '3.6.11', got: %s", parsedSnap.Version)
	}
	if parsedSnap.Confinement != "strict" {
		t.Errorf("Expected snap confinement 'strict', got: %s", parsedSnap.Confinement)
	}
}

func TestSnapdFindOneParsing(t *testing.T) {
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

// TestNewSnapdClient ensures the client is created correctly.
func TestNewSnapdClient(t *testing.T) {
	client := NewSnapdClient()
	if client == nil {
		t.Fatal("Expected non-nil client")
	}
	if client.httpClient == nil {
		t.Fatal("Expected non-nil HTTP client")
	}
}

// TestSnapdClient_Integration_RealSocket is an integration test that only runs if snapd socket is available.
func TestSnapdClient_Integration_RealSocket(t *testing.T) {
	if _, err := os.Stat("/run/snapd.socket"); os.IsNotExist(err) {
		t.Skip("Skipping integration test: snapd socket not available")
	}
	client := NewSnapdClient()
	snap, _, err := client.FindOne("core")
	if err != nil {
		t.Skipf("Skipping integration test: could not find snap in store: %v", err)
	}
	if snap.Name != "core" {
		t.Errorf("Expected snap name 'core', got: %s", snap.Name)
	}
	if len(snap.Channels) == 0 {
		t.Error("Expected snap to have channels")
	}
}

// TestSnapdClient_GetSnap_WithRealSocket tests against the actual snapd socket if available.
func TestSnapdClient_GetSnap_WithRealSocket(t *testing.T) {
	socketPath := "/run/snapd.socket"
	if _, err := os.Stat(socketPath); os.IsNotExist(err) {
		t.Skip("Skipping test: snapd socket not available")
	}
	conn, err := net.Dial("unix", socketPath)
	if err != nil {
		t.Skipf("Skipping test: cannot connect to snapd socket: %v", err)
	}
	conn.Close()
	client := NewSnapdClient()
	_, _, err = client.GetSnap("this-snap-definitely-does-not-exist-12345")
	if err == nil {
		t.Error("Expected error for non-existent snap")
	}
}
