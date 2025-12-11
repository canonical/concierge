package snapd

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
	// StatusActive represents an active snap installation.
	StatusActive = "active"
)

// Client is a minimal client for the snapd REST API.
type Client struct {
	httpClient *http.Client
	socketPath string
}

// Config configures the snapd client.
type Config struct {
	// Socket is the path to the snapd socket.
	// If empty, the default "/run/snapd.socket" is used.
	Socket string
}

// NewClient creates a new snapd API client.
func NewClient(config *Config) *Client {
	socketPath := "/run/snapd.socket"
	if config != nil && config.Socket != "" {
		socketPath = config.Socket
	}

	return &Client{
		httpClient: &http.Client{
			Transport: &http.Transport{
				DialContext: func(ctx context.Context, _, _ string) (net.Conn, error) {
					dialer := net.Dialer{
						Timeout:   30 * time.Second,
						KeepAlive: 30 * time.Second,
					}
					return dialer.DialContext(ctx, "unix", socketPath)
				},
			},
			Timeout: 60 * time.Second,
		},
		socketPath: socketPath,
	}
}

// response represents the common structure of snapd API responses.
// See https://snapcraft.io/docs/using-the-api
type response struct {
	Type   string          `json:"type"`
	Status string          `json:"status"`
	Result json.RawMessage `json:"result"`
}

// Snap represents information about a snap from the snapd API.
// See https://snapcraft.io/docs/snapd-rest-api#heading--snaps
type Snap struct {
	ID              string                 `json:"id"`
	Name            string                 `json:"name"`
	Status          string                 `json:"status"`
	Version         string                 `json:"version"`
	Revision        string                 `json:"revision"`
	Channel         string                 `json:"channel"`
	TrackingChannel string                 `json:"tracking-channel"`
	Confinement     string                 `json:"confinement"`
	Channels        map[string]ChannelInfo `json:"channels"`
}

// ChannelInfo represents channel-specific information for a snap.
type ChannelInfo struct {
	Revision    string `json:"revision"`
	Confinement string `json:"confinement"`
	Version     string `json:"version"`
	Channel     string `json:"channel"`
}

// Snap queries information about an installed snap.
func (c *Client) Snap(name string) (*Snap, error) {
	apiURL := fmt.Sprintf("http://localhost/v2/snaps/%s", url.PathEscape(name))

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("snap not installed: %s", name)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var snapdResp response
	if err := json.Unmarshal(body, &snapdResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var snapInfo Snap
	if err := json.Unmarshal(snapdResp.Result, &snapInfo); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snap info: %w", err)
	}

	return &snapInfo, nil
}

// FindOne searches for a snap in the snap store.
func (c *Client) FindOne(name string) (*Snap, error) {
	query := url.Values{"name": []string{name}}
	apiURL := "http://localhost/v2/find?" + query.Encode()

	req, err := http.NewRequest("GET", apiURL, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == 404 {
		return nil, fmt.Errorf("snap not found: %s", name)
	}
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("unexpected HTTP status code: %d", resp.StatusCode)
	}

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("failed to read response body: %w", err)
	}

	var snapdResp response
	if err := json.Unmarshal(body, &snapdResp); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	var snaps []Snap
	if err := json.Unmarshal(snapdResp.Result, &snaps); err != nil {
		return nil, fmt.Errorf("failed to unmarshal snap list: %w", err)
	}

	if len(snaps) == 0 {
		return nil, fmt.Errorf("snap not found: %s", name)
	}

	// Return the first matching snap.
	return &snaps[0], nil
}
