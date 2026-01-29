# Development / HACKING

## Getting started

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

## Testing

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

