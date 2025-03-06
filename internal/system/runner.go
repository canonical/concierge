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
	"path"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"

	retry "github.com/sethvargo/go-retry"
	client "github.com/snapcore/snapd/client"
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
		snapd:      *client.New(nil),
	}, nil
}

// System represents a struct that can run commands.
type System struct {
	trace bool
	user  *user.User
	snapd client.Client
	// Map of mutexes to prevent the concurrent execution of certain commands.
	cmdMutexes map[string]*sync.Mutex
}

// User returns a user struct containing details of the "real" user, which
// may differ from the current user when concierge is executed with `sudo`.
func (s *System) User() *user.User { return s.user }

// Run executes the command, returning the stdout/stderr where appropriate.
func (s *System) Run(c *Command) ([]byte, error) {
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

// RunWithRetries executes the command, retrying utilising an exponential backoff pattern,
// which starts at 1 second. Retries will be attempted up to the specified maximum duration.
func (s *System) RunWithRetries(c *Command, maxDuration time.Duration) ([]byte, error) {
	backoff := retry.NewExponential(1 * time.Second)
	backoff = retry.WithMaxDuration(maxDuration, backoff)
	ctx := context.Background()

	return retry.DoValue(ctx, backoff, func(ctx context.Context) ([]byte, error) {
		output, err := s.Run(c)
		if err != nil {
			return nil, retry.RetryableError(err)
		}

		return output, nil
	})
}

// RunMany takes a variadic number of Command's, and runs them in a loop, returning
// and error if any command fails.
func (s *System) RunMany(commands ...*Command) error {
	for _, cmd := range commands {
		_, err := s.Run(cmd)
		if err != nil {
			return err
		}
	}
	return nil
}

// RunExclusive is a wrapper around Run that uses a mutex to ensure that only one of that
// particular command can be run at a time.
func (s *System) RunExclusive(c *Command) ([]byte, error) {
	mtx, ok := s.cmdMutexes[c.Executable]
	if !ok {
		mtx = &sync.Mutex{}
		s.cmdMutexes[c.Executable] = mtx
	}

	mtx.Lock()
	defer mtx.Unlock()

	output, err := s.Run(c)
	return output, err
}

// WriteHomeDirFile takes a path relative to the real user's home dir, and writes the contents
// specified to it.
func (s *System) WriteHomeDirFile(filePath string, contents []byte) error {
	dir := path.Dir(filePath)

	err := s.MkHomeSubdirectory(dir)
	if err != nil {
		return err
	}

	filePath = path.Join(path.Join(s.user.HomeDir, filePath))

	if err := os.WriteFile(filePath, contents, 0644); err != nil {
		return fmt.Errorf("failed to write file '%s': %w", filePath, err)
	}

	err = s.chownRecursively(filePath, s.user)
	if err != nil {
		return fmt.Errorf("failed to change ownership of file '%s': %w", filePath, err)
	}

	return nil
}

// MkHomeSubdirectory takes a relative folder path and creates it recursively in the real
// user's home directory.
func (s *System) MkHomeSubdirectory(subdirectory string) error {
	if path.IsAbs(subdirectory) {
		return fmt.Errorf("only relative paths supported")
	}

	dir := path.Join(s.user.HomeDir, subdirectory)

	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	parts := strings.Split(subdirectory, "/")
	if len(parts) > 0 {
		dir = path.Join(s.user.HomeDir, parts[0])
	}

	err = s.chownRecursively(dir, s.user)
	if err != nil {
		return fmt.Errorf("failed to change ownership of directory '%s': %w", dir, err)
	}

	return nil
}

// ReadHomeDirFile takes a path relative to the real user's home dir, and reads the content
// from the file
func (s *System) ReadHomeDirFile(filePath string) ([]byte, error) {
	homePath := path.Join(s.user.HomeDir, filePath)
	return s.ReadFile(homePath)
}

// ReadFile takes a path and reads the content from the specified file.
func (s *System) ReadFile(filePath string) ([]byte, error) {
	if _, err := os.Stat(filePath); errors.Is(err, os.ErrNotExist) {
		return nil, fmt.Errorf("file '%s' does not exist: %w", filePath, err)
	}
	return os.ReadFile(filePath)
}

// RemoveAllHome recursively removes a file path from the user's home directory.
func (s *System) RemoveAllHome(filePath string) error {
	return os.RemoveAll(path.Join(s.user.HomeDir, filePath))
}

// ChownRecursively recursively changes ownership of a given filepath to the uid/gid of
// the specified user.
func (s *System) chownRecursively(path string, user *user.User) error {
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

		err = os.Chown(path, uid, gid)
		if err != nil {
			return err
		}

		return nil
	})

	slog.Debug("Filesystem ownership changed", "user", user.Username, "group", user.Gid, "path", path)
	return err
}
