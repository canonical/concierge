package concierge

import (
	"os"
	"path"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
	"gopkg.in/yaml.v3"
)

// TestPreviousBootstrappedControllers checks that a prepare reads the
// controllers recorded by an earlier prepare out of the cached runtime config,
// so they can be carried forward and still torn down on restore.
func TestPreviousBootstrappedControllers(t *testing.T) {
	sys := system.NewMockSystem()
	m := &Manager{config: &config.Config{}, system: sys}

	// With no cache on disk, there is nothing to carry forward.
	if got := m.previousBootstrappedControllers(); got != nil {
		t.Fatalf("expected nil with no cached config, got: %v", got)
	}

	// Seed a cached runtime config recording a bootstrapped controller.
	cached, err := yaml.Marshal(&config.Config{
		BootstrappedControllers: map[string]bool{"gce-dev": true},
	})
	if err != nil {
		t.Fatal(err)
	}
	sys.MockFile(path.Join(os.TempDir(), ".cache", "concierge", "concierge.yaml"), cached)

	got := m.previousBootstrappedControllers()
	if !got["gce-dev"] {
		t.Fatalf("expected gce-dev to be carried forward, got: %v", got)
	}
}
