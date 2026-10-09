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

// TestIsLocalCloud keeps the list of local clouds in step with the providers:
// restore skips controllers on local clouds, so a provider whose controllers
// run elsewhere (and cost money) must never be reported as local.
func TestIsLocalCloud(t *testing.T) {
	system := system.NewMockSystem()
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.32-strict/stable"

	local := map[string]bool{"lxd": true, "microk8s": true, "k8s": true, "google": false}

	for _, name := range SupportedProviders {
		want, ok := local[name]
		if !ok {
			t.Fatalf("provider %q is missing from this test: decide whether its controllers run on this machine", name)
		}
		var p Provider
		switch name {
		case "lxd":
			p = NewLXD(system, cfg)
		case "microk8s":
			p = NewMicroK8s(system, cfg)
		case "k8s":
			p = NewK8s(system, cfg)
		case "google":
			p = NewGoogle(system, cfg)
		}
		if got := IsLocalCloud(p.CloudName()); got != want {
			t.Errorf("%s (cloud %q): expected IsLocalCloud %v, got %v", name, p.CloudName(), want, got)
		}
	}
}
