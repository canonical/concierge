package system

import (
	"bytes"
	"errors"
	"os"
	"os/user"
	"testing"
	"time"
)

func TestDryRunWorkerAutoPrintsCommands(t *testing.T) {
	var buf bytes.Buffer
	drw := &DryRunWorker{
		realSystem: nil,
		out:        &buf,
	}

	cmd := NewCommand("echo", []string{"hello", "world"})

	// Test Run - should auto-print the command
	output, err := drw.Run(cmd)
	if err != nil {
		t.Fatalf("Run should not return error, got: %v", err)
	}
	if len(output) != 0 {
		t.Fatalf("Run should return empty output, got: %v", output)
	}
	// Check that the command is printed exactly (copy-paste friendly)
	if buf.String() != "echo hello world\n" {
		t.Fatalf("Run should print command, got: %q", buf.String())
	}

	buf.Reset()

	// Test RunMany - should auto-print each command
	err = drw.RunMany(cmd, cmd)
	if err != nil {
		t.Fatalf("RunMany should not return error, got: %v", err)
	}
	if buf.String() != "echo hello world\necho hello world\n" {
		t.Fatalf("RunMany should print 2 commands, got: %q", buf.String())
	}

	buf.Reset()

	// Test RunExclusive - should auto-print the command
	output, err = drw.RunExclusive(cmd)
	if err != nil {
		t.Fatalf("RunExclusive should not return error, got: %v", err)
	}
	if buf.String() != "echo hello world\n" {
		t.Fatalf("RunExclusive should print command, got: %q", buf.String())
	}

	buf.Reset()

	// Test RunWithRetries - should auto-print the command
	output, err = drw.RunWithRetries(cmd, 1*time.Second)
	if err != nil {
		t.Fatalf("RunWithRetries should not return error, got: %v", err)
	}
	if buf.String() != "echo hello world\n" {
		t.Fatalf("RunWithRetries should print command, got: %q", buf.String())
	}
}

func TestDryRunWorkerAutoPrintsFileOperations(t *testing.T) {
	var buf bytes.Buffer

	// Use a mock system for realSystem since WriteHomeDirFile needs User().HomeDir
	mock := NewMockSystem()

	drw := &DryRunWorker{
		realSystem: mock,
		out:        &buf,
	}

	// Test WriteHomeDirFile - should print as a comment (not directly executable)
	err := drw.WriteHomeDirFile("test/path", []byte("content"))
	if err != nil {
		t.Fatalf("WriteHomeDirFile should not return error, got: %v", err)
	}
	expectedPath := mock.User().HomeDir + "/test/path"
	if buf.String() != "# Write file: "+expectedPath+"\n" {
		t.Fatalf("WriteHomeDirFile should print file path, got: %q", buf.String())
	}

	buf.Reset()

	// Test MkdirAll - should print shell command
	err = drw.MkdirAll("/test/path", os.ModePerm)
	if err != nil {
		t.Fatalf("MkdirAll should not return error, got: %v", err)
	}
	if buf.String() != "mkdir -p /test/path\n" {
		t.Fatalf("MkdirAll should print mkdir command, got: %q", buf.String())
	}

	buf.Reset()

	// Test RemovePath - should print shell command
	err = drw.RemovePath("/test/path")
	if err != nil {
		t.Fatalf("RemovePath should not return error, got: %v", err)
	}
	if buf.String() != "rm -rf /test/path\n" {
		t.Fatalf("RemovePath should print rm command, got: %q", buf.String())
	}

	buf.Reset()

	// Test ChownAll - should print shell command
	err = drw.ChownAll("/test/path", &user.User{Uid: "1000", Gid: "1000"})
	if err != nil {
		t.Fatalf("ChownAll should not return error, got: %v", err)
	}
	if buf.String() != "chown -R 1000:1000 /test/path\n" {
		t.Fatalf("ChownAll should print chown command, got: %q", buf.String())
	}
}

func TestDryRunWorkerReadOnlyCommandsDelegateToRealSystem(t *testing.T) {
	mock := NewMockSystem()
	mock.MockCommandReturn("echo hello", []byte("hello\n"), nil)

	var buf bytes.Buffer
	drw := &DryRunWorker{
		realSystem: mock,
		out:        &buf,
	}

	// A ReadOnly command should delegate to the real system, not print
	cmd := NewCommand("echo", []string{"hello"})
	cmd.ReadOnly = true
	output, err := drw.Run(cmd)
	if err != nil {
		t.Fatalf("ReadOnly Run should delegate to real system, got error: %v", err)
	}
	if string(output) != "hello\n" {
		t.Fatalf("ReadOnly Run should return real output, got: %q", string(output))
	}
	if buf.Len() != 0 {
		t.Fatalf("ReadOnly Run should not print anything, got: %q", buf.String())
	}
}

func TestDryRunWorkerReadOnlyReturnsErrNotInstalled(t *testing.T) {
	mock := NewMockSystem()

	var buf bytes.Buffer
	drw := &DryRunWorker{
		realSystem: mock,
		out:        &buf,
	}

	// A ReadOnly command for a binary that doesn't exist should return ErrNotInstalled
	cmd := NewCommand("nonexistent-binary-xyz", []string{"status"})
	cmd.ReadOnly = true
	_, err := drw.Run(cmd)
	if !errors.Is(err, ErrNotInstalled) {
		t.Fatalf("ReadOnly Run with missing binary should return ErrNotInstalled, got: %v", err)
	}
	if buf.Len() != 0 {
		t.Fatalf("ReadOnly Run should not print anything, got: %q", buf.String())
	}
}

func TestDryRunWorkerDelegatesReadOperations(t *testing.T) {
	// Create a mock system with test data
	mock := NewMockSystem()
	mock.MockFile("test-file.txt", []byte("test content"))
	mock.MockFile("home/dir/file.txt", []byte("home content"))
	mock.MockSnapStoreLookup("test-snap", "stable", false, true)
	mock.MockSnapChannels("test-snap", []string{"stable", "edge", "beta"})

	var buf bytes.Buffer
	drw := &DryRunWorker{
		realSystem: mock,
		out:        &buf,
	}

	// Test User() delegates to real system
	user := drw.User()
	if user.Username != "test-user" {
		t.Fatalf("User() should delegate to real system, got username: %s", user.Username)
	}

	// Test ReadFile delegates to real system
	content, err := drw.ReadFile("test-file.txt")
	if err != nil {
		t.Fatalf("ReadFile should delegate to real system, got error: %v", err)
	}
	if string(content) != "test content" {
		t.Fatalf("ReadFile should return mock content, got: %s", string(content))
	}

	// Test ReadHomeDirFile delegates to real system
	content, err = drw.ReadHomeDirFile("home/dir/file.txt")
	if err != nil {
		t.Fatalf("ReadHomeDirFile should delegate to real system, got error: %v", err)
	}
	if string(content) != "home content" {
		t.Fatalf("ReadHomeDirFile should return mock content, got: %s", string(content))
	}

	// Test SnapInfo delegates to real system
	snapInfo, err := drw.SnapInfo("test-snap", "stable")
	if err != nil {
		t.Fatalf("SnapInfo should delegate to real system, got error: %v", err)
	}
	if !snapInfo.Installed {
		t.Fatalf("SnapInfo should return mock data showing snap is installed")
	}

	// Test SnapChannels delegates to real system
	channels, err := drw.SnapChannels("test-snap")
	if err != nil {
		t.Fatalf("SnapChannels should delegate to real system, got error: %v", err)
	}
	if len(channels) != 3 || channels[0] != "stable" {
		t.Fatalf("SnapChannels should return mock channels, got: %v", channels)
	}
}
