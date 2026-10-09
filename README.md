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

See the [full documentation](https://canonical.com/juju/docs/concierge/) for:

- Common tasks such as [writing a custom config](https://canonical.com/juju/docs/concierge/how-to/write-a-custom-config/)
- The [configuration schema](https://canonical.com/juju/docs/concierge/reference/configuration/) and [presets](https://canonical.com/juju/docs/concierge/reference/presets/)
- A deeper explanation of [what Concierge is for](https://canonical.com/juju/docs/concierge/explanation/what-is-concierge/)

And more!
