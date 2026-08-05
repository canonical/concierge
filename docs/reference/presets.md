---
myst:
  html_meta:
    description: The built-in Concierge presets (crafts, dev, k8s, microk8s, machine) with their full YAML source included verbatim.
---

(reference-presets)=
# Presets

Concierge ships with several built-in presets that cover common charm development setups. Choose one with `-p / --preset`:

```bash
sudo concierge prepare -p dev
```

| Preset     | Included                                                                             |
| :--------- | :----------------------------------------------------------------------------------- |
| `crafts`   | `lxd`, `snapcraft`, `charmcraft`, `rockcraft`                                        |
| `dev`      | `juju`, `k8s`, `lxd`, `snapcraft`, `charmcraft`, `rockcraft`, `jhack`, `astral-uv`   |
| `k8s`      | `juju`, `k8s`, `lxd`, `rockcraft`, `charmcraft`                                      |
| `microk8s` | `juju`, `microk8s`, `lxd`, `rockcraft`, `charmcraft`                                 |
| `machine`  | `juju`, `lxd`, `snapcraft`, `charmcraft`                                             |

In the `k8s` and `microk8s` presets, LXD is installed but not bootstrapped — it is present only so Charmcraft can use it as a build backend.

To adapt one of these presets into your own config, see [Write a custom config](../how-to/write-a-custom-config).

## `crafts.yaml`

```{literalinclude} ../../presets/crafts.yaml
:language: yaml
```

## `dev.yaml`

```{literalinclude} ../../presets/dev.yaml
:language: yaml
```

## `k8s.yaml`

```{literalinclude} ../../presets/k8s.yaml
:language: yaml
```

## `microk8s.yaml`

```{literalinclude} ../../presets/microk8s.yaml
:language: yaml
```

## `machine.yaml`

```{literalinclude} ../../presets/machine.yaml
:language: yaml
```
