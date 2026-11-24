package providers

import (
	"bytes"
	"fmt"
	"log"
	"log/slog"
	"path"
	"reflect"
	"slices"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/system"
)

// requesterRegistries is a Docker Hub mirror plus ghcr.io pull credentials:
// the case from https://github.com/canonical/concierge/issues/181.
var requesterRegistries = []config.RegistryConfig{
	{Registry: "docker.io", Mirror: "https://mirror.example.com"},
	{Registry: "ghcr.io", Username: "ghcr-user", Password: "ghcr-token"},
}

const mirrorHostsToml = `[host."https://mirror.example.com"]
capabilities = ["pull", "resolve"]
`

const k8sRequesterBootstrapConfig = `cluster-config:
    dns:
        enabled: true
    gateway:
        enabled: true
    local-storage:
        enabled: true
    network:
        enabled: true
extra-node-containerd-config:
    plugins:
        io.containerd.grpc.v1.cri:
            registry:
                configs:
                    ghcr.io:
                        auth:
                            password: ghcr-token
                            username: ghcr-user
`

const microk8sBaseTemplate = `version = 2

[plugins."io.containerd.grpc.v1.cri".registry]
  config_path = "${SNAP_DATA}/args/certs.d"
`

const microk8sRequesterTemplate = microk8sBaseTemplate + `
# BEGIN concierge image-registries (managed by concierge, do not edit)
[plugins."io.containerd.grpc.v1.cri".registry.configs."ghcr.io".auth]
  username = "ghcr-user"
  password = "ghcr-token"
# END concierge image-registries
`

// captureLogs redirects the default logger to the returned buffer for the rest
// of the test.
func captureLogs(t *testing.T) *bytes.Buffer {
	t.Helper()
	var buf bytes.Buffer
	defaultLogger := slog.Default()
	// slog.SetDefault also redirects the log package's output, and restoring
	// the old default doesn't undo that, so restore it separately.
	logWriter, logFlags := log.Writer(), log.Flags()
	t.Cleanup(func() {
		slog.SetDefault(defaultLogger)
		log.SetOutput(logWriter)
		log.SetFlags(logFlags)
	})
	slog.SetDefault(slog.New(slog.NewTextHandler(&buf, nil)))
	return &buf
}

func TestValidateRegistries(t *testing.T) {
	tests := []struct {
		name       string
		registries []config.RegistryConfig
		legacy     config.ImageRegistryConfig
		wantErr    string
	}{
		{name: "requester", registries: requesterRegistries},
		{name: "mirror and auth", registries: []config.RegistryConfig{{Registry: "registry.example.com:5000", Mirror: "http://localhost:5000", Username: "u", Password: "p"}}},
		{name: "legacy plus ghcr.io", registries: requesterRegistries[1:], legacy: config.ImageRegistryConfig{URL: "https://mirror.example.com"}},
		{name: "no registry", registries: []config.RegistryConfig{{Mirror: "https://mirror.example.com"}}, wantErr: "invalid registry"},
		{name: "path in registry", registries: []config.RegistryConfig{{Registry: "../etc", Mirror: "https://mirror.example.com"}}, wantErr: "invalid registry"},
		{name: "scheme in registry", registries: []config.RegistryConfig{{Registry: "https://ghcr.io", Username: "u", Password: "p"}}, wantErr: "invalid registry"},
		{name: "duplicate", registries: []config.RegistryConfig{requesterRegistries[1], requesterRegistries[1]}, wantErr: "more than one entry"},
		{name: "legacy plus docker.io", registries: requesterRegistries[:1], legacy: config.ImageRegistryConfig{URL: "https://mirror.example.com"}, wantErr: "more than one entry"},
		{name: "username only", registries: []config.RegistryConfig{{Registry: "ghcr.io", Username: "u"}}, wantErr: "both username and password"},
		{name: "password only", registries: []config.RegistryConfig{{Registry: "ghcr.io", Password: "p"}}, wantErr: "both username and password"},
		{name: "nothing to do", registries: []config.RegistryConfig{{Registry: "ghcr.io"}}, wantErr: "must set a mirror"},
		{name: "same credentials host", registries: []config.RegistryConfig{{Registry: "docker.io", Mirror: "http://localhost:5000", Username: "u", Password: "p"}, {Registry: "quay.io", Mirror: "http://localhost:5000/quay", Username: "u", Password: "p"}}, wantErr: "both set credentials"},
		{name: "mirror without scheme", registries: []config.RegistryConfig{{Registry: "docker.io", Mirror: "mirror.example.com"}}, wantErr: "invalid mirror"},
	}

	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			err := validateRegistries(tc.registries, tc.legacy)
			if tc.wantErr == "" {
				if err != nil {
					t.Fatalf("expected no error, got: %v", err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("expected error containing %q, got: %v", tc.wantErr, err)
			}
		})
	}
}

func TestRegistryAuthHost(t *testing.T) {
	tests := []struct {
		registry config.RegistryConfig
		want     string
	}{
		{config.RegistryConfig{Registry: "ghcr.io"}, "ghcr.io"},
		{config.RegistryConfig{Registry: "docker.io"}, "registry-1.docker.io"},
		{config.RegistryConfig{Registry: "docker.io", Mirror: "https://mirror.example.com"}, "mirror.example.com"},
		{config.RegistryConfig{Registry: "ghcr.io", Mirror: "http://localhost:5000/v2"}, "localhost:5000"},
	}

	for _, tc := range tests {
		if got := registryAuthHost(tc.registry); got != tc.want {
			t.Fatalf("registryAuthHost(%+v): expected %q, got %q", tc.registry, tc.want, got)
		}
	}
}

func TestTomlString(t *testing.T) {
	got := tomlString("a\"b\\c\nd\x01é")
	want := `"a\"b\\c\u000Ad\u0001é"`
	if got != want {
		t.Fatalf("expected %s, got %s", want, got)
	}
}

func TestK8sPrepareWithImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistries = requesterRegistries

	sys := system.NewMockSystem()
	sys.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)
	sys.MockCommandReturn("k8s status", []byte("Error: The node is not part of a Kubernetes cluster."), fmt.Errorf("command error"))

	ck8s := NewK8s(sys, cfg)
	if err := ck8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	expectedFiles := map[string]string{
		path.Join(sys.User().HomeDir, ".kube", "config"): "",
		"/etc/containerd/hosts.d/docker.io/hosts.toml":   mirrorHostsToml,
		k8sBootstrapConfigPath:                           k8sRequesterBootstrapConfig,
	}
	if !reflect.DeepEqual(expectedFiles, sys.CreatedFiles) {
		t.Fatalf("expected files: %v, got: %v", expectedFiles, sys.CreatedFiles)
	}

	expectedDirs := []string{
		"/etc/containerd/hosts.d/docker.io",
		path.Join(sys.User().HomeDir, ".kube"),
	}
	if !slices.Equal(expectedDirs, sys.CreatedDirectories) {
		t.Fatalf("expected directories: %v, got: %v", expectedDirs, sys.CreatedDirectories)
	}

	// No hosts.toml for an entry with only credentials.
	if _, ok := sys.CreatedFiles["/etc/containerd/hosts.d/ghcr.io/hosts.toml"]; ok {
		t.Fatal("expected no hosts.toml for ghcr.io")
	}

	bootstrap := "k8s bootstrap --file " + k8sBootstrapConfigPath
	if !slices.Contains(sys.ExecutedCommands, bootstrap) {
		t.Fatalf("expected %q to run, got: %v", bootstrap, sys.ExecutedCommands)
	}

	// The bootstrap config holds the credentials, so it is removed afterwards.
	if !slices.Equal([]string{"/run/containerd", k8sBootstrapConfigPath}, sys.RemovedPaths) {
		t.Fatalf("expected the bootstrap config to be removed, got: %v", sys.RemovedPaths)
	}
}

func TestK8sPrepareWithMirrorOnlyImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistries = requesterRegistries[:1]

	sys := system.NewMockSystem()
	sys.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)
	sys.MockCommandReturn("k8s status", []byte("Error: The node is not part of a Kubernetes cluster."), fmt.Errorf("command error"))

	ck8s := NewK8s(sys, cfg)
	if err := ck8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	// Without credentials, bootstrap is the same as before.
	if !slices.Contains(sys.ExecutedCommands, "k8s bootstrap") {
		t.Fatalf("expected a plain 'k8s bootstrap', got: %v", sys.ExecutedCommands)
	}
	if _, ok := sys.CreatedFiles[k8sBootstrapConfigPath]; ok {
		t.Fatal("expected no bootstrap config without credentials")
	}
	if sys.CreatedFiles["/etc/containerd/hosts.d/docker.io/hosts.toml"] != mirrorHostsToml {
		t.Fatalf("expected docker.io hosts.toml, got: %v", sys.CreatedFiles)
	}
}

func TestK8sPrepareWithImageRegistriesAlreadyBootstrapped(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistries = requesterRegistries

	sys := system.NewMockSystem()
	sys.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)
	logs := captureLogs(t)

	ck8s := NewK8s(sys, cfg)
	if err := ck8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	if !strings.Contains(logs.String(), "credentials are only applied at bootstrap") {
		t.Fatalf("expected a warning that credentials weren't applied, got logs:\n%s", logs)
	}

	// Re-running prepare on a bootstrapped cluster rewrites the same mirror
	// file and doesn't bootstrap again or write the bootstrap config.
	for _, cmd := range sys.ExecutedCommands {
		if strings.HasPrefix(cmd, "k8s bootstrap") {
			t.Fatalf("expected no bootstrap, got: %v", sys.ExecutedCommands)
		}
	}
	if _, ok := sys.CreatedFiles[k8sBootstrapConfigPath]; ok {
		t.Fatal("expected no bootstrap config for an existing cluster")
	}
	if sys.CreatedFiles["/etc/containerd/hosts.d/docker.io/hosts.toml"] != mirrorHostsToml {
		t.Fatalf("expected docker.io hosts.toml, got: %v", sys.CreatedFiles)
	}
}

func TestK8sPrepareWithInvalidImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistries = []config.RegistryConfig{{Registry: "ghcr.io", Username: "u"}}

	sys := system.NewMockSystem()
	sys.MockCommandReturn("which iptables", []byte("/usr/sbin/iptables"), nil)

	ck8s := NewK8s(sys, cfg)
	err := ck8s.Prepare()
	if err == nil || !strings.Contains(err.Error(), "both username and password") {
		t.Fatalf("expected a validation error, got: %v", err)
	}
	if len(sys.CreatedFiles) != 0 {
		t.Fatalf("expected no files written, got: %v", sys.CreatedFiles)
	}
}

func TestK8sRestoreWithImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistries = requesterRegistries

	for run := range 2 {
		sys := system.NewMockSystem()
		sys.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("0 unit files listed."), nil)

		ck8s := NewK8s(sys, cfg)
		if err := ck8s.Restore(); err != nil {
			t.Fatalf("run %d: %v", run, err)
		}

		expectedRemovedPaths := []string{
			path.Join(sys.User().HomeDir, ".kube"),
			"/etc/containerd/hosts.d/docker.io",
			k8sBootstrapConfigPath,
		}
		if !slices.Equal(expectedRemovedPaths, sys.RemovedPaths) {
			t.Fatalf("run %d: expected: %v, got: %v", run, expectedRemovedPaths, sys.RemovedPaths)
		}
	}
}

func TestK8sRestoreWithImageRegistryAndImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.K8s.Channel = defaultK8sChannel
	cfg.Providers.K8s.ImageRegistry.URL = "https://mirror.example.com"
	cfg.Providers.K8s.ImageRegistries = requesterRegistries[1:]

	sys := system.NewMockSystem()
	sys.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("0 unit files listed."), nil)

	ck8s := NewK8s(sys, cfg)
	if err := ck8s.Restore(); err != nil {
		t.Fatal(err)
	}

	expectedRemovedPaths := []string{
		path.Join(sys.User().HomeDir, ".kube"),
		"/etc/containerd/hosts.d/docker.io",
		k8sBootstrapConfigPath,
	}
	if !slices.Equal(expectedRemovedPaths, sys.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, sys.RemovedPaths)
	}
}

func TestMicroK8sPrepareWithImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = requesterRegistries

	sys := system.NewMockSystem()
	sys.MockFile(microk8sContainerdTemplate, []byte(microk8sBaseTemplate))

	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	expectedCommands := []string{
		"snap install microk8s --channel 1.31-strict/stable",
		"snap install kubectl --channel stable",
		"microk8s status --wait-ready --timeout 270",
		"systemctl restart snap.microk8s.daemon-containerd.service",
		"microk8s status --wait-ready --timeout 270",
		"usermod -a -G snap_microk8s test-user",
		"microk8s config",
	}
	if !slices.Equal(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected commands: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}

	expectedFiles := map[string]string{
		path.Join(sys.User().HomeDir, ".kube", "config"):               "",
		"/var/snap/microk8s/current/args/certs.d/docker.io/hosts.toml": mirrorHostsToml,
		microk8sContainerdTemplate:                                     microk8sRequesterTemplate,
	}
	if !reflect.DeepEqual(expectedFiles, sys.CreatedFiles) {
		t.Fatalf("expected files: %v, got: %v", expectedFiles, sys.CreatedFiles)
	}

	expectedDirs := []string{
		"/var/snap/microk8s/current/args/certs.d/docker.io",
		path.Join(sys.User().HomeDir, ".kube"),
	}
	if !slices.Equal(expectedDirs, sys.CreatedDirectories) {
		t.Fatalf("expected directories: %v, got: %v", expectedDirs, sys.CreatedDirectories)
	}
}

func TestMicroK8sPrepareWithImageRegistriesIsIdempotent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = requesterRegistries

	// A second prepare finds the template as the first one left it, so it
	// neither rewrites the template nor restarts containerd.
	sys := system.NewMockSystem()
	sys.MockFile(microk8sContainerdTemplate, []byte(microk8sRequesterTemplate))

	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	if _, ok := sys.CreatedFiles[microk8sContainerdTemplate]; ok {
		t.Fatalf("expected the template not to be rewritten, got:\n%s", sys.CreatedFiles[microk8sContainerdTemplate])
	}
	if slices.Contains(sys.ExecutedCommands, "systemctl restart snap.microk8s.daemon-containerd.service") {
		t.Fatalf("expected no containerd restart, got: %v", sys.ExecutedCommands)
	}
}

func TestMicroK8sPrepareWithChangedImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = []config.RegistryConfig{
		{Registry: "ghcr.io", Username: "ghcr-user", Password: "new-token"},
	}

	// Changed credentials replace the block rather than adding a second one.
	sys := system.NewMockSystem()
	sys.MockFile(microk8sContainerdTemplate, []byte(microk8sRequesterTemplate))

	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	want := strings.Replace(microk8sRequesterTemplate, "ghcr-token", "new-token", 1)
	if got := sys.CreatedFiles[microk8sContainerdTemplate]; got != want {
		t.Fatalf("expected template:\n%s\ngot:\n%s", want, got)
	}
	if !slices.Contains(sys.ExecutedCommands, "systemctl restart snap.microk8s.daemon-containerd.service") {
		t.Fatalf("expected a containerd restart, got: %v", sys.ExecutedCommands)
	}
}

func TestMicroK8sPrepareWithMirrorOnlyImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = requesterRegistries[:1]

	// No template is mocked: without credentials it isn't read.
	sys := system.NewMockSystem()
	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Prepare(); err != nil {
		t.Fatal(err)
	}

	if slices.Contains(sys.ExecutedCommands, "systemctl restart snap.microk8s.daemon-containerd.service") {
		t.Fatalf("expected no containerd restart for a mirror, got: %v", sys.ExecutedCommands)
	}
	if sys.CreatedFiles["/var/snap/microk8s/current/args/certs.d/docker.io/hosts.toml"] != mirrorHostsToml {
		t.Fatalf("expected docker.io hosts.toml, got: %v", sys.CreatedFiles)
	}
}

func TestMicroK8sPrepareWithBrokenTemplateMarkers(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = requesterRegistries[1:]

	sys := system.NewMockSystem()
	sys.MockFile(microk8sContainerdTemplate, []byte(microk8sBaseTemplate+microk8sRegistriesBegin+"\n"))

	uk8s := NewMicroK8s(sys, cfg)
	err := uk8s.Prepare()
	if err == nil || !strings.Contains(err.Error(), "no matching") {
		t.Fatalf("expected an error about the unmatched marker, got: %v", err)
	}
	if _, ok := sys.CreatedFiles[microk8sContainerdTemplate]; ok {
		t.Fatal("expected the template not to be rewritten")
	}
}

func TestMicroK8sRestoreWithImageRegistries(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = requesterRegistries

	sys := system.NewMockSystem()
	sys.MockFile(microk8sContainerdTemplate, []byte(microk8sRequesterTemplate))

	uk8s := NewMicroK8s(sys, cfg)
	if err := uk8s.Restore(); err != nil {
		t.Fatal(err)
	}

	// The template is back to exactly what MicroK8s shipped.
	if got := sys.CreatedFiles[microk8sContainerdTemplate]; got != microk8sBaseTemplate {
		t.Fatalf("expected template:\n%s\ngot:\n%s", microk8sBaseTemplate, got)
	}

	expectedRemovedPaths := []string{
		"/var/snap/microk8s/current/args/certs.d/docker.io",
		path.Join(sys.User().HomeDir, ".kube"),
	}
	if !slices.Equal(expectedRemovedPaths, sys.RemovedPaths) {
		t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, sys.RemovedPaths)
	}

	expectedCommands := []string{
		"snap remove microk8s --purge",
		"snap remove kubectl --purge",
	}
	if !slices.Equal(expectedCommands, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expectedCommands, sys.ExecutedCommands)
	}
}

func TestMicroK8sRestoreWithImageRegistriesIsIdempotent(t *testing.T) {
	cfg := &config.Config{}
	cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
	cfg.Providers.MicroK8s.ImageRegistries = requesterRegistries

	base := microk8sBaseTemplate
	for name, template := range map[string]*string{
		"block already removed": &base,
		"snap already removed":  nil,
	} {
		t.Run(name, func(t *testing.T) {
			sys := system.NewMockSystem()
			if template != nil {
				sys.MockFile(microk8sContainerdTemplate, []byte(*template))
			}

			uk8s := NewMicroK8s(sys, cfg)
			if err := uk8s.Restore(); err != nil {
				t.Fatal(err)
			}

			if _, ok := sys.CreatedFiles[microk8sContainerdTemplate]; ok {
				t.Fatal("expected the template not to be rewritten")
			}
		})
	}
}

func TestMicroK8sTemplateRoundTrip(t *testing.T) {
	auth := buildRegistryAuthToml(requesterRegistries)

	for _, base := range []string{microk8sBaseTemplate, microk8sBaseTemplate + "\n", ""} {
		added := appendMicroK8sRegistries(base, auth)
		if strings.Count(added, microk8sRegistriesBegin) != 1 {
			t.Fatalf("expected one block, got:\n%s", added)
		}

		stripped, err := stripMicroK8sRegistries(added)
		if err != nil {
			t.Fatal(err)
		}
		if stripped != base {
			t.Fatalf("expected strip to undo append:\n%q\ngot:\n%q", base, stripped)
		}

		// Stripping again is a no-op.
		again, err := stripMicroK8sRegistries(stripped)
		if err != nil {
			t.Fatal(err)
		}
		if again != stripped {
			t.Fatalf("expected a second strip to be a no-op, got:\n%q", again)
		}
	}
}

// Restore reads the cached config without validating it, so a registry name
// that isn't a hostname must not be joined into a path and removed.
func TestRestoreSkipsInvalidImageRegistriesNames(t *testing.T) {
	registries := []config.RegistryConfig{{Registry: "../../../../etc", Mirror: "https://mirror.example.com"}}

	t.Run("k8s", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Providers.K8s.Channel = defaultK8sChannel
		cfg.Providers.K8s.ImageRegistries = registries

		sys := system.NewMockSystem()
		sys.MockCommandReturn("systemctl list-unit-files containerd.service", []byte("0 unit files listed."), nil)

		if err := NewK8s(sys, cfg).Restore(); err != nil {
			t.Fatal(err)
		}

		expectedRemovedPaths := []string{path.Join(sys.User().HomeDir, ".kube")}
		if !slices.Equal(expectedRemovedPaths, sys.RemovedPaths) {
			t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, sys.RemovedPaths)
		}
	})

	t.Run("microk8s", func(t *testing.T) {
		cfg := &config.Config{}
		cfg.Providers.MicroK8s.Channel = "1.31-strict/stable"
		cfg.Providers.MicroK8s.ImageRegistries = registries

		sys := system.NewMockSystem()

		if err := NewMicroK8s(sys, cfg).Restore(); err != nil {
			t.Fatal(err)
		}

		expectedRemovedPaths := []string{path.Join(sys.User().HomeDir, ".kube")}
		if !slices.Equal(expectedRemovedPaths, sys.RemovedPaths) {
			t.Fatalf("expected: %v, got: %v", expectedRemovedPaths, sys.RemovedPaths)
		}
	})
}
