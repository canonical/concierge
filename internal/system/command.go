package system

import (
	"log/slog"
	"os/exec"
	"regexp"

	"github.com/canonical/x-go/strutil/shlex"
)

// Command represents a given command to be executed by Concierge, along with the
// user and group that should be assumed if required.
type Command struct {
	Executable string
	Args       []string
	User       string
	Group      string
	// ReadOnly indicates that the command only reads system state and makes no changes.
	// In dry-run mode, read-only commands are executed for real to provide accurate
	// conditional logic (e.g., checking if a service is already bootstrapped).
	ReadOnly bool
	// ExpectedError is a regular expression pattern that, if set, is matched against
	// the combined output of a failed command. If the pattern matches, the output is
	// logged at trace level rather than printed by default. This is used for commands
	// where certain errors are an expected part of normal operation (e.g., checking
	// if a controller exists before bootstrapping).
	ExpectedError string
}

// NewCommand constructs a command to be run as the current user/group.
func NewCommand(executable string, args []string) *Command {
	return &Command{
		Executable: executable,
		Args:       args,
		User:       "",
		Group:      "",
	}
}

// NewCommandAs constructs a command to be run as the specified user/group.
func NewCommandAs(user string, group string, executable string, args []string) *Command {
	if user == "root" {
		return NewCommand(executable, args)
	}

	return &Command{
		Executable: executable,
		Args:       args,
		User:       user,
		Group:      group,
	}
}

// IsExpectedError reports whether the given output matches the ExpectedError pattern.
// Returns false if ExpectedError is empty or the pattern fails to compile.
func (c *Command) IsExpectedError(output []byte) bool {
	if c.ExpectedError == "" {
		return false
	}
	re, err := regexp.Compile(c.ExpectedError)
	if err != nil {
		slog.Warn("Invalid ExpectedError regex", "pattern", c.ExpectedError, "error", err)
		return false
	}
	return re.Match(output)
}

// CommandString puts together a command to be executed in a shell, including the `sudo`
// command and its arguments where appropriate. It verifies the executable exists via
// LookPath for consistent error behavior, but uses the short name in output.
func (c *Command) CommandString() string {
	_, err := exec.LookPath(c.Executable)
	if err != nil {
		slog.Debug("Failed to lookup command in path", "command", c.Executable)
	}

	cmdArgs := []string{}

	if len(c.User) > 0 || len(c.Group) > 0 {
		cmdArgs = append(cmdArgs, "sudo")
	}

	if len(c.User) > 0 {
		cmdArgs = append(cmdArgs, "-u", c.User)
	}

	if len(c.Group) > 0 {
		cmdArgs = append(cmdArgs, "-g", c.Group)
	}

	cmdArgs = append(cmdArgs, c.Executable)
	cmdArgs = append(cmdArgs, c.Args...)

	return shlex.Join(cmdArgs)
}
