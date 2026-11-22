package providers

import (
	"fmt"
	"os"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

var defaultAddons []string = []string{
	"hostpath-storage",
	"dns",
	"rbac",
	"metallb:10.64.140.43-10.64.140.49",
}

func TestNewMicroK8s(t *testing.T) {
	type test struct {
		config         *config.Config
		expected       *MicroK8s
		snapChannel    string
		defaultChannel string
	}

	noOverrides := &config.Config{}

	channelInConfig := &config.Config{}
	channelInConfig.Providers.MicroK8s.Channel = "1.29-strict/stable"

	overrides := &config.Config{}
	overrides.Overrides.MicroK8sChannel = "1.30/edge"
	overrides.Providers.MicroK8s.Addons = defaultAddons

	system := system.NewMockSystem()

	tests := []test{
		{
			config:         noOverrides,
			expected:       &MicroK8s{Channel: defaultMicroK8sChannel, system: system},
			snapChannel:    "",
			defaultChannel: defaultMicroK8sChannel,
		},
		{
			config:         channelInConfig,
			expected:       &MicroK8s{Channel: "1.29-strict/stable", system: system},
			snapChannel:    "1.29-strict/stable",
			defaultChannel: "",
		},
		{
			config:         overrides,
			expected:       &MicroK8s{Channel: "1.30/edge", Addons: defaultAddons, system: system},
			snapChannel:    "1.30/edge",
			defaultChannel: "",
		},
	}

	for _, tc := range tests {
		uk8s := NewMicroK8s(system, tc.config)

		// Check the constructed snaps are correct
		if uk8s.snaps[0].Channel != tc.snapChannel {
			t.Fatalf("expected: %v, got: %v", tc.snapChannel, uk8s.snaps[0].Channel)
		}
		if uk8s.snaps[0].DefaultChannel != tc.defaultChannel {
			t.Fatalf("expected: %v, got: %v", tc.defaultChannel, uk8s.snaps[0].DefaultChannel)
		}

		// Remove the snaps so the rest of the object can be compared
		uk8s.snaps = nil
		if !reflect.DeepEqual(tc.expected, uk8s) {
			t.Fatalf("expected: %v, got: %v", tc.expected, uk8s)
		}
	}
}

func TestMicroK8sGroupName(t *testing.T) {
	type test struct {
		channel  string
		expected string
	}

	tests := []test{
		{channel: "1.30-strict/stable", expected: "snap_microk8s"},
		{channel: "1.30/stable", expected: "microk8s"},
	}

	for _, tc := range tests {
		config := &config.Config{}
		config.Providers.MicroK8s.Channel = tc.channel
		uk8s := NewMicroK8s(system.NewMockSystem(), config)

		if !reflect.DeepEqual(tc.expected, uk8s.GroupName()) {
			t.Fatalf("expected: %v, got: %v", tc.expected, uk8s.GroupName())
		}
	}
}

func TestMicroK8sPrepareCommands(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroK8s.Channel = "1.31-strict/stable"
	config.Providers.MicroK8s.Addons = defaultAddons

	expectedCommands := []string{
		"snap install microk8s --channel 1.31-strict/stable",
		"snap install kubectl --channel stable",
		"microk8s status --wait-ready --timeout 270",
		"microk8s enable hostpath-storage",
		"microk8s enable dns",
		"microk8s enable rbac",
		"microk8s enable metallb:10.64.140.43-10.64.140.49",
	}
	expectedCommands = append(expectedCommands, metalLBCheckCommands(false)...)
	expectedCommands = append(expectedCommands,
		"usermod -a -G snap_microk8s test-user",
		"microk8s config",
	)

	expectedFiles := map[string]string{
		path.Join(os.TempDir(), ".kube", "config"): "",
	}

	system := system.NewMockSystem()
	mockMetalLBAddress(system, "10.64.140.43")
	uk8s := NewMicroK8s(system, config)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}

	if !reflect.DeepEqual(expectedFiles, system.CreatedFiles) {
		t.Fatalf("expected: %v, got: %v", expectedFiles, system.CreatedFiles)
	}
}

func TestMicroK8sRestore(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroK8s.Channel = "1.31-strict/stable"
	config.Providers.MicroK8s.Addons = defaultAddons

	system := system.NewMockSystem()
	uk8s := NewMicroK8s(system, config)
	if err := uk8s.Restore(); err != nil {
		t.Fatal(err)
	}

	expectedRemovedPaths := []string{path.Join(os.TempDir(), ".kube")}

	if !slices.Equal(expectedRemovedPaths, system.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, system.RemovedPaths)
	}

	expectedCommands := []string{
		"snap remove microk8s --purge",
		"snap remove kubectl --purge",
	}

	if !slices.Equal(expectedCommands, system.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, system.ExecutedCommands)
	}
}

func TestMicroK8sImageRegistryConfig(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = defaultAddons
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)

	// Check that ImageRegistry was set correctly
	if uk8s.ImageRegistry.URL != "https://mirror.example.com" {
		t.Fatalf("expected ImageRegistry URL to be 'https://mirror.example.com', got: %v", uk8s.ImageRegistry.URL)
	}
}

func TestMicroK8sPrepareWithImageRegistry(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = defaultAddons
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"

	expectedCommands := []string{
		"snap install microk8s --channel 1.31-strict/stable",
		"snap install kubectl --channel stable",
		"microk8s status --wait-ready --timeout 270",
		"microk8s stop",
		"microk8s start",
		"microk8s status --wait-ready --timeout 270",
		"microk8s enable hostpath-storage",
		"microk8s enable dns",
		"microk8s enable rbac",
		"microk8s enable metallb:10.64.140.43-10.64.140.49",
	}
	expectedCommands = append(expectedCommands, metalLBCheckCommands(false)...)
	expectedCommands = append(expectedCommands,
		"usermod -a -G snap_microk8s test-user",
		"microk8s config",
	)

	sys := system.NewMockSystem()
	mockMetalLBAddress(sys, "10.64.140.43")
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	kubeConfigPath := path.Join(sys.User().HomeDir, ".kube", "config")
	kubeDir := path.Join(sys.User().HomeDir, ".kube")
	expectedFiles := map[string]string{
		kubeConfigPath: "",
		"/var/snap/microk8s/current/args/certs.d/docker.io/hosts.toml": "server = \"https://mirror.example.com\"\n\n[host.\"https://mirror.example.com\"]\ncapabilities = [\"pull\", \"resolve\"]\n",
	}

	expectedDirs := []string{
		"/var/snap/microk8s/current/args/certs.d/docker.io",
		kubeDir,
	}

	if !slices.Equal(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected commands: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}

	if !reflect.DeepEqual(expectedFiles, sys.CreatedFiles) {
		t.Fatalf("expected files: %v, got: %v", expectedFiles, sys.CreatedFiles)
	}

	if !slices.Equal(expectedDirs, sys.CreatedDirectories) {
		t.Fatalf("expected directories: %v, got: %v", expectedDirs, sys.CreatedDirectories)
	}
}

func TestMicroK8sPrepareWithImageRegistryAndAuth(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{}
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"
	cfg.Providers.MicroK8s.ImageRegistry.Username = "testuser"
	cfg.Providers.MicroK8s.ImageRegistry.Password = "testpass"

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	hostsToml := sys.CreatedFiles["/var/snap/microk8s/current/args/certs.d/docker.io/hosts.toml"]

	// Check that the auth header is present (base64 of "testuser:testpass")
	expectedAuth := "dGVzdHVzZXI6dGVzdHBhc3M=" // base64("testuser:testpass")
	if !strings.Contains(hostsToml, expectedAuth) {
		t.Fatalf("expected hosts.toml to contain base64-encoded credentials, got: %v", hostsToml)
	}

	if !strings.Contains(hostsToml, "Authorization = [\"Basic") {
		t.Fatalf("expected hosts.toml to contain authorization header, got: %v", hostsToml)
	}
}

func TestMicroK8sBuildHostsToml(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistry.URL = "https://mirror.example.com"

	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)

	hostsToml := uk8s.buildHostsToml()

	expectedContent := `server = "https://mirror.example.com"

[host."https://mirror.example.com"]
capabilities = ["pull", "resolve"]
`

	if hostsToml != expectedContent {
		t.Fatalf("expected:\n%v\ngot:\n%v", expectedContent, hostsToml)
	}
}

func TestMicroK8sPrepareKeepsInstalledChannel(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroK8s.Addons = defaultAddons

	system := system.NewMockSystem()
	// Confinement is looked up on the strict default channel, so it's not
	// classic, and a refresh on the tracking channel doesn't need --classic.
	system.MockSnapStoreLookup("microk8s", "1.30/stable", false, true)
	mockMetalLBAddress(system, "10.64.140.43")

	uk8s := NewMicroK8s(system, config)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	if !slices.Contains(system.ExecutedCommands, "snap refresh microk8s") {
		t.Fatalf("expected microk8s to be refreshed on its tracking channel, got: %v", system.ExecutedCommands)
	}

	// The group follows the installed channel, which is not strict.
	if !slices.Contains(system.ExecutedCommands, "usermod -a -G microk8s test-user") {
		t.Fatalf("expected the user to be added to the microk8s group, got: %v", system.ExecutedCommands)
	}
}

func TestMicroK8sInstallSkipSnapRefresh(t *testing.T) {
	config := &config.Config{}
	config.Providers.MicroK8s.Channel = "1.31-strict/stable"
	config.Overrides.SkipSnapRefresh = true

	system := system.NewMockSystem()
	system.MockSnapStoreLookup("microk8s", "1.31-strict/stable", false, true)
	system.MockSnapStoreLookup("kubectl", "latest/stable", true, true)
	system.StoreLookups = nil // ignore lookups made while setting up

	uk8s := NewMicroK8s(system, config)
	if err := uk8s.install(); err != nil {
		t.Fatal(err)
	}

	// Both snaps are already on the requested channel, so neither is refreshed or looked up.
	for _, cmd := range system.ExecutedCommands {
		if strings.HasPrefix(cmd, "snap ") {
			t.Fatalf("expected no snap commands, got: %v", system.ExecutedCommands)
		}
	}
	if len(system.StoreLookups) != 0 {
		t.Fatalf("expected no snap store lookups, got: %v", system.StoreLookups)
	}
}

func TestMicroK8sInstallSkipSnapRefreshKeepsInstalledChannel(t *testing.T) {
	config := &config.Config{}
	config.Overrides.SkipSnapRefresh = true

	system := system.NewMockSystem()
	system.MockSnapStoreLookup("microk8s", "1.30/stable", false, true)
	system.MockSnapStoreLookup("kubectl", "latest/stable", true, true)

	uk8s := NewMicroK8s(system, config)
	system.StoreLookups = nil // ignore lookups made while setting up
	if err := uk8s.install(); err != nil {
		t.Fatal(err)
	}

	// With no channel requested, the installed channel will do, so nothing is
	// refreshed, and finding that channel for GroupName needs only the local snapd.
	for _, cmd := range system.ExecutedCommands {
		if strings.HasPrefix(cmd, "snap ") {
			t.Fatalf("expected no snap commands, got: %v", system.ExecutedCommands)
		}
	}
	if len(system.StoreLookups) != 0 {
		t.Fatalf("expected no snap store lookups, got: %v", system.StoreLookups)
	}
	if uk8s.Channel != "1.30/stable" {
		t.Fatalf("expected the channel to be the installed one, got: %q", uk8s.Channel)
	}
}

// kubectlString returns the command string MicroK8s.kubectl produces for args.
func kubectlString(args ...string) string {
	return (&MicroK8s{}).kubectl(args...).CommandString()
}

var (
	metalLBCRDCheck = kubectlString("get", "crd", "configurationstates.metallb.io", "--ignore-not-found", "-o", "name")
	metalLBCanI     = kubectlString("auth", "can-i", "list", "configurationstates.metallb.io",
		"--as=system:serviceaccount:metallb-system:controller", "-n", "metallb-system")
	metalLBCreate = kubectlString("-n", "default", "create", "service", "loadbalancer", metalLBCheckService, "--tcp=80:80")
	metalLBGet    = kubectlString("-n", "default", "get", "service", metalLBCheckService,
		"-o", "jsonpath={.status.loadBalancer.ingress[0].ip}")
	metalLBDelete = kubectlString("-n", "default", "delete", "service", metalLBCheckService, "--ignore-not-found")
)

// mockMetalLBAddress makes the MetalLB check Service get address straight away.
func mockMetalLBAddress(sys *system.MockSystem, address string) {
	sys.MockCommandReturn(metalLBGet, []byte(address), nil)
}

// metalLBCheckCommands returns the commands run after enabling MetalLB on an
// addon without the ConfigurationState CRD, or one with it whose controller
// can already use it (checkedRBAC).
func metalLBCheckCommands(checkedRBAC bool) []string {
	cmds := []string{metalLBCRDCheck}
	if checkedRBAC {
		cmds = append(cmds, metalLBCanI)
	}
	return append(cmds, metalLBDelete, metalLBCreate, metalLBGet, metalLBDelete)
}

func TestNewMicroK8sMetalLBIPRange(t *testing.T) {
	tests := map[string]struct {
		configured string
		override   string
		expected   string
		forced     bool
	}{
		"none":       {expected: ""},
		"configured": {configured: "192.168.1.240-192.168.1.250", expected: "192.168.1.240-192.168.1.250"},
		"override":   {override: "10.0.0.1-10.0.0.9", expected: "10.0.0.1-10.0.0.9", forced: true},
		"both":       {configured: "192.168.1.240-192.168.1.250", override: "10.0.0.1-10.0.0.9", expected: "10.0.0.1-10.0.0.9", forced: true},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
			cfg.Providers.MicroK8s.MetalLBIPRange = tc.configured
			cfg.Overrides.MetalLBIPRange = tc.override

			uk8s := NewMicroK8s(system.NewMockSystem(), cfg)
			if uk8s.MetalLBIPRange != tc.expected || uk8s.metalLBIPRangeForced != tc.forced {
				t.Fatalf("expected range %q (forced %v), got %q (forced %v)",
					tc.expected, tc.forced, uk8s.MetalLBIPRange, uk8s.metalLBIPRangeForced)
			}
		})
	}
}

func TestMicroK8sEnableAddonsMetalLBRange(t *testing.T) {
	tests := map[string]struct {
		addon      string
		configured string
		override   string
		expected   string
	}{
		"bare uses fallback": {
			addon: "metallb", expected: "microk8s enable metallb:10.64.140.43-10.64.140.49",
		},
		"bare uses configured range": {
			addon: "metallb", configured: "192.168.1.240-192.168.1.250",
			expected: "microk8s enable metallb:192.168.1.240-192.168.1.250",
		},
		"explicit range kept over configured range": {
			addon: "metallb:10.1.1.1-10.1.1.5", configured: "192.168.1.240-192.168.1.250",
			expected: "microk8s enable metallb:10.1.1.1-10.1.1.5",
		},
		"override replaces explicit range": {
			addon: "metallb:10.64.140.43-10.64.140.49", override: "192.168.1.240-192.168.1.250",
			expected: "microk8s enable metallb:192.168.1.240-192.168.1.250",
		},
	}

	for name, tc := range tests {
		t.Run(name, func(t *testing.T) {
			cfg := &config.Config{}
			cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
			cfg.Providers.MicroK8s.Addons = []string{tc.addon}
			cfg.Providers.MicroK8s.MetalLBIPRange = tc.configured
			cfg.Overrides.MetalLBIPRange = tc.override

			sys := system.NewMockSystem()
			mockMetalLBAddress(sys, "10.64.140.43")
			if err := NewMicroK8s(sys, cfg).enableAddons(); err != nil {
				t.Fatal(err)
			}

			expected := append([]string{tc.expected}, metalLBCheckCommands(false)...)
			if !slices.Equal(expected, sys.ExecutedCommands) {
				t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
			}
		})
	}
}

func TestMicroK8sNoMetalLBCheckWithoutMetalLB(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"dns"}

	sys := system.NewMockSystem()
	if err := NewMicroK8s(sys, cfg).enableAddons(); err != nil {
		t.Fatal(err)
	}

	expected := []string{"microk8s enable dns"}
	if !slices.Equal(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}
}

func TestMicroK8sPatchesMetalLBRBAC(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.36-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := system.NewMockSystem()
	sys.MockCommandReturn(metalLBCRDCheck, []byte("customresourcedefinition.apiextensions.k8s.io/configurationstates.metallb.io\n"), nil)
	sys.MockCommandReturn(metalLBCanI, []byte("no\n"), fmt.Errorf("exit status 1"))
	mockMetalLBAddress(sys, "10.64.140.43")

	if err := NewMicroK8s(sys, cfg).enableAddons(); err != nil {
		t.Fatal(err)
	}

	for _, want := range []string{
		"clusterrole metallb-system:controller --type=json",
		"clusterrole metallb-system:speaker --type=json",
		"rollout restart deployment/controller",
		"rollout restart daemonset/speaker",
		"rollout status deployment/controller",
		"rollout status daemonset/speaker",
	} {
		if !slices.ContainsFunc(sys.ExecutedCommands, func(c string) bool { return strings.Contains(c, want) }) {
			t.Fatalf("expected a command containing %q, got: %v", want, sys.ExecutedCommands)
		}
	}
	if !slices.Contains(sys.ExecutedCommands, metalLBCreate) {
		t.Fatalf("expected the MetalLB check after patching, got: %v", sys.ExecutedCommands)
	}
}

func TestMicroK8sSkipsMetalLBRBACPatchWhenControllerCanUseCRD(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.36-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := system.NewMockSystem()
	sys.MockCommandReturn(metalLBCRDCheck, []byte("customresourcedefinition.apiextensions.k8s.io/configurationstates.metallb.io\n"), nil)
	sys.MockCommandReturn(metalLBCanI, []byte("yes\n"), nil)
	mockMetalLBAddress(sys, "10.64.140.43")

	if err := NewMicroK8s(sys, cfg).enableAddons(); err != nil {
		t.Fatal(err)
	}

	expected := append([]string{"microk8s enable metallb:10.64.140.43-10.64.140.49"}, metalLBCheckCommands(true)...)
	if !slices.Equal(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}
}

func TestMicroK8sMetalLBCheckTimesOut(t *testing.T) {
	timeout, interval := metalLBCheckTimeout, metalLBCheckInterval
	metalLBCheckTimeout, metalLBCheckInterval = 0, 0
	t.Cleanup(func() { metalLBCheckTimeout, metalLBCheckInterval = timeout, interval })

	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := system.NewMockSystem()
	sys.MockCommandReturn(kubectlString("-n", "metallb-system", "logs", "deployment/controller", "--tail=20"),
		[]byte("failed to list *v1beta1.ConfigurationState: forbidden"), nil)

	err := NewMicroK8s(sys, cfg).enableAddons()
	if err == nil || !strings.Contains(err.Error(), "ConfigurationState: forbidden") {
		t.Fatalf("expected an error with the controller log, got: %v", err)
	}
	if !slices.Contains(sys.ExecutedCommands, metalLBDelete) {
		t.Fatalf("expected the check Service to be deleted, got: %v", sys.ExecutedCommands)
	}
}

func TestMicroK8sMetalLBCheckDoesNotWaitInDryRun(t *testing.T) {
	// With no wait, a check that runs in a dry run fails straight away.
	timeout, interval := metalLBCheckTimeout, metalLBCheckInterval
	metalLBCheckTimeout, metalLBCheckInterval = 0, 0
	t.Cleanup(func() { metalLBCheckTimeout, metalLBCheckInterval = timeout, interval })

	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := system.NewMockSystem()
	if err := NewMicroK8s(system.NewDryRunWorker(sys), cfg).enableAddons(); err != nil {
		t.Fatal(err)
	}
}

func TestMicroK8sMetalLBRBACCheckFails(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.36-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := system.NewMockSystem()
	sys.MockCommandReturn(metalLBCRDCheck, []byte("customresourcedefinition.apiextensions.k8s.io/configurationstates.metallb.io\n"), nil)
	sys.MockCommandReturn(metalLBCanI, []byte("error: connection refused\n"), fmt.Errorf("exit status 1"))

	err := NewMicroK8s(sys, cfg).enableAddons()
	if err == nil || !strings.Contains(err.Error(), "permissions") {
		t.Fatalf("expected a permissions check error, got: %v", err)
	}
}

func TestMicroK8sMetalLBCRDLookupFails(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.36-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := system.NewMockSystem()
	sys.MockCommandReturn(metalLBCRDCheck, []byte("The connection to the server was refused\n"), fmt.Errorf("exit status 1"))

	err := NewMicroK8s(sys, cfg).enableAddons()
	if err == nil || !strings.Contains(err.Error(), "ConfigurationState CRD") {
		t.Fatalf("expected a CRD lookup error, got: %v", err)
	}
	if slices.Contains(sys.ExecutedCommands, metalLBCanI) {
		t.Fatalf("expected no permissions check after the lookup failed, got: %v", sys.ExecutedCommands)
	}
}

// failingCleanupSystem fails the second delete of the MetalLB check Service,
// which is the cleanup after the check (the first clears out an old one).
type failingCleanupSystem struct {
	*system.MockSystem
	deletes int
}

func (s *failingCleanupSystem) Run(c *system.Command) ([]byte, error) {
	output, err := s.MockSystem.Run(c)
	if c.CommandString() == metalLBDelete {
		s.deletes++
		if s.deletes == 2 {
			return nil, fmt.Errorf("exit status 1")
		}
	}
	return output, err
}

func TestMicroK8sMetalLBCheckReportsFailedCleanup(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.Addons = []string{"metallb"}

	sys := &failingCleanupSystem{MockSystem: system.NewMockSystem()}
	mockMetalLBAddress(sys.MockSystem, "10.64.140.43")

	err := NewMicroK8s(sys, cfg).enableAddons()
	if err == nil || !strings.Contains(err.Error(), "failed to delete the MetalLB check Service") {
		t.Fatalf("expected a cleanup error, got: %v", err)
	}
}
