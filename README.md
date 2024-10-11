<p align="center">
  <img width="250px" src="snap/concierge.png" alt="concierge logo">
</p>

<h1 align="center">concierge</h1>
<p align="center">
  <!--<a href="https://snapcraft.io/concierge"><img src="https://snapcraft.io/concierge/badge.svg" alt="Snap Status"></a>-->
  <a href="https://github.com/jnsgruk/concierge/actions/workflows/release.yaml"><img src="https://github.com/jnsgruk/concierge/actions/workflows/release.yaml/badge.svg"></a>
</p>

`concierge` is an opinionated utility for provisioning charm development and testing machines.

It's role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.

`concierge` also provides the facility to "restore" a machine to its pre-provisioned state if the
tool has previously been run on the machine.

## Installation

<!--

The easiest way to consume `concierge` is using the [Snap](https://snapcraft.io/concierge):

```shell
sudo snap install --classic concierge
```
-->

You can install `concierge` with the `go install` command:

```shell
go install github.com/jnsgruk/concierge@latest
```

Or you can clone, build and run like so:

```shell
git clone https://github.com/jnsgruk/concierge
cd concierge
go build -o concierge main.go
./concierge -h
```

## Usage

The output of `concierge --help` can be seen below.

```
concierge is an opinionated utility for provisioning charm development and testing machines.

It's role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.

Usage:
  concierge [flags]
  concierge [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  prepare     Provision the machine according to the configuration.
  restore     Restore the machine to it's pre-provisioned state.

Flags:
  -h, --help      help for concierge
      --trace     enable trace logging
  -v, --verbose   enable verbose logging
      --version   version for concierge

Use "concierge [command] --help" for more information about a command.
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
|    `--extra-snaps`     |    `CONCIERGE_EXTRA_SNAPS`     |
|     `--extra-debs`     |     `CONCIERGE_EXTRA_DEBS`     |

### Command Examples

The best source of examples for how to invoke `concierge` can be found in the
[tests](./tests/) directory, but otherwise:

1. Run `concierge` using the `dev` preset, adding one additional snap, using CLI flags:

```bash
concierge prepare -p dev --extra-snaps node/22/stable
```

2. Run `concierge` using the `dev` preset, overriding the Juju channel:

```bash
export CONCIERGE_JUJU_CHANNEL=3.6/beta
concierge prepare -p dev
```

## Configuration

### Presets

`concierge` comes with a number of presets that are likely to serve most charm development needs:

| Preset Name | Included                                                         |
| :---------: | :--------------------------------------------------------------- |
|    `dev`    | `juju`, `microk8s`, `lxd` `snapcraft`, `charmcraft`, `rockcraft` |
|    `k8s`    | `juju`, `microk8s`, `rockcraft`, `charmcraft`                    |
|  `machine`  | `juju`, `lxd`, `snapcraft`, `charmcraft`                         |

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
# ...
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
