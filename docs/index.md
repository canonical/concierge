---
myst:
  html_meta:
    description: Documentation for Concierge, an opinionated utility for provisioning charm development and testing machines with Juju, LXD, Kubernetes, and the craft tools.
---

# Concierge

Concierge is an opinionated utility for provisioning charm development and testing machines. It installs the "craft" tools and providers you need, bootstraps a Juju controller onto each provider, and installs supporting packages from the snap store or the Ubuntu archive.

Concierge is fully declarative: a single `concierge prepare` command takes a machine from a fresh install to a ready environment, and `concierge restore` reverses it.

## In this documentation

::::{grid} 1 1 2 2
:gutter: 3

:::{grid-item-card} [How-to guides](how-to/index)

**Step-by-step guides** covering common tasks such as writing a custom config or providing cloud credentials.
:::

:::{grid-item-card} [Reference](reference/index)

**Technical information** — commands, flags, environment variables, the configuration schema, and the built-in presets.
:::

:::{grid-item-card} [Explanation](explanation/index)

**Discussion and background** on what Concierge is for and how `prepare` and `restore` relate to each other.
:::

::::

## Project and community

Concierge is a member of the Ubuntu family and released under the [Apache 2.0 licence](https://github.com/canonical/concierge/blob/main/LICENSE).

- [Report a bug or request a feature](https://github.com/canonical/concierge/issues)
- [Contribute](https://github.com/canonical/concierge/blob/main/CONTRIBUTING.md)
- [Report a security issue](project:./explanation/security.md)

```{toctree}
:hidden:
:maxdepth: 1

how-to/index
reference/index
explanation/index
```
