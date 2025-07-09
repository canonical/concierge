package system

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/user"

	"github.com/fatih/color"
)

// generateTraceMessage creates a formatted string that is written to stdout, representing
// a command and it's output when concierge is run with `--trace`.
func generateTraceMessage(cmd string, output []byte) string {
	green := color.New(color.FgGreen, color.Bold, color.Underline)
	bold := color.New(color.Bold)

	result := fmt.Sprintf("%s %s\n", green.Sprint("Command:"), bold.Sprint(cmd))
	if len(output) > 0 {
		result = fmt.Sprintf("%s%s\n%s", result, green.Sprintf("Output:"), string(output))
	}
	return result
}

// getShellPath tries to find the path to the user's preferred shell, as per the `SHELL“
// environment variable. If that cannot be found, it looks for a path to "bash", and to
// "sh" in that order. If no shell can be found, then an error is returned.
func getShellPath() (string, error) {
	// If the `SHELL` var is set, return that.
	shellVar := os.Getenv("SHELL")
	if len(shellVar) > 0 {
		return shellVar, nil
	}

	// Try both the command name (to lookup in PATH), and common default paths.
	for _, shell := range []string{"bash", "/bin/bash", "sh", "/bin/sh"} {
		// Check if the shell path exists
		if _, err := os.Stat(shell); errors.Is(err, os.ErrNotExist) {
			// If the path doesn't exist, the lookup the value in the `PATH` variable
			path, err := exec.LookPath(shell)
			if err != nil {
				continue
			}
			return path, nil
		}
		return shell, nil
	}

	return "", fmt.Errorf("could not find path to a shell")
}

// realUser returns a user struct containing details of the "real" user, which
// may differ from the current user when concierge is executed with `sudo`.
func realUser() (*user.User, error) {
	realUser := os.Getenv("SUDO_USER")
	if len(realUser) == 0 {
		return user.Lookup("root")
	}

	return user.Lookup(realUser)
}
