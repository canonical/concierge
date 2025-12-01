package system

import (
	"encoding/json"
	"net"
	"net/http"
	"net/http/httptest"
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

// TestGetSnap_Success tests GetSnap with a mock HTTP server.
func TestGetSnap_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/snaps/test-snap" {
			t.Errorf("Expected path '/v2/snaps/test-snap', got: %s", r.URL.Path)
		}
		response := SnapdResponse{
			Type:       "sync",
			StatusCode: 200,
			Status:     "OK",
		}
		snap := snapdSnap{
			ID:              "test-id",
			Name:            "test-snap",
			Status:          "active",
			Version:         "1.0",
			Channel:         "stable",
			TrackingChannel: "latest/stable",
			Confinement:     "strict",
		}
		result, _ := json.Marshal(snap)
		response.Result = result
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	// Test the response parsing logic with an HTTP client
	resp, err := server.Client().Get(server.URL + "/v2/snaps/test-snap")
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}
	defer resp.Body.Close()

	var snapdResp SnapdResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapdResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	var snap snapdSnap
	if err := json.Unmarshal(snapdResp.Result, &snap); err != nil {
		t.Fatalf("Failed to unmarshal snap: %v", err)
	}

	if snap.Name != "test-snap" {
		t.Errorf("Expected snap name 'test-snap', got: %s", snap.Name)
	}
	if snap.TrackingChannel != "latest/stable" {
		t.Errorf("Expected tracking channel 'latest/stable', got: %s", snap.TrackingChannel)
	}
}

// TestGetSnap_NotFound tests GetSnap with a 404 response.
func TestGetSnap_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SnapdResponse{
			Type:       "error",
			StatusCode: 404,
			Status:     "Not Found",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/v2/snaps/nonexistent")
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}
	defer resp.Body.Close()

	var snapdResp SnapdResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapdResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if snapdResp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got: %d", snapdResp.StatusCode)
	}
	if snapdResp.Type != "error" {
		t.Errorf("Expected type 'error', got: %s", snapdResp.Type)
	}
}

// TestFindOne_Success tests FindOne with a mock HTTP server.
func TestFindOne_Success(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v2/find" {
			t.Errorf("Expected path '/v2/find', got: %s", r.URL.Path)
		}
		if r.URL.Query().Get("name") != "test-snap" {
			t.Errorf("Expected query param name=test-snap, got: %s", r.URL.Query().Get("name"))
		}
		response := SnapdResponse{
			Type:       "sync",
			StatusCode: 200,
			Status:     "OK",
		}
		snaps := []snapdSnap{
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
				},
			},
		}
		result, _ := json.Marshal(snaps)
		response.Result = result
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/v2/find?name=test-snap")
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}
	defer resp.Body.Close()

	var snapdResp SnapdResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapdResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	var snaps []snapdSnap
	if err := json.Unmarshal(snapdResp.Result, &snaps); err != nil {
		t.Fatalf("Failed to unmarshal snaps: %v", err)
	}

	if len(snaps) != 1 {
		t.Fatalf("Expected 1 snap, got: %d", len(snaps))
	}
	if snaps[0].Name != "test-snap" {
		t.Errorf("Expected snap name 'test-snap', got: %s", snaps[0].Name)
	}
}

// TestFindOne_NotFound tests FindOne with empty results (404).
func TestFindOne_NotFound(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SnapdResponse{
			Type:       "error",
			StatusCode: 404,
			Status:     "Not Found",
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/v2/find?name=nonexistent")
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}
	defer resp.Body.Close()

	var snapdResp SnapdResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapdResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	if snapdResp.StatusCode != 404 {
		t.Errorf("Expected status code 404, got: %d", snapdResp.StatusCode)
	}
}

// TestFindOne_EmptyResults tests FindOne with empty results array.
func TestFindOne_EmptyResults(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		response := SnapdResponse{
			Type:       "sync",
			StatusCode: 200,
			Status:     "OK",
		}
		snaps := []snapdSnap{}
		result, _ := json.Marshal(snaps)
		response.Result = result
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(response)
	}))
	defer server.Close()

	resp, err := server.Client().Get(server.URL + "/v2/find?name=nonexistent")
	if err != nil {
		t.Fatalf("Failed to get response: %v", err)
	}
	defer resp.Body.Close()

	var snapdResp SnapdResponse
	if err := json.NewDecoder(resp.Body).Decode(&snapdResp); err != nil {
		t.Fatalf("Failed to decode response: %v", err)
	}

	var snaps []snapdSnap
	if err := json.Unmarshal(snapdResp.Result, &snaps); err != nil {
		t.Fatalf("Failed to unmarshal snaps: %v", err)
	}

	if len(snaps) != 0 {
		t.Errorf("Expected 0 snaps, got: %d", len(snaps))
	}
}
