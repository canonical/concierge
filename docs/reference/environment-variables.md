---
myst:
  html_meta:
    description: The CONCIERGE_* environment variables that override concierge prepare flags.
---

(reference-environment-variables)=
# Environment variables

Most `concierge prepare` flags have an environment-variable equivalent. When the same setting comes from more than one place, the first of these wins:

1. The environment variable
2. The command-line flag
3. The configuration file or preset

The `*_CHANNEL` variables take a snap channel, such as `3.6/stable` or `latest/edge`. A channel is a track plus a risk level, and it selects the version of the snap that Concierge installs.

`CONCIERGE_DISABLE_JUJU` is a Boolean: set it to `true` or `1` to skip installing and bootstrapping Juju. Any other value, including `false`, has no effect.

`CONCIERGE_EXTRA_SNAPS` and `CONCIERGE_EXTRA_DEBS` take a comma-separated list, for example `jq,yq`. Snaps may include a channel, for example `astral-uv/latest/beta`. Unlike the other variables, these are added to any values given by the flag rather than replacing them.

Each flag is described in {ref}`reference-commands`.

| Flag                       | Environment variable               |
| :------------------------- | :--------------------------------- |
| `--disable-juju`           | `CONCIERGE_DISABLE_JUJU`           |
| `--juju-channel`           | `CONCIERGE_JUJU_CHANNEL`           |
| `--juju-revision`          | `CONCIERGE_JUJU_REVISION`          |
| `--k8s-channel`            | `CONCIERGE_K8S_CHANNEL`            |
| `--microk8s-channel`       | `CONCIERGE_MICROK8S_CHANNEL`       |
| `--lxd-channel`            | `CONCIERGE_LXD_CHANNEL`            |
| `--charmcraft-channel`     | `CONCIERGE_CHARMCRAFT_CHANNEL`     |
| `--snapcraft-channel`      | `CONCIERGE_SNAPCRAFT_CHANNEL`      |
| `--rockcraft-channel`      | `CONCIERGE_ROCKCRAFT_CHANNEL`      |
| `--google-credential-file` | `CONCIERGE_GOOGLE_CREDENTIAL_FILE` |
| `--extra-snaps`            | `CONCIERGE_EXTRA_SNAPS`            |
| `--extra-debs`             | `CONCIERGE_EXTRA_DEBS`             |
