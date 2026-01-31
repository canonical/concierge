package packages

import (
	"bytes"
	"fmt"
	"os"
	"os/user"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/canonical/concierge/internal/system"
)

// testDryRunWorker is a test implementation that captures Print output
// and tracks commands for verification without actually executing them
type testDryRunWorker struct {
	printOutput  *bytes.Buffer
	mu           sync.Mutex
	executedCmds []string
	mockSnapInfo map[string]*system.SnapInfo
}

func newTestDryRunWorker() *testDryRunWorker {
	return &testDryRunWorker{
		printOutput:  &bytes.Buffer{},
		executedCmds: []string{},
		mockSnapInfo: map[string]*system.SnapInfo{},
	}
}

func (t *testDryRunWorker) Print(msg string) {
	t.mu.Lock()
	defer t.mu.Unlock()
	fmt.Fprintln(t.printOutput, msg)
}

func (t *testDryRunWorker) User() *user.User {
	return &user.User{Username: "test", Uid: "1000", Gid: "1000", HomeDir: "/tmp"}
}

func (t *testDryRunWorker) Run(c *system.Command) ([]byte, error) {
	// This mock returns success without actually executing the command.
	// We track the command strings for potential test assertions.
	t.mu.Lock()
	t.executedCmds = append(t.executedCmds, c.CommandString())
	t.mu.Unlock()
	return []byte{}, nil
}

func (t *testDryRunWorker) RunMany(commands ...*system.Command) error {
	for _, c := range commands {
		t.Run(c)
	}
	return nil
}

func (t *testDryRunWorker) RunExclusive(c *system.Command) ([]byte, error) {
	return t.Run(c)
}

func (t *testDryRunWorker) RunWithRetries(c *system.Command, maxDuration time.Duration) ([]byte, error) {
	return t.Run(c)
}

func (t *testDryRunWorker) WriteHomeDirFile(filepath string, contents []byte) error {
	return nil
}

func (t *testDryRunWorker) ReadHomeDirFile(filepath string) ([]byte, error) {
	return nil, fmt.Errorf("file not found")
}

func (t *testDryRunWorker) ReadFile(filePath string) ([]byte, error) {
	return nil, fmt.Errorf("file not found")
}

func (t *testDryRunWorker) SnapInfo(snap string, channel string) (*system.SnapInfo, error) {
	if info, ok := t.mockSnapInfo[snap]; ok {
		return info, nil
	}
	return &system.SnapInfo{Installed: false, Classic: false}, nil
}

func (t *testDryRunWorker) SnapChannels(snap string) ([]string, error) {
	return []string{"stable", "edge"}, nil
}

func (t *testDryRunWorker) RemovePath(path string) error {
	return nil
}

func (t *testDryRunWorker) MkdirAll(path string, perm os.FileMode) error {
	return nil
}

func (t *testDryRunWorker) ChownAll(path string, user *user.User) error {
	return nil
}

func (t *testDryRunWorker) MockSnapInfo(name string, installed, classic bool) {
	t.mockSnapInfo[name] = &system.SnapInfo{
		Installed: installed,
		Classic:   classic,
	}
}

// Verify testDryRunWorker implements system.Worker
var _ system.Worker = (*testDryRunWorker)(nil)

func TestSnapHandlerExecutesCorrectCommands(t *testing.T) {
	drw := newTestDryRunWorker()
	drw.MockSnapInfo("existing-snap", true, false)

	snaps := []*system.Snap{
		system.NewSnap("new-snap", "stable", []string{}),
		system.NewSnap("existing-snap", "stable", []string{}),
	}

	handler := NewSnapHandler(drw, snaps)
	err := handler.Prepare()
	if err != nil {
		t.Fatalf("Prepare should not fail: %v", err)
	}

	// Verify the correct commands were issued
	foundInstall := false
	foundRefresh := false
	for _, cmd := range drw.executedCmds {
		if strings.Contains(cmd, "snap install new-snap") {
			foundInstall = true
		}
		if strings.Contains(cmd, "snap refresh existing-snap") {
			foundRefresh = true
		}
	}

	if !foundInstall {
		t.Errorf("expected 'snap install new-snap' command, got: %v", drw.executedCmds)
	}
	if !foundRefresh {
		t.Errorf("expected 'snap refresh existing-snap' command, got: %v", drw.executedCmds)
	}
}

func TestSnapHandlerRestoreExecutesCorrectCommands(t *testing.T) {
	drw := newTestDryRunWorker()

	snaps := []*system.Snap{
		system.NewSnap("snap-to-remove", "stable", []string{}),
	}

	handler := NewSnapHandler(drw, snaps)
	err := handler.Restore()
	if err != nil {
		t.Fatalf("Restore should not fail: %v", err)
	}

	foundRemove := false
	for _, cmd := range drw.executedCmds {
		if strings.Contains(cmd, "snap remove snap-to-remove") {
			foundRemove = true
		}
	}

	if !foundRemove {
		t.Errorf("expected 'snap remove snap-to-remove' command, got: %v", drw.executedCmds)
	}
}

func TestDebHandlerExecutesCorrectCommands(t *testing.T) {
	drw := newTestDryRunWorker()

	debs := []*Deb{
		{Name: "make"},
		{Name: "python3"},
	}

	handler := NewDebHandler(drw, debs)
	err := handler.Prepare()
	if err != nil {
		t.Fatalf("Prepare should not fail: %v", err)
	}

	foundUpdate := false
	foundMake := false
	foundPython := false
	for _, cmd := range drw.executedCmds {
		if strings.Contains(cmd, "apt-get update") {
			foundUpdate = true
		}
		if strings.Contains(cmd, "apt-get install") && strings.Contains(cmd, "make") {
			foundMake = true
		}
		if strings.Contains(cmd, "apt-get install") && strings.Contains(cmd, "python3") {
			foundPython = true
		}
	}

	if !foundUpdate {
		t.Errorf("expected 'apt-get update' command, got: %v", drw.executedCmds)
	}
	if !foundMake {
		t.Errorf("expected 'apt-get install make' command, got: %v", drw.executedCmds)
	}
	if !foundPython {
		t.Errorf("expected 'apt-get install python3' command, got: %v", drw.executedCmds)
	}
}
