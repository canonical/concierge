package providers

import (
	"fmt"
	"log/slog"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
	"gopkg.in/yaml.v3"
)

// NewGoogle constructs a new Google provider instance.
func NewGoogle(system system.Worker, config *config.Config) *Google {
	credentialsFile := config.Providers.Google.CredentialsFile
	if config.Overrides.GoogleCredentialFile != "" {
		credentialsFile = config.Overrides.GoogleCredentialFile
	}

	return &Google{
		system:               system,
		bootstrap:            config.Providers.Google.Bootstrap,
		credentialsFile:      credentialsFile,
		credentials:          map[string]any{},
		modelDefaults:        config.Providers.Google.ModelDefaults,
		bootstrapConstraints: config.Providers.Google.BootstrapConstraints,
	}
}

// Google represents a Google cloud to bootstrap.
type Google struct {
	bootstrap            bool
	system               system.Worker
	credentialsFile      string
	credentials          map[string]any
	modelDefaults        map[string]string
	bootstrapConstraints map[string]string
}

// Prepare installs and configures Google such that it can work in testing environments.
// This includes installing the snap, enabling the user who ran concierge to interact
// with Google without sudo, and deconflicting the firewall rules with docker.
func (l *Google) Prepare() error {
	contents, err := l.system.ReadFile(l.credentialsFile)
	if err != nil {
		return fmt.Errorf("failed to read credentials file: %w", err)
	}

	credentials := make(map[string]any)

	err = yaml.Unmarshal(contents, &credentials)
	if err != nil {
		return fmt.Errorf("failed to parse google cloud credentials: %w", err)
	}

	l.credentials = credentials

	slog.Info("Prepared provider", "provider", l.Name())
	return nil
}

// Name reports the name of the provider for Concierge's purposes.
func (l *Google) Name() string { return "google" }

// Bootstrap reports whether a Juju controller should be bootstrapped on Google.
func (l *Google) Bootstrap() bool { return l.bootstrap }

// CloudName reports the name of the provider as Juju sees it.
func (l *Google) CloudName() string { return "google" }

// GroupName reports the name of the POSIX group with permissions over the Google socket.
func (l *Google) GroupName() string { return "" }

// Credentials reports the section of Juju's credentials.yaml for the provider.
func (l *Google) Credentials() map[string]any { return l.credentials }

// ModelDefaults reports the Juju model-defaults specific to the provider.
func (l *Google) ModelDefaults() map[string]string { return l.modelDefaults }

// BootstrapConstraints reports the Juju bootstrap-constraints specific to the provider.
func (l *Google) BootstrapConstraints() map[string]string { return l.bootstrapConstraints }

// Remove Google provider.
func (l *Google) Restore() error {
	slog.Info("Restored provider", "provider", l.Name())
	return nil
}
