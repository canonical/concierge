package providers

import (
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestControllerName(t *testing.T) {
	system := system.NewMockSystem()

	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.32-strict/stable"
	for _, p := range []Provider{NewLXD(system, cfg), NewMicroK8s(system, cfg), NewK8s(system, cfg), NewGoogle(system, cfg)} {
		if got, want := p.ControllerName(), "concierge-"+p.Name(); got != want {
			t.Errorf("%s: expected: %v, got: %v", p.Name(), want, got)
		}
	}

	cfg.Providers.LXD.ControllerName = "lxd-dev"
	cfg.Providers.MicroK8s.ControllerName = "microk8s-dev"
	cfg.Providers.K8s.ControllerName = "k8s-dev"
	cfg.Providers.Google.ControllerName = "google-dev"
	for _, p := range []Provider{NewLXD(system, cfg), NewMicroK8s(system, cfg), NewK8s(system, cfg), NewGoogle(system, cfg)} {
		if got, want := p.ControllerName(), p.Name()+"-dev"; got != want {
			t.Errorf("%s: expected: %v, got: %v", p.Name(), want, got)
		}
	}
}
