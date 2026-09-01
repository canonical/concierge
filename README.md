<p align="center">
  <img width="250px" src=".github/concierge.png" alt="concierge logo">
</p>

<h1 align="center">concierge</h1>
<p align="center">
  <a href="https://snapcraft.io/concierge"><img src="https://snapcraft.io/concierge/badge.svg" alt="Snap Status"></a>
  <a href="https://github.com/canonical/concierge/actions/workflows/release.yaml"><img src="https://github.com/canonical/concierge/actions/workflows/release.yaml/badge.svg"></a>
</p>

`concierge` is an opinionated utility for provisioning charm development and testing machines. It installs the "craft" tools and providers you need, bootstraps Juju onto each provider, and installs supporting snaps or apt packages — all from a single declarative config.

## Install

```shell
sudo snap install --classic concierge
```

## Quick start

```shell
sudo concierge prepare -p dev
```

## Documentation

The full documentation lives in [`docs/`](docs/) and covers:

- [how-to guides](docs/how-to/) for common tasks such as [writing a custom config](docs/how-to/write-a-custom-config.md);
- [reference](docs/reference/) for commands, the configuration schema, environment variables, and the built-in presets;
- [explanation](docs/explanation/) of what Concierge is for and why `prepare` and `restore` are strict opposites.

To build and browse the docs locally:

```shell
make -C docs run
```
