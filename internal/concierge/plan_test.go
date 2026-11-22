package concierge

import (
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

	tests := map[string][]string{
		PrepareAction: {
			"timeout 180 add-apt-repository -y -n ppa:deadsnakes/ppa",
			"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y update",
			"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y install -o Dpkg::Options::=--force-confdef -o Dpkg::Options::=--force-confold terraform",
		},
		RestoreAction: {
			"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y remove terraform",
			"DEBIAN_FRONTEND=noninteractive NEEDRESTART_MODE=a apt-get -y autoremove",
			"timeout 180 add-apt-repository -y -n -r ppa:deadsnakes/ppa",
		},
	}

	for action, expected := range tests {
		sys := system.NewMockSystem()
		if err := NewPlan(cfg, sys).Execute(action); err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
			t.Fatalf("%s: expected: %v, got: %v", action, expected, sys.ExecutedCommands)
		}
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
