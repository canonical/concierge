package packages

import (
	"fmt"
	"reflect"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

const testKey = "-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBGO9\n-----END PGP PUBLIC KEY BLOCK-----\n"

// launchpadKey is testKey encoded as a JSON string, the way getSigningKeyData returns it.
const launchpadKey = `"-----BEGIN PGP PUBLIC KEY BLOCK-----\n\nmQINBGO9\n-----END PGP PUBLIC KEY BLOCK-----\n"`

const launchpadKeyCommand = "curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 'https://api.launchpad.net/devel/~deadsnakes/+archive/ubuntu/ppa?ws.op=getSigningKeyData'"

const testKeyFile = `Signed-By:
 -----BEGIN PGP PUBLIC KEY BLOCK-----
 .
 mQINBGO9
 -----END PGP PUBLIC KEY BLOCK-----
`

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
	sys.MockCommandReturn("curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 https://apt.releases.hashicorp.com/gpg", []byte(testKey), nil)
	sys.MockCommandReturn(launchpadKeyCommand, []byte(launchpadKey), nil)

	if err := NewRepoHandler(sys, testRepos).Prepare(); err != nil {
		t.Fatal(err)
	}

	expectedCommands := []string{
		launchpadKeyCommand,
		"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update --error-on=any -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/concierge-deadsnakes.sources -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0",
		"curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 https://apt.releases.hashicorp.com/gpg",
		"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update --error-on=any -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/concierge-hashicorp.sources -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0",
	}
	if !reflect.DeepEqual(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}

	expectedFiles := map[string]string{
		"/etc/apt/sources.list.d/concierge-hashicorp.sources":  "Types: deb\nURIs: https://apt.releases.hashicorp.com\nSuites: noble\nComponents: main\n" + testKeyFile,
		"/etc/apt/sources.list.d/concierge-deadsnakes.sources": "Types: deb\nURIs: https://ppa.launchpadcontent.net/deadsnakes/ppa/ubuntu\nSuites: noble\nComponents: main\n" + testKeyFile,
	}
	if !reflect.DeepEqual(expectedFiles, sys.CreatedFiles) {
		t.Fatalf("expected files: %v, got: %v", expectedFiles, sys.CreatedFiles)
	}
}

func TestRepoHandlerRestore(t *testing.T) {
	sys := system.NewMockSystem()

	if err := NewRepoHandler(sys, testRepos).Restore(); err != nil {
		t.Fatal(err)
	}

	// Only concierge's own files are removed, so a PPA added before prepare stays.
	if len(sys.ExecutedCommands) != 0 {
		t.Fatalf("expected no commands, got: %v", sys.ExecutedCommands)
	}
	expectedRemoved := []string{
		"/etc/apt/sources.list.d/concierge-deadsnakes.sources",
		"/etc/apt/sources.list.d/concierge-hashicorp.sources",
	}
	if !reflect.DeepEqual(expectedRemoved, sys.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemoved, sys.RemovedPaths)
	}
}

func TestRepoHandlerBadRepositoryRemoved(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockFile("/etc/os-release", []byte("VERSION_CODENAME=noble\n"))
	sys.MockCommandReturn("curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 https://apt.releases.hashicorp.com/gpg", []byte(testKey), nil)
	sys.MockCommandReturn("DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update --error-on=any -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/concierge-hashicorp.sources -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0", nil, fmt.Errorf("exit status 100"))

	repos := map[string]config.RepositoryConfig{"hashicorp": testRepos["hashicorp"]}
	if err := NewRepoHandler(sys, repos).Prepare(); err == nil {
		t.Fatal("expected an error, got nil")
	}

	expectedRemoved := []string{"/etc/apt/sources.list.d/concierge-hashicorp.sources"}
	if !reflect.DeepEqual(expectedRemoved, sys.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemoved, sys.RemovedPaths)
	}
}

func TestRepoHandlerPPAWithoutReleaseRemoved(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockFile("/etc/os-release", []byte("VERSION_CODENAME=noble\n"))
	sys.MockCommandReturn(launchpadKeyCommand, []byte(launchpadKey), nil)
	sys.MockCommandReturn("DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update --error-on=any -o Dir::Etc::sourcelist=/etc/apt/sources.list.d/concierge-deadsnakes.sources -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0", nil, fmt.Errorf("exit status 100"))

	repos := map[string]config.RepositoryConfig{"deadsnakes": testRepos["deadsnakes"]}
	err := NewRepoHandler(sys, repos).Prepare()
	if err == nil || !strings.Contains(err.Error(), "check that the PPA has packages for noble") {
		t.Fatalf("expected an error about the PPA's packages, got: %v", err)
	}

	expectedRemoved := []string{"/etc/apt/sources.list.d/concierge-deadsnakes.sources"}
	if !reflect.DeepEqual(expectedRemoved, sys.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemoved, sys.RemovedPaths)
	}
}

func TestRepoHandlerMissingPPA(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockFile("/etc/os-release", []byte("VERSION_CODENAME=noble\n"))
	// curl retries transient errors itself. A 404 from Launchpad exits with 22 and isn't retried.
	sys.MockCommandReturn(launchpadKeyCommand, nil, fmt.Errorf("exit status 22"))

	repos := map[string]config.RepositoryConfig{"deadsnakes": testRepos["deadsnakes"]}
	if err := NewRepoHandler(sys, repos).Prepare(); err == nil {
		t.Fatal("expected an error, got nil")
	}

	if !reflect.DeepEqual([]string{launchpadKeyCommand}, sys.ExecutedCommands) {
		t.Fatalf("expected a single download attempt, got: %v", sys.ExecutedCommands)
	}
	if len(sys.CreatedFiles) != 0 {
		t.Fatalf("expected no files written, got: %v", sys.CreatedFiles)
	}
}

func TestRepoHandlerExplicitSuite(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockCommandReturn("curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 https://example.com/key.asc", []byte(testKey), nil)

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
		name    string
		repo    config.RepositoryConfig
		wantErr string
	}{
		"path traversal in name": {name: "../evil", repo: valid, wantErr: "invalid repository name"},
		"url without scheme":     {name: "r", repo: config.RepositoryConfig{URL: "example.com", Key: valid.Key, Suite: "stable", Components: []string{"main"}}, wantErr: "url must start with"},
		"key over http":          {name: "r", repo: config.RepositoryConfig{URL: valid.URL, Key: "http://example.com/key.asc", Suite: "stable", Components: []string{"main"}}, wantErr: "key must be an https:// URL"},
		"no components":          {name: "r", repo: config.RepositoryConfig{URL: valid.URL, Key: valid.Key, Suite: "stable"}, wantErr: "at least one component"},
		"neither ppa nor url":    {name: "r", repo: config.RepositoryConfig{}, wantErr: "either ppa or url must be set"},
		"ppa and url":            {name: "r", repo: config.RepositoryConfig{PPA: "deadsnakes/ppa", URL: valid.URL}, wantErr: "ppa cannot be combined"},
		"invalid ppa":            {name: "r", repo: config.RepositoryConfig{PPA: "deadsnakes/ppa --foo"}, wantErr: "ppa must be in the form user/name"},
		"newline in suite":       {name: "r", repo: config.RepositoryConfig{URL: valid.URL, Key: valid.Key, Suite: "stable\nTrusted: yes", Components: []string{"main"}}, wantErr: "cannot contain whitespace"},
		"space in a component":   {name: "r", repo: config.RepositoryConfig{URL: valid.URL, Key: valid.Key, Suite: "stable", Components: []string{"main contrib"}}, wantErr: "cannot contain whitespace"},
	}

	for desc, tc := range tests {
		t.Run(desc, func(t *testing.T) {
			err := ValidateRepository(tc.name, tc.repo)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected an error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}

	// The key's contents can only be checked once it is downloaded.
	sys := system.NewMockSystem()
	sys.MockCommandReturn("curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 https://example.com/key.asc", []byte("\x99\x02\x0d"), nil)
	err := NewRepoHandler(sys, map[string]config.RepositoryConfig{"r": valid}).Prepare()
	if err == nil || !strings.Contains(err.Error(), "not an ASCII-armored PGP public key") {
		t.Fatalf("expected a key format error, got: %v", err)
	}
	if len(sys.CreatedFiles) != 0 {
		t.Fatalf("expected no files written, got: %v", sys.CreatedFiles)
	}
}
