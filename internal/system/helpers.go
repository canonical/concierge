package system

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path"
	"strings"
	"sync"
	"time"

	"github.com/sethvargo/go-retry"
)

// Per-executable mutexes preventing concurrent execution of the same command.
// sync.Map.LoadOrStore atomically returns the existing mutex or stores a new
// one, so no separate guard mutex is needed.
var cmdMutexes sync.Map

// RunExclusive acquires a per-executable mutex before running the command,
// ensuring only one instance of that executable runs at a time.
func RunExclusive(w Worker, c *Command) ([]byte, error) {
	// LoadOrStore's second return is a "loaded" bool indicating whether our
	// new mutex was stored (false) or an earlier caller's was already present
	// (true). Either way v is a valid *sync.Mutex that all racing callers
	// agree on, which is what we need for mutual exclusion.
	v, _ := cmdMutexes.LoadOrStore(c.Executable, &sync.Mutex{})
	mtx := v.(*sync.Mutex)
	mtx.Lock()
	defer mtx.Unlock()

	return w.Run(c)
}

// RunWithRetries retries the command using exponential backoff, starting at
// 1 second. Retries will be attempted up to the specified maximum duration.
// Errors that are known to be permanent (e.g. ErrNotInstalled) are returned
// immediately without retrying.
func RunWithRetries(w Worker, c *Command, maxDuration time.Duration) ([]byte, error) {
	backoff := retry.NewExponential(1 * time.Second)
	backoff = retry.WithMaxDuration(maxDuration, backoff)
	ctx := context.Background()

	return retry.DoValue(ctx, backoff, func(ctx context.Context) ([]byte, error) {
		output, err := w.Run(c)
		if err != nil {
			if errors.Is(err, ErrNotInstalled) {
				return nil, err
			}
			return nil, retry.RetryableError(err)
		}

		return output, nil
	})
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
