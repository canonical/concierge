---
myst:
  html_meta:
    description: The complete YAML schema for Concierge configuration files, covering Juju, providers (LXD, K8s, MicroK8s, Google), and host packages and snaps.
---

(reference-configuration)=
# Configuration schema

Concierge reads a YAML configuration file. The schema below describes every field. All top-level blocks are optional; a minimal configuration might set just one provider.

For task-oriented guidance, see [Write a custom config](../how-to/write-a-custom-config).

```yaml
# (Optional) Target Juju configuration.
juju:
  # (Optional) Disable installation of Juju (and therefore all bootstrapping).
  disable: true | false
  # (Optional) Channel from which to install Juju.
  channel: <channel>
  # (Optional) Snap revision from which to install Juju, for example "31429". When
  # combined with `channel`, snap installs the specified revision and the
  # channel is only used for tracking after install.
  revision: <revision>
  # (Optional) Juju agent version to use when bootstrapping, for example "3.6.11".
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
    channel: <channel>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>
    # (Optional) K8s features to configure.
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
    channel: <channel>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>

  google:
    enable: true | false
    bootstrap: true | false
    # See "Provide cloud credentials" for the expected file format.
    credentials-file: <path>
    model-defaults:
      <model-default>: <value>
    bootstrap-constraints:
      <bootstrap-constraint>: <value>

# (Optional) Additional host configuration.
host:
  # (Optional) apt packages to install.
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
