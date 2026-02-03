package providers

import (
	"reflect"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
	"gopkg.in/yaml.v3"
)

func TestNewGoogle(t *testing.T) {
	type test struct {
		config   *config.Config
		expected *Google
	}

	noOverrides := &config.Config{}

	credsInConfig := &config.Config{}
	credsInConfig.Providers.Google.CredentialsFile = "/home/ubuntu/credentials.yaml"

	overrides := &config.Config{}
	overrides.Overrides.GoogleCredentialFile = "/home/ubuntu/alternate-credentials.yaml"

	system := system.NewMockSystem()

	tests := []test{
		{
			config: noOverrides,
			expected: &Google{
				system:      system,
				credentials: map[string]any{},
			},
		},
		{
			config: credsInConfig,
			expected: &Google{
				system:          system,
				credentialsFile: "/home/ubuntu/credentials.yaml",
				credentials:     map[string]any{},
			},
		},
		{
			config: overrides,
			expected: &Google{
				system:          system,
				credentialsFile: "/home/ubuntu/alternate-credentials.yaml",
				credentials:     map[string]any{},
			},
		},
	}

	for _, tc := range tests {
		uk8s := NewGoogle(system, tc.config)
		if !reflect.DeepEqual(tc.expected, uk8s) {
			t.Fatalf("expected: %v, got: %v", tc.expected, uk8s)
		}
	}
}

func TestGooglePrepareCommands(t *testing.T) {
	config := &config.Config{}
	config.Providers.Google.CredentialsFile = "/home/ubuntu/credentials.yaml"

	system := system.NewMockSystem()
	uk8s := NewGoogle(system, config)
	uk8s.Prepare()

	if len(system.ExecutedCommands) != 0 {
		t.Fatalf("expected no commands to have been run")
	}

	if len(system.CreatedFiles) != 0 {
		t.Fatalf("expected no files to have been created")
	}
}

func TestGoogleReadCredentials(t *testing.T) {
	config := &config.Config{}
	config.Providers.Google.CredentialsFile = "credentials.yaml"

	system := system.NewMockSystem()

	creds := []byte(`auth-type: oauth2
client-email: juju-gce-1-sa@concierge.iam.gserviceaccount.com
client-id: "12345678912345"
private-key: |
  -----BEGIN PRIVATE KEY-----
  deadbeef
  -----END PRIVATE KEY-----
project-id: concierge
`)

	fakeCredsMarshalled := make(map[string]any)
	err := yaml.Unmarshal(creds, &fakeCredsMarshalled)
	if err != nil {
		t.Fatal(err)
	}

	system.MockFile("credentials.yaml", creds)

	google := NewGoogle(system, config)
	google.Prepare()

	if !reflect.DeepEqual(google.Credentials(), fakeCredsMarshalled) {
		t.Fatalf("expected: %v, got: %v", fakeCredsMarshalled, google.Credentials())
	}
}
