---
myst:
  html_meta:
    description: Reference for the concierge command-line interface — prepare, restore, status, and their flags.
---

(reference-commands)=
# Commands

Concierge has three primary commands: `prepare`, `restore`, and `status`.

## Global flags

The following flags apply to every command.

| Flag              | Description                     |
| :---------------- | :------------------------------ |
| `-v, --verbose`   | Enable verbose logging.         |
| `--trace`         | Enable trace logging.           |
| `-h, --help`      | Show help for the command.      |
| `--version`       | Print the Concierge version.    |

## `concierge prepare`

Provision the machine according to the configuration. Must be run as root.

```
sudo concierge prepare [flags]
```

Concierge selects its configuration in this order:

1. `-c, --config <path>` — an explicit config file.
2. `-p, --preset <name>` — one of the built-in [presets](presets).
3. A `concierge.yaml` file in the current working directory.
4. The `dev` preset, if none of the above are found.

### Flags

| Flag                       | Description                                                          |
| :------------------------- | :------------------------------------------------------------------- |
| `-c, --config <path>`      | Path to a specific config file to use.                               |
| `-p, --preset <name>`      | Built-in preset to use: `crafts`, `dev`, `k8s`, `machine`, `microk8s`. |
| `--dry-run`                | Print the commands that would run without executing them.            |
| `--disable-juju`           | Skip the installation and bootstrap of Juju.                         |
| `--juju-channel <ch>`      | Override the snap channel for Juju.                                  |
| `--juju-revision <rev>`    | Override the snap revision for Juju.                                 |
| `--k8s-channel <ch>`       | Override snap channel for the `k8s` snap.                            |
| `--microk8s-channel <ch>`  | Override snap channel for MicroK8s.                                  |
| `--lxd-channel <ch>`       | Override snap channel for LXD.                                       |
| `--charmcraft-channel <ch>` | Override snap channel for Charmcraft.                               |
| `--snapcraft-channel <ch>` | Override snap channel for Snapcraft.                                 |
| `--rockcraft-channel <ch>` | Override snap channel for Rockcraft.                                 |
| `--google-credential-file <path>` | Override path to the Google credentials file.                 |
| `--extra-snaps <list>`     | Additional snaps to install (comma-separated, `name/channel`).       |
| `--extra-debs <list>`      | Additional apt packages to install (comma-separated).                |

## `concierge restore`

Run the reverse of `concierge prepare`, removing everything Concierge would have installed. Must be run as root.

```
sudo concierge restore [flags]
```

### Flags

| Flag        | Description                                       |
| :---------- | :------------------------------------------------ |
| `--dry-run` | Print the commands that would run without executing them. |

:::{important}
Restore removes everything Concierge would install, regardless of whether it was on the machine beforehand. See [what Concierge is for](../explanation/what-is-concierge) for the reasoning.
:::

## `concierge status`

Report the status of Concierge on the machine — which providers are up, which Juju controllers are bootstrapped, and which snaps and packages are installed. Must be run as root.

```
sudo concierge status
```
