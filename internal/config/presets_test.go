package config

import (
	"reflect"
	"testing"
)

func TestValidPresets(t *testing.T) {
	expected := []string{"crafts", "dev", "k8s", "machine", "microk8s"}
	got := ValidPresets()
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected: %v, got: %v", expected, got)
	}
}

func TestPresetInvalidName(t *testing.T) {
	_, err := Preset("definitely-not-a-preset")
	if err == nil {
		t.Fatal("expected error for invalid preset name, got nil")
	}
}

func TestPresetLoadsSuccessfully(t *testing.T) {
	// Every preset defines at least these bare-key snaps.
	commonSnaps := []string{"charmcraft", "jq", "yq"}

	for _, name := range ValidPresets() {
		t.Run(name, func(t *testing.T) {
			conf, err := Preset(name)
			if err != nil {
				t.Fatalf("failed to load preset '%s': %v", name, err)
			}
			if conf == nil {
				t.Fatalf("preset '%s' returned nil config", name)
			}

			for _, snap := range commonSnaps {
				if _, ok := conf.Host.Snaps[snap]; !ok {
					t.Fatalf("preset '%s': expected snap '%s' to be present", name, snap)
				}
			}
		})
	}
}
