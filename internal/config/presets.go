package config

import "fmt"

// Preset returns a configuration preset by name.
func Preset(preset string) (*Config, error) {
	switch preset {
	case "k8s":
		return k8sPreset, nil
	case "microk8s":
		return microk8sPreset, nil
	case "machine":
		return machinePreset, nil
	case "dev":
		return devPreset, nil
	case "crafts":
		return craftsPreset, nil
	default:
		return nil, fmt.Errorf("unknown preset '%s'", preset)
	}
}

// defaultJujuConfig is the default Juju config for all presets.
var defaultJujuConfig jujuConfig = jujuConfig{
	Disable: false,
	ModelDefaults: map[string]string{
		"test-mode":                 "true",
		"automatically-retry-hooks": "false",
	},
}

// defaultPackages is the set of packages installed for all presets.
var defaultPackages []string = []string{
	"gnome-keyring",
	"python3-pip",
	"python3-venv",
}

// defaultSnaps is the set of snaps installed for all presets.
var defaultSnaps map[string]SnapConfig = map[string]SnapConfig{
	"charmcraft": {Channel: "latest/stable"},
	"jq":         {Channel: "latest/stable"},
	"yq":         {Channel: "latest/stable"},
}

// defaultLXDConfig is the standard LXD config used throughout presets.
var defaultLXDConfig lxdConfig = lxdConfig{
	Enable:    true,
	Bootstrap: true,
}

// defaultMicroK8sConfig is the standard MicroK8s config used throughout presets.
var defaultMicroK8sConfig microk8sConfig = microk8sConfig{
	Enable:    true,
	Bootstrap: true,
	Addons: []string{
		"hostpath-storage",
		"dns",
		"rbac",
		"metallb:10.64.140.43-10.64.140.49",
	},
}

// defaultK8sConfig is the standard K8s config used throughout presets.
var defaultK8sConfig k8sConfig = k8sConfig{
	Enable:               true,
	Bootstrap:            true,
	BootstrapConstraints: map[string]string{"root-disk": "2G"},
	Features: map[string]map[string]string{
		"load-balancer": {
			"l2-mode": "true",
			"cidrs":   "10.43.45.0/28",
		},
		"local-storage": {},
		"network":       {},
	},
}

// machinePreset is a configuration preset designed to be used when testing
// machine charms.
var machinePreset *Config = &Config{
	Juju: defaultJujuConfig,
	Providers: providerConfig{
		LXD: defaultLXDConfig,
	},
	Host: hostConfig{
		Packages: defaultPackages,
		Snaps: MergeMaps(defaultSnaps, map[string]SnapConfig{
			"snapcraft": {Channel: "latest/stable"},
		}),
	},
}

// k8sPreset is a configuration preset designed to be used when testing
// k8s charms.
var k8sPreset *Config = &Config{
	Juju: defaultJujuConfig,
	Providers: providerConfig{
		// Enable LXD so charms can be built, but don't bootstrap onto it.
		LXD: lxdConfig{Enable: true},
		K8s: defaultK8sConfig,
	},
	Host: hostConfig{
		Packages: defaultPackages,
		Snaps: MergeMaps(defaultSnaps, map[string]SnapConfig{
			"rockcraft": {Channel: "latest/stable"},
		}),
	},
}

// microk8sPreset is a configuration preset designed to be used when testing
// k8s charms.
var microk8sPreset *Config = &Config{
	Juju: defaultJujuConfig,
	Providers: providerConfig{
		// Enable LXD so charms can be built, but don't bootstrap onto it.
		LXD:      lxdConfig{Enable: true},
		MicroK8s: defaultMicroK8sConfig,
	},
	Host: hostConfig{
		Packages: defaultPackages,
		Snaps: MergeMaps(defaultSnaps, map[string]SnapConfig{
			"rockcraft": {Channel: "latest/stable"},
		}),
	},
}

// devPreset combines both the LXD and K8s presets, designed to be used by
// developers when iterating on charms.
var devPreset *Config = &Config{
	Juju: defaultJujuConfig,
	Providers: providerConfig{
		LXD: defaultLXDConfig,
		K8s: defaultK8sConfig,
	},
	Host: hostConfig{
		Packages: defaultPackages,
		Snaps: MergeMaps(defaultSnaps, map[string]SnapConfig{
			"rockcraft": {Channel: "latest/stable"},
			"snapcraft": {Channel: "latest/stable"},
			"jhack":     {Channel: "latest/stable", Connections: []string{"jhack:dot-local-share-juju"}},
		}),
	},
}

// craftsPreset installs each of the crafts, and configures LXD, but disables Juju.
// Useful for workflows where only artifacts need to be built.
var craftsPreset *Config = &Config{
	Juju: jujuConfig{
		Disable: true,
	},
	Providers: providerConfig{
		LXD: defaultLXDConfig,
	},
	Host: hostConfig{
		Packages: defaultPackages,
		Snaps: MergeMaps(defaultSnaps, map[string]SnapConfig{
			"rockcraft": {Channel: "latest/stable"},
			"snapcraft": {Channel: "latest/stable"},
		}),
	},
}
