package config

type ConfigOverrides struct {
	DisableJuju       bool
	K8sChannel        string
	JujuChannel       string
	MicroK8sChannel   string
	LXDChannel        string
	CharmcraftChannel string
	SnapcraftChannel  string
	RockcraftChannel  string

	// GlobalTimeout is the timeout value in seconds for provider wait/status commands
	GlobalTimeout string

	GoogleCredentialFile string

	ExtraSnaps []string
	ExtraDebs  []string
}
