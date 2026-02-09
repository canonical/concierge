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

func TestPresetLoadsSuccessfully(t *testing.T) {
	for _, name := range ValidPresets() {
		t.Run(name, func(t *testing.T) {
			conf, err := Preset(name)
			if err != nil {
				t.Fatalf("failed to load preset '%s': %v", name, err)
			}
			if conf == nil {
				t.Fatalf("preset '%s' returned nil config", name)
			}
		})
	}
}
