---
myst:
  html_meta:
    description: The complete YAML schema for Concierge configuration files, covering Juju, providers (LXD, Canonical Kubernetes, MicroK8s, Google), and host packages and snaps.
---

(reference-configuration)=
# Configuration schema

Concierge reads a YAML configuration file. The schema below describes every field. All top-level blocks are optional; a minimal configuration might set just one provider.

For task-oriented guidance, see {ref}`how-to-write-a-custom-config`.

```yaml
# (Optional) Target Juju configuration.
juju:
  # (Optional) Disable installation of Juju (and therefore all bootstrapping).
  disable: true | false
  # (Optional) Channel from which to install Juju.
  channel: <channel>
  # (Optional) Snap revision from which to install Juju, for example "31429".
  # If `channel` is also set, it only tracks updates after install.
  revision: <revision>
  # (Optional) Juju agent version to bootstrap with, for example "3.6.11".
  agent-version: <version>
  # (Optional) A map of model-defaults to set when bootstrapping *all* Juju
  # controllers.
  model-defaults:
    <model-default>: <value>
  # (Optional) A map of bootstrap-constraints to set when bootstrapping *all*
  # Juju controllers.
  bootstrap-constraints:
    <bootstrap-constraint>: <value>
  # (Optional) Extra arguments to append to the `juju bootstrap` command.
  # Parsed using shell-style splitting rules.
  extra-bootstrap-args: <args>

# (Required) Providers to install and bootstrap.
providers:
  microk8s:
    enable: true | false
    bootstrap: true | false
    # (Optional) Name of the Juju controller. Defaults to concierge-<provider>.
    controller-name: <name>
    # (Optional) Channel for the `microk8s` snap. If omitted, a fresh install
    # uses the latest strict stable channel, and an existing install stays on
    # the channel it is tracking.
    channel: <channel>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>
    # (Optional) MicroK8s addons to enable.
    addons:
      - <addon>[:<params>]
    # (Optional) Image registry mirror for Docker Hub. Values support ${VAR}
    # interpolation.
    image-registry:
      url: <url>
      username: <username>
      password: <password>
    # (Optional) Mirrors and pull credentials for one or more registries.
    # See "Image registries" below. Values support ${VAR} interpolation.
    image-registries:
      - registry: <hostname>
        mirror: <url>
        username: <username>
        password: <password>

  k8s:
    enable: true | false
    bootstrap: true | false
    controller-name: <name>
    # (Optional) Channel for the `k8s` snap. If omitted, a fresh install uses
    # `1.32-classic/stable` (the Canonical Kubernetes LTS), and an existing
    # install stays on the channel it is tracking.
    channel: <channel>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>
    # (Optional) Canonical Kubernetes features to configure.
    features:
      <feature>:
        <key>: <value>
    image-registry:
      url: <url>
      username: <username>
      password: <password>
    image-registries:
      - registry: <hostname>
        mirror: <url>
        username: <username>
        password: <password>

  lxd:
    enable: true | false
    bootstrap: true | false
    controller-name: <name>
    channel: <channel>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>

  google:
    enable: true | false
    bootstrap: true | false
    controller-name: <name>
    # See "Provide cloud credentials" for the expected file format.
    credentials-file: <path>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>

# (Optional) Additional host configuration.
host:
  # (Optional) Third-party APT repositories to add before installing packages,
  # keyed by name. Each is removed again on restore.
  repositories:
    # A Launchpad PPA, written to /etc/apt/sources.list.d/concierge-<name>.sources
    # with the key Launchpad publishes for it.
    <name>:
      # Can't be combined with url, key, suite or components.
      ppa: <user>/<ppa-name>
    # A repository written to /etc/apt/sources.list.d/concierge-<name>.sources.
    <name>:
      # Base URL of the repository.
      url: <url>
      # HTTPS URL of the ASCII-armored signing key.
      key: <key-url>
      # (Optional) Suite; if omitted, the host's release codename is used.
      suite: <suite>
      components:
        - <component>
  # (Optional) APT packages to install.
  packages:
    - <package>
  # (Optional) Snap packages to install, keyed by name.
  snaps:
    <snap>:
      # (Optional) Channel; if omitted, snapd's default is used.
      channel: <channel>
      # (Optional) Snap connections to form.
      connections:
        - <snap>:<plug-interface>
        - <snap>:<plug-interface> <snap>:<plug-interface>
```

(reference-configuration-image-registries)=
## Image registries

The `k8s` and `microk8s` providers accept an `image-registries` list. Each entry configures how containerd pulls images from one registry.

| Key | Required | Description |
| :-- | :------- | :---------- |
| `registry` | Yes | The host name that image references use for the registry, such as `docker.io` or `ghcr.io`. A port is allowed, such as `registry.example.com:5000`. |
| `mirror` | No | A URL to pull the registry's images through. If the mirror can't serve an image, containerd pulls it from the registry instead. |
| `username`, `password` | No | Pull credentials. They're used for the mirror if `mirror` is set, and for the registry otherwise. Set both or neither. |

If an entry sets both `mirror` and credentials, containerd sends the credentials only to the mirror, so if the mirror can't serve an image, containerd pulls it from the registry without them.

Each entry needs a `mirror`, credentials, or both. Each registry can appear only once. If you also set `image-registry`, it configures `docker.io`, so don't add a `docker.io` entry to `image-registries`.

For example, to pull Docker Hub images through a mirror and private images from GitHub Container Registry:

```yaml
providers:
  k8s:
    enable: true
    bootstrap: true
    image-registries:
      - registry: docker.io
        mirror: https://dockerhub-mirror.example.com
      - registry: ghcr.io
        username: ${GITHUB_ACTOR}
        password: ${GITHUB_TOKEN}
```

Concierge writes a containerd `hosts.toml` file for each mirror. It adds credentials to containerd's registry configuration:

- For Canonical Kubernetes, Concierge passes the credentials to `k8s bootstrap`. Canonical Kubernetes only reads them at bootstrap. If the cluster is already bootstrapped, Concierge logs a warning and doesn't change the credentials.
- For MicroK8s, Concierge adds the credentials to `/var/snap/microk8s/current/args/containerd-template.toml` and restarts containerd.

`concierge restore` removes the mirror files and the credentials.

`image-registry` is the older form, which configures a Docker Hub mirror. It sends the credentials as a fixed HTTP `Authorization` header, which doesn't work for registries such as `ghcr.io` that issue tokens. Use `image-registries` for those.
