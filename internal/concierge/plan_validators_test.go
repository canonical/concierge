package concierge

import (
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

func TestSingleK8sValidator(t *testing.T) {
	system := system.NewMockSystem()

	twoK8s := &config.Config{}
	twoK8s.Providers.K8s.Enable = true
	twoK8s.Providers.MicroK8s.Enable = true

	plan := NewPlan(twoK8s, system)
	err := plan.validate()
	if err == nil {
		t.Fatalf("should not allow enabling two local kubernetes providers")
	}

	justK8s := &config.Config{}
	justK8s.Providers.K8s.Enable = true
	plan = NewPlan(justK8s, system)
	err = plan.validate()
	if err != nil {
		t.Fatalf("single kubernetes provider should be permitted")
	}

	justMicroK8s := &config.Config{}
	justMicroK8s.Providers.MicroK8s.Enable = true
	plan = NewPlan(justMicroK8s, system)
	err = plan.validate()
	if err != nil {
		t.Fatalf("single kubernetes provider should be permitted")
	}

}

func TestControllerNameValidator(t *testing.T) {
	system := system.NewMockSystem()

	tests := []struct {
		lxdName     string
		k8sName     string
		bootstrap   bool
		disableJuju bool
		valid       bool
	}{
		{"", "", true, false, true},
		{"dev", "prod-mirror", true, false, true},
		{"shared", "shared", true, false, false},
		{"Dev", "", true, false, false},
		{"-dev", "", true, false, false},
		{"my controller", "", true, false, false},
		{"shared", "shared", false, false, true},
		// An invalid name must not fail the plan when Juju is disabled, as
		// nothing is bootstrapped (the controller name is never used).
		{"Dev_Mirror", "", true, true, true},
	}

	for _, tc := range tests {
		cfg := &config.Config{}
		cfg.Providers.LXD.Enable = true
		cfg.Providers.LXD.Bootstrap = tc.bootstrap
		cfg.Providers.LXD.ControllerName = tc.lxdName
		cfg.Providers.K8s.Enable = true
		cfg.Providers.K8s.Bootstrap = tc.bootstrap
		cfg.Providers.K8s.ControllerName = tc.k8sName
		cfg.Overrides.DisableJuju = tc.disableJuju

		err := NewPlan(cfg, system).validate()
		if tc.valid && err != nil {
			t.Errorf("names %q and %q (bootstrap %v, disableJuju %v): unexpected error: %v", tc.lxdName, tc.k8sName, tc.bootstrap, tc.disableJuju, err)
		} else if !tc.valid && err == nil {
			t.Errorf("names %q and %q (bootstrap %v, disableJuju %v): expected an error", tc.lxdName, tc.k8sName, tc.bootstrap, tc.disableJuju)
		}
	}
}
