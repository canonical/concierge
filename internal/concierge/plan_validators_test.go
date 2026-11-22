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

func TestMetalLBIPRangeValidator(t *testing.T) {
	system := system.NewMockSystem()

	tests := []struct {
		configured string
		override   string
		valid      bool
	}{
		{"", "", true},
		{"10.64.140.43-10.64.140.49", "", true},
		{"192.168.1.0/28", "", true},
		{"10.64.140.43-10.64.140.49,192.168.1.0/28", "", true},
		{"fd00::10-fd00::20", "", true},
		{"fd00::/120", "", true},
		{"10.64.140.43", "", false},
		{"10.64.140.49-10.64.140.43", "", false},
		{"10.64.140.43-fd00::20", "", false},
		{"10.64.140.43-10.64.140.300", "", false},
		{"10.64.140.43-10.64.140.49,", "", false},
		{"auto", "", false},
		// The override is what gets used, so it's what gets checked.
		{"auto", "10.0.0.1-10.0.0.9", true},
		{"10.0.0.1-10.0.0.9", "auto", false},
	}

	for _, tc := range tests {
		cfg := &config.Config{}
		cfg.Providers.MicroK8s.Enable = true
		cfg.Providers.MicroK8s.MetalLBIPRange = tc.configured
		cfg.Overrides.MetalLBIPRange = tc.override

		err := NewPlan(cfg, system).validate()
		if tc.valid && err != nil {
			t.Errorf("range %q (override %q): unexpected error: %v", tc.configured, tc.override, err)
		} else if !tc.valid && err == nil {
			t.Errorf("range %q (override %q): expected an error", tc.configured, tc.override)
		}
	}
}

func TestMetalLBIPRangeValidatorIgnoredWithoutMicroK8s(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.LXD.Enable = true
	cfg.Overrides.MetalLBIPRange = "not a range"

	if err := NewPlan(cfg, system.NewMockSystem()).validate(); err != nil {
		t.Fatalf("unexpected error without MicroK8s: %v", err)
	}
}
