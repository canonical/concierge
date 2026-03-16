package juju

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/providers"
	"github.com/canonical/concierge/internal/system"
	"gopkg.in/yaml.v3"
)

var fakeGoogleCreds = []byte(`auth-type: oauth2
client-email: juju-gce-1-sa@concierge.iam.gserviceaccount.com
client-id: "12345678912345"
private-key: |
  -----BEGIN PRIVATE KEY-----
  deadbeef
  -----END PRIVATE KEY-----
project-id: concierge
`)

func setupHandlerWithPreset(preset string) (*system.MockSystem, *JujuHandler, error) {
	var err error
	var cfg *config.Config
	var provider providers.Provider

	system := system.NewMockSystem()
	system.MockCommandReturn(
		"sudo -u test-user juju show-controller concierge-lxd",
		[]byte("ERROR controller concierge-lxd not found"),
		fmt.Errorf("Test error"),
	)
	system.MockCommandReturn(
		"sudo -u test-user juju show-controller concierge-microk8s",
		[]byte("ERROR controller concierge-microk8s not found"),
		fmt.Errorf("Test error"),
	)
	system.MockCommandReturn(
		"sudo -u test-user juju show-controller concierge-k8s",
		[]byte("ERROR controller concierge-k8s not found"),
		fmt.Errorf("Test error"),
	)

	cfg, err = config.Preset(preset)
	if err != nil {
		return nil, nil, err
	}

	switch preset {
	case "machine":
		provider = providers.NewLXD(system, cfg)
	case "microk8s":
		provider = providers.NewMicroK8s(system, cfg)
	case "k8s":
		provider = providers.NewK8s(system, cfg)
	}

	handler := NewJujuHandler(cfg, system, []providers.Provider{provider})

	return system, handler, nil
}

func setupHandlerWithGoogleProvider() (*system.MockSystem, *JujuHandler, error) {
	cfg := &config.Config{}
	cfg.Providers.Google.Enable = true
	cfg.Providers.Google.Bootstrap = true
	cfg.Providers.Google.CredentialsFile = "google.yaml"

	system := system.NewMockSystem()
	system.MockFile("google.yaml", fakeGoogleCreds)

	provider := providers.NewProvider("google", system, cfg)

	err := provider.Prepare()
	if err != nil {
		return nil, nil, fmt.Errorf("failed to prepare google provider: %w", err)
	}

	handler := NewJujuHandler(cfg, system, []providers.Provider{provider})
	return system, handler, nil
}

func TestJujuHandlerCommandsPresets(t *testing.T) {
	type test struct {
		preset           string
		expectedCommands []string
		expectedDirs     []string
	}

	tests := []test{
		{
			preset: "machine",
			expectedCommands: []string{
				"snap install juju",
				"sudo -u test-user juju show-controller concierge-lxd",
				"sudo -u test-user -g lxd juju bootstrap localhost concierge-lxd --verbose --model-default automatically-retry-hooks=false --model-default test-mode=true",
				"sudo -u test-user juju add-model -c concierge-lxd testing",
				fmt.Sprintf("sudo -u test-user juju set-model-constraints -m concierge-lxd:testing arch=%s", goArchToJujuArch(runtime.GOARCH)),
			},
			expectedDirs: []string{path.Join(os.TempDir(), ".local/share/juju")},
		},
		{
			preset: "microk8s",
			expectedCommands: []string{
				"snap install juju",
				"sudo -u test-user juju show-controller concierge-microk8s",
				"sudo -u test-user -g snap_microk8s juju bootstrap microk8s concierge-microk8s --verbose --model-default automatically-retry-hooks=false --model-default test-mode=true",
				"sudo -u test-user juju add-model -c concierge-microk8s testing",
				fmt.Sprintf("sudo -u test-user juju set-model-constraints -m concierge-microk8s:testing arch=%s", goArchToJujuArch(runtime.GOARCH)),
			},
			expectedDirs: []string{path.Join(os.TempDir(), ".local/share/juju")},
		},
		{
			preset: "k8s",
			expectedCommands: []string{
				"snap install juju",
				"sudo -u test-user juju show-controller concierge-k8s",
				"sudo -u test-user juju bootstrap k8s concierge-k8s --verbose --model-default automatically-retry-hooks=false --model-default test-mode=true --bootstrap-constraints root-disk=2G",
				"sudo -u test-user juju add-model -c concierge-k8s testing",
				fmt.Sprintf("sudo -u test-user juju set-model-constraints -m concierge-k8s:testing arch=%s", goArchToJujuArch(runtime.GOARCH)),
			},
			expectedDirs: []string{path.Join(os.TempDir(), ".local/share/juju")},
		},
	}

	for _, tc := range tests {
		system, handler, err := setupHandlerWithPreset(tc.preset)
		if err != nil {
			t.Fatal(err.Error())
		}

		err = handler.Prepare()
		if err != nil {
			t.Fatal(err.Error())
		}

		if !slices.Equal(tc.expectedCommands, system.ExecutedCommands) {
			t.Fatalf("expected: %v, got: %v", tc.expectedCommands, system.ExecutedCommands)
		}
		if !slices.Equal(tc.expectedDirs, system.CreatedDirectories) {
			t.Fatalf("expected: %v, got: %v", tc.expectedDirs, system.CreatedDirectories)
		}
		if len(system.CreatedFiles) > 0 {
			t.Fatalf("expected no files to be created, got: %v", system.CreatedFiles)
		}
	}
}

// mockProvider is a minimal Provider implementation for testing credential merging.
type mockProvider struct {
	name        string
	cloudName   string
	credentials map[string]any
}

func (m *mockProvider) Prepare() error                          { return nil }
func (m *mockProvider) Restore() error                          { return nil }
func (m *mockProvider) Name() string                            { return m.name }
func (m *mockProvider) Bootstrap() bool                         { return false }
func (m *mockProvider) CloudName() string                       { return m.cloudName }
func (m *mockProvider) GroupName() string                       { return "" }
func (m *mockProvider) Credentials() map[string]any             { return m.credentials }
func (m *mockProvider) ModelDefaults() map[string]string        { return nil }
func (m *mockProvider) BootstrapConstraints() map[string]string { return nil }

func TestJujuHandlerWithCredentialedProvider(t *testing.T) {
	expectedCredsFileContent := []byte(`credentials:
    google:
        concierge:
            auth-type: oauth2
            client-email: juju-gce-1-sa@concierge.iam.gserviceaccount.com
            client-id: "12345678912345"
            private-key: |
                -----BEGIN PRIVATE KEY-----
                deadbeef
                -----END PRIVATE KEY-----
            project-id: concierge
`)

	system, handler, err := setupHandlerWithGoogleProvider()
	if err != nil {
		t.Fatal(err.Error())
	}

	err = handler.Prepare()
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedFiles := map[string]string{path.Join(os.TempDir(), ".local", "share", "juju", "credentials.yaml"): string(expectedCredsFileContent)}

	if !reflect.DeepEqual(expectedFiles, system.CreatedFiles) {
		t.Fatalf("expected: %v, got: %v", expectedFiles, system.CreatedFiles)
	}
}

func TestJujuHandlerMergesMultipleCredentialedProviders(t *testing.T) {
	cfg := &config.Config{}

	sys := system.NewMockSystem()

	providerA := &mockProvider{
		name:      "cloud-a",
		cloudName: "cloud-a",
		credentials: map[string]any{
			"auth-type": "userpass",
			"username":  "alice",
			"password":  "secret-a",
		},
	}
	providerB := &mockProvider{
		name:      "cloud-b",
		cloudName: "cloud-b",
		credentials: map[string]any{
			"auth-type": "userpass",
			"username":  "bob",
			"password":  "secret-b",
		},
	}

	handler := NewJujuHandler(cfg, sys, []providers.Provider{providerA, providerB})

	err := handler.Prepare()
	if err != nil {
		t.Fatal(err.Error())
	}

	credsFile := path.Join(os.TempDir(), ".local", "share", "juju", "credentials.yaml")
	content, ok := sys.CreatedFiles[credsFile]
	if !ok {
		t.Fatal("credentials.yaml was not created")
	}

	// Parse the written YAML and verify both clouds are present.
	var parsed map[string]any
	if err := yaml.Unmarshal([]byte(content), &parsed); err != nil {
		t.Fatalf("failed to parse credentials.yaml: %v", err)
	}

	credMap, ok := parsed["credentials"].(map[string]any)
	if !ok {
		t.Fatal("missing or invalid top-level 'credentials' key")
	}

	if len(credMap) != 2 {
		t.Fatalf("expected 2 clouds in credentials, got %d: %v", len(credMap), credMap)
	}

	for _, cloud := range []string{"cloud-a", "cloud-b"} {
		entry, ok := credMap[cloud]
		if !ok {
			t.Fatalf("credentials for %q missing — merge overwrote earlier entries", cloud)
		}
		cloudMap, ok := entry.(map[string]any)
		if !ok {
			t.Fatalf("credentials for %q is not a map", cloud)
		}
		if _, ok := cloudMap["concierge"]; !ok {
			t.Fatalf("credentials for %q missing 'concierge' key", cloud)
		}
	}
}

func TestJujuRestoreNoKillController(t *testing.T) {
	system, handler, err := setupHandlerWithPreset("machine")
	if err != nil {
		t.Fatal(err.Error())
	}

	handler.Restore()

	expectedRemovedPaths := []string{path.Join(os.TempDir(), ".local", "share", "juju")}
	expectedCommands := []string{"snap remove juju --purge"}

	if !slices.Equal(expectedRemovedPaths, system.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, system.RemovedPaths)
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestJujuRestoreKillController(t *testing.T) {
	system, handler, err := setupHandlerWithGoogleProvider()
	if err != nil {
		t.Fatal(err.Error())
	}

	handler.Restore()

	expectedRemovedPaths := []string{path.Join(os.TempDir(), ".local", "share", "juju")}
	expectedCommands := []string{
		"sudo -u test-user juju show-controller concierge-google",
		"sudo -u test-user juju kill-controller --verbose --no-prompt concierge-google",
		"snap remove juju --purge",
	}

	if !slices.Equal(expectedRemovedPaths, system.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, system.RemovedPaths)
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestJujuHandlerWithAgentVersion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Juju.AgentVersion = "3.6.2"
	cfg.Providers.LXD.Enable = true
	cfg.Providers.LXD.Bootstrap = true

	cfg.Juju.ModelDefaults = map[string]string{
		"test-mode":                 "true",
		"automatically-retry-hooks": "false",
	}

	system := system.NewMockSystem()
	system.MockCommandReturn(
		"sudo -u test-user juju show-controller concierge-lxd",
		[]byte("ERROR controller concierge-lxd not found"),
		fmt.Errorf("Test error"),
	)

	provider := providers.NewLXD(system, cfg)
	handler := NewJujuHandler(cfg, system, []providers.Provider{provider})

	err := handler.Prepare()
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedCommands := []string{
		"snap install juju",
		"sudo -u test-user juju show-controller concierge-lxd",
		"sudo -u test-user -g lxd juju bootstrap localhost concierge-lxd --verbose --agent-version 3.6.2 --model-default automatically-retry-hooks=false --model-default test-mode=true",
		"sudo -u test-user juju add-model -c concierge-lxd testing",
		fmt.Sprintf("sudo -u test-user juju set-model-constraints -m concierge-lxd:testing arch=%s", goArchToJujuArch(runtime.GOARCH)),
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestJujuHandlerWithExtraBootstrapArgs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.LXD.Enable = true
	cfg.Providers.LXD.Bootstrap = true

	cfg.Juju.ModelDefaults = map[string]string{
		"test-mode":                 "true",
		"automatically-retry-hooks": "false",
	}
	cfg.Juju.ExtraBootstrapArgs = "--config idle-connection-timeout=90s"

	system := system.NewMockSystem()
	system.MockCommandReturn(
		"sudo -u test-user juju show-controller concierge-lxd",
		[]byte("ERROR controller concierge-lxd not found"),
		fmt.Errorf("Test error"),
	)

	provider := providers.NewLXD(system, cfg)
	handler := NewJujuHandler(cfg, system, []providers.Provider{provider})

	err := handler.Prepare()
	if err != nil {
		t.Fatal(err.Error())
	}

	expectedCommands := []string{
		"snap install juju",
		"sudo -u test-user juju show-controller concierge-lxd",
		"sudo -u test-user -g lxd juju bootstrap localhost concierge-lxd --verbose --model-default automatically-retry-hooks=false --model-default test-mode=true --config idle-connection-timeout=90s",
		"sudo -u test-user juju add-model -c concierge-lxd testing",
		fmt.Sprintf("sudo -u test-user juju set-model-constraints -m concierge-lxd:testing arch=%s", goArchToJujuArch(runtime.GOARCH)),
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestJujuHandlerWithInvalidExtraBootstrapArgs(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.LXD.Enable = true
	cfg.Providers.LXD.Bootstrap = true
	cfg.Juju.ExtraBootstrapArgs = `--config "unclosed`

	system := system.NewMockSystem()
	system.MockCommandReturn(
		"sudo -u test-user juju show-controller concierge-lxd",
		[]byte("ERROR controller concierge-lxd not found"),
		fmt.Errorf("Test error"),
	)

	provider := providers.NewLXD(system, cfg)
	handler := NewJujuHandler(cfg, system, []providers.Provider{provider})

	err := handler.Prepare()
	if err == nil {
		t.Fatal("expected error for invalid extra-bootstrap-args")
	}
	if !strings.Contains(err.Error(), "failed to parse extra-bootstrap-args") {
		t.Fatalf("expected parse error, got: %v", err)
	}
}

func TestGoArchToJujuArch(t *testing.T) {
	tests := []struct {
		goarch   string
		expected string
	}{
		{"amd64", "amd64"},
		{"arm64", "arm64"},
		{"ppc64le", "ppc64el"}, // Go uses ppc64le, Juju/Debian use ppc64el
		{"s390x", "s390x"},
		{"riscv64", "riscv64"},
		{"arm", "arm"},
		{"386", "386"},
	}

	for _, tc := range tests {
		result := goArchToJujuArch(tc.goarch)
		if result != tc.expected {
			t.Errorf("goArchToJujuArch(%s) = %s, expected %s", tc.goarch, result, tc.expected)
		}
	}
}
