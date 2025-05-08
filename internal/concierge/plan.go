package concierge

import (
	"fmt"
	"log/slog"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/juju"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/providers"
	"github.com/canonical/concierge/internal/system"
	"golang.org/x/sync/errgroup"
)

// Plan represents a set of packages and providers that are to be prepared/restored.
type Plan struct {
	Providers []providers.Provider
	Snaps     []*system.Snap
	Debs      []*packages.Deb

	config *config.Config
	system system.Worker
}

// NewPlan constructs a new plan consisting of snaps/debs/providers & juju.
func NewPlan(cfg *config.Config, worker system.Worker) *Plan {
	plan := &Plan{config: cfg, system: worker}

	for name, snapConfig := range cfg.Host.Snaps {
		snap := system.NewSnap(name, snapConfig.Channel, snapConfig.Connections)
		// Check if the channel has been overridden by a CLI argument/env var
		channelOverride := getSnapChannelOverride(cfg, snap.Name)
		if channelOverride != "" {
			snap.Channel = channelOverride
		}
		plan.Snaps = append(plan.Snaps, snap)
	}

	// Add the ExtraSnaps specified in the overrides
	for _, s := range cfg.Overrides.ExtraSnaps {
		snap := system.NewSnapFromString(s)
		// Check if the channel has been overridden by a CLI argument/env var
		channelOverride := getSnapChannelOverride(cfg, snap.Name)
		if channelOverride != "" {
			snap.Channel = channelOverride
		}
		plan.Snaps = append(plan.Snaps, snap)
	}

	for _, p := range append(cfg.Host.Packages, cfg.Overrides.ExtraDebs...) {
		plan.Debs = append(plan.Debs, packages.NewDeb(p))
	}

	for _, providerName := range providers.SupportedProviders {
		if p := providers.NewProvider(providerName, worker, cfg); p != nil {
			plan.Providers = append(plan.Providers, p)

			// Warn if the configuration specifies to bootstrap the provider, but the config or
			// overrides disable Juju.
			if cfg.Overrides.DisableJuju && p.Bootstrap() {
				slog.Warn("provider will not be bootstrapped because juju is disabled", "provider", providerName)
			}
		}
	}

	if cfg.Overrides.DisableJuju {
		plan.config.Juju.Disable = true
	}

	return plan
}

// Execute either prepares or restores a given plan
func (p *Plan) Execute(action string) error {
	err := p.validate()
	if err != nil {
		return fmt.Errorf("failed to validate plan: %w", err)
	}

	var eg errgroup.Group

	snapHandler := packages.NewSnapHandler(p.system, p.Snaps)
	debHandler := packages.NewDebHandler(p.system, p.Debs)

	// Prepare/restore package handlers concurrently
	eg.Go(func() error { return DoAction(snapHandler, action) })
	eg.Go(func() error { return DoAction(debHandler, action) })
	if err := eg.Wait(); err != nil {
		return err
	}

	// Prepare/restore providers concurrently
	for _, provider := range p.Providers {
		eg.Go(func() error { return DoAction(provider, action) })
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	// Skip Juju handler if Juju is disabled in the config
	if p.config.Juju.Disable {
		return nil
	}

	// Prepare/Restore juju controllers
	jujuHandler := juju.NewJujuHandler(p.config, p.system, p.Providers)
	err = DoAction(jujuHandler, action)
	if err != nil {
		return fmt.Errorf("failed to prepare Juju: %w", err)
	}

	return nil
}

// validate returns an error if the generated plan contains errors that would prevent a successful
// configuration of the machine.
func (p *Plan) validate() error {
	var eg errgroup.Group

	// Run the validators in parallel in an errgroup
	for _, v := range planValidators {
		eg.Go(func() error { return v(p) })
	}
	if err := eg.Wait(); err != nil {
		return err
	}

	return nil
}

// getSnapChannelOverride takes the name of a snap. If the snap's version
// is overridden, the overridden channel is returned.
func getSnapChannelOverride(config *config.Config, snap string) string {
	switch snap {
	case "charmcraft":
		return config.Overrides.CharmcraftChannel
	case "snapcraft":
		return config.Overrides.SnapcraftChannel
	case "rockcraft":
		return config.Overrides.RockcraftChannel
	default:
		return ""
	}
}
