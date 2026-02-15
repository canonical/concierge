package system

import (
	"fmt"
	"os"
	"os/user"
	"sync"
)

// NewMockSystem constructs a new mock command
func NewMockSystem() *MockSystem {
	return &MockSystem{
		CreatedFiles: map[string]string{},
		mockReturns:  map[string]MockCommandReturn{},
		mockFiles:    map[string][]byte{},
		mockSnapInfo: map[string]*SnapInfo{},
		mockPaths:    map[string]bool{},
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

	mockFiles        map[string][]byte
	mockReturns      map[string]MockCommandReturn
	mockSnapInfo     map[string]*SnapInfo
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
		Installed:       installed,
		Classic:         classic,
		TrackingChannel: trackingChannel,
	}
	return &Snap{Name: name, Channel: channel}
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
func (r *MockSystem) Run(c *Command, opts ...RunOption) ([]byte, error) {
	r.cmdMutex.Lock()
	// Prevent the path of the test machine interfering with the test results.
	path := os.Getenv("PATH")
	defer os.Setenv("PATH", path)
	os.Setenv("PATH", "")

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
		return nil, fmt.Errorf("file not found")
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
	snapInfo, ok := r.mockSnapInfo[snap]
	if ok {
		return snapInfo, nil
	}

	return &SnapInfo{
		Installed: false,
		Classic:   false,
	}, nil
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
