---
myst:
  html_meta:
    description: The CONCIERGE_* environment variables that override concierge prepare flags, and the naming rule that generates them.
---

(reference-environment-variables)=
# Environment variables

Most `concierge prepare` flags have an environment-variable equivalent. When a value is set both by flag and by environment variable, the environment variable wins.

The variable name is the flag name, uppercased, with dashes replaced by underscores, and prefixed with `CONCIERGE_`.

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
