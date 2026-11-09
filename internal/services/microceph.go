package services

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"path"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/aws/aws-sdk-go-v2/aws"
	v4 "github.com/aws/aws-sdk-go-v2/aws/signer/v4"
	"github.com/canonical/concierge/internal/config"
	"github.com/canonical/concierge/internal/packages"
	"github.com/canonical/concierge/internal/providers"
	"github.com/canonical/concierge/internal/system"
	"gopkg.in/yaml.v3"
)

const (
	// defaultMicrocephChannel is used when services.microceph.channel is unset.
	defaultMicrocephChannel = "squid/stable"
	// defaultMicrocephDiskSize is used when services.microceph.disk-size is unset.
	defaultMicrocephDiskSize = "4G"
	// defaultMicrocephRGWAccessKey/defaultMicrocephRGWSecretKey are fixed,
	// insecure fallback RGW credentials used when rgw.access-key/secret-key
	// are left unset in config.
	defaultMicrocephRGWAccessKey = "concierge"
	defaultMicrocephRGWSecretKey = "concierge-insecure-default-secret"

	// microcephRGWUID is the fixed RGW/S3 user concierge creates and manages.
	microcephRGWUID = "concierge"

	// defaultMicrocephOSDCount is the number of loop-backed OSDs created on
	// bootstrap. Ceph's default pool replication factor is 3.
	defaultMicrocephOSDCount = 3

	// cephCSIStorageClassName is the name given to the additive, ceph-backed
	// storage class.
	cephCSIStorageClassName = "concierge-microceph-rbd"

	// manifestDir/manifestFile is where the generated storage-class manifest
	// is written before being applied/deleted via kubectl.
	manifestDir  = "/etc/concierge"
	manifestFile = "microceph-storageclass.yaml"
)

// NewMicroceph constructs a new Microceph service instance.
func NewMicroceph(r system.Worker, cfg *config.Config, providerList []providers.Provider) *Microceph {
	channel := cfg.Services.Microceph.Channel
	if channel == "" {
		channel = defaultMicrocephChannel
	}

	diskSize := cfg.Services.Microceph.DiskSize
	if diskSize == "" {
		diskSize = defaultMicrocephDiskSize
	}

	accessKey := cfg.Services.Microceph.RGW.AccessKey
	if accessKey == "" {
		accessKey = defaultMicrocephRGWAccessKey
	}

	secretKey := cfg.Services.Microceph.RGW.SecretKey
	if secretKey == "" {
		secretKey = defaultMicrocephRGWSecretKey
	}

	return &Microceph{
		Channel:            channel,
		DiskSize:           diskSize,
		RGWEnabled:         cfg.Services.Microceph.RGW.Port != 0,
		RGWPort:            cfg.Services.Microceph.RGW.Port,
		RGWAccessKey:       accessKey,
		RGWSecretKey:       secretKey,
		RGWDNSName:         cfg.Services.Microceph.RGW.DNSName,
		RGWBucket:          cfg.Services.Microceph.RGW.Bucket,
		StorageClassEnable: cfg.Services.Microceph.StorageClass.Enable,
		system:             r,
		providers:          providerList,
		snaps:              []*system.Snap{{Name: "microceph", Channel: channel}},
		httpClient:         http.DefaultClient,
	}
}

// httpDoer is the subset of *http.Client used to issue the signed RGW
// bucket-creation request. It allows tests to substitute a fake transport
// instead of making real network calls.
type httpDoer interface {
	Do(req *http.Request) (*http.Response, error)
}

// Microceph represents a MicroCeph install on a given machine, optionally
// exposing an RGW/S3 gateway, and/or integrating an additive storage class
// into whichever k8s/microk8s provider is active.
type Microceph struct {
	Channel            string
	DiskSize           string
	RGWEnabled         bool
	RGWPort            int
	RGWAccessKey       string
	RGWSecretKey       string
	RGWDNSName         string
	RGWBucket          string
	StorageClassEnable bool

	system     system.Worker
	providers  []providers.Provider
	snaps      []*system.Snap
	httpClient httpDoer
}

// Name reports the name of the service used internally by concierge.
func (m *Microceph) Name() string { return "microceph" }

// Prepare installs microceph, bootstraps a single-node cluster, adds
// loop-backed disks, and optionally enables the RGW/S3 gateway.
func (m *Microceph) Prepare() error {
	err := m.install()
	if err != nil {
		return fmt.Errorf("failed to install microceph: %w", err)
	}

	// cluster bootstrap/disk add are not idempotent (a second disk add
	// would add further OSDs), so only run them the first time.
	if !m.isBootstrapped() {
		err = m.bootstrapCluster()
		if err != nil {
			return fmt.Errorf("failed to bootstrap microceph cluster: %w", err)
		}

		err = m.addDisks()
		if err != nil {
			return fmt.Errorf("failed to add disks to microceph cluster: %w", err)
		}
	}

	if m.RGWEnabled {
		err = m.configureRGW()
		if err != nil {
			return fmt.Errorf("failed to configure microceph RGW: %w", err)
		}
	}

	slog.Info("Prepared service", "service", m.Name())

	return nil
}

// Integrate wires the microceph cluster into the active k8s provider, by
// additively applying a ceph-csi-style StorageClass/Secret/ConfigMap. No-op
// if storage-class.enable is false. Does not deploy the ceph-csi driver
// itself; a real environment is expected to already have the driver
// installed separately.
func (m *Microceph) Integrate() error {
	if !m.StorageClassEnable {
		return nil
	}

	provider := m.integratedProvider()
	if provider == nil {
		// Already rejected by validateServiceProviderDependencies; defensive.
		return fmt.Errorf("storage-class.enable is set but no active k8s provider was found")
	}

	remote := remoteName(provider.Name())

	info, err := m.exportClusterConfig(remote)
	if err != nil {
		return fmt.Errorf("failed to export microceph cluster connection info: %w", err)
	}

	manifest := buildStorageClassManifest(remote, info)

	manifestPath, err := m.writeManifest(manifest)
	if err != nil {
		return err
	}

	err = m.applyManifest(provider.Name(), manifestPath)
	if err != nil {
		return fmt.Errorf("failed to apply storage class manifest: %w", err)
	}

	slog.Info("Integrated service with provider", "service", m.Name(), "provider", provider.Name())

	return nil
}

// Disintegrate reverses Integrate(), removing the StorageClass/Secret/
// ConfigMap that were applied. Runs before providers are restored.
func (m *Microceph) Disintegrate() error {
	if !m.StorageClassEnable {
		return nil
	}

	provider := m.integratedProvider()
	if provider == nil {
		// Provider already gone/never active - nothing to clean up.
		return nil
	}

	manifestPath := path.Join(manifestDir, manifestFile)

	err := m.deleteManifest(provider.Name(), manifestPath)
	if err != nil {
		return fmt.Errorf("failed to remove storage class manifest: %w", err)
	}

	err = m.system.RemovePath(manifestPath)
	if err != nil {
		return fmt.Errorf("failed to remove '%s': %w", manifestPath, err)
	}

	slog.Info("Disintegrated service from provider", "service", m.Name(), "provider", provider.Name())

	return nil
}

// Restore uninstalls microceph. `snap remove --purge` clears all of the
// snap's state, including the bootstrapped cluster, disks, and RGW config.
func (m *Microceph) Restore() error {
	snapHandler := packages.NewSnapHandler(m.system, m.snaps)

	err := snapHandler.Restore()
	if err != nil {
		return err
	}

	slog.Info("Restored service", "service", m.Name())

	return nil
}

// install ensures that microceph is installed.
func (m *Microceph) install() error {
	snapHandler := packages.NewSnapHandler(m.system, m.snaps)
	return snapHandler.Prepare()
}

// isBootstrapped reports whether the microceph cluster has already been
// bootstrapped on this host.
func (m *Microceph) isBootstrapped() bool {
	cmd := system.NewCommand("microceph", []string{"status"})
	cmd.ReadOnly = true
	cmd.ExpectedError = `.*`
	_, err := m.system.Run(cmd)
	return err == nil
}

// bootstrapCluster bootstraps a new, single-node microceph cluster.
func (m *Microceph) bootstrapCluster() error {
	cmd := system.NewCommand("microceph", []string{"cluster", "bootstrap"})
	_, err := system.RunWithRetries(m.system, cmd, 5*time.Minute)
	return err
}

// addDisks adds defaultMicrocephOSDCount loop-backed disks of the
// configured size as OSDs.
func (m *Microceph) addDisks() error {
	cmd := system.NewCommand("microceph", []string{"disk", "add", fmt.Sprintf("loop,%s,%d", m.DiskSize, defaultMicrocephOSDCount)})
	_, err := m.system.Run(cmd)
	return err
}

// configureRGW enables the RGW/S3 gateway (if not already enabled), applies
// the configured DNS name (if any), and ensures the configured access/secret
// key pair and bucket exist for the concierge RGW user.
func (m *Microceph) configureRGW() error {
	if !m.isRGWEnabled() {
		cmd := system.NewCommand("microceph", []string{"enable", "rgw", "--port", strconv.Itoa(m.RGWPort)})
		_, err := system.RunWithRetries(m.system, cmd, 5*time.Minute)
		if err != nil {
			return fmt.Errorf("failed to enable rgw: %w", err)
		}
	}

	if m.RGWDNSName != "" {
		if err := m.ensureRGWDNSName(); err != nil {
			return err
		}
	}

	if err := m.ensureRGWUser(); err != nil {
		return err
	}

	if m.RGWBucket != "" {
		return m.ensureRGWBucket()
	}

	return nil
}

// ensureRGWDNSName sets rgw_dns_name (used for virtual-hosted-style bucket
// addressing) and restarts the RGW daemon to pick it up.
func (m *Microceph) ensureRGWDNSName() error {
	cmd := system.NewCommand("microceph.ceph", []string{"config", "get", "client", "rgw_dns_name"})
	cmd.ReadOnly = true
	output, err := m.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to get rgw_dns_name: %w", err)
	}
	if strings.TrimSpace(string(output)) == m.RGWDNSName {
		return nil
	}

	cmd = system.NewCommand("microceph.ceph", []string{"config", "set", "client", "rgw_dns_name", m.RGWDNSName})
	if _, err := m.system.Run(cmd); err != nil {
		return fmt.Errorf("failed to set rgw_dns_name: %w", err)
	}

	cmd = system.NewCommand("snap", []string{"restart", "microceph.rgw"})
	if _, err := m.system.Run(cmd); err != nil {
		return fmt.Errorf("failed to restart microceph.rgw: %w", err)
	}

	return nil
}

// isRGWEnabled reports whether the RGW service is already enabled on the
// cluster, by checking for its presence in `microceph status`'s per-node
// service listing.
func (m *Microceph) isRGWEnabled() bool {
	cmd := system.NewCommand("microceph", []string{"status"})
	cmd.ReadOnly = true
	output, err := m.system.Run(cmd)
	if err != nil {
		return false
	}
	return strings.Contains(string(output), "rgw")
}

// ensureRGWUser creates the concierge RGW/S3 user if it doesn't already
// exist, then (re-)applies the configured access/secret key pair - so that
// changing the keys in config and re-running concierge updates them.
func (m *Microceph) ensureRGWUser() error {
	exists, err := m.rgwUserExists()
	if err != nil {
		return err
	}

	if !exists {
		cmd := system.NewCommand("microceph.radosgw-admin", []string{
			"user", "create",
			"--uid=" + microcephRGWUID,
			"--display-name=concierge",
		})
		_, err := m.system.Run(cmd)
		if err != nil {
			return fmt.Errorf("failed to create rgw user: %w", err)
		}
	}

	cmd := system.NewCommand("microceph.radosgw-admin", []string{
		"key", "create",
		"--uid=" + microcephRGWUID,
		"--key-type=s3",
		"--access-key", m.RGWAccessKey,
		"--secret-key", m.RGWSecretKey,
	})
	_, err = m.system.Run(cmd)
	if err != nil {
		return fmt.Errorf("failed to set rgw access/secret key: %w", err)
	}

	return nil
}

// rgwUserExists reports whether the concierge RGW/S3 user already exists.
func (m *Microceph) rgwUserExists() (bool, error) {
	cmd := system.NewCommand("microceph.radosgw-admin", []string{"user", "list"})
	cmd.ReadOnly = true
	output, err := m.system.Run(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to list rgw users: %w", err)
	}
	return strings.Contains(string(output), microcephRGWUID), nil
}

// ensureRGWBucket creates the configured bucket if it doesn't already exist,
// signing the request with the concierge RGW access/secret key pair.
func (m *Microceph) ensureRGWBucket() error {
	exists, err := m.rgwBucketExists()
	if err != nil {
		return err
	}
	if exists {
		return nil
	}

	url := fmt.Sprintf("http://127.0.0.1:%d/%s", m.RGWPort, m.RGWBucket)
	req, err := http.NewRequest(http.MethodPut, url, nil)
	if err != nil {
		return fmt.Errorf("failed to build rgw bucket creation request: %w", err)
	}

	// Empty-body PUT requests are still signed with the hash of an empty
	// payload, as required by the SigV4 spec. The X-Amz-Content-Sha256
	// header must be set before signing, since SigV4 only includes
	// headers present on the request in SignedHeaders — RGW rejects
	// requests signed without it.
	emptyPayloadHash := sha256.Sum256(nil)
	payloadHash := hex.EncodeToString(emptyPayloadHash[:])
	req.Header.Set("X-Amz-Content-Sha256", payloadHash)

	creds := aws.Credentials{AccessKeyID: m.RGWAccessKey, SecretAccessKey: m.RGWSecretKey}
	signer := v4.NewSigner()
	if err := signer.SignHTTP(context.Background(), creds, req, payloadHash, "s3", "us-east-1", time.Now()); err != nil {
		return fmt.Errorf("failed to sign rgw bucket creation request: %w", err)
	}

	resp, err := m.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("failed to create rgw bucket %q: %w", m.RGWBucket, err)
	}
	defer func() { _ = resp.Body.Close() }() // Read-only body; close error is not actionable
	if resp.StatusCode >= 300 {
		return fmt.Errorf("failed to create rgw bucket %q: unexpected status %s", m.RGWBucket, resp.Status)
	}

	return nil
}

// rgwBucketExists reports whether the configured bucket already exists.
func (m *Microceph) rgwBucketExists() (bool, error) {
	cmd := system.NewCommand("microceph.radosgw-admin", []string{"bucket", "list"})
	cmd.ReadOnly = true
	output, err := m.system.Run(cmd)
	if err != nil {
		return false, fmt.Errorf("failed to list rgw buckets: %w", err)
	}
	return strings.Contains(string(output), fmt.Sprintf("%q", m.RGWBucket)), nil
}

// integratedProvider returns the active k8s provider, or nil if it isn't
// active.
func (m *Microceph) integratedProvider() providers.Provider {
	for _, p := range m.providers {
		if p.Name() == "k8s" {
			return p
		}
	}
	return nil
}

// remoteName derives the microceph "remote" name used both for
// `cluster export` and as the ceph-csi userID.
func remoteName(providerName string) string {
	return fmt.Sprintf("concierge%s", providerName)
}

// cephClusterInfo holds the connection details needed to integrate an
// external consumer with a microceph cluster, as returned by
// `microceph cluster export <remote> --json`.
type cephClusterInfo struct {
	FSID     string
	Monitors []string
	AdminKey string
}

// exportClusterConfig fetches and parses microceph's exported connection
// info for the given remote name.
func (m *Microceph) exportClusterConfig(remote string) (*cephClusterInfo, error) {
	cmd := system.NewCommand("microceph", []string{"cluster", "export", remote, "--json"})
	output, err := m.system.Run(cmd)
	if err != nil {
		return nil, err
	}

	var raw map[string]string
	err = json.Unmarshal(output, &raw)
	if err != nil {
		return nil, fmt.Errorf("failed to parse 'microceph cluster export' output: %w", err)
	}

	info := &cephClusterInfo{FSID: raw["fsid"]}

	for key, value := range raw {
		switch {
		case strings.HasPrefix(key, "mon.host."):
			info.Monitors = append(info.Monitors, value)
		case strings.HasPrefix(key, "keyring.client."):
			info.AdminKey = value
		}
	}
	// Sort for deterministic manifest output across runs.
	sort.Strings(info.Monitors)

	if info.FSID == "" || len(info.Monitors) == 0 || info.AdminKey == "" {
		return nil, fmt.Errorf("incomplete 'microceph cluster export' output for remote '%s'", remote)
	}

	return info, nil
}

// objectMeta mirrors the subset of Kubernetes' metav1.ObjectMeta fields
// this manifest needs; Namespace is empty for cluster-scoped kinds.
type objectMeta struct {
	Name      string `yaml:"name"`
	Namespace string `yaml:"namespace,omitempty"`
}

type configMapManifest struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   objectMeta        `yaml:"metadata"`
	Data       map[string]string `yaml:"data"`
}

type secretManifest struct {
	APIVersion string            `yaml:"apiVersion"`
	Kind       string            `yaml:"kind"`
	Metadata   objectMeta        `yaml:"metadata"`
	StringData map[string]string `yaml:"stringData"`
}

type storageClassManifest struct {
	APIVersion    string            `yaml:"apiVersion"`
	Kind          string            `yaml:"kind"`
	Metadata      objectMeta        `yaml:"metadata"`
	Provisioner   string            `yaml:"provisioner"`
	Parameters    map[string]string `yaml:"parameters"`
	ReclaimPolicy string            `yaml:"reclaimPolicy"`
}

// cephCSIConfig is the JSON value expected by ceph-csi's "config.json" key.
type cephCSIConfig struct {
	ClusterID string   `json:"clusterID"`
	Monitors  []string `json:"monitors"`
}

// buildStorageClassManifest renders the ceph-csi ConfigMap/Secret/
// StorageClass manifest for the given remote name and cluster connection
// info.
func buildStorageClassManifest(remote string, info *cephClusterInfo) string {
	secretName := cephCSIStorageClassName + "-admin-secret"

	csiConfig, err := json.Marshal([]cephCSIConfig{{ClusterID: info.FSID, Monitors: info.Monitors}})
	if err != nil {
		// Only fails on unsupported types, which []cephCSIConfig isn't.
		panic(err)
	}

	configMap := configMapManifest{
		APIVersion: "v1",
		Kind:       "ConfigMap",
		Metadata:   objectMeta{Name: "ceph-csi-config", Namespace: "default"},
		Data:       map[string]string{"config.json": string(csiConfig)},
	}

	secret := secretManifest{
		APIVersion: "v1",
		Kind:       "Secret",
		Metadata:   objectMeta{Name: secretName, Namespace: "default"},
		StringData: map[string]string{"userID": remote, "userKey": info.AdminKey},
	}

	storageClass := storageClassManifest{
		APIVersion:  "storage.k8s.io/v1",
		Kind:        "StorageClass",
		Metadata:    objectMeta{Name: cephCSIStorageClassName},
		Provisioner: "rbd.csi.ceph.com",
		Parameters: map[string]string{
			"clusterID": info.FSID,
			"pool":      "concierge",
			"csi.storage.k8s.io/provisioner-secret-name":      secretName,
			"csi.storage.k8s.io/provisioner-secret-namespace": "default",
			"csi.storage.k8s.io/node-stage-secret-name":       secretName,
			"csi.storage.k8s.io/node-stage-secret-namespace":  "default",
		},
		ReclaimPolicy: "Delete",
	}

	var docs []string
	for _, doc := range []any{configMap, secret, storageClass} {
		b, err := yaml.Marshal(doc)
		if err != nil {
			// Only fails on unsupported types, which the manifest structs aren't.
			panic(err)
		}
		docs = append(docs, string(b))
	}

	return strings.Join(docs, "---\n")
}

// writeManifest writes the given manifest content to the known manifest
// path, creating its parent directory as needed.
func (m *Microceph) writeManifest(manifest string) (string, error) {
	err := m.system.MkdirAll(manifestDir, 0755)
	if err != nil {
		return "", fmt.Errorf("failed to create '%s': %w", manifestDir, err)
	}

	manifestPath := path.Join(manifestDir, manifestFile)

	err = m.system.WriteFile(manifestPath, []byte(manifest), 0600)
	if err != nil {
		return "", fmt.Errorf("failed to write '%s': %w", manifestPath, err)
	}

	return manifestPath, nil
}

// applyManifest applies the manifest at the given path using the active
// provider's own vendored kubectl.
func (m *Microceph) applyManifest(providerName string, manifestPath string) error {
	cmd := system.NewCommand(providerName, []string{"kubectl", "apply", "-f", manifestPath})
	_, err := m.system.Run(cmd)
	return err
}

// deleteManifest removes the previously applied manifest resources.
func (m *Microceph) deleteManifest(providerName string, manifestPath string) error {
	cmd := system.NewCommand(providerName, []string{"kubectl", "delete", "-f", manifestPath, "--ignore-not-found"})
	_, err := m.system.Run(cmd)
	return err
}
