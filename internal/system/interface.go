package system

import (
	"os"
	"os/user"
)

// Worker is an interface for a struct that can run commands on the underlying system.
type Worker interface {
	// User returns the 'real user' the system executes command as. This may be different from
	// the current user since the command is often executed with `sudo`.
	User() *user.User
	// Run takes a single command and runs it, returning the combined output and an error value.
	Run(c *Command) ([]byte, error)
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
