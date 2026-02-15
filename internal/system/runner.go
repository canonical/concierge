package system

import (
	"context"
	"errors"
	"fmt"
	"io/fs"
	"log/slog"
	"os"
	"os/exec"
	"os/user"
	"path/filepath"
	"strconv"
	"sync"
	"time"

	"github.com/canonical/concierge/internal/snapd"
	retry "github.com/sethvargo/go-retry"
)

// NewSystem constructs a new command system.
func NewSystem(trace bool) (*System, error) {
	realUser, err := realUser()
	if err != nil {
		return nil, fmt.Errorf("failed to lookup effective user details: %w", err)
	}
	return &System{
		trace:      trace,
		user:       realUser,
		cmdMutexes: map[string]*sync.Mutex{},
		snapd:      snapd.NewClient(nil),
	}, nil
}

// System represents a struct that can run commands.
type System struct {
	trace bool
	user  *user.User
	snapd *snapd.Client
	// Guards access to cmdMutexes.
	cmdMu sync.Mutex
	// Map of mutexes to prevent the concurrent execution of certain commands.
	cmdMutexes map[string]*sync.Mutex
}

// User returns a user struct containing details of the "real" user, which
// may differ from the current user when concierge is executed with `sudo`.
func (s *System) User() *user.User { return s.user }

// Run executes the command, returning the stdout/stderr where appropriate.
// RunOptions can be provided to alter the behaviour (e.g. Exclusive, WithRetries).
func (s *System) Run(c *Command, opts ...RunOption) ([]byte, error) {
	var cfg runConfig
	for _, o := range opts {
		o(&cfg)
	}

	if cfg.exclusive {
		s.cmdMu.Lock()
		mtx, ok := s.cmdMutexes[c.Executable]
		if !ok {
			mtx = &sync.Mutex{}
			s.cmdMutexes[c.Executable] = mtx
		}
		s.cmdMu.Unlock()
		mtx.Lock()
		defer mtx.Unlock()
	}

	if cfg.maxRetryDuration > 0 {
		backoff := retry.NewExponential(1 * time.Second)
		backoff = retry.WithMaxDuration(cfg.maxRetryDuration, backoff)
		ctx := context.Background()

		return retry.DoValue(ctx, backoff, func(ctx context.Context) ([]byte, error) {
			output, err := s.runOnce(c)
			if err != nil {
				return nil, retry.RetryableError(err)
			}
			return output, nil
		})
	}

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
