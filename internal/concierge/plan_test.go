package concierge

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
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
