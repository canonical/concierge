package services

import (
	"errors"
	"fmt"
	"io"
	"net/http"
	"reflect"
	"strings"
	"testing"

	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/providers"
	"github.com/canonical/concierge/internal/system"
)

// fakeHTTPClient is a test double for httpDoer that records every request
// it receives and returns a fixed status code without any real network I/O.
type fakeHTTPClient struct {
	statusCode int
	err        error
	requests   []*http.Request
}

func (f *fakeHTTPClient) Do(req *http.Request) (*http.Response, error) {
	f.requests = append(f.requests, req)
	if f.err != nil {
		return nil, f.err
	}
	return &http.Response{
		StatusCode: f.statusCode,
		Body:       io.NopCloser(strings.NewReader("")),
	}, nil
}

func TestNewMicroceph(t *testing.T) {
	type test struct {
		config   *config.Config
		expected *Microceph
	}

	noOverrides := &config.Config{}

	withConfig := &config.Config{}
	withConfig.Services.Microceph.Channel = "latest/edge"
	withConfig.Services.Microceph.DiskSize = "8G"
	withConfig.Services.Microceph.RGW.Port = 8080
	withConfig.Services.Microceph.RGW.AccessKey = "my-access-key"
	withConfig.Services.Microceph.RGW.SecretKey = "my-secret-key"
	withConfig.Services.Microceph.StorageClass.Enable = true

	sys := system.NewMockSystem()

	tests := []test{
		{
			config: noOverrides,
			expected: &Microceph{
				Channel:      defaultMicrocephChannel,
				DiskSize:     defaultMicrocephDiskSize,
				RGWEnabled:   false,
				RGWAccessKey: defaultMicrocephRGWAccessKey,
				RGWSecretKey: defaultMicrocephRGWSecretKey,
				system:       sys,
			},
		},
		{
			config: withConfig,
			expected: &Microceph{
				Channel:            "latest/edge",
				DiskSize:           "8G",
				RGWEnabled:         true,
				RGWPort:            8080,
				RGWAccessKey:       "my-access-key",
				RGWSecretKey:       "my-secret-key",
				StorageClassEnable: true,
				system:             sys,
			},
		},
	}

	for _, tc := range tests {
		microceph := NewMicroceph(sys, tc.config, nil)

		// Check the constructed snap channel is correct, then remove the
		// snaps field so the rest of the object can be compared.
		if microceph.snaps[0].Channel != tc.expected.Channel {
			t.Fatalf("expected: %v, got: %v", tc.expected.Channel, microceph.snaps[0].Channel)
		}
		microceph.snaps = nil

		// httpClient defaults to http.DefaultClient and isn't part of the
		// config-driven comparison below.
		if microceph.httpClient == nil {
			t.Fatalf("expected httpClient to default to a non-nil client")
		}
		microceph.httpClient = nil

		if !reflect.DeepEqual(tc.expected, microceph) {
			t.Fatalf("expected: %v, got: %v", tc.expected, microceph)
		}
	}
}

func TestMicrocephName(t *testing.T) {
	microceph := NewMicroceph(system.NewMockSystem(), &config.Config{}, nil)
	if microceph.Name() != "microceph" {
		t.Fatalf("expected: microceph, got: %v", microceph.Name())
	}
}

func TestMicrocephPrepareCommandsFirstRun(t *testing.T) {
	cfg := &config.Config{}

	sys := system.NewMockSystem()
	// Simulate a not-yet-bootstrapped cluster.
	sys.MockCommandReturn("microceph status", []byte{}, errors.New("not bootstrapped"))

	microceph := NewMicroceph(sys, cfg, nil)
	if err := microceph.Prepare(); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"snap install microceph --channel squid/stable",
		"microceph status",
		"microceph cluster bootstrap",
		fmt.Sprintf("microceph disk add loop,%s,%d", defaultMicrocephDiskSize, defaultMicrocephOSDCount),
	}

	if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}
}

func TestMicrocephPrepareCommandsAlreadyBootstrapped(t *testing.T) {
	cfg := &config.Config{}

	sys := system.NewMockSystem()
	// Default mock Run() returns a nil error, so "microceph status" will
	// report the cluster as already bootstrapped.

	microceph := NewMicroceph(sys, cfg, nil)
	if err := microceph.Prepare(); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"snap install microceph --channel squid/stable",
		"microceph status",
	}

	if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}
}

func TestMicrocephPrepareCommandsWithRGW(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.RGW.Port = 8080

	sys := system.NewMockSystem()
	// Already bootstrapped, RGW not yet enabled, RGW user doesn't exist yet.
	sys.MockCommandReturn("microceph.radosgw-admin user list", []byte{}, nil)

	microceph := NewMicroceph(sys, cfg, nil)
	if err := microceph.Prepare(); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"snap install microceph --channel squid/stable",
		"microceph status",
		"microceph status",
		"microceph enable rgw --port 8080",
		"microceph.radosgw-admin user list",
		"microceph.radosgw-admin user create --uid=concierge --display-name=concierge",
		fmt.Sprintf("microceph.radosgw-admin key create --uid=concierge --key-type=s3 --access-key %s --secret-key %s",
			defaultMicrocephRGWAccessKey, defaultMicrocephRGWSecretKey),
	}

	if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}
}

func TestMicrocephPrepareCommandsWithRGWDNSNameAndBucket(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.RGW.Port = 8080
	cfg.Services.Microceph.RGW.DNSName = "s3.example.test"
	cfg.Services.Microceph.RGW.Bucket = "tests"

	sys := system.NewMockSystem()
	// Already bootstrapped, RGW already enabled, dns-name not yet set,
	// RGW user and bucket don't exist yet.
	sys.MockCommandReturn("microceph status", []byte("...rgw..."), nil)
	sys.MockCommandReturn("microceph.ceph config get client rgw_dns_name", []byte(""), nil)
	sys.MockCommandReturn("microceph.radosgw-admin user list", []byte{}, nil)
	sys.MockCommandReturn("microceph.radosgw-admin bucket list", []byte("[]"), nil)

	httpClient := &fakeHTTPClient{statusCode: http.StatusOK}
	microceph := NewMicroceph(sys, cfg, nil)
	microceph.httpClient = httpClient
	if err := microceph.Prepare(); err != nil {
		t.Fatal(err)
	}

	expected := []string{
		"snap install microceph --channel squid/stable",
		"microceph status",
		"microceph status",
		"microceph.ceph config get client rgw_dns_name",
		"microceph.ceph config set client rgw_dns_name s3.example.test",
		"snap restart microceph.rgw",
		"microceph.radosgw-admin user list",
		"microceph.radosgw-admin user create --uid=concierge --display-name=concierge",
		fmt.Sprintf("microceph.radosgw-admin key create --uid=concierge --key-type=s3 --access-key %s --secret-key %s",
			defaultMicrocephRGWAccessKey, defaultMicrocephRGWSecretKey),
		"microceph.radosgw-admin bucket list",
	}

	if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}

	if len(httpClient.requests) != 1 {
		t.Fatalf("expected exactly one bucket-creation request, got: %d", len(httpClient.requests))
	}
	req := httpClient.requests[0]
	if req.Method != http.MethodPut {
		t.Fatalf("expected PUT request, got: %s", req.Method)
	}
	if req.URL.String() != "http://127.0.0.1:8080/tests" {
		t.Fatalf("expected request to the configured bucket URL, got: %s", req.URL.String())
	}
	if req.Header.Get("Authorization") == "" {
		t.Fatalf("expected request to carry a SigV4 Authorization header")
	}
	// RGW rejects SigV4 requests that don't sign x-amz-content-sha256, so
	// it must be both set and included in SignedHeaders.
	if req.Header.Get("X-Amz-Content-Sha256") == "" {
		t.Fatalf("expected request to carry an X-Amz-Content-Sha256 header")
	}
	if !strings.Contains(req.Header.Get("Authorization"), "x-amz-content-sha256") {
		t.Fatalf("expected X-Amz-Content-Sha256 to be part of SignedHeaders, got: %s", req.Header.Get("Authorization"))
	}
}

func TestMicrocephRGWDNSNameAlreadySet(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.RGW.Port = 8080
	cfg.Services.Microceph.RGW.DNSName = "s3.example.test"

	sys := system.NewMockSystem()
	sys.MockCommandReturn("microceph status", []byte("...rgw..."), nil)
	sys.MockCommandReturn("microceph.ceph config get client rgw_dns_name", []byte("s3.example.test\n"), nil)
	sys.MockCommandReturn("microceph.radosgw-admin user list", []byte("concierge"), nil)

	microceph := NewMicroceph(sys, cfg, nil)
	if err := microceph.Prepare(); err != nil {
		t.Fatal(err)
	}

	for _, cmd := range sys.ExecutedCommands {
		if strings.Contains(cmd, "snap restart microceph.rgw") {
			t.Fatalf("expected no RGW restart when dns-name is already set, got: %v", sys.ExecutedCommands)
		}
	}
}

func TestMicrocephRGWBucketAlreadyExists(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.RGW.Port = 8080
	cfg.Services.Microceph.RGW.Bucket = "tests"

	sys := system.NewMockSystem()
	sys.MockCommandReturn("microceph status", []byte("...rgw..."), nil)
	sys.MockCommandReturn("microceph.radosgw-admin user list", []byte("concierge"), nil)
	sys.MockCommandReturn("microceph.radosgw-admin bucket list", []byte(`["tests"]`), nil)

	httpClient := &fakeHTTPClient{statusCode: http.StatusOK}
	microceph := NewMicroceph(sys, cfg, nil)
	microceph.httpClient = httpClient
	if err := microceph.Prepare(); err != nil {
		t.Fatal(err)
	}

	if len(httpClient.requests) != 0 {
		t.Fatalf("expected no bucket creation request when bucket already exists, got: %d", len(httpClient.requests))
	}
}

func TestMicrocephRestore(t *testing.T) {
	cfg := &config.Config{}

	sys := system.NewMockSystem()
	microceph := NewMicroceph(sys, cfg, nil)
	if err := microceph.Restore(); err != nil {
		t.Fatal(err)
	}

	expected := []string{"snap remove microceph --purge"}

	if !reflect.DeepEqual(expected, sys.ExecutedCommands) {
		t.Fatalf("expected: %v, got: %v", expected, sys.ExecutedCommands)
	}
}

func TestMicrocephIntegrateDisabled(t *testing.T) {
	sys := system.NewMockSystem()
	microceph := NewMicroceph(sys, &config.Config{}, nil)

	if err := microceph.Integrate(); err != nil {
		t.Fatal(err)
	}

	if len(sys.ExecutedCommands) != 0 {
		t.Fatalf("expected no commands to be run, got: %v", sys.ExecutedCommands)
	}
}

func TestMicrocephIntegrateNoProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.StorageClass.Enable = true

	sys := system.NewMockSystem()
	microceph := NewMicroceph(sys, cfg, nil)

	if err := microceph.Integrate(); err == nil {
		t.Fatal("expected an error when no k8s provider is configured")
	}
}

func TestMicrocephDisintegrateNoProvider(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.StorageClass.Enable = true

	sys := system.NewMockSystem()
	microceph := NewMicroceph(sys, cfg, nil)

	// No provider configured - should be a no-op, not an error.
	if err := microceph.Disintegrate(); err != nil {
		t.Fatal(err)
	}

	if len(sys.ExecutedCommands) != 0 {
		t.Fatalf("expected no commands to be run, got: %v", sys.ExecutedCommands)
	}
}

func TestMicrocephIntegrateDisintegrate(t *testing.T) {
	cfg := &config.Config{}
	cfg.Services.Microceph.StorageClass.Enable = true

	sys := system.NewMockSystem()
	sys.MockCommandReturn(
		"microceph cluster export conciergek8s --json",
		[]byte(`{"fsid":"abc-123","mon.host.node1":"10.0.0.1","keyring.client.conciergek8s":"AQsecretkey=="}`),
		nil,
	)

	k8s := providers.NewK8s(sys, cfg)
	microceph := NewMicroceph(sys, cfg, []providers.Provider{k8s})

	if err := microceph.Integrate(); err != nil {
		t.Fatal(err)
	}

	manifestPath := "/etc/concierge/microceph-storageclass.yaml"
	if _, ok := sys.CreatedFiles[manifestPath]; !ok {
		t.Fatalf("expected manifest to be written to '%s'", manifestPath)
	}

	manifest := sys.CreatedFiles[manifestPath]
	for _, substr := range []string{"abc-123", "10.0.0.1", "AQsecretkey==", "concierge-microceph-rbd"} {
		if !strings.Contains(manifest, substr) {
			t.Fatalf("expected manifest to contain '%s', got: %s", substr, manifest)
		}
	}

	expectedLastCommand := fmt.Sprintf("k8s kubectl apply -f %s", manifestPath)
	if sys.ExecutedCommands[len(sys.ExecutedCommands)-1] != expectedLastCommand {
		t.Fatalf("expected last command: %v, got: %v", expectedLastCommand, sys.ExecutedCommands[len(sys.ExecutedCommands)-1])
	}

	if err := microceph.Disintegrate(); err != nil {
		t.Fatal(err)
	}

	expectedDeleteCommand := fmt.Sprintf("k8s kubectl delete -f %s --ignore-not-found", manifestPath)
	if sys.ExecutedCommands[len(sys.ExecutedCommands)-1] != expectedDeleteCommand {
		t.Fatalf("expected last command: %v, got: %v", expectedDeleteCommand, sys.ExecutedCommands[len(sys.ExecutedCommands)-1])
	}

	found := false
	for _, p := range sys.RemovedPaths {
		if p == manifestPath {
			found = true
		}
	}
	if !found {
		t.Fatalf("expected manifest path '%s' to be removed, got: %v", manifestPath, sys.RemovedPaths)
	}
}

func TestExportClusterConfigIncomplete(t *testing.T) {
	sys := system.NewMockSystem()
	sys.MockCommandReturn("microceph cluster export conciergek8s --json", []byte(`{"fsid":"abc-123"}`), nil)

	microceph := NewMicroceph(sys, &config.Config{}, nil)
	if _, err := microceph.exportClusterConfig("conciergek8s"); err == nil {
		t.Fatal("expected an error for incomplete export output")
	}
}

func TestBuildStorageClassManifest(t *testing.T) {
	info := &cephClusterInfo{
		FSID:     "abc-123",
		Monitors: []string{"10.0.0.1", "10.0.0.2"},
		AdminKey: "AQsecretkey==",
	}

	manifest := buildStorageClassManifest("conciergek8s", info)

	for _, substr := range []string{
		"ConfigMap", "Secret", "StorageClass",
		"abc-123", "10.0.0.1", "10.0.0.2", "AQsecretkey==",
		"concierge-microceph-rbd", "rbd.csi.ceph.com",
	} {
		if !strings.Contains(manifest, substr) {
			t.Fatalf("expected manifest to contain '%s', got: %s", substr, manifest)
		}
	}
}
