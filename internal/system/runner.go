package system

import (
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"time"

	"github.com/canonical/concierge/internal/snapd"
)

// NewSystem constructs a new command system.
func NewSystem(trace bool) (*System, error) {
	realUser, err := realUser()
	if err != nil {
		return nil, fmt.Errorf("failed to lookup effective user details: %w", err)
	}
	return &System{
		trace: trace,
		user:  realUser,
		snapd: snapd.NewClient(nil),
	}, nil
}

// System represents a struct that can run commands.
type System struct {
	trace bool
	user  *user.User
	snapd *snapd.Client
}

// User returns a user struct containing details of the "real" user, which
// may differ from the current user when concierge is executed with `sudo`.
func (s *System) User() *user.User { return s.user }

// Run executes the command, returning the stdout/stderr where appropriate.
func (s *System) Run(c *Command) ([]byte, error) {
	return s.runOnce(c)
}

// runOnce executes the command a single time.
func (s *System) runOnce(c *Command) ([]byte, error) {
	logger := slog.Default()
	if len(c.User) > 0 {
		logger = slog.With("user", c.User)
	}
	if len(c.Group) > 0 {
		logger = slog.With("group", c.Group)
	}

	shell, err := getShellPath()
	if err != nil {
		return nil, fmt.Errorf("unable to determine shell path to run command")
	}

	commandString := c.CommandString()
	cmd := exec.Command(shell, "-c", commandString)

	logger.Debug("Starting command", "command", commandString)

	start := time.Now()
	output, err := cmd.CombinedOutput()

	elapsed := time.Since(start)
	logger.Debug("Finished command", "command", commandString, "elapsed", elapsed)

	if s.trace || err != nil {
		fmt.Print(generateTraceMessage(commandString, output))
	}

	return output, err
}

// ReadFile takes a path and reads the content from the specified file.
func (s *System) ReadFile(filePath string) ([]byte, error) {
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("file '%s' does not exist: %w", filePath, err)
	}
	return os.ReadFile(filePath)
}

// WriteFile writes the given contents to the specified file path with the given permissions.
func (s *System) WriteFile(filePath string, contents []byte, perm os.FileMode) error {
	return os.WriteFile(filePath, contents, perm)
}

// ChownAll recursively changes the ownership of a path to the specified user.
func (s *System) ChownAll(path string, user *user.User) error {
	uid, err := strconv.Atoi(user.Uid)
	if err != nil {
		return fmt.Errorf("failed to convert user id string to int: %w", err)
	}
	gid, err := strconv.Atoi(user.Gid)
	if err != nil {
		return fmt.Errorf("failed to convert group id string to int: %w", err)
	}

	err = filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}

		err = os.Lchown(path, uid, gid)
		if err != nil {
			return err
		}

		return nil
	})

	slog.Debug("Filesystem ownership changed", "user", user.Username, "group", user.Gid, "path", path)
	return err
}

// RemovePath recursively removes a path from the filesystem.
func (s *System) RemovePath(path string) error {
	return os.RemoveAll(path)
}

// MkdirAll creates a directory and all parent directories with the specified permissions.
func (s *System) MkdirAll(path string, perm os.FileMode) error {
	return os.MkdirAll(path, perm)
}
