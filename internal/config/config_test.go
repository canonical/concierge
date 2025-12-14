package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/spf13/viper"
)

func TestFlagToEnvVar(t *testing.T) {
	type test struct {
		flag     string
		expected string
	}

	viper.SetEnvPrefix("CONCIERGE")

	tests := []test{
		{flag: "juju-channel", expected: "CONCIERGE_JUJU_CHANNEL"},
		{flag: "rockcraft-channel", expected: "CONCIERGE_ROCKCRAFT_CHANNEL"},
		{flag: "foobar", expected: "CONCIERGE_FOOBAR"},
	}

	for _, tc := range tests {
		ev := flagToEnvVar(tc.flag)
		if !reflect.DeepEqual(tc.expected, ev) {
			t.Fatalf("expected: %v, got: %v", tc.expected, ev)
		}
	}
}

func TestMapMerge(t *testing.T) {
	type test struct {
		m1       map[string]string
		m2       map[string]string
		expected map[string]string
	}

	tests := []test{
		{
			m1:       map[string]string{"foo": "bar", "baz": "qux"},
			m2:       map[string]string{"foo": "baz"},
			expected: map[string]string{"foo": "baz", "baz": "qux"},
		},
		{
			m1:       map[string]string{},
			m2:       map[string]string{"foo": "baz"},
			expected: map[string]string{"foo": "baz"},
		},
		{
			m1:       map[string]string{"foo": "baz"},
			m2:       map[string]string{},
			expected: map[string]string{"foo": "baz"},
		},
		{
			m1:       map[string]string{"foo": "baz"},
			m2:       map[string]string{"baz": "qux"},
			expected: map[string]string{"foo": "baz", "baz": "qux"},
		},
	}

	for _, tc := range tests {
		merged := MergeMaps(tc.m1, tc.m2)
		if !reflect.DeepEqual(tc.expected, merged) {
			t.Fatalf("expected: %v, got: %v", tc.expected, merged)
		}
	}
}

func TestExtraBootstrapArgsFromYAML(t *testing.T) {
	yamlConfig := `
juju:
  channel: 3.6/stable
  extra-bootstrap-args: --config idle-connection-timeout=90s --auto-upgrade=true

providers:
  lxd:
    enable: true
    bootstrap: false
`

	// Write to a temporary file
	tmpFile, err := os.CreateTemp("", "concierge-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer os.Remove(tmpFile.Name())

	if _, err := tmpFile.Write([]byte(yamlConfig)); err != nil {
		t.Fatal(err)
	}
	tmpFile.Close()

	// Reset viper
	viper.Reset()
	viper.SetConfigType("yaml")
	viper.SetConfigFile(tmpFile.Name())

	err = viper.ReadInConfig()
	if err != nil {
		t.Fatalf("Failed to read config: %v", err)
	}

	cfg := &Config{}
	err = viper.Unmarshal(cfg)
	if err != nil {
		t.Fatalf("Failed to unmarshal config: %v", err)
	}

	expected := "--config idle-connection-timeout=90s --auto-upgrade=true"
	if cfg.Juju.ExtraBootstrapArgs != expected {
		t.Fatalf("expected: %v, got: %v", expected, cfg.Juju.ExtraBootstrapArgs)
	}
}
