package system

import (
	"fmt"
	"os"
	"os/user"
	"path"
	"strings"
	"time"
)

// Worker is an interface for a struct that can run commands on the underlying system.
type Worker interface {
	// User returns the 'real user' the system executes command as. This may be different from
	// the current user since the command is often executed with `sudo`.
	User() *user.User
	// Run takes a single command and runs it, returning the combined output and an error value.
	Run(c *Command) ([]byte, error)
	// RunMany takes multiple commands and runs them in sequence, returning an error on the
	// first error encountered.
	RunMany(commands ...*Command) error
	// RunExclusive is a wrapper around Run that uses a mutex to ensure that only one of that
	// particular command can be run at a time.
	RunExclusive(c *Command) ([]byte, error)
	// RunWithRetries executes the command, retrying utilising an exponential backoff pattern,
	// which starts at 1 second. Retries will be attempted up to the specified maximum duration.
	RunWithRetries(c *Command, maxDuration time.Duration) ([]byte, error)
	// WriteHomeDirFile takes a path relative to the real user's home dir, and writes the contents
	// specified to it.
	WriteHomeDirFile(filepath string, contents []byte) error
	// ReadHomeDirFile reads a file from the user's home directory.
	ReadHomeDirFile(filepath string) ([]byte, error)
	// ReadFile reads a file with an arbitrary path from the system.
	ReadFile(filePath string) ([]byte, error)
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
	// Print outputs a message. In dry-run mode, outputs to stdout.
	// In normal mode, this is a no-op.
	Print(msg string)
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
