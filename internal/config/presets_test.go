package config

import (
	"reflect"
	"testing"
)

func TestValidPresets(t *testing.T) {
	expected := []string{"crafts", "dev", "k8s", "machine", "microk8s"}
	got := ValidPresets()
	if !reflect.DeepEqual(expected, got) {
		t.Fatalf("expected: %v, got: %v", expected, got)
	}
}

func TestPresetLoadsSuccessfully(t *testing.T) {
	for _, name := range ValidPresets() {
		t.Run(name, func(t *testing.T) {
			conf, err := Preset(name)
			if err != nil {
				t.Fatalf("failed to load preset '%s': %v", name, err)
			}
			if conf == nil {
				t.Fatalf("preset '%s' returned nil config", name)
			}
		})
	}
}

func TestPresetUnknownReturnsError(t *testing.T) {
	_, err := Preset("nonexistent")
	if err == nil {
		t.Fatal("expected error for unknown preset, got nil")
	}
}

func TestPresetMachineValues(t *testing.T) {
	conf, err := Preset("machine")
	if err != nil {
		t.Fatal(err)
	}

	// Juju should be enabled with model-defaults
	if conf.Juju.Disable {
		t.Fatal("expected juju to be enabled")
	}
	if conf.Juju.ModelDefaults["test-mode"] != "true" {
		t.Fatal("expected test-mode model default")
	}
	if conf.Juju.ModelDefaults["automatically-retry-hooks"] != "false" {
		t.Fatal("expected automatically-retry-hooks model default")
	}

	// LXD should be enabled with bootstrap
	if !conf.Providers.LXD.Enable {
		t.Fatal("expected LXD to be enabled")
	}
	if !conf.Providers.LXD.Bootstrap {
		t.Fatal("expected LXD bootstrap to be enabled")
	}

	// K8s and MicroK8s should not be enabled
	if conf.Providers.K8s.Enable {
		t.Fatal("expected K8s to be disabled")
	}
	if conf.Providers.MicroK8s.Enable {
		t.Fatal("expected MicroK8s to be disabled")
	}

	// Should have snapcraft snap
	if _, ok := conf.Host.Snaps["snapcraft"]; !ok {
		t.Fatal("expected snapcraft snap")
	}

	// Should have gnome-keyring package
	hasGnomeKeyring := false
	for _, p := range conf.Host.Packages {
		if p == "gnome-keyring" {
			hasGnomeKeyring = true
			break
		}
	}
	if !hasGnomeKeyring {
		t.Fatal("expected gnome-keyring in packages")
	}
}

func TestPresetK8sValues(t *testing.T) {
	conf, err := Preset("k8s")
	if err != nil {
		t.Fatal(err)
	}

	// LXD enabled but no bootstrap
	if !conf.Providers.LXD.Enable {
		t.Fatal("expected LXD to be enabled")
	}
	if conf.Providers.LXD.Bootstrap {
		t.Fatal("expected LXD bootstrap to be disabled")
	}

	// K8s should be enabled with bootstrap
	if !conf.Providers.K8s.Enable {
		t.Fatal("expected K8s to be enabled")
	}
	if !conf.Providers.K8s.Bootstrap {
		t.Fatal("expected K8s bootstrap to be enabled")
	}

	// K8s bootstrap constraints
	if conf.Providers.K8s.BootstrapConstraints["root-disk"] != "2G" {
		t.Fatalf("expected root-disk constraint '2G', got '%s'", conf.Providers.K8s.BootstrapConstraints["root-disk"])
	}

	// K8s features
	if _, ok := conf.Providers.K8s.Features["load-balancer"]; !ok {
		t.Fatal("expected load-balancer feature")
	}
	if conf.Providers.K8s.Features["load-balancer"]["l2-mode"] != "true" {
		t.Fatal("expected l2-mode in load-balancer feature")
	}

	// Should have rockcraft snap
	if _, ok := conf.Host.Snaps["rockcraft"]; !ok {
		t.Fatal("expected rockcraft snap")
	}
}

func TestPresetMicroK8sValues(t *testing.T) {
	conf, err := Preset("microk8s")
	if err != nil {
		t.Fatal(err)
	}

	// LXD enabled but no bootstrap
	if !conf.Providers.LXD.Enable {
		t.Fatal("expected LXD to be enabled")
	}
	if conf.Providers.LXD.Bootstrap {
		t.Fatal("expected LXD bootstrap to be disabled")
	}

	// MicroK8s should be enabled with bootstrap and addons
	if !conf.Providers.MicroK8s.Enable {
		t.Fatal("expected MicroK8s to be enabled")
	}
	if !conf.Providers.MicroK8s.Bootstrap {
		t.Fatal("expected MicroK8s bootstrap to be enabled")
	}
	expectedAddons := []string{
		"hostpath-storage",
		"dns",
		"rbac",
		"metallb:10.64.140.43-10.64.140.49",
	}
	if !reflect.DeepEqual(conf.Providers.MicroK8s.Addons, expectedAddons) {
		t.Fatalf("expected addons %v, got %v", expectedAddons, conf.Providers.MicroK8s.Addons)
	}

	// Should have rockcraft snap
	if _, ok := conf.Host.Snaps["rockcraft"]; !ok {
		t.Fatal("expected rockcraft snap")
	}
}

func TestPresetDevValues(t *testing.T) {
	conf, err := Preset("dev")
	if err != nil {
		t.Fatal(err)
	}

	// Both LXD and K8s should be enabled with bootstrap
	if !conf.Providers.LXD.Enable || !conf.Providers.LXD.Bootstrap {
		t.Fatal("expected LXD enabled with bootstrap")
	}
	if !conf.Providers.K8s.Enable || !conf.Providers.K8s.Bootstrap {
		t.Fatal("expected K8s enabled with bootstrap")
	}

	// Should have jhack snap with connections
	jhack, ok := conf.Host.Snaps["jhack"]
	if !ok {
		t.Fatal("expected jhack snap")
	}
	if len(jhack.Connections) == 0 {
		t.Fatal("expected jhack connections")
	}
	if jhack.Connections[0] != "jhack:dot-local-share-juju" {
		t.Fatalf("expected jhack connection 'jhack:dot-local-share-juju', got '%s'", jhack.Connections[0])
	}

	// Should have rockcraft and snapcraft
	if _, ok := conf.Host.Snaps["rockcraft"]; !ok {
		t.Fatal("expected rockcraft snap")
	}
	if _, ok := conf.Host.Snaps["snapcraft"]; !ok {
		t.Fatal("expected snapcraft snap")
	}
}

func TestPresetCraftsValues(t *testing.T) {
	conf, err := Preset("crafts")
	if err != nil {
		t.Fatal(err)
	}

	// Juju should be disabled
	if !conf.Juju.Disable {
		t.Fatal("expected juju to be disabled")
	}

	// LXD should be enabled with bootstrap
	if !conf.Providers.LXD.Enable || !conf.Providers.LXD.Bootstrap {
		t.Fatal("expected LXD enabled with bootstrap")
	}

	// Should have rockcraft and snapcraft
	if _, ok := conf.Host.Snaps["rockcraft"]; !ok {
		t.Fatal("expected rockcraft snap")
	}
	if _, ok := conf.Host.Snaps["snapcraft"]; !ok {
		t.Fatal("expected snapcraft snap")
	}

	// Should have default snaps too
	if _, ok := conf.Host.Snaps["charmcraft"]; !ok {
		t.Fatal("expected charmcraft snap")
	}
}
