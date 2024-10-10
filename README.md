# concierge

<!--
<a href="https://snapcraft.io/concierge"><img src="https://snapcraft.io/concierge/badge.svg" alt="Snap Status"></a>
<a href="https://github.com/jnsgruk/concierge/actions/workflows/release.yaml"><img src="https://github.com/jnsgruk/concierge/actions/workflows/release.yaml/badge.svg"></a>
-->

`concierge` is an opinionated utility for provisioning charm development and testing machines.

It's role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.

## Installation

The easiest way to consume `concierge` is using the [Snap](https://snapcraft.io/concierge):

```shell
sudo snap install --classic concierge
```

Or you can clone, build and run like so:

```shell
git clone https://github.com/jnsgruk/concierge
cd concierge
go build -o concierge main.go
./concierge
```

## Usage

The output of `concierge --help` can be seen below.

```
concierge is an opinionated utility for provisioning charm development and testing machines.

It's role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.

Configuration is by flags/environment variables, or by configuration file. The configuration file
must be in the current working directory and named 'concierge.yaml', or the path specified using
the '-c' flag.

There are 3 presets available by default: 'machine', 'k8s' and 'dev'.

Some aspects of presets and config files can be overridden using flags such as '--juju-channel'.
Each of the override flags has an environment variable equivalent,
such as 'CONCIERGE_JUJU_CHANNEL'.

More information at https://github.com/jnsgruk/concierge.

Usage:
  concierge [flags]

Flags:
      --charmcraft-channel string   override snap channel for charmcraft
  -c, --config string               path to a specific config file to use
      --extra-debs strings          comma-separated list of extra debs to install. E.g. 'make,python3-tox'
      --extra-snaps strings         comma-separated list of extra snaps to install. Each item can simply be the name of a snap, but also include the channel. E.g. 'astral-uv/latest/edge,jhack'
  -h, --help                        help for concierge
      --juju-channel string         override the snap channel for juju
      --lxd-channel string          override snap channel for lxd
      --microk8s-channel string     override snap channel for microk8s
  -p, --preset string               config preset to use (k8s | machine | dev)
      --rockcraft-channel string    override snap channel for rockcraft
      --snapcraft-channel string    override snap channel for snapcraft
  -v, --verbose                     enable verbose logging
      --version                     version for concierge
```

Some flags can be set by environment variable, and if specified by flag and environment variable,
the environment variable version will always take precedent. The equivalents are:

|          Flag          |            Env Var             |
| :--------------------: | :----------------------------: |
|    `--juju-channel`    |    `CONCIERGE_JUJU_CHANNEL`    |
|  `--microk8s-channel`  |  `CONCIERGE_MICROK8S_CHANNEL`  |
|    `--lxd-channel`     |    `CONCIERGE_LXD_CHANNEL`     |
| `--charmcraft-channel` | `CONCIERGE_CHARMCRAFT_CHANNEL` |
| `--snapcraft-channel`  | `CONCIERGE_SNAPCRAFT_CHANNEL`  |
| `--rockcraft-channel`  | `CONCIERGE_ROCKCRAFT_CHANNEL`  |

### Command Examples

1. Run `concierge` using the `dev` preset, adding one additional snap, using CLI flags:

```bash
concierge -p dev --extra-snaps node/22/stable
```

1. Run `concierge` using the `dev` preset, overriding the Juju channel:

```bash
export CONCIERGE_JUJU_CHANNEL=3.6/beta
concierge -p dev
```

## Configuration

### Presets

`concierge` comes with a number of presets that are likely to serve most charm development needs:

| Preset Name | Purpose                          | Included                                                         |
| :---------: | :------------------------------- | :--------------------------------------------------------------- |
|    `dev`    | Dev testing of all charms        | `juju`, `microk8s`, `lxd` `snapcraft`, `charmcraft`, `rockcraft` |
|    `k8s`    | Dev/testing of Kubernetes charms | `juju`, `microk8s`, `rockcraft`, `charmcraft`                    |
|  `machine`  | Dev/testing of machine charms    | `juju`, `lxd`, `snapcraft`, `charmcraft`                         |

### Config File

If the presets do not meet your needs, you can create your own config file to instruct `concierge`
how to provision your machine.

`concierge` takes configuration in the form of a YAML file named `concierge.yaml` in the current
working directory.

```yaml
# (Optional): Target Juju configuration.
juju:
  # (Optional): Channel from which to install Juju.
  channel: <channel>
  # (Optional): A map of model-defaults to set when bootstrapping Juju controllers.
  model-defaults:
    <model-default>: <value>

# (Required): Define the providers to be installed and bootstrapped.
providers:
  # (Optional) MicroK8s provider configuration.
  microk8s:
    # (Optional) Enable or disable MicroK8s.
    enable: true | false
    # (Optional): Channel from which to install MicroK8s.
    channel: <channel>
    # (Optional): MicroK8s addons to enable.
    addons:
      - <addon>[:<params>]

  # (Optional) LXD provider configuration.
  lxd:
    # (Optional) Enable or disable LXD.
    enable: true | false
    # (Optional): Channel from which to install LXD.
    channel: <channel>

# (Optional) Additional host configuration.
host:
  # (Optional) List of apt packages to install on the host.
  packages:
    - <package name>
  # (Optional) List of snap packages to install on the host.
  snaps:
    - <snap name/channel>
```

An example config file can be seen below:

```yaml
juju:
  channel: 3.5/stable
  model-defaults:
    test-mode: "true"
    automatically-retry-hooks: "false"

providers:
  microk8s:
    enable: true
    channel: 1.31-strict/stable
    addons:
      - hostpath-storage
      - dns
      - rbac
      - metallb:10.64.140.43-10.64.140.49

  lxd:
    enable: true
    channel: latest/stable

host:
  packages:
    - python3-pip
    - python3-venv
  snaps:
    - charmcraft/latest/stable
    - rockcraft/latest/stable
    - snapcraft/latest/stable
```

## Development / HACKING

This project uses [goreleaser](https://goreleaser.com/) to build and release, and `spread` for
integration testing,

You can get started by just using Go, or with `goreleaser`:

```shell
# Clone the repository
git clone https://github.com/jnsgruk/concierge
cd concierge

# Build/run with Go
go run main.go

# Run the unit tests
go test ./...

# Build a snapshot release with goreleaser (output in ./dist)
goreleaser build --clean --snapshot
```

### Testing

Most of the code within tries to call a shell command, or manipulate the system in some way, which
makes unit testing much more awkward. As a result, the majority of the testing is done with
[`spread`](https://github.com/canonical/spread).

Currently, there are two supported backends - tests can either be run in LXD virtual machines, or
on a pre-provisioned server (such as a Github Actions runner or development VM).

To show the available integration tests, you can:

```bash
$ spread -list lxd:
lxd:ubuntu-24.04:tests/extra-debs
lxd:ubuntu-24.04:tests/extra-packages-config-file
lxd:ubuntu-24.04:tests/extra-snaps
lxd:ubuntu-24.04:tests/juju-model-defaults
lxd:ubuntu-24.04:tests/overrides-env
lxd:ubuntu-24.04:tests/overrides-priority
lxd:ubuntu-24.04:tests/preset-dev
lxd:ubuntu-24.04:tests/preset-k8s
lxd:ubuntu-24.04:tests/preset-machine
lxd:ubuntu-24.04:tests/provider-lxd
lxd:ubuntu-24.04:tests/provider-microk8s
lxd:ubuntu-24.04:tests/provider-none
```

From there, you can either run all of the tests, or a selection:

```bash
# Run all of the tests
$ spread -v lxd:
# Run a particular test
$ spread -v lxd:ubuntu-24.04:tests/juju-model-defaults
```

To run any of the tests on a locally provisioned machine, use the `github-ci` backend, e.g.

```bash
# List available tests
$ spread --list github-ci:
# Run all of the tests
$ spread -v github-ci:
# Run a particular test
$ spread -v github-ci:ubuntu-24.04:tests/juju-model-defaults
```
