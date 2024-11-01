package juju

import (
	"fmt"
	"log/slog"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/jnsgruk/concierge/internal/config"
	"github.com/jnsgruk/concierge/internal/packages"
	"github.com/jnsgruk/concierge/internal/providers"
	"github.com/jnsgruk/concierge/internal/system"
	"golang.org/x/sync/errgroup"
	"gopkg.in/yaml.v3"
)

// NewJujuHandler constructs a new JujuHandler instance.
func NewJujuHandler(config *config.Config, r system.Worker, providers []providers.Provider) *JujuHandler {
	var channel string
	if config.Overrides.JujuChannel != "" {
		channel = config.Overrides.JujuChannel
	} else {
		channel = config.Juju.Channel
	}

	return &JujuHandler{
		channel:              channel,
		bootstrapConstraints: config.Juju.BootstrapConstraints,
		modelDefaults:        config.Juju.ModelDefaults,
		providers:            providers,
		system:               r,
		snaps:                []*system.Snap{{Name: "juju", Channel: channel}},
	}
}

// JujuHandler represents a Juju installation on the system.
type JujuHandler struct {
	channel              string
	bootstrapConstraints map[string]string
	modelDefaults        map[string]string
	providers            []providers.Provider
	system               system.Worker
	snaps                []*system.Snap
}

// Prepare bootstraps Juju on the configured providers.
func (j *JujuHandler) Prepare() error {
	err := j.install()
	if err != nil {
		return fmt.Errorf("failed to install Juju: %w", err)
	}

	dir := path.Join(".local", "share", "juju")

	err = j.system.MkHomeSubdirectory(dir)
	if err != nil {
		return fmt.Errorf("failed to create directory '%s': %w", dir, err)
	}

	err = j.writeCredentials()
	if err != nil {
		return fmt.Errorf("failed to write juju credentials file: %w", err)
	}

	err = j.bootstrap()
	if err != nil {
		return fmt.Errorf("failed to bootstrap Juju controller: %w", err)
	}

	return nil
}

// Restore uninstalls Juju from the system.
func (j *JujuHandler) Restore() error {
	// Kill controllers for credentialed providers.
	for _, p := range j.providers {
		if p.Credentials() == nil {
			continue
		}

		err := j.killProvider(p)
		if err != nil {
			return err
		}
	}

	err := j.system.RemoveAllHome(path.Join(".local", "share", "juju"))
	if err != nil {
		return fmt.Errorf("failed to remove '.local/share/juju' subdirectory from user's home directory: %w", err)
	}

	snapHandler := packages.NewSnapHandler(j.system, j.snaps)

	err = snapHandler.Restore()
	if err != nil {
		return err
	}

	slog.Info("Restored Juju")

	return nil
}

// install ensures that Juju is installed.
func (j *JujuHandler) install() error {
	snapHandler := packages.NewSnapHandler(j.system, j.snaps)

	err := snapHandler.Prepare()
	if err != nil {
		return err
	}

	return nil
}

// writeCredentials iterates over any provided cloud credentials and authors Juju's
// credentials.yaml
func (j *JujuHandler) writeCredentials() error {
	credentials := map[string]interface{}{"credentials": map[string]interface{}{}}
	addedCredentials := false

	// Iterate over the providers
	for _, p := range j.providers {
		// If the provider doesn't specify any credentials, move on to the next.
		if p.Credentials() == nil {
			continue
		}

		// Set the credentials for the provider, under the credential name "concierge".
		credentials["credentials"] = map[string]interface{}{
			p.CloudName(): map[string]interface{}{
				"concierge": p.Credentials(),
			},
		}
		addedCredentials = true
	}

	// Don't write the file if there are no credentials to add
	if !addedCredentials {
		return nil
	}

	// Marshall the credentials map and write it to the credentials.yaml file.
	content, err := yaml.Marshal(credentials)
	if err != nil {
		return fmt.Errorf("failed to marshal juju credentials to yaml: %w", err)
	}

	err = j.system.WriteHomeDirFile(path.Join(".local", "share", "juju", "credentials.yaml"), content)
	if err != nil {
		return fmt.Errorf("failed to write credentials.yaml: %w", err)
	}

	return nil
}

// bootstrap iterates over the set of configured providers, and bootstraps each of
// them in parallel with a unique controller name.
func (j *JujuHandler) bootstrap() error {
	var eg errgroup.Group

	for _, provider := range j.providers {
		eg.Go(func() error { return j.bootstrapProvider(provider) })
	}

	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// bootstrapProvider bootstraps one specific provider.
func (j *JujuHandler) bootstrapProvider(provider providers.Provider) error {
	if !provider.Bootstrap() {
		return nil
	}

	controllerName := fmt.Sprintf("concierge-%s", provider.Name())

	bootstrapped, err := j.checkBootstrapped(controllerName)
	if err != nil {
		return fmt.Errorf("error checking bootstrap status for provider '%s'", provider.Name())
	}

	if bootstrapped {
		slog.Info("Previous Juju controller found", "provider", provider.Name())
		return nil
	}

	slog.Info("Bootstrapping Juju", "provider", provider.Name())

	bootstrapArgs := []string{
		"bootstrap",
		provider.CloudName(),
		controllerName,
		"--verbose",
	}

	// Combine the global and provider-local model-defaults and bootstrap-constraints.
	modelDefaults := config.MergeMaps(j.modelDefaults, provider.ModelDefaults())
	bootstrapConstraints := config.MergeMaps(j.bootstrapConstraints, provider.BootstrapConstraints())

	// Iterate over the model-defaults and append them to the bootstrapArgs
	for _, k := range sortedKeys(modelDefaults) {
		bootstrapArgs = append(bootstrapArgs, "--model-default", fmt.Sprintf("%s=%s", k, modelDefaults[k]))
	}

	// Iterate over the bootstrap-constraints and append them to the bootstrapArgs
	for _, k := range sortedKeys(bootstrapConstraints) {
		bootstrapArgs = append(bootstrapArgs, "--bootstrap-constraints", fmt.Sprintf("%s=%s", k, bootstrapConstraints[k]))
	}

	user := j.system.User().Username

	cmd := system.NewCommandAs(user, provider.GroupName(), "juju", bootstrapArgs)
	_, err = j.system.RunWithRetries(cmd, (5 * time.Minute))
	if err != nil {
		return err
	}

	cmd = system.NewCommandAs(user, "", "juju", []string{"add-model", "-c", controllerName, "testing"})
	_, err = j.system.Run(cmd)
	if err != nil {
		return err
	}

	slog.Info("Bootstrapped Juju", "provider", provider.Name())
	return nil
}

// killProvider destroys the controller for a specific provider.
func (j *JujuHandler) killProvider(provider providers.Provider) error {
	controllerName := fmt.Sprintf("concierge-%s", provider.Name())

	bootstrapped, err := j.checkBootstrapped(controllerName)
	if err != nil {
		return fmt.Errorf("error checking bootstrap status for provider '%s'", provider.Name())
	}

	if !bootstrapped {
		slog.Info("No Juju controller found", "provider", provider.Name())
		return nil
	}

	slog.Info("Destroying Juju controller", "provider", provider.Name())

	killArgs := []string{"kill-controller", "--verbose", "--no-prompt", controllerName}

	cmd := system.NewCommandAs(j.system.User().Username, "", "juju", killArgs)
	_, err = j.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to destroy controller: '%s': %w", controllerName, err)
	}

	slog.Info("Destroyed Juju controller", "provider", provider.Name())
	return nil
}

// checkBootstrapped checks whether concierge has already been bootstrapped on a given provider.
func (j *JujuHandler) checkBootstrapped(controllerName string) (bool, error) {
	user := j.system.User().Username
	cmd := system.NewCommandAs(user, "", "juju", []string{"show-controller", controllerName})

	result, err := j.system.Run(cmd)
	if err != nil && strings.Contains(string(result), "not found") {
		return false, nil
	} else if err != nil {
		return false, err
	}

	return true, nil
}

// sortedKeys gets an alphabetically sorted list of keys from a map.
func sortedKeys(m map[string]string) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	slices.Sort(keys)
	return keys
}
