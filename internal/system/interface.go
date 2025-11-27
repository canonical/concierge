package system

import (
	"os/user"
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
	// MkHomeSubdirectory takes a relative folder path and creates it recursively in the real
	// user's home directory.
	MkHomeSubdirectory(subdirectory string) error
	// RemoveAllHome recursively removes a file path from the user's home directory.
	RemoveAllHome(filePath string) error
	// ReadHomeDirFile reads a file from the user's home directory.
	ReadHomeDirFile(filepath string) ([]byte, error)
	// ReadFile reads a file with an arbitrary path from the system.
	ReadFile(filePath string) ([]byte, error)
	// SnapInfo returns information about a given snap, looking up details in the snap
	// store using the snapd client API where necessary.
	SnapInfo(snap string, channel string) (*SnapInfo, error)
	// SnapChannels returns the list of channels available for a given snap.
	SnapChannels(snap string) ([]string, error)
}
