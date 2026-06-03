package config

type ConfigOverrides struct {
	DisableJuju       bool
	K8sChannel        string
	JujuChannel       string
	JujuRevision      string
	MicroK8sChannel   string
	LXDChannel        string
	CharmcraftChannel string
	SnapcraftChannel  string
	RockcraftChannel  string

	GoogleCredentialFile string

	ExtraSnaps []string
	ExtraDebs  []string
}
