package services

import (
	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/providers"
	"github.com/canonical/concierge/internal/system"
)

// SupportedServices is a list of stringified names of supported services.
var SupportedServices []string = []string{
	"microceph",
}

// Service describes the set of methods expected to be available on an
// optional piece of software that concierge can install and bring to a
// working state, independently of any one provider's Juju-cloud lifecycle.
// See specs/services.md for the full design rationale.
type Service interface {
	// Prepare installs/configures the service, independent of any
	// provider. Runs before providers are prepared.
	Prepare() error
	// Integrate wires the service into whichever provider(s) are active
	// (e.g. applying a storage class). Runs after providers are prepared.
	// No-op for services that don't integrate with a provider.
	Integrate() error
	// Disintegrate reverses Integrate(). Runs before providers are restored.
	Disintegrate() error
	// Restore uninstalls/tears down whatever Prepare() set up.
	Restore() error
	// Name reports the name of the service used internally by concierge.
	Name() string
}

// NewService returns a newly constructed service based on a stringified
// name of the service, or nil if the service is not enabled in config.
func NewService(serviceName string, system system.Worker, config *config.Config, providerList []providers.Provider) Service {
	if serviceName == "microceph" && config.Services.Microceph.Enable {
		return NewMicroceph(system, config, providerList)
	}

	return nil
}
