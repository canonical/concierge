---
myst:
  html_meta:
    description: How to install Concierge and prepare a machine with a built-in preset.
---

(how-to-set-up-a-machine)=
# Set up a machine

Install Concierge from the snap store:

```bash
sudo snap install --classic concierge
```

Run `prepare` with the preset that suits your work — `dev` installs Juju, LXD, Kubernetes, and the craft tools:

```bash
sudo concierge prepare -p dev
```

The other presets are listed in the [presets reference](../reference/presets). If none of them fit, [write a custom config](write-a-custom-config).

Check what Concierge installed:

```bash
sudo concierge status
```
