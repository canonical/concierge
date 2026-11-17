package concierge

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/services"
	"github.com/canonical/concierge/internal/system"
)

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

func TestExecuteServicePhaseOrdering(t *testing.T) {
	cfg := &config.Config{}
	cfg.Juju.Disable = true

	fake := &fakeService{}

	plan := &Plan{config: cfg, system: system.NewMockSystem(), Services: []services.Service{fake}}

	if err := plan.Execute(PrepareAction); err != nil {
		t.Fatal(err)
	}
	if !fake.prepared || !fake.integrated {
		t.Fatalf("expected Prepare() and Integrate() to be called on prepare, got: %+v", fake)
	}

	fake = &fakeService{}
	plan.Services = []services.Service{fake}

	if err := plan.Execute(RestoreAction); err != nil {
		t.Fatal(err)
	}
	if !fake.disintegrated || !fake.restored {
		t.Fatalf("expected Disintegrate() and Restore() to be called on restore, got: %+v", fake)
	}
}
