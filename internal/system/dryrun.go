package system

import (
	"fmt"
	"io"
	"os"
	"os/user"
	"path"
	"time"
)

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

// Run prints the command that would be executed and returns success.
func (d *DryRunWorker) Run(c *Command) ([]byte, error) {
	fmt.Fprintln(d.out, c.CommandString())
	return []byte{}, nil
}

// RunMany prints each command that would be executed and returns success.
func (d *DryRunWorker) RunMany(commands ...*Command) error {
	for _, c := range commands {
		fmt.Fprintln(d.out, c.CommandString())
	}
	return nil
}

// RunExclusive prints the command that would be executed and returns success.
func (d *DryRunWorker) RunExclusive(c *Command) ([]byte, error) {
	fmt.Fprintln(d.out, c.CommandString())
	return []byte{}, nil
}

// RunWithRetries prints the command that would be executed and returns success.
func (d *DryRunWorker) RunWithRetries(c *Command, maxDuration time.Duration) ([]byte, error) {
	fmt.Fprintln(d.out, c.CommandString())
	return []byte{}, nil
}

// WriteHomeDirFile prints what file would be written and returns success.
func (d *DryRunWorker) WriteHomeDirFile(filepath string, contents []byte) error {
	fullPath := path.Join(d.realSystem.User().HomeDir, filepath)
	fmt.Fprintln(d.out, "Would write file:", fullPath)
	return nil
}

// ReadHomeDirFile delegates to real system for accurate conditional logic.
func (d *DryRunWorker) ReadHomeDirFile(filepath string) ([]byte, error) {
	return d.realSystem.ReadHomeDirFile(filepath)
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
	fmt.Fprintln(d.out, "Would remove:", path)
	return nil
}

// MkdirAll prints what directory would be created and returns success.
func (d *DryRunWorker) MkdirAll(path string, perm os.FileMode) error {
	fmt.Fprintln(d.out, "Would create directory:", path)
	return nil
}

// ChownAll prints what ownership change would occur and returns success.
func (d *DryRunWorker) ChownAll(path string, user *user.User) error {
	fmt.Fprintf(d.out, "Would chown %s to %s:%s\n", path, user.Uid, user.Gid)
	return nil
}
