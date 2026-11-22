package concierge

import (
	"cmp"
	"fmt"
	"net/netip"
	"regexp"
	"slices"
	"strings"

	"github.com/canonical/concierge/internal/providers"
)

// planValidators is a list of planValidators used to verify a plan
var planValidators = []func(p *Plan) error{
	validateSingleLocalKubernetesInstance,
	validateControllerNames,
	validateMetalLBIPRange,
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

// validateMetalLBIPRange checks the MetalLB range before anything is installed.
// The metallb addon rejects a bad range too, but only after the earlier
// addons are enabled, and concierge retries it for five minutes first.
func validateMetalLBIPRange(plan *Plan) error {
	ipRange := cmp.Or(plan.config.Overrides.MetalLBIPRange, plan.config.Providers.MicroK8s.MetalLBIPRange)
	if ipRange == "" || !slices.ContainsFunc(plan.Providers, func(p providers.Provider) bool { return p.Name() == "microk8s" }) {
		return nil
	}

	// Accept what the addon accepts: a comma-separated list of ranges and CIDRs.
	for item := range strings.SplitSeq(ipRange, ",") {
		if !validMetalLBIPRangeItem(item) {
			return fmt.Errorf("invalid MetalLB IP range %q: %q is not a range (<start>-<end>) or a CIDR", ipRange, item)
		}
	}

	return nil
}

// validMetalLBIPRangeItem reports whether s is a CIDR, or a range of two
// addresses of the same family in ascending order.
func validMetalLBIPRangeItem(s string) bool {
	if _, err := netip.ParsePrefix(s); err == nil {
		return true
	}

	start, end, ok := strings.Cut(s, "-")
	if !ok {
		return false
	}
	startAddr, err := netip.ParseAddr(start)
	if err != nil {
		return false
	}
	endAddr, err := netip.ParseAddr(end)
	if err != nil {
		return false
	}
	return startAddr.Is4() == endAddr.Is4() && startAddr.Compare(endAddr) <= 0
}
