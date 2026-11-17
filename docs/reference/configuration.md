---
myst:
  html_meta:
    description: The complete YAML schema for Concierge configuration files, covering Juju, providers (LXD, Canonical Kubernetes, MicroK8s, Google), services (microceph), and host packages and snaps.
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
    channel: <channel>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>
    # (Optional) MicroK8s addons to enable.
    addons:
      - <addon>[:<params>]
    # (Optional) Image registry mirror. Values support ${VAR} interpolation.
    image-registry:
      url: <url>
      username: <username>
      password: <password>

  k8s:
    enable: true | false
    bootstrap: true | false
    controller-name: <name>
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

# (Optional) Services to install and configure, independently of providers.
services:
  microceph:
    enable: true | false
    # (Optional) Channel from which to install the microceph snap.
    channel: <channel>
    # (Optional) Size of each of the 3 loop-backed disks added as OSDs
    disk-size: <size>
    # (Optional) RGW/S3 gateway. Only enabled if this block is present and
    # `port` is non-zero.
    rgw:
      port: <port>
      # (Optional) S3 access/secret key for the managed RGW user. Values
      # support ${VAR} interpolation. If omitted, fixed, documented
      # (insecure) defaults are used.
      access-key: <access-key>
      secret-key: <secret-key>
      # (Optional) Configures rgw_dns_name for virtual-hosted-style bucket
      # addressing (bucket.host instead of host/bucket) and restarts RGW to
      # apply it. Only needed by S3 clients that default to that style.
      dns-name: <dns-name>
      # (Optional) Bucket to create under the managed RGW user.
      bucket: <bucket-name>
    # (Optional) Additively integrate a ceph-backed storage class into the
    # k8s provider enabled above. Requires `providers.k8s.enable: true`.
    # Not supported for microk8s — use providers.microk8s.addons instead.
    storage-class:
      enable: true | false

# (Optional) Additional host configuration.
host:
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
