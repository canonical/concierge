package packages

import (
	"reflect"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

const testKey = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBGO9\n-----END PGP PUBLIC KEY BLOCK-----\n"

var testRepos = map[string]config.RepositoryConfig{
	"deadsnakes": {PPA: "deadsnakes/ppa"},
	"hashicorp": {
		URL:        "https://apt.releases.hashicorp.com",
		Key:        "https://apt.releases.hashicorp.com/gpg",
		Components: []string{"main"},
	},
}

func TestRepoHandlerPrepare(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockFile("/etc/os-release", []byte("NAME=\"Ubuntu\"\nVERSION_CODENAME=noble\n"))
	sys.MockCommandReturn("curl -fsSL --max-time 30 https://apt.releases.hashicorp.com/gpg", []byte(testKey), nil)

	if err := NewRepoHandler(sys, testRepos).Prepare(); err != nil {
		t.Fatal(err)
	}

	expectedCommands := []string{
		"timeout 180 add-apt-repository -y -n ppa:deadsnakes/ppa",
		"curl -fsSL --max-time 30 https://apt.releases.hashicorp.com/gpg",
	}
	if !reflect.DeepEqual(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}

	expectedFile := `Types: deb
URIs: https://apt.releases.hashicorp.com
Suites: noble
Components: main
Signed-By:
 -----BEGIN PGP PUBLIC KEY BLOCK-----
 .
 mQINBGO9
 -----END PGP PUBLIC KEY BLOCK-----
`
	got := sys.CreatedFiles["/etc/apt/sources.list.d/concierge-hashicorp.sources"]
	if got != expectedFile {
		t.Fatalf("expected file:\n%s\ngot:\n%s", expectedFile, got)
	}
}

func TestRepoHandlerRestore(t *testing.T) {
	sys := system.NewMockSystem()

	if err := NewRepoHandler(sys, testRepos).Restore(); err != nil {
		t.Fatal(err)
	}

	expectedCommands := []string{"timeout 180 add-apt-repository -y -n -r ppa:deadsnakes/ppa"}
	if !reflect.DeepEqual(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}
	expectedRemoved := []string{"/etc/apt/sources.list.d/concierge-hashicorp.sources"}
	if !reflect.DeepEqual(expectedRemoved, sys.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemoved, sys.RemovedPaths)
	}
}

func TestRepoHandlerExplicitSuite(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockCommandReturn("curl -fsSL --max-time 30 https://example.com/key.asc", []byte(testKey), nil)

	repos := map[string]config.RepositoryConfig{
		"example": {URL: "https://example.com/apt", Key: "https://example.com/key.asc", Suite: "stable", Components: []string{"main", "extra"}},
	}
	if err := NewRepoHandler(sys, repos).Prepare(); err != nil {
		t.Fatal(err)
	}

	got := sys.CreatedFiles["/etc/apt/sources.list.d/concierge-example.sources"]
	if !strings.Contains(got, "Suites: stable\nComponents: main extra\n") {
		t.Fatalf("unexpected sources file:\n%s", got)
	}
}

func TestRepoHandlerInvalid(t *testing.T) {
	valid := config.RepositoryConfig{URL: "https://example.com/apt", Key: "https://example.com/key.asc", Suite: "stable", Components: []string{"main"}}

	tests := map[string]struct {
		name string
		repo config.RepositoryConfig
		key  string
	}{
		"path traversal in name": {name: "../evil", repo: valid, key: testKey},
		"url without scheme":     {name: "r", repo: config.RepositoryConfig{URL: "example.com", Key: valid.Key, Suite: "stable", Components: []string{"main"}}, key: testKey},
		"key over http":          {name: "r", repo: config.RepositoryConfig{URL: valid.URL, Key: "http://example.com/key.asc", Suite: "stable", Components: []string{"main"}}, key: testKey},
		"no components":          {name: "r", repo: config.RepositoryConfig{URL: valid.URL, Key: valid.Key, Suite: "stable"}, key: testKey},
		"neither ppa nor url":    {name: "r", repo: config.RepositoryConfig{}, key: testKey},
		"binary key":             {name: "r", repo: valid, key: "\x99\x02\x0d"},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			sys := system.NewMockSystem()
			sys.MockCommandReturn("curl -fsSL --max-time 30 https://example.com/key.asc", []byte(tc.key), nil)

			err := NewRepoHandler(sys, map[string]config.RepositoryConfig{tc.name: tc.repo}).Prepare()
			if err == nil {
				t.Fatal("expected an error, got nil")
			}
			if len(sys.CreatedFiles) != 0 {
				t.Fatalf("expected no files written, got: %v", sys.CreatedFiles)
			}
		})
	}
}
