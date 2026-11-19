// Package snapdtest provides helpers for testing code that talks to the snapd API.
package snapdtest

import (
	"context"
	"net"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"
)

// NewServer starts a fake snapd server that serves handler on a Unix socket in a
// temporary directory, and returns the path of the socket. The server is closed
// when the test finishes.
func NewServer(t testing.TB, handler http.Handler) string {
	t.Helper()

	socketPath := filepath.Join(t.TempDir(), "snapd.socket")

	lc := net.ListenConfig{}
	listener, err := lc.Listen(context.Background(), "unix", socketPath)
	if err != nil {
		t.Fatalf("failed to create Unix listener: %v", err)
	}

	server := httptest.NewUnstartedServer(handler)
	server.Listener = listener
	server.Start()
	t.Cleanup(server.Close)

	return socketPath
}
