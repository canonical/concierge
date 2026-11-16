package providers

import (
	"fmt"
	"net/url"
	"regexp"
	"strings"

	"github.com/canonical/concierge/internal/config"
)

// registryHostPattern matches a registry hostname with an optional port, such
// as "ghcr.io" or "registry.example.com:5000". The name is used as a directory
// name under containerd's hosts directory, so anything else is rejected.
var registryHostPattern = regexp.MustCompile(`^[A-Za-z0-9]([A-Za-z0-9.-]*[A-Za-z0-9])?(:[0-9]+)?$`)

// validateRegistries checks the image-registries entries for a provider.
// legacy is the provider's image-registry config, which configures docker.io
// and so can't be combined with a docker.io entry.
func validateRegistries(registries []config.RegistryConfig, legacy config.ImageRegistryConfig) error {
	seen := map[string]bool{}
	if legacy.URL != "" {
		seen["docker.io"] = true
	}
	authHosts := map[string]string{}
	for _, r := range registries {
		if !registryHostPattern.MatchString(r.Registry) {
			return fmt.Errorf("image-registries entry has an invalid registry %q: expected a hostname such as ghcr.io", r.Registry)
		}
		if seen[r.Registry] {
			return fmt.Errorf("image-registries has more than one entry for registry %q (image-registry, if set, configures docker.io)", r.Registry)
		}
		seen[r.Registry] = true

		if (r.Username == "") != (r.Password == "") {
			return fmt.Errorf("image-registries entry for %q must set both username and password, or neither", r.Registry)
		}
		if r.Mirror == "" && r.Username == "" {
			return fmt.Errorf("image-registries entry for %q must set a mirror, credentials, or both", r.Registry)
		}
		if r.Mirror != "" {
			u, err := url.Parse(r.Mirror)
			if err != nil || u.Scheme == "" || u.Host == "" {
				return fmt.Errorf("image-registries entry for %q has an invalid mirror %q: expected a URL such as https://mirror.example.com", r.Registry, r.Mirror)
			}
		}
		if r.Username != "" {
			// containerd keys credentials by host, so two entries can't
			// set credentials for the same one.
			host := registryAuthHost(r)
			if other, ok := authHosts[host]; ok {
				return fmt.Errorf("image-registries entries for %q and %q both set credentials for %q", other, r.Registry, host)
			}
			authHosts[host] = r.Registry
		}
	}
	return nil
}

// hasRegistryAuth reports whether any image-registries entry has credentials.
func hasRegistryAuth(registries []config.RegistryConfig) bool {
	for _, r := range registries {
		if r.Username != "" {
			return true
		}
	}
	return false
}

// registryAuthHost returns the host that containerd looks up
// `registry.configs."<host>".auth` by for this entry. containerd asks for
// credentials using the host of the endpoint it is talking to, so that is the
// mirror's host when a mirror is set. For docker.io with no mirror, containerd
// talks to registry-1.docker.io, so that is the key it uses.
func registryAuthHost(r config.RegistryConfig) string {
	if r.Mirror != "" {
		if u, err := url.Parse(r.Mirror); err == nil && u.Host != "" {
			return u.Host
		}
	}
	if r.Registry == "docker.io" {
		return "registry-1.docker.io"
	}
	return r.Registry
}

// buildMirrorHostsToml generates a containerd hosts.toml that pulls through the
// given mirror. There is no `server` line, so containerd falls back to the
// registry itself if the mirror can't serve an image.
func buildMirrorHostsToml(mirror string) string {
	return fmt.Sprintf("[host.%s]\ncapabilities = [\"pull\", \"resolve\"]\n", tomlString(mirror))
}

// buildRegistryAuthToml generates containerd CRI `registry.configs` auth tables
// for every entry that has credentials, in config order.
func buildRegistryAuthToml(registries []config.RegistryConfig) string {
	var sb strings.Builder
	for _, r := range registries {
		if r.Username == "" {
			continue
		}
		fmt.Fprintf(&sb, "[plugins.\"io.containerd.grpc.v1.cri\".registry.configs.%s.auth]\n", tomlString(registryAuthHost(r)))
		fmt.Fprintf(&sb, "  username = %s\n", tomlString(r.Username))
		fmt.Fprintf(&sb, "  password = %s\n", tomlString(r.Password))
	}
	return sb.String()
}

// registryAuthConfigs returns the `registry.configs` map for every entry that
// has credentials, keyed by the host containerd looks the credentials up by.
func registryAuthConfigs(registries []config.RegistryConfig) map[string]any {
	configs := map[string]any{}
	for _, r := range registries {
		if r.Username == "" {
			continue
		}
		configs[registryAuthHost(r)] = map[string]any{
			"auth": map[string]any{
				"username": r.Username,
				"password": r.Password,
			},
		}
	}
	return configs
}

// tomlString quotes s as a TOML basic string.
func tomlString(s string) string {
	var sb strings.Builder
	sb.WriteByte('"')
	for _, c := range s {
		switch {
		case c == '"':
			sb.WriteString(`\"`)
		case c == '\\':
			sb.WriteString(`\\`)
		case c < 0x20 || c == 0x7f:
			fmt.Fprintf(&sb, `\u%04X`, c)
		default:
			sb.WriteRune(c)
		}
	}
	sb.WriteByte('"')
	return sb.String()
}
