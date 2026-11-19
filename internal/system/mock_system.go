package system

import (
	"context"
	"fmt"
	"os"
	"os/user"
	"sync"
)

// NewMockSystem constructs a new mock command
func NewMockSystem() *MockSystem {
	return &MockSystem{
		CreatedFiles:     map[string]string{},
		mockReturns:      map[string]MockCommandReturn{},
		mockFiles:        map[string][]byte{},
		mockSnapInfo:     map[string]*SnapInfo{},
		mockSnapInfoErr:  map[string]error{},
		mockInstalled:    map[string]*InstalledSnapInfo{},
		mockInstalledErr: map[string]error{},
		mockSnapChannels: map[string][]string{},
		mockPaths:        map[string]bool{},
	}
}

// MockCommandReturn contains mocked Output and Error from a given command.
type MockCommandReturn struct {
	Output []byte
	Error  error
}

// MockSystem represents a struct that can emulate running commands.
type MockSystem struct {
	ExecutedCommands   []string
	CreatedFiles       map[string]string
	CreatedDirectories []string
	Deleted            []string
	RemovedPaths       []string

	// StoreLookups records the snaps passed to SnapInfo, which on a real
	// system queries the snap store.
	StoreLookups []string

	mockFiles        map[string][]byte
	mockReturns      map[string]MockCommandReturn
	mockSnapInfo     map[string]*SnapInfo
	mockSnapInfoErr  map[string]error
	mockInstalled    map[string]*InstalledSnapInfo
	mockInstalledErr map[string]error
	mockSnapChannels map[string][]string
	mockPaths        map[string]bool

	// Used to guard access to the ExecutedCommands list
	cmdMutex sync.Mutex
}

// MockCommandReturn sets a static return value representing command combined output,
// and a desired error return for the specified command.
func (r *MockSystem) MockCommandReturn(command string, b []byte, err error) {
	r.mockReturns[command] = MockCommandReturn{Output: b, Error: err}
}

// MockFile sets a faked expected file contents for a given file.
func (r *MockSystem) MockFile(filePath string, contents []byte) {
	r.mockFiles[filePath] = contents
}

// MockSnapStoreLookup gets a new test snap and adds a mock snap into the mock test
func (r *MockSystem) MockSnapStoreLookup(name, channel string, classic, installed bool) *Snap {
	trackingChannel := ""
	if installed {
		trackingChannel = channel
	}
	r.mockSnapInfo[name] = &SnapInfo{
		InstalledSnapInfo: InstalledSnapInfo{
			Installed:       installed,
			Active:          installed,
			TrackingChannel: trackingChannel,
		},
		Classic: classic,
	}
	// An installed snap is also visible to the local snapd. Share the struct so
	// that a test that changes one (say, to disable the snap) changes both.
	if installed {
		r.mockInstalled[name] = &r.mockSnapInfo[name].InstalledSnapInfo
	}
	return &Snap{Name: name, Channel: channel}
}

// MockSnapInfo sets the information returned by SnapInfo for a snap.
func (r *MockSystem) MockSnapInfo(name string, info *SnapInfo) {
	r.mockSnapInfo[name] = info
}

// MockSnapInfoError sets an error to be returned by SnapInfo for a snap,
// emulating a failure to query snapd or the snap store.
func (r *MockSystem) MockSnapInfoError(name string, err error) {
	r.mockSnapInfoErr[name] = err
}

// MockSnapInstalledInfo sets the information returned by SnapInstalledInfo for a
// snap, such as its state after an install or refresh. This is independent of the
// information set for SnapInfo, so tests can tell which of the two was called.
func (r *MockSystem) MockSnapInstalledInfo(name string, info *InstalledSnapInfo) {
	r.mockInstalled[name] = info
}

// MockSnapInstalledInfoError sets an error to be returned by SnapInstalledInfo for
// a snap, emulating a failure to query snapd.
func (r *MockSystem) MockSnapInstalledInfoError(name string, err error) {
	r.mockInstalledErr[name] = err
}

// MockSnapChannels mocks the set of available channels for a snap in the store.
func (r *MockSystem) MockSnapChannels(snap string, channels []string) {
	r.mockSnapChannels[snap] = channels
}

// User returns the user the system executes commands on behalf of.
func (r *MockSystem) User() *user.User {
	return &user.User{
		Username: "test-user",
		Uid:      "666",
		Gid:      "666",
		HomeDir:  os.TempDir(),
	}
}

// Run executes the command, returning the stdout/stderr where appropriate.
func (r *MockSystem) Run(c *Command) ([]byte, error) {
	r.cmdMutex.Lock()
	// Prevent the path of the test machine interfering with the test results.
	path := os.Getenv("PATH")
	// PATH is temporarily cleared to isolate test results from the host machine.
	// Setenv errors are ignored as they only fail on invalid key names.
	defer func() { _ = os.Setenv("PATH", path) }()
	_ = os.Setenv("PATH", "")

	cmd := c.CommandString()

	r.ExecutedCommands = append(r.ExecutedCommands, cmd)
	r.cmdMutex.Unlock()

	val, ok := r.mockReturns[cmd]
	if ok {
		return val.Output, val.Error
	}
	return []byte{}, nil
}

// ReadFile takes a path and reads the content from the specified file.
func (r *MockSystem) ReadFile(filePath string) ([]byte, error) {
	val, ok := r.mockFiles[filePath]
	if !ok {
		return nil, fmt.Errorf("file '%s' does not exist: %w", filePath, os.ErrNotExist)
	}
	return val, nil
}

// WriteFile writes the given contents to the specified file path (mocked).
func (r *MockSystem) WriteFile(filePath string, contents []byte, perm os.FileMode) error {
	r.CreatedFiles[filePath] = string(contents)
	return nil
}

// SnapInfo returns information about a given snap, looking up details in the snap
// store using the snapd client API where necessary.
func (r *MockSystem) SnapInfo(snap string, channel string) (*SnapInfo, error) {
	r.StoreLookups = append(r.StoreLookups, snap)
	if err, ok := r.mockSnapInfoErr[snap]; ok {
		return nil, err
	}

	snapInfo, ok := r.mockSnapInfo[snap]
	if ok {
		return snapInfo, nil
	}

	return &SnapInfo{}, nil
}

// SnapInstalledInfo returns information about an installed snap.
func (r *MockSystem) SnapInstalledInfo(ctx context.Context, snap string) (*InstalledSnapInfo, error) {
	if err, ok := r.mockInstalledErr[snap]; ok {
		return nil, err
	}

	if snapInfo, ok := r.mockInstalled[snap]; ok {
		return snapInfo, nil
	}

	return &InstalledSnapInfo{}, nil
}

// SnapChannels returns the list of channels available for a given snap.
func (r *MockSystem) SnapChannels(snap string) ([]string, error) {
	val, ok := r.mockSnapChannels[snap]
	if ok {
		return val, nil
	}

	return nil, fmt.Errorf("channels for snap '%s' not found", snap)
}

// RemovePath recursively removes a path from the filesystem (mocked).
func (r *MockSystem) RemovePath(path string) error {
	r.RemovedPaths = append(r.RemovedPaths, path)
	delete(r.mockPaths, path)
	return nil
}

// MkdirAll creates a directory and all parent directories (mocked).
func (r *MockSystem) MkdirAll(path string, perm os.FileMode) error {
	r.CreatedDirectories = append(r.CreatedDirectories, path)
	r.mockPaths[path] = true
	return nil
}

// ChownAll recursively changes the ownership of a path to the specified user.
func (r *MockSystem) ChownAll(path string, user *user.User) error {
	return nil
}
