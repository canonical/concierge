package system

import (
	"reflect"
	"testing"
)

func TestNewCommand(t *testing.T) {
	expected := &Command{
		Executable: "juju",
		Args:       []string{"add-model", "testing"},
		User:       "",
		Group:      "",
	}

	command := NewCommand("juju", []string{"add-model", "testing"})
	if !reflect.DeepEqual(expected, command) {
		t.Fatalf("expected: %v, got: %v", expected, command)
	}
}

func TestNewCommandAs(t *testing.T) {
	expected := &Command{
		Executable: "apt-get",
		Args:       []string{"install", "-y", "cowsay"},
		User:       "test-user",
		Group:      "foo",
	}

	command := NewCommandAs("test-user", "foo", "apt-get", []string{"install", "-y", "cowsay"})
	if !reflect.DeepEqual(expected, command) {
		t.Fatalf("expected: %+v, got: %+v", expected, command)
	}
}

func TestNewCommandAsRoot(t *testing.T) {
	expected := &Command{
		Executable: "apt-get",
		Args:       []string{"install", "-y", "cowsay"},
		User:       "",
		Group:      "",
	}

	command := NewCommandAs("root", "foo", "apt-get", []string{"install", "-y", "cowsay"})
	if !reflect.DeepEqual(expected, command) {
		t.Fatalf("expected: %+v, got: %+v", expected, command)
	}
}

func TestIsExpectedError(t *testing.T) {
	tests := []struct {
		name          string
		expectedError string
		output        string
		want          bool
	}{
		{
			name:          "empty pattern never matches",
			expectedError: "",
			output:        "some error output",
			want:          false,
		},
		{
			name:          "matching pattern",
			expectedError: `controller \S+ not found`,
			output:        "controller my-controller not found",
			want:          true,
		},
		{
			name:          "non-matching pattern",
			expectedError: `controller \S+ not found`,
			output:        "connection refused",
			want:          false,
		},
		{
			name:          "match anywhere in output",
			expectedError: `not part of a Kubernetes cluster`,
			output:        "Error: The node is not part of a Kubernetes cluster. Please run k8s bootstrap",
			want:          true,
		},
		{
			name:          "invalid regex returns false",
			expectedError: `[invalid`,
			output:        "some output",
			want:          false,
		},
		{
			name:          "dot-star matches anything",
			expectedError: `.*`,
			output:        "any error",
			want:          true,
		},
		{
			name:          "dot-star matches empty output",
			expectedError: `.*`,
			output:        "",
			want:          true,
		},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			cmd := &Command{ExpectedError: tc.expectedError}
			got := cmd.IsExpectedError([]byte(tc.output))
			if got != tc.want {
				t.Fatalf("IsExpectedError(%q) with pattern %q: got %v, want %v",
					tc.output, tc.expectedError, got, tc.want)
			}
		})
	}
}

func TestCommandString(t *testing.T) {
	type test struct {
		command  *Command
		expected string
	}

	// Use CONCIERGE_TEST_COMMAND to avoid $PATH lookups making tests flaky
	tests := []test{
		{
			command:  NewCommand("CONCIERGE_TEST_COMMAND", []string{"add-model", "testing"}),
			expected: "CONCIERGE_TEST_COMMAND add-model testing",
		},
		{
			command:  NewCommandAs("test-user", "", "CONCIERGE_TEST_COMMAND", []string{"install", "-y", "cowsay"}),
			expected: "sudo -u test-user CONCIERGE_TEST_COMMAND install -y cowsay",
		},
		{
			command:  NewCommandAs("test-user", "apters", "CONCIERGE_TEST_COMMAND", []string{"install", "-y", "cowsay"}),
			expected: "sudo -u test-user -g apters CONCIERGE_TEST_COMMAND install -y cowsay",
		},
	}

	for _, tc := range tests {
		commandString := tc.command.CommandString()
		if !reflect.DeepEqual(tc.expected, commandString) {
			t.Fatalf("expected: %v, got: %v", tc.expected, commandString)
		}
	}
}
