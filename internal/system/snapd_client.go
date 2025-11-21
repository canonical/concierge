package system

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	// StatusActive represents an active snap installation
	StatusActive = "active"
)

// SnapdClient is a minimal client for the snapd REST API
type SnapdClient struct {
	httpClient *http.Client
}

// NewSnapdClient creates a new snapd API client
func NewSnapdClient() *SnapdClient {
	return &SnapdClient{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					return (&net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}).DialContext(ctx, "unix", "/run/snapd.socket")
				},
			},
		},
	}
}

// SnapdResponse represents the common structure of snapd API responses
type SnapdResponse struct {
	Type       string          `json:"type"`
	StatusCode int             `json:"status-code"`
	Status     string          `json:"status"`
	Result     json.RawMessage `json:"result"`
}

// snapdSnap represents information about a snap from the snapd API
type snapdSnap struct {
	ID          string                 `json:"id"`
	Name        string                 `json:"name"`
	Status      string                 `json:"status"`
	Version     string                 `json:"version"`
	Revision    string                 `json:"revision"`
	Channel     string                 `json:"channel"`
	Confinement string                 `json:"confinement"`
	Channels    map[string]ChannelInfo `json:"channels"`
}

// ChannelInfo represents channel-specific information for a snap
type ChannelInfo struct {
	Revision    string `json:"revision"`
	Confinement string `json:"confinement"`
	Version     string `json:"version"`
	Channel     string `json:"channel"`
}

// Snap queries information about an installed snap
func (c *SnapdClient) Snap(name string) (*snapdSnap, *SnapdResponse, error) {
	url := fmt.Sprintf("http://localhost/v2/snaps/%s", url.PathEscape(name))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var snapdResp SnapdResponse
	if err := json.Unmarshal(body, &snapdResp); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	// If the snap is not installed, snapd returns a 404 with an error message
	if snapdResp.StatusCode == 404 {
		return nil, &snapdResp, fmt.Errorf("snap not installed: %s", name)
	}

	if snapdResp.StatusCode != 200 {
		return nil, &snapdResp, fmt.Errorf("unexpected status code: %d", snapdResp.StatusCode)
	}

	var snapInfo snapdSnap
	if err := json.Unmarshal(snapdResp.Result, &snapInfo); err != nil {
		return nil, &snapdResp, fmt.Errorf("failed to unmarshal snap info: %w", err)
	}

	return &snapInfo, &snapdResp, nil
}

// FindOne searches for a snap in the snap store
func (c *SnapdClient) FindOne(name string) (*snapdSnap, *SnapdResponse, error) {
	url := fmt.Sprintf("http://localhost/v2/find?name=%s", url.QueryEscape(name))

	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var snapdResp SnapdResponse
	if err := json.Unmarshal(body, &snapdResp); err != nil {
		return nil, nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	if snapdResp.StatusCode != 200 {
		return nil, &snapdResp, fmt.Errorf("unexpected status code: %d", snapdResp.StatusCode)
	}

	var snaps []snapdSnap
	if err := json.Unmarshal(snapdResp.Result, &snaps); err != nil {
		return nil, &snapdResp, fmt.Errorf("failed to unmarshal snap list: %w", err)
	}

	if len(snaps) == 0 {
		return nil, &snapdResp, fmt.Errorf("snap not found: %s", name)
	}

	// Return the first matching snap
	return &snaps[0], &snapdResp, nil
}
