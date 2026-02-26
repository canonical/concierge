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

func TestSnapConfigWithoutChannel(t *testing.T) {
	yamlConfig := `
host:
  snaps:
    charmcraft:
    jq:
    yq:
      channel: latest/edge
    jhack:
      channel: latest/stable
      connections:
        - jhack:dot-local-share-juju
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

	cfg, err := parseConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	// Bare-key snaps must be present in the map
	if _, ok := cfg.Host.Snaps["charmcraft"]; !ok {
		t.Fatal("expected charmcraft snap to be present in map")
	}
	if _, ok := cfg.Host.Snaps["jq"]; !ok {
		t.Fatal("expected jq snap to be present in map")
	}

	// Snaps with no channel should have empty string (snapd defaults to latest/stable)
	if cfg.Host.Snaps["charmcraft"].Channel != "" {
		t.Fatalf("expected empty channel for charmcraft, got: %v", cfg.Host.Snaps["charmcraft"].Channel)
	}
	if cfg.Host.Snaps["jq"].Channel != "" {
		t.Fatalf("expected empty channel for jq, got: %v", cfg.Host.Snaps["jq"].Channel)
	}

	// Snaps with explicit channel should preserve it
	if cfg.Host.Snaps["yq"].Channel != "latest/edge" {
		t.Fatalf("expected latest/edge for yq, got: %v", cfg.Host.Snaps["yq"].Channel)
	}
	if cfg.Host.Snaps["jhack"].Channel != "latest/stable" {
		t.Fatalf("expected latest/stable for jhack, got: %v", cfg.Host.Snaps["jhack"].Channel)
	}

	// Connections should work
	if len(cfg.Host.Snaps["jhack"].Connections) != 1 || cfg.Host.Snaps["jhack"].Connections[0] != "jhack:dot-local-share-juju" {
		t.Fatalf("expected jhack connections, got: %v", cfg.Host.Snaps["jhack"].Connections)
	}

	// Bare key snaps should have nil connections
	if cfg.Host.Snaps["charmcraft"].Connections != nil {
		t.Fatalf("expected nil connections for charmcraft, got: %v", cfg.Host.Snaps["charmcraft"].Connections)
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

func TestExpandEnvVars(t *testing.T) {
	type test struct {
		input    string
		envVars  map[string]string
		expected string
	}

	tests := []test{
		{
			input:    "https://example.com",
			envVars:  map[string]string{},
			expected: "https://example.com",
		},
		{
			input:    "$REGISTRY_URL",
			envVars:  map[string]string{"REGISTRY_URL": "https://mirror.example.com"},
			expected: "https://mirror.example.com",
		},
		{
			input:    "${REGISTRY_URL}",
			envVars:  map[string]string{"REGISTRY_URL": "https://mirror.example.com"},
			expected: "https://mirror.example.com",
		},
		{
			input:    "https://$HOST:$PORT/v2",
			envVars:  map[string]string{"HOST": "registry.example.com", "PORT": "5000"},
			expected: "https://registry.example.com:5000/v2",
		},
		{
			input:    "$UNDEFINED_VAR",
			envVars:  map[string]string{},
			expected: "",
		},
	}

	for _, tc := range tests {
		// Set environment variables for this test
		for k, v := range tc.envVars {
			os.Setenv(k, v)
		}

		result := expandEnvVars(tc.input)

		// Clean up environment variables
		for k := range tc.envVars {
			os.Unsetenv(k)
		}

		if result != tc.expected {
			t.Fatalf("expandEnvVars(%q): expected %q, got %q", tc.input, tc.expected, result)
		}
	}
}

func TestImageRegistryConfigFromYAML(t *testing.T) {
	yamlConfig := `
providers:
  microk8s:
    enable: true
    bootstrap: true
    image-registry:
      url: https://mirror.example.com
      username: testuser
      password: testpass
  k8s:
    enable: true
    bootstrap: true
    image-registry:
      url: https://k8s-mirror.example.com
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

	// Test MicroK8s image registry
	if cfg.Providers.MicroK8s.ImageRegistry.URL != "https://mirror.example.com" {
		t.Fatalf("expected MicroK8s image-registry URL to be 'https://mirror.example.com', got: %v", cfg.Providers.MicroK8s.ImageRegistry.URL)
	}
	if cfg.Providers.MicroK8s.ImageRegistry.Username != "testuser" {
		t.Fatalf("expected MicroK8s image-registry username to be 'testuser', got: %v", cfg.Providers.MicroK8s.ImageRegistry.Username)
	}
	if cfg.Providers.MicroK8s.ImageRegistry.Password != "testpass" {
		t.Fatalf("expected MicroK8s image-registry password to be 'testpass', got: %v", cfg.Providers.MicroK8s.ImageRegistry.Password)
	}

	// Test K8s image registry
	if cfg.Providers.K8s.ImageRegistry.URL != "https://k8s-mirror.example.com" {
		t.Fatalf("expected K8s image-registry URL to be 'https://k8s-mirror.example.com', got: %v", cfg.Providers.K8s.ImageRegistry.URL)
	}
}

func TestImageRegistryEnvVarExpansion(t *testing.T) {
	// Set test environment variables
	os.Setenv("DOCKERHUB_MIRROR", "https://dockerhub-mirror.example.com")
	os.Setenv("REGISTRY_USER", "envuser")
	os.Setenv("REGISTRY_PASS", "envpass")
	defer func() {
		os.Unsetenv("DOCKERHUB_MIRROR")
		os.Unsetenv("REGISTRY_USER")
		os.Unsetenv("REGISTRY_PASS")
	}()

	yamlConfig := `
providers:
  microk8s:
    enable: true
    bootstrap: true
    image-registry:
      url: $DOCKERHUB_MIRROR
      username: ${REGISTRY_USER}
      password: ${REGISTRY_PASS}
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

	// Expand environment variables
	expandConfigEnvVars(cfg)

	// Test that environment variables were expanded
	if cfg.Providers.MicroK8s.ImageRegistry.URL != "https://dockerhub-mirror.example.com" {
		t.Fatalf("expected URL to be expanded from env var, got: %v", cfg.Providers.MicroK8s.ImageRegistry.URL)
	}
	if cfg.Providers.MicroK8s.ImageRegistry.Username != "envuser" {
		t.Fatalf("expected username to be expanded from env var, got: %v", cfg.Providers.MicroK8s.ImageRegistry.Username)
	}
	if cfg.Providers.MicroK8s.ImageRegistry.Password != "envpass" {
		t.Fatalf("expected password to be expanded from env var, got: %v", cfg.Providers.MicroK8s.ImageRegistry.Password)
	}
}
