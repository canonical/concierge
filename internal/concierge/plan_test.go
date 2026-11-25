package concierge

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestPlanRepositoriesWrapDebs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Juju.Disable = true
	cfg.Host.Packages = []string{"terraform"}
	cfg.Host.Repositories = map[string]config.RepositoryConfig{"deadsnakes": {PPA: "deadsnakes/ppa"}}

	keyCommand := "curl -fsL --proto =https --max-time 30 --retry 10 --retry-connrefused --retry-max-time 60 'https://api.launchpad.net/devel/~deadsnakes/+archive/ubuntu/ppa?ws.op=getSigningKeyData'"
	sourcesFile := "/etc/apt/sources.list.d/concierge-deadsnakes.sources"

	sys := system.NewMockSystem()
	sys.MockFile("/etc/os-release", []byte("VERSION_CODENAME=noble\n"))
	sys.MockCommandReturn(keyCommand, []byte(`"-----BEGIN PGP PUBLIC KEY BLOCK-----\n-----END PGP PUBLIC KEY BLOCK-----\n"`), nil)
	if err := NewPlan(cfg, sys).Execute(PrepareAction); err != nil {
		t.Fatal(err)
	}
	expected := []string{
		keyCommand,
		"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update --error-on=any -o Dir::Etc::sourcelist=" + sourcesFile + " -o Dir::Etc::sourceparts=- -o APT::Get::List-Cleanup=0",
		"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update",
		"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y install -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold terraform",
	}
	if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
		t.Fatalf("prepare: expected: %v, got: %v", expected, sys.ExecutedCommands)
	}

	sys = system.NewMockSystem()
	if err := NewPlan(cfg, sys).Execute(RestoreAction); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual([]string{sourcesFile}, sys.RemovedPaths) {
		t.Fatalf("restore: expected %s removed, got: %v", sourcesFile, sys.RemovedPaths)
	}

	// The repository is removed after the debs, so it is kept if removing them fails.
	sys = system.NewMockSystem()
	sys.MockCommandReturn("DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y remove terraform", nil, fmt.Errorf("exit status 100"))
	if err := NewPlan(cfg, sys).Execute(RestoreAction); err == nil {
		t.Fatal("expected an error, got nil")
	}
	if len(sys.RemovedPaths) != 0 {
		t.Fatalf("restore: expected the repository to be kept, got removed: %v", sys.RemovedPaths)
	}
}

func TestGetSnapChannelOverride(t *testing.T) {
	type test struct {
		snap     string
		expected string
	}

	config := &config.Config{}
	config.Overrides.CharmcraftChannel = "latest/edge"
	config.Overrides.RockcraftChannel = "latest/edge"
	config.Overrides.SnapcraftChannel = "latest/edge"

	tests := []test{
		{snap: "snapcraft", expected: "latest/edge"},
		{snap: "rockcraft", expected: "latest/edge"},
		{snap: "charmcraft", expected: "latest/edge"},
		{snap: "foobar", expected: ""},
	}

	for _, tc := range tests {
		override := getSnapChannelOverride(config, tc.snap)
		if !reflect.DeepEqual(tc.expected, override) {
			t.Fatalf("expected: %v, got: %v", tc.expected, override)
		}
	}
}
