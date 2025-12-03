package system

import (
	"fmt"
	"os"
	"os/user"
	"sync"
	"time"
)

// NewMockSystem constructs a new mock command
func NewMockSystem() *MockSystem {
	return &MockSystem{
		CreatedFiles: map[string]string{},
		mockReturns:  map[string]MockCommandReturn{},
		mockFiles:    map[string][]byte{},
		mockSnapInfo: map[string]*SnapInfo{},
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

	mockFiles        map[string][]byte
	mockReturns      map[string]MockCommandReturn
	mockSnapInfo     map[string]*SnapInfo
	mockSnapChannels map[string][]string

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
func (r *MockSystem) Run(c *Command) ([]byte, error) {
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

// RunWithRetries executes the command, retrying utilising an exponential backoff pattern,
// which starts at 1 second. Retries will be attempted up to the specified maximum duration.
func (r *MockSystem) RunWithRetries(c *Command, maxDuration time.Duration) ([]byte, error) {
	return r.Run(c)
}

// RunMany takes a variadic number of Command's, and runs them in a loop, returning
// and error if any command fails.
func (r *MockSystem) RunMany(commands ...*Command) error {
	for _, cmd := range commands {
		_, err := r.Run(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunExclusive is a wrapper around Run that uses a mutex to ensure that only one of that
// particular command can be run at a time.
func (r *MockSystem) RunExclusive(c *Command) ([]byte, error) {
	return r.Run(c)
}

// WriteHomeDirFile takes a path relative to the real user's home dir, and writes the contents
// specified to it.
func (r *MockSystem) WriteHomeDirFile(filepath string, contents []byte) error {
	r.CreatedFiles[filepath] = string(contents)
	return nil
}

// MkHomeSubdirectory takes a relative folder path and creates it recursively in the real
// user's home directory.
func (r *MockSystem) MkHomeSubdirectory(subdirectory string) error {
	r.CreatedDirectories = append(r.CreatedDirectories, subdirectory)
	return nil
}

// ReadHomeDirFile takes a path relative to the real user's home dir, and reads the content
// from the file
func (r *MockSystem) ReadHomeDirFile(filePath string) ([]byte, error) {
	val, ok := r.mockFiles[filePath]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	return val, nil
}

// ReadFile takes a path and reads the content from the specified file.
func (r *MockSystem) ReadFile(filePath string) ([]byte, error) {
	val, ok := r.mockFiles[filePath]
	if !ok {
		return nil, fmt.Errorf("file not found")
	}
	return val, nil
}

// RemoveAllHome recursively removes a file path from the user's home directory.
func (r *MockSystem) RemoveAllHome(filePath string) error {
	r.Deleted = append(r.Deleted, filePath)
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
