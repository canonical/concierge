---
myst:
  html_meta:
    description: How to adapt a built-in Concierge preset into a custom concierge.yaml — adding snaps, changing the Juju version, disabling providers, or adding new ones.
---

(how-to-write-a-custom-config)=
# How to write a custom config

When the built-in presets don't fit your needs, write your own YAML configuration file.

## Start from a preset

Open {ref}`reference-presets`, find the preset that most closely matches what you want, and copy its YAML into a local file — for example, `concierge.yaml`. Edit your file to suit your needs.

For how to point Concierge at your file, see `prepare` in {ref}`reference-commands`.

## Common adaptations

### Add or remove a snap

List the snaps you need as keys within `host.snaps`. If you need a specific channel of a snap, add `channel:` within the snap's key. For example:

```yaml
host:
  snaps:
    charmcraft:
    jq:
    astral-uv:
      channel: latest/beta
```

### Change the Juju version

Set `juju.channel`, and optionally pin an `agent-version`:

```yaml
juju:
  channel: 3.6/stable
  agent-version: "3.6.11"
```

### Turn off Kubernetes

Start from `dev.yaml` and either delete the `k8s:` block from `providers:` or disable it explicitly:

```yaml
providers:
  k8s:
    enable: false
```

### Add a provider that presets don't cover

To add a Google cloud provider on top of an existing preset, extend the `providers:` block and provide credentials:

```yaml
providers:
  google:
    enable: true
    bootstrap: true
    credentials-file: /home/ubuntu/google-credentials.yaml
```

See more: {ref}`how-to-provide-credentials`

## Full reference

Every field is documented in {ref}`reference-configuration`.

