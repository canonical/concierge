package system

import (
	"os/exec"
	"os/user"
	"testing"
)

func TestLookupUserGetent(t *testing.T) {
	// Verify getent is available on this system.
	if _, err := exec.LookPath("getent"); err != nil {
		t.Skip("getent not available on this system")
	}

	// user.Current works even for SSSD/LDAP users not in /etc/passwd, because
	// it reads the UID from /proc/self rather than searching /etc/passwd by
	// name. Use it as the reference to verify lookupUserGetent returns the
	// same information.
	current, err := user.Current()
	if err != nil {
		t.Fatalf("user.Current() failed: %v", err)
	}

	got, err := lookupUserGetent(current.Username)
	if err != nil {
		t.Fatalf("lookupUserGetent(%q) failed: %v", current.Username, err)
	}

	if got.Username != current.Username {
		t.Errorf("Username: got %q, want %q", got.Username, current.Username)
	}
	if got.Uid != current.Uid {
		t.Errorf("Uid: got %q, want %q", got.Uid, current.Uid)
	}
	if got.Gid != current.Gid {
		t.Errorf("Gid: got %q, want %q", got.Gid, current.Gid)
	}
	if got.HomeDir != current.HomeDir {
		t.Errorf("HomeDir: got %q, want %q", got.HomeDir, current.HomeDir)
	}
}

func TestLookupUserGetentUnknownUser(t *testing.T) {
	if _, err := exec.LookPath("getent"); err != nil {
		t.Skip("getent not available on this system")
	}

	_, err := lookupUserGetent("nonexistent-user-that-should-not-exist")
	if err == nil {
		t.Fatal("expected error for nonexistent user, got nil")
	}
}
