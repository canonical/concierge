package system

import (
	"os"
	"os/user"
	"path"
	"strconv"
	"syscall"
	"testing"
)

// setUmask sets the process umask and returns the previous value, so that a test
// can check the mode that concierge asks for rather than the mode the ambient
// umask happens to allow.
func setUmask(t *testing.T, mask int) int {
	t.Helper()
	return syscall.Umask(mask)
}

// realFileSystem is a Worker that writes to a real filesystem, so that tests can
// assert on the permissions that end up on disk. The MockSystem discards the
// mode it is given, which is exactly the detail these tests are about.
type realFileSystem struct {
	*MockSystem
	user *user.User
}

func (r *realFileSystem) User() *user.User { return r.user }

func (r *realFileSystem) WriteFile(filePath string, contents []byte, perm os.FileMode) error {
	return os.WriteFile(filePath, contents, perm)
}

func (r *realFileSystem) MkdirAll(dirPath string, perm os.FileMode) error {
	return os.MkdirAll(dirPath, perm)
}

// ChownAll is a no-op: the test user already owns the temporary directory, and
// changing ownership needs privileges the test doesn't have.
func (r *realFileSystem) ChownAll(string, *user.User) error { return nil }

func newRealFileSystem(t *testing.T) *realFileSystem {
	t.Helper()
	return &realFileSystem{
		MockSystem: NewMockSystem(),
		user: &user.User{
			HomeDir:  t.TempDir(),
			Uid:      strconv.Itoa(os.Getuid()),
			Gid:      strconv.Itoa(os.Getgid()),
			Username: "test",
		},
	}
}

// TestWriteHomeDirFilePermissions checks that files written into the user's home
// directory are readable only by that user. Every caller of WriteHomeDirFile
// writes a file that can carry secrets, so a wider mode would expose Juju cloud
// credentials, a kubeconfig, or a registry password to other local users.
func TestWriteHomeDirFilePermissions(t *testing.T) {
	w := newRealFileSystem(t)

	err := WriteHomeDirFile(w, path.Join(".local", "share", "juju", "credentials.yaml"), []byte("credentials: {}\n"))
	if err != nil {
		t.Fatalf("WriteHomeDirFile returned an error: %v", err)
	}

	written := path.Join(w.User().HomeDir, ".local", "share", "juju", "credentials.yaml")
	info, err := os.Stat(written)
	if err != nil {
		t.Fatalf("stat of the written file failed: %v", err)
	}

	if got, want := info.Mode().Perm(), os.FileMode(0600); got != want {
		t.Errorf("file mode is %#o, want %#o", got, want)
	}
}

// TestMkHomeSubdirectoryPermissions checks that the directories created along the
// way are not group- or world-writable. The mode is set explicitly rather than
// left to the umask, because concierge runs as root and a permissive umask would
// otherwise produce a world-writable directory.
func TestMkHomeSubdirectoryPermissions(t *testing.T) {
	umask := setUmask(t, 0)
	defer setUmask(t, umask)

	w := newRealFileSystem(t)

	if err := MkHomeSubdirectory(w, path.Join(".local", "share", "juju")); err != nil {
		t.Fatalf("MkHomeSubdirectory returned an error: %v", err)
	}

	for _, dir := range []string{".local", path.Join(".local", "share"), path.Join(".local", "share", "juju")} {
		info, err := os.Stat(path.Join(w.User().HomeDir, dir))
		if err != nil {
			t.Fatalf("stat of %q failed: %v", dir, err)
		}
		if got, want := info.Mode().Perm(), os.FileMode(0755); got != want {
			t.Errorf("mode of %q is %#o, want %#o", dir, got, want)
		}
	}
}
