package system

import (
	"bytes"
	"os"
	"os/user"
	"strings"
	"testing"
	"time"
)

func TestDryRunWorkerPrint(t *testing.T) {
	// Create a buffer to capture output
	var buf bytes.Buffer

	drw := &DryRunWorker{
		realSystem: nil,
		out:        &buf,
	}

	drw.Print("test message")

	expected := "test message\n"
	if buf.String() != expected {
		t.Fatalf("expected: %q, got: %q", expected, buf.String())
	}
}

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
	// Check for "Would run:" prefix and the arguments (path may vary by system)
	if !strings.Contains(buf.String(), "Would run:") || !strings.Contains(buf.String(), "echo hello world") {
		t.Fatalf("Run should print command, got: %s", buf.String())
	}

	buf.Reset()

	// Test RunMany - should auto-print each command
	err = drw.RunMany(cmd, cmd)
	if err != nil {
		t.Fatalf("RunMany should not return error, got: %v", err)
	}
	if strings.Count(buf.String(), "Would run:") != 2 {
		t.Fatalf("RunMany should print 2 commands, got: %s", buf.String())
	}

	buf.Reset()

	// Test RunExclusive - should auto-print the command
	output, err = drw.RunExclusive(cmd)
	if err != nil {
		t.Fatalf("RunExclusive should not return error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would run:") {
		t.Fatalf("RunExclusive should print command, got: %s", buf.String())
	}

	buf.Reset()

	// Test RunWithRetries - should auto-print the command
	output, err = drw.RunWithRetries(cmd, 1*time.Second)
	if err != nil {
		t.Fatalf("RunWithRetries should not return error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would run:") {
		t.Fatalf("RunWithRetries should print command, got: %s", buf.String())
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

	// Test WriteHomeDirFile - should print what would be written
	err := drw.WriteHomeDirFile("test/path", []byte("content"))
	if err != nil {
		t.Fatalf("WriteHomeDirFile should not return error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would write file:") {
		t.Fatalf("WriteHomeDirFile should print file path, got: %s", buf.String())
	}

	buf.Reset()

	// Test MkdirAll - should print what directory would be created
	err = drw.MkdirAll("/test/path", os.ModePerm)
	if err != nil {
		t.Fatalf("MkdirAll should not return error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would create directory: /test/path") {
		t.Fatalf("MkdirAll should print directory path, got: %s", buf.String())
	}

	buf.Reset()

	// Test RemovePath - should print what would be removed
	err = drw.RemovePath("/test/path")
	if err != nil {
		t.Fatalf("RemovePath should not return error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would remove: /test/path") {
		t.Fatalf("RemovePath should print path, got: %s", buf.String())
	}

	buf.Reset()

	// Test ChownAll - should print what ownership change would occur
	err = drw.ChownAll("/test/path", &user.User{Uid: "1000", Gid: "1000"})
	if err != nil {
		t.Fatalf("ChownAll should not return error, got: %v", err)
	}
	if !strings.Contains(buf.String(), "Would chown /test/path") {
		t.Fatalf("ChownAll should print chown info, got: %s", buf.String())
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

func TestDryRunWorkerImplementsWorkerInterface(t *testing.T) {
	// Verify DryRunWorker implements Worker interface
	var _ Worker = (*DryRunWorker)(nil)
}
