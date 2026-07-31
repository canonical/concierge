package config

// Config represents concierge's configuration format.
type Config struct {
	Juju      jujuConfig     `yaml:"juju"`
	Providers providerConfig `yaml:"providers"`
	Host      hostConfig     `yaml:"host"`

	// The following are added at runtime according to CLI flags
	Overrides ConfigOverrides `yaml:"overrides"`
	Status    Status          `yaml:"status"`
	Verbose   bool            `yaml:"-"`
	Trace     bool            `yaml:"-"`
	DryRun    bool            `yaml:"-"`
}

// Status represents the status of concierge on a given machine.
type Status int

const (
	Provisioning Status = iota
	Succeeded
	Failed
)

// String returns a string representation of a given concierge status.
func (s Status) String() string {
	return [...]string{"provisioning", "succeeded", "failed"}[s]
}

// jujuConfig represents the configuration for juju, including the desired version,
// and defaults/constraints for the bootstrap process.
type jujuConfig struct {
	// Optionally disable the installation of Juju
	Disable bool `yaml:"disable"`
	// The Snap Store channel from which to install Juju
	Channel string `yaml:"channel"`
	// The Snap Store revision from which to install Juju. If both Channel and
	// Revision are specified, snap will use the revision and the channel is
	// only used for tracking after install.
	Revision string `yaml:"revision"`
	// The Juju agent version to use during bootstrap
	AgentVersion string `yaml:"agent-version"`
	// The set of model-defaults to be passed to Juju during bootstrap
	ModelDefaults map[string]string `yaml:"model-defaults"`
	// The set of bootstrap constraints to be passed to Juju
	BootstrapConstraints map[string]string `yaml:"bootstrap-constraints"`
	// Additional arbitrary arguments to be appended to the bootstrap command
	ExtraBootstrapArgs string `yaml:"extra-bootstrap-args"`
}

// providerConfig represents the set of providers to be configured and bootstrapped.
type providerConfig struct {
	K8s      k8sConfig      `yaml:"k8s"`
	LXD      lxdConfig      `yaml:"lxd"`
	Google   googleConfig   `yaml:"google"`
	MicroK8s microk8sConfig `yaml:"microk8s"`
}

// lxdConfig represents how LXD should be configured on the host.
type lxdConfig struct {
	Enable               bool              `yaml:"enable"`
	Bootstrap            bool              `yaml:"bootstrap"`
	Channel              string            `yaml:"channel"`
	ModelDefaults        map[string]string `yaml:"model-defaults"`
	BootstrapConstraints map[string]string `yaml:"bootstrap-constraints"`
}

// googleConfig represents how Juju should be configured for Google Cloud use.
type googleConfig struct {
	Enable               bool              `yaml:"enable"`
	Bootstrap            bool              `yaml:"bootstrap"`
	CredentialsFile      string            `yaml:"credentials-file"`
	ModelDefaults        map[string]string `yaml:"model-defaults"`
	BootstrapConstraints map[string]string `yaml:"bootstrap-constraints"`
}

// ImageRegistryConfig represents configuration for an image registry mirror.
type ImageRegistryConfig struct {
	URL      string `yaml:"url"`
	Username string `yaml:"username"`
	Password string `yaml:"password"`
}

// microk8sConfig represents how MicroK8s should be configured on the host.
type microk8sConfig struct {
	Enable               bool                `yaml:"enable"`
	Bootstrap            bool                `yaml:"bootstrap"`
	Channel              string              `yaml:"channel"`
	Addons               []string            `yaml:"addons"`
	ImageRegistry        ImageRegistryConfig `yaml:"image-registry"`
	ModelDefaults        map[string]string   `yaml:"model-defaults"`
	BootstrapConstraints map[string]string   `yaml:"bootstrap-constraints"`
}

// k8sConfig represents how K8s should be configured on the host.
type k8sConfig struct {
	Enable               bool                         `yaml:"enable"`
	Bootstrap            bool                         `yaml:"bootstrap"`
	Channel              string                       `yaml:"channel"`
	Features             map[string]map[string]string `yaml:"features"`
	ImageRegistry        ImageRegistryConfig          `yaml:"image-registry"`
	ModelDefaults        map[string]string            `yaml:"model-defaults"`
	BootstrapConstraints map[string]string            `yaml:"bootstrap-constraints"`
}

// SnapConfig represents the configuration for a specific snap to be installed.
type SnapConfig struct {
	// Channel is the channel from which to install the snap. If omitted, the default behaviour is decided by snapd.
	Channel string `yaml:"channel"`
	// Connections is a list of snap connections to form.
	Connections []string `yaml:"connections"`
}

// hostConfig is a top-level field containing addition configuration for the host being
// configured.
type hostConfig struct {
	// Packages is a of apt packages to be installed from the archive
	Packages []string `yaml:"packages"`
	// Snaps is a map of snaps to be installed.
	Snaps map[string]SnapConfig `yaml:"snaps"`
}
