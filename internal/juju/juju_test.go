package juju

import (
	"fmt"
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/providers"
	"github.com/canonical/concierge/internal/system"
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
			},
			expectedDirs: []string{".local/share/juju"},
		},
		{
			preset: "microk8s",
			expectedCommands: []string{
				"snap install juju",
				"sudo -u test-user juju show-controller concierge-microk8s",
				"sudo -u test-user -g snap_microk8s juju bootstrap microk8s concierge-microk8s --verbose --model-default automatically-retry-hooks=false --model-default test-mode=true",
				"sudo -u test-user juju add-model -c concierge-microk8s testing",
			},
			expectedDirs: []string{".local/share/juju"},
		},
		{
			preset: "k8s",
			expectedCommands: []string{
				"snap install juju",
				"sudo -u test-user juju show-controller concierge-k8s",
				"sudo -u test-user juju bootstrap k8s concierge-k8s --verbose --model-default automatically-retry-hooks=false --model-default test-mode=true --bootstrap-constraints root-disk=2G",
				"sudo -u test-user juju add-model -c concierge-k8s testing",
			},
			expectedDirs: []string{".local/share/juju"},
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

		if !reflect.DeepEqual(tc.expectedCommands, system.ExecutedCommands) {
			t.Fatalf("expected: %v, got: %v", tc.expectedCommands, system.ExecutedCommands)
		}
		if !reflect.DeepEqual(tc.expectedDirs, system.CreatedDirectories) {
			t.Fatalf("expected: %v, got: %v", tc.expectedDirs, system.CreatedDirectories)
		}
		if len(system.CreatedFiles) > 0 {
			t.Fatalf("expected no files to be created, got: %v", system.CreatedFiles)
		}
	}
}

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

	expectedFiles := map[string]string{".local/share/juju/credentials.yaml": string(expectedCredsFileContent)}

	if !reflect.DeepEqual(expectedFiles, system.CreatedFiles) {
		t.Fatalf("expected: %v, got: %v", expectedFiles, system.CreatedFiles)
	}
}

func TestJujuRestoreNoKillController(t *testing.T) {
	system, handler, err := setupHandlerWithPreset("machine")
	if err != nil {
		t.Fatal(err.Error())
	}

	handler.Restore()

	expectedDeleted := []string{".local/share/juju"}
	expectedCommands := []string{"snap remove juju --purge"}

	if !reflect.DeepEqual(expectedDeleted, system.Deleted) {
		t.Fatalf("expected: %v, got: %v", expectedDeleted, system.Deleted)
	}

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestJujuRestoreKillController(t *testing.T) {
	system, handler, err := setupHandlerWithGoogleProvider()
	if err != nil {
		t.Fatal(err.Error())
	}

	handler.Restore()

	expectedDeleted := []string{".local/share/juju"}
	expectedCommands := []string{
		"sudo -u test-user juju show-controller concierge-google",
		"sudo -u test-user juju kill-controller --verbose --no-prompt concierge-google",
		"snap remove juju --purge",
	}

	if !reflect.DeepEqual(expectedDeleted, system.Deleted) {
		t.Fatalf("expected: %v, got: %v", expectedDeleted, system.Deleted)
	}

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestJujuHandlerWithAgentVersion(t *testing.T) {
	cfg := &config.Config{}
	cfg.Juju.AgentVersion = "3.5.2"
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
		"sudo -u test-user -g lxd juju bootstrap localhost concierge-lxd --verbose --agent-version 3.5.2 --model-default automatically-retry-hooks=false --model-default test-mode=true",
		"sudo -u test-user juju add-model -c concierge-lxd testing",
	}

	if !reflect.DeepEqual(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}
