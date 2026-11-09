package concierge

import (
	"testing"

	"github.com/canonical/concierge/internal/services"
)

// fakeService is a minimal services.Service implementation used to test
// dispatch logic without depending on a real service's side effects.
type fakeService struct {
	prepared      bool
	integrated    bool
	disintegrated bool
	restored      bool
}

func (f *fakeService) Prepare() error      { f.prepared = true; return nil }
func (f *fakeService) Integrate() error    { f.integrated = true; return nil }
func (f *fakeService) Disintegrate() error { f.disintegrated = true; return nil }
func (f *fakeService) Restore() error      { f.restored = true; return nil }
func (f *fakeService) Name() string        { return "fake" }

func TestDoServicePrePhase(t *testing.T) {
	var s services.Service = &fakeService{}
	fake := s.(*fakeService)

	if err := doServicePrePhase(s, PrepareAction); err != nil {
		t.Fatal(err)
	}
	if !fake.prepared {
		t.Fatal("expected Prepare() to be called")
	}

	if err := doServicePrePhase(s, RestoreAction); err != nil {
		t.Fatal(err)
	}
	if !fake.disintegrated {
		t.Fatal("expected Disintegrate() to be called")
	}

	if err := doServicePrePhase(s, "unknown"); err == nil {
		t.Fatal("expected an error for an unknown action")
	}
}

func TestDoServicePostPhase(t *testing.T) {
	var s services.Service = &fakeService{}
	fake := s.(*fakeService)

	if err := doServicePostPhase(s, PrepareAction); err != nil {
		t.Fatal(err)
	}
	if !fake.integrated {
		t.Fatal("expected Integrate() to be called")
	}

	if err := doServicePostPhase(s, RestoreAction); err != nil {
		t.Fatal(err)
	}
	if !fake.restored {
		t.Fatal("expected Restore() to be called")
	}

	if err := doServicePostPhase(s, "unknown"); err == nil {
		t.Fatal("expected an error for an unknown action")
	}
}
