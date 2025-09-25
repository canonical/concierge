<p align="center">
  <img width="250px" src=".github/concierge.png" alt="concierge logo">
</p>

<h1 align="center">concierge</h1>
<p align="center">
  <a href="https://snapcraft.io/concierge"><img src="https://snapcraft.io/concierge/badge.svg" alt="Snap Status"></a>
  <a href="https://github.com/canonical/concierge/actions/workflows/release.yaml"><img src="https://github.com/canonical/concierge/actions/workflows/release.yaml/badge.svg"></a>
</p>

`concierge` is an opinionated utility for provisioning charm development and testing machines.

Its role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.

`concierge` also provides the facility to "restore" a machine, performing the opposite to "prepare"
meaning that, for example, any snaps that would have been installed by `concierge`, would then be
removed.

> [!IMPORTANT]
> Take care with `concierge restore`. If the machine already had a given snap or configuration
> prior to running `concierge prepare`, this will not be taken into account during `restore`.
> Running `concierge restore` is the literal opposite of `concierge prepare`, so any packages,
> files or configuration that would normally be created during `prepare` will be removed.

## Installation

The easiest way to consume `concierge` is using the [Snap](https://snapcraft.io/concierge):

```shell
sudo snap install --classic concierge
```

Or, you can install `concierge` with the `go install` command:

```shell
go install github.com/canonical/concierge@latest
```

Or you can clone, build and run like so:

```shell
git clone https://github.com/canonical/concierge
cd concierge
go build
sudo ./concierge -h
```

## Usage

The output of `concierge --help` can be seen below.

```
concierge is an opinionated utility for provisioning charm development and testing machines.

Its role is to ensure that a given machine has the relevant "craft" tools and providers installed,
then bootstrap a Juju controller onto each of the providers. Additionally, it can install selected
tools from the [snap store](https://snapcraft.io) or the Ubuntu archive.

Usage:
  concierge [flags]
  concierge [command]

Available Commands:
  completion  Generate the autocompletion script for the specified shell
  help        Help about any command
  prepare     Provision the machine according to the configuration.
  restore     Run the reverse of `concierge prepare`.
  status      Report the status of `concierge` on the machine.

Flags:
  -h, --help      help for concierge
      --trace     enable trace logging
  -v, --verbose   enable verbose logging
      --version   version for concierge

Use "concierge [command] --help" for more information about a command.
```

Some flags can be set by environment variable, and if specified by flag and environment variable,
the environment variable version will always take precedent. The equivalents are:

|            Flag            |              Env Var               |
| :------------------------: | :--------------------------------: |
|      `--disable-juju`      |      `CONCIERGE_DISABLE_JUJU`      |
|      `--juju-channel`      |      `CONCIERGE_JUJU_CHANNEL`      |
|      `--k8s-channel`       |      `CONCIERGE_K8S_CHANNEL`       |
|    `--microk8s-channel`    |    `CONCIERGE_MICROK8S_CHANNEL`    |
|      `--lxd-channel`       |      `CONCIERGE_LXD_CHANNEL`       |
|   `--charmcraft-channel`   |   `CONCIERGE_CHARMCRAFT_CHANNEL`   |
|   `--snapcraft-channel`    |   `CONCIERGE_SNAPCRAFT_CHANNEL`    |
|   `--rockcraft-channel`    |   `CONCIERGE_ROCKCRAFT_CHANNEL`    |
| `--google-credential-file` | `CONCIERGE_GOOGLE_CREDENTIAL_FILE` |
|      `--extra-snaps`       |      `CONCIERGE_EXTRA_SNAPS`       |
|       `--extra-debs`       |       `CONCIERGE_EXTRA_DEBS`       |

### Command Examples

The best source of examples for how to invoke `concierge` can be found in the
[tests](./tests/) directory, but otherwise:

1. Run `concierge` using the `dev` preset, adding one additional snap, using CLI flags:

```bash
sudo concierge prepare -p dev --extra-snaps node/22/stable
```

2. Run `concierge` using the `dev` preset, overriding the Juju channel:

```bash
export CONCIERGE_JUJU_CHANNEL=3.6/beta
sudo concierge prepare -p dev
```

## Configuration

### Presets

`concierge` comes with a number of presets that are likely to serve most charm development needs:

| Preset Name | Included                                                                  |
| :---------: | :------------------------------------------------------------------------ |
|  `crafts`   | `lxd`, `snapcraft`, `charmcraft`, `rockcraft`                             |
|    `dev`    | `juju`, `k8s`, `lxd`, `snapcraft`, `charmcraft`, `rockcraft`, `jhack`     |
|    `k8s`    | `juju`, `k8s`, `lxd`, `rockcraft`, `charmcraft`                           |
| `microk8s`  | `juju`, `microk8s`, `lxd`, `rockcraft`, `charmcraft`                      |
|  `machine`  | `juju`, `lxd`, `snapcraft`, `charmcraft`                                  |

Note that in the `microk8s`/`k8s` presets, while `lxd` is installed, it is not bootstrapped. It is
installed and initialised with enough config such that `charmcraft` can use it as a build backend.

### Config File

If the presets do not meet your needs, you can create your own config file to instruct `concierge`
how to provision your machine.

`concierge` takes configuration in the form of a YAML file named `concierge.yaml` in the current
working directory.

#### Schema

```yaml
# (Optional): Target Juju configuration.
juju:
  # (Optional): Disable installation of Juju (and therefore all bootstrapping).
  disable: true | false
  # (Optional): Channel from which to install Juju.
  channel: <channel>
  # (Optional): Juju agent version to use when bootstrapping (e.g. "3.5.2").
  agent-version: <version>
  # (Optional): A map of model-defaults to set when bootstrapping *all* Juju controllers.
  model-defaults:
    <model-default>: <value>
  # (Optional): A map of bootstrap-constraints to set when bootstrapping *all* Juju controllers.
  bootstrap-constraints:
    <bootstrap-constraint>: <value>

# (Required): Define the providers to be installed and bootstrapped.
providers:
  # (Optional) MicroK8s provider configuration.
  microk8s:
    # (Optional) Enable or disable MicroK8s.
    enable: true | false
    # (Optional) Whether or not to bootstrap a controller onto MicroK8s.
    bootstrap: true | false
    # (Optional): A map of model-defaults to set when bootstrapping the Juju controller.
    model-defaults:
      <model-default>: <value>
    # (Optional): A map of bootstrap-constraints to set when bootstrapping the Juju controller.
    bootstrap-constraints:
      <bootstrap-constraint>: <value>
    # (Optional): Channel from which to install MicroK8s.
    channel: <channel>
    # (Optional): MicroK8s addons to enable.
    addons:
      - <addon>[:<params>]

  # (Optional) K8s provider configuration.
  k8s:
    # (Optional) Enable or disable K8s.
    enable: true | false
    # (Optional) Whether or not to bootstrap a controller onto K8s.
    bootstrap: true | false
    # (Optional): Channel from which to install K8s.
    channel: <channel>
    # (Optional): A map of model-defaults to set when bootstrapping the Juju controller.
    model-defaults:
      <model-default>: <value>
    # (Optional): A map of bootstrap-constraints to set when bootstrapping the Juju controller.
    bootstrap-constraints:
      <bootstrap-constraint>: <value>
    # (Optional): K8s features to configure.
    features:
      <feature>:
        <key>: <value>

  # (Optional) LXD provider configuration.
  lxd:
    # (Optional) Enable or disable LXD.
    enable: true | false
    # (Optional) Whether or not to bootstrap a controller onto LXD.
    bootstrap: true | false
    # (Optional): Channel from which to install LXD.
    channel: <channel>
    # (Optional): A map of model-defaults to set when bootstrapping the Juju controller.
    model-defaults:
      <model-default>: <value>
    # (Optional): A map of bootstrap-constraints to set when bootstrapping the Juju controller.
    bootstrap-constraints:
      <bootstrap-constraint>: <value>

  # (Optional) Google provider configuration.
  google:
    # (Optional) Enable or disable the Google provider.
    enable: true | false
    # (Optional) Whether or not to bootstrap a controller onto Google Cloud.
    bootstrap: true | false
    # (Optional): File containing credentials for Google cloud.
    # See below note on the credentials file format.
    credentials-file: <path>
    # (Optional): A map of model-defaults to set when bootstrapping the Juju controller.
    model-defaults:
      <model-default>: <value>
    # (Optional): A map of bootstrap-constraints to set when bootstrapping the Juju controller.
    bootstrap-constraints:
      <bootstrap-constraint>: <value>

# (Optional) Additional host configuration.
host:
  # (Optional) List of apt packages to install on the host.
  packages:
    - <package name>
  # (Optional) Map of snap packages to install on the host.
  snaps:
    <snap name>:
      # (Required) Channel from which to install the snap.
      channel: <channel>
      # (Optional) List of snap connections to form.
      connections:
        - <snap>:<plug-interface>
        - <snap>:<plug-interface> <snap>:<plug-interface>
```

#### Providing Credentials Files

Juju has some "built-in" clouds for which it can obtain credentials automatically, such as LXD and MicroK8s. Other clouds require credentials for the bootstrap process.

Concierge handles this with the `credentials-file` option for supported providers.

Juju's credentials are specified in `~/.local/share/juju/credentials.yaml`, in the following format:

```yaml
credentials:
  <cloud-name>:
    <credential-name>:
      key: value
      key: value
```

For example, a pre-configured `credentials.yaml` might look like so where Google Cloud had already been configured:

```yaml
credentials:
  google:
    mycred:
      auth-type: oauth2
      client-email: juju-gce-1-sa@myname.iam.gserviceaccount.com
      client-id: "1234567891234"
      private-key: |
        -----BEGIN PRIVATE KEY-----
        deadbeef
        -----END PRIVATE KEY-----
      project-id: foobar
```

When providing the path to a `credentials-file` for `concierge`, it should contain _only_ the details for a specific credential, so the example above would require a file like so:

```yaml
auth-type: oauth2
client-email: juju-gce-1-sa@myname.iam.gserviceaccount.com
client-id: "1234567891234"
private-key: |
  -----BEGIN PRIVATE KEY-----
  deadbeef
  -----END PRIVATE KEY-----
project-id: foobar
```

If you already have a `credentials.yaml` pre-populated with credentials, you can use `yq` to build the file for you, for example:

```bash
cat ~/.local/share/juju/credentials.yaml | yq -r '.credentials.google.mycred' > google-creds.yaml
```

In the above example `google-creds.yaml` would be valid for the `credentials-file` option.

#### Example Config

An example config file can be seen below:

```yaml
juju:
  channel: 3.5/stable
  agent-version: "3.5.2"
  model-defaults:
    test-mode: "true"
    automatically-retry-hooks: "false"
  bootstrap-constraints:
    arch: amd64

providers:
  microk8s:
    enable: true
    bootstrap: true
    channel: 1.31-strict/stable
    addons:
      - hostpath-storage
      - dns
      - rbac
      - metallb:10.64.140.43-10.64.140.49

  k8s:
    enable: true
    bootstrap: true
    channel: 1.32-classic/stable
    features:
      local-storage:
      load-balancer:
        l2-mode: true
        cidrs: 10.64.140.43/32
    bootstrap-constraints:
      root-disk: 2G

  lxd:
    enable: true
    bootstrap: false
    channel: latest/stable

  google:
    enable: true
    bootstrap: false
    credentials-file: /home/ubuntu/google-credentials.yaml

host:
  packages:
    - python3-pip
    - python3-venv
  snaps:
    astral-uv:
      channel: latest/stable
    charmcraft:
      channel: latest/stable
    jhack:
      channel: latest/edge
      connections:
        - jhack:dot-local-share-juju
```

## Development / HACKING

This project uses [goreleaser](https://goreleaser.com/) to build and release, and `spread` for
integration testing,

You can get started by just using Go, or with `goreleaser`:

```shell
# Clone the repository
git clone https://github.com/canonical/concierge
cd concierge

# Build/run with Go
go run .

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
