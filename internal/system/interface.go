package system

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"
	"time"
)

// RunOption is a functional option for configuring the behaviour of Run.
type RunOption func(*runConfig)

type runConfig struct {
	exclusive        bool
	maxRetryDuration time.Duration
}

// Exclusive returns a RunOption that causes Run to acquire a per-executable
// mutex, ensuring only one instance of that executable runs at a time.
func Exclusive() RunOption {
	return func(c *runConfig) { c.exclusive = true }
}

// WithRetries returns a RunOption that causes Run to retry the command using
// exponential backoff, up to the specified maximum duration.
func WithRetries(maxDuration time.Duration) RunOption {
	return func(c *runConfig) { c.maxRetryDuration = maxDuration }
}

// Worker is an interface for a struct that can run commands on the underlying system.
type Worker interface {
	// User returns the 'real user' the system executes command as. This may be different from
	// the current user since the command is often executed with `sudo`.
	User() *user.User
	// Run takes a single command and runs it, returning the combined output and an error value.
	// RunOptions can be provided to alter the behaviour (e.g. Exclusive, WithRetries).
	Run(c *Command, opts ...RunOption) ([]byte, error)
	// ReadFile reads a file with an arbitrary path from the system.
	ReadFile(filePath string) ([]byte, error)
	// WriteFile writes the given contents to the specified file path with the given permissions.
	WriteFile(filePath string, contents []byte, perm os.FileMode) error
	// SnapInfo returns information about a given snap, looking up details in the snap
	// store using the snapd client API where necessary.
	SnapInfo(snap string, channel string) (*SnapInfo, error)
	// SnapChannels returns the list of channels available for a given snap.
	SnapChannels(snap string) ([]string, error)
	// RemovePath recursively removes a path from the filesystem.
	RemovePath(path string) error
	// MkdirAll creates a directory and all parent directories with the specified permissions.
	MkdirAll(path string, perm os.FileMode) error
	// ChownAll recursively changes the ownership of a path to the specified user.
	ChownAll(path string, user *user.User) error
}

// RunMany takes multiple commands and runs them in sequence via the Worker,
// returning an error on the first error encountered.
func RunMany(w Worker, commands ...*Command) error {
	for _, cmd := range commands {
		_, err := w.Run(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

// ReadHomeDirFile reads a file at a path relative to the real user's home directory.
func ReadHomeDirFile(w Worker, filePath string) ([]byte, error) {
	homePath := path.Join(w.User().HomeDir, filePath)
	return w.ReadFile(homePath)
}

// WriteHomeDirFile writes contents to a path relative to the real user's home directory,
// creating parent directories and adjusting ownership as needed.
func WriteHomeDirFile(w Worker, filePath string, contents []byte) error {
	dir := path.Dir(filePath)

	err := MkHomeSubdirectory(w, dir)
	if err != nil {
		return err
	}

	absPath := path.Join(w.User().HomeDir, filePath)

	if err := w.WriteFile(absPath, contents, 0644); err != nil {
		return fmt.Errorf("failed to write file '%s': %w", absPath, err)
	}

	err = w.ChownAll(absPath, w.User())
	if err != nil {
		return fmt.Errorf("failed to change ownership of file '%s': %w", absPath, err)
	}

	return nil
}

// MkHomeSubdirectory is a helper function that takes a relative folder path and creates it
// recursively in the real user's home directory using the Worker interface.
func MkHomeSubdirectory(w Worker, subdirectory string) error {
	if path.IsAbs(subdirectory) {
		return fmt.Errorf("only relative paths supported")
	}

	user := w.User()
	dir := path.Join(user.HomeDir, subdirectory)

	err := w.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	parts := strings.Split(subdirectory, "/")
	if len(parts) > 0 {
		dir = path.Join(user.HomeDir, parts[0])
	}

	err = w.ChownAll(dir, user)
	if err != nil {
		return fmt.Errorf("failed to change ownership of directory '%s': %w", dir, err)
	}

	return nil
}
