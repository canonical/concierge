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
	want := config.BootstrappedController{Cloud: "google", UUID: "6a1b2c3d-0000-4000-8000-000000000001"}
	cached, err := yaml.Marshal(&config.Config{
		BootstrappedControllers: map[string]config.BootstrappedController{"gce-dev": want},
	})
	if err != nil {
		t.Fatal(err)
	}
	cachePath := path.Join(os.TempDir(), ".cache", "concierge", "concierge.yaml")
	sys.MockFile(cachePath, cached)

	if got := m.previousBootstrappedControllers()["gce-dev"]; got != want {
		t.Fatalf("expected gce-dev carried forward as %+v, got: %+v", want, got)
	}

	// A cache that can't be parsed is reported and treated as empty, rather
	// than failing the prepare.
	sys.MockFile(cachePath, []byte("bootstrapped-controllers: [not, a, map"))
	if got := m.previousBootstrappedControllers(); got != nil {
		t.Fatalf("expected nil for an unparseable cache, got: %v", got)
	}
}
