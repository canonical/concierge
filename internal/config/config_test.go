package config

import (
	"os"
	"reflect"
	"testing"

	"github.com/spf13/cobra"
	"github.com/spf13/pflag"
)

func TestFlagToEnvVar(t *testing.T) {
	type test struct {
		flag     string
		expected string
	}

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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.Write([]byte(yamlConfig)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

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

func TestK8sFeaturesWithoutConfig(t *testing.T) {
	yamlConfig := `
providers:
  k8s:
    enable: true
    bootstrap: true
    features:
      dns:
      ingress:
      local-storage:
      network:
      load-balancer:
        l2-mode: "true"
        cidrs: 10.0.0.0/24
`

	tmpFile, err := os.CreateTemp("", "concierge-test-*.yaml")
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.Write([]byte(yamlConfig)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

	for _, name := range []string{"dns", "ingress", "local-storage", "network", "load-balancer"} {
		if _, ok := cfg.Providers.K8s.Features[name]; !ok {
			t.Fatalf("expected k8s feature %q to be present in map, got: %v", name, cfg.Providers.K8s.Features)
		}
	}

	lb := cfg.Providers.K8s.Features["load-balancer"]
	if lb["l2-mode"] != "true" || lb["cidrs"] != "10.0.0.0/24" {
		t.Fatalf("expected load-balancer config preserved, got: %v", lb)
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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.Write([]byte(yamlConfig)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
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
			t.Setenv(k, v)
		}

		result := expandEnvVars(tc.input)

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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.Write([]byte(yamlConfig)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
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
	t.Setenv("DOCKERHUB_MIRROR", "https://dockerhub-mirror.example.com")
	t.Setenv("REGISTRY_USER", "envuser")
	t.Setenv("REGISTRY_PASS", "envpass")

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
	defer func() { _ = os.Remove(tmpFile.Name()) }()

	if _, err := tmpFile.Write([]byte(yamlConfig)); err != nil {
		t.Fatal(err)
	}
	if err := tmpFile.Close(); err != nil {
		t.Fatal(err)
	}

	cfg, err := parseConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("Failed to parse config: %v", err)
	}

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

func TestEnvOrFlagBool(t *testing.T) {
	tests := []struct {
		name        string
		flagDefault bool
		envSet      bool
		envValue    string
		want        bool
	}{
		{name: "no env, flag default false", flagDefault: false, want: false},
		{name: "no env, flag default true", flagDefault: true, want: true},
		{name: "env true overrides flag false", flagDefault: false, envSet: true, envValue: "true", want: true},
		{name: "env 1 overrides flag false", flagDefault: false, envSet: true, envValue: "1", want: true},
		{name: "env false does not override flag true", flagDefault: true, envSet: true, envValue: "false", want: true},
		{name: "invalid env value is ignored", flagDefault: false, envSet: true, envValue: "notabool", want: false},
		{name: "empty env is ignored", flagDefault: true, envSet: true, envValue: "", want: true},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			flags.Bool("disable-juju", tc.flagDefault, "")
			if tc.envSet {
				t.Setenv("CONCIERGE_DISABLE_JUJU", tc.envValue)
			} else {
				_ = os.Unsetenv("CONCIERGE_DISABLE_JUJU")
			}
			got := envOrFlagBool(flags, "disable-juju")
			if got != tc.want {
				t.Fatalf("envOrFlagBool: want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestEnvOrFlagString(t *testing.T) {
	tests := []struct {
		name        string
		flagDefault string
		envSet      bool
		envValue    string
		want        string
	}{
		{name: "no env returns flag default", flagDefault: "stable", want: "stable"},
		{name: "env overrides flag", flagDefault: "stable", envSet: true, envValue: "edge", want: "edge"},
		{name: "empty env does not override", flagDefault: "stable", envSet: true, envValue: "", want: "stable"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			flags.String("juju-channel", tc.flagDefault, "")
			if tc.envSet {
				t.Setenv("CONCIERGE_JUJU_CHANNEL", tc.envValue)
			} else {
				_ = os.Unsetenv("CONCIERGE_JUJU_CHANNEL")
			}
			got := envOrFlagString(flags, "juju-channel")
			if got != tc.want {
				t.Fatalf("envOrFlagString: want %q, got %q", tc.want, got)
			}
		})
	}
}

func TestEnvOrFlagSlice(t *testing.T) {
	tests := []struct {
		name        string
		flagDefault []string
		envSet      bool
		envValue    string
		want        []string
	}{
		{name: "no env returns flag default", flagDefault: []string{"a", "b"}, want: []string{"a", "b"}},
		{name: "empty env returns flag default", flagDefault: []string{"a"}, envSet: true, envValue: "", want: []string{"a"}},
		{name: "env appends to flag default", flagDefault: []string{"a"}, envSet: true, envValue: "b,c", want: []string{"a", "b", "c"}},
		{name: "single env value appends", flagDefault: nil, envSet: true, envValue: "only", want: []string{"only"}},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			flags := pflag.NewFlagSet("test", pflag.ContinueOnError)
			flags.StringSlice("extra-snaps", tc.flagDefault, "")
			if tc.envSet {
				t.Setenv("CONCIERGE_EXTRA_SNAPS", tc.envValue)
			} else {
				_ = os.Unsetenv("CONCIERGE_EXTRA_SNAPS")
			}
			got := envOrFlagSlice(flags, "extra-snaps")
			if !reflect.DeepEqual(got, tc.want) {
				t.Fatalf("envOrFlagSlice: want %v, got %v", tc.want, got)
			}
		})
	}
}

func TestBindFlagsAppliesEnvVar(t *testing.T) {
	cmd := &cobra.Command{Use: "concierge"}
	cmd.Flags().String("juju-channel", "stable", "")
	t.Setenv("CONCIERGE_JUJU_CHANNEL", "edge")

	bindFlags(cmd)

	got, err := cmd.Flags().GetString("juju-channel")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if got != "edge" {
		t.Fatalf("want flag set to %q from env, got %q", "edge", got)
	}
}

func TestBindFlagsDoesNotOverrideChangedFlag(t *testing.T) {
	cmd := &cobra.Command{Use: "concierge"}
	cmd.Flags().String("juju-channel", "stable", "")
	if err := cmd.Flags().Set("juju-channel", "beta"); err != nil {
		t.Fatal(err)
	}
	t.Setenv("CONCIERGE_JUJU_CHANNEL", "edge")

	bindFlags(cmd)

	got, err := cmd.Flags().GetString("juju-channel")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if got != "beta" {
		t.Fatalf("explicitly set flag should not be overridden by env; want %q, got %q", "beta", got)
	}
}

func TestParseConfigDefaultFileFallsBackToDevPreset(t *testing.T) {
	t.Chdir(t.TempDir())

	cfg, err := parseConfig("")
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}

	dev, err := Preset("dev")
	if err != nil {
		t.Fatalf("Preset(dev): %v", err)
	}
	if !reflect.DeepEqual(cfg, dev) {
		t.Fatalf("fallback config does not match dev preset")
	}
}

func TestParseConfigDefaultFileReadsCwd(t *testing.T) {
	dir := t.TempDir()
	yamlConfig := `
juju:
  channel: 3.6/stable
providers:
  lxd:
    enable: true
`
	if err := os.WriteFile(dir+"/concierge.yaml", []byte(yamlConfig), 0o600); err != nil {
		t.Fatal(err)
	}
	t.Chdir(dir)

	cfg, err := parseConfig("")
	if err != nil {
		t.Fatalf("parseConfig: %v", err)
	}
	if cfg.Juju.Channel != "3.6/stable" {
		t.Fatalf("want juju channel from default-path file, got %q", cfg.Juju.Channel)
	}
	if !cfg.Providers.LXD.Enable {
		t.Fatalf("want LXD enabled from default-path file")
	}
}

func TestParseConfigExplicitFileMissing(t *testing.T) {
	_, err := parseConfig(t.TempDir() + "/does-not-exist.yaml")
	if err == nil {
		t.Fatal("want error for missing explicit config file, got nil")
	}
}

func TestBindFlagsNoEnvLeavesDefault(t *testing.T) {
	_ = os.Unsetenv("CONCIERGE_JUJU_CHANNEL")
	cmd := &cobra.Command{Use: "concierge"}
	cmd.Flags().String("juju-channel", "stable", "")

	bindFlags(cmd)

	got, err := cmd.Flags().GetString("juju-channel")
	if err != nil {
		t.Fatalf("GetString: %v", err)
	}
	if got != "stable" {
		t.Fatalf("want flag left at default %q, got %q", "stable", got)
	}
}
