package system

import (
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
	"os/user"
)

// ErrNotInstalled is returned by DryRunWorker when a read-only command's
// binary is not found on the system.
var ErrNotInstalled = errors.New("command not installed")

// DryRunWorker is a Worker implementation that outputs what would be done
// without actually executing any commands or making any changes.
type DryRunWorker struct {
	realSystem Worker
	out        io.Writer
}

// NewDryRunWorker constructs a new DryRunWorker that wraps a real System
// for read operations while skipping execution operations.
func NewDryRunWorker(realSystem Worker) *DryRunWorker {
	return &DryRunWorker{
		realSystem: realSystem,
		out:        os.Stdout,
	}
}

// User returns the real user - delegates to real system.
func (d *DryRunWorker) User() *user.User {
	return d.realSystem.User()
}

// runReadOnly delegates a read-only command to the real system if the binary
// is available. If the binary is not installed, it returns ErrNotInstalled.
func (d *DryRunWorker) runReadOnly(c *Command) ([]byte, error) {
	_, err := exec.LookPath(c.Executable)
	if err != nil {
		return nil, ErrNotInstalled
	}
	return d.realSystem.Run(c)
}

// Run prints the command that would be executed and returns success.
// Read-only commands are delegated to the real system for accurate results.
func (d *DryRunWorker) Run(c *Command) ([]byte, error) {
	if c.ReadOnly {
		return d.runReadOnly(c)
	}
	fmt.Fprintln(d.out, c.CommandString())
	return []byte{}, nil
}

// WriteFile prints what file would be written and returns success.
func (d *DryRunWorker) WriteFile(filePath string, contents []byte, perm os.FileMode) error {
	fmt.Fprintln(d.out, "# Write file:", filePath)
	return nil
}

// ReadFile delegates to real system for accurate conditional logic.
func (d *DryRunWorker) ReadFile(filePath string) ([]byte, error) {
	return d.realSystem.ReadFile(filePath)
}

// SnapInfo delegates to real system for accurate conditional logic.
func (d *DryRunWorker) SnapInfo(snap string, channel string) (*SnapInfo, error) {
	return d.realSystem.SnapInfo(snap, channel)
}

// SnapChannels delegates to real system for accurate conditional logic.
func (d *DryRunWorker) SnapChannels(snap string) ([]string, error) {
	return d.realSystem.SnapChannels(snap)
}

// RemovePath prints what path would be removed and returns success.
func (d *DryRunWorker) RemovePath(path string) error {
	fmt.Fprintln(d.out, "rm -rf", path)
	return nil
}

// MkdirAll prints what directory would be created and returns success.
func (d *DryRunWorker) MkdirAll(path string, perm os.FileMode) error {
	fmt.Fprintln(d.out, "mkdir -p", path)
	return nil
}

// ChownAll prints what ownership change would occur and returns success.
func (d *DryRunWorker) ChownAll(path string, user *user.User) error {
	fmt.Fprintln(d.out, "chown -R", user.Uid+":"+user.Gid, path)
	return nil
}
