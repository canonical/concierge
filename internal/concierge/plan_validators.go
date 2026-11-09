package concierge

import (
	"fmt"
	"regexp"
	"slices"
)

// planValidators is a list of planValidators used to verify a plan
var planValidators = []func(p *Plan) error{
	validateSingleLocalKubernetesInstance,
	validateControllerNames,
	validateServiceProviderDependencies,
}

// validateSingleLocalKubernetesInstance ensures the plan won't try and install multiple
// local Kubernetes providers, which would conflict.
func validateSingleLocalKubernetesInstance(plan *Plan) error {
	providerNames := []string{}

	for _, p := range plan.Providers {
		providerNames = append(providerNames, p.Name())
	}

	if slices.Contains(providerNames, "microk8s") && slices.Contains(providerNames, "k8s") {
		return fmt.Errorf("cannot configure multiple local kubernetes providers")
	}

	return nil
}

// validControllerName matches the controller names that Juju accepts.
var validControllerName = regexp.MustCompile(`^[a-z0-9]+[a-z0-9-]*$`)

// validateControllerNames ensures that each provider to be bootstrapped has a
// controller name that Juju accepts, and that no two providers share one.
func validateControllerNames(plan *Plan) error {
	// Controller names only matter when Juju is bootstrapped. When Juju is
	// disabled the providers are never bootstrapped (NewPlan only warns), so
	// an otherwise-unused controller name must not fail the plan.
	if plan.config.Juju.Disable {
		return nil
	}

	seen := map[string]string{}

	for _, p := range plan.Providers {
		if !p.Bootstrap() {
			continue
		}

		name := p.ControllerName()
		if !validControllerName.MatchString(name) {
			return fmt.Errorf("invalid controller name %q for provider %q", name, p.Name())
		}

		if other, ok := seen[name]; ok {
			return fmt.Errorf("providers %q and %q cannot share the controller name %q", other, p.Name(), name)
		}
		seen[name] = p.Name()
	}

	return nil
}

// validateServiceProviderDependencies ensures that any service configured to
// integrate with a provider (e.g. microceph's storage-class) has a matching
// provider present in the plan.
func validateServiceProviderDependencies(plan *Plan) error {
	if plan.config.Services.Microceph.Enable && plan.config.Services.Microceph.StorageClass.Enable {
		providerNames := []string{}

		for _, p := range plan.Providers {
			providerNames = append(providerNames, p.Name())
		}

		if !slices.Contains(providerNames, "k8s") {
			return fmt.Errorf(
				"services.microceph.storage-class.enable requires the k8s provider " +
					"to be enabled (microk8s users should use the addons instead)",
			)
		}
	}

	return nil
}
