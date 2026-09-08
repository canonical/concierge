---
myst:
  html_meta:
    description: Concierge's trust boundaries, the files it writes, the security events it emits, and how to report a vulnerability.
---

(security)=
# Security

Concierge provisions a machine for charm development. It runs as root, installs snaps and Debian packages, bootstraps Juju controllers, and writes credentials into the home directory of the user who invoked it.

Concierge is for throwaway machines: virtual machines, CI runners, and dedicated test hosts. Don't run it on your daily workstation. See [](explanation-prepare-restore-opposites) for why.

## Product architecture

Concierge is a short-lived command, not a daemon. It reads a configuration, runs a sequence of privileged commands, and exits.

```{mermaid}
flowchart LR
    User["Invoking user<br/>(via sudo)"]
    Config["Preset, config file,<br/>environment, flags"]
    Concierge["concierge<br/>(runs as root)"]
    Packages["snapd, APT"]
    Providers["LXD, Canonical K8s,<br/>MicroK8s"]
    Juju["Juju controllers"]
    Home["User's home directory<br/>(credentials, kubeconfig,<br/>cached config)"]
    Journal["System journal"]

    User -->|invokes| Concierge
    Config -->|read by| Concierge
    Concierge -->|installs packages| Packages
    Concierge -->|configures| Providers
    Concierge -->|bootstraps| Juju
    Concierge -->|writes files| Home
    Concierge -->|security events| Journal
```

Its trust boundaries follow from that position.

The first is the jump to root. Concierge is invoked with `sudo`, and everything it does afterwards runs with full privileges. It executes its steps as shell commands, which is deliberate: Concierge is a wrapper around the same commands you would otherwise run by hand.

The second is the configuration. A preset, a config file, environment variables, and command-line flags all feed the same plan, and that plan decides which commands run as root. Treat a `concierge.yaml` file as you would a shell script.

The third is the user's home directory. Concierge writes files there as root and then changes their ownership to the invoking user. See [](#files-concierge-writes).

Concierge opens no listening ports and runs no background process. Once it exits, you interact with the tools it installed directly.

Concierge is a classic snap, so snapd doesn't confine it. Classic confinement is what lets it manage other snaps, run `apt`, and write outside its own directories.

## Secure by design

Concierge adds no privileges beyond those needed to install and configure the same tools by hand. It implements no cryptography, no authentication, and no network service of its own.

The tool is declarative, so the set of privileged actions is determined by configuration you can read, diff, and commit before it runs. `concierge prepare --dry-run` prints what would happen without changing anything.

Credentials are never written to the security event log. When Concierge records that it wrote a credentials file, the event carries the path and the number of clouds, not the contents.

(files-concierge-writes)=
## Files Concierge writes

Concierge writes three files into the home directory of the user it is provisioning for. All three can contain secrets, and all three are written mode 0600, readable only by that user.

| File | Contains |
| :--- | :--- |
| `~/.local/share/juju/credentials.yaml` | Juju cloud credentials, including any Google service-account credentials you supplied |
| `~/.kube/config` | Kubernetes cluster credentials, when a Kubernetes provider is configured |
| `~/.cache/concierge/concierge.yaml` | The merged runtime configuration, including the image registry password when one is configured |

Concierge also reads a credentials file you provide for the Google provider, through `--google-credential-file` or the `google.credentials-file` config key. Concierge doesn't change that file's permissions. Store it as you would any other cloud credential.

## Cryptographic technology

Concierge implements no cryptography. It relies on the package managers it drives:

- `snap` verifies snap assertions and downloads.
- `apt` verifies archive signatures.

The tools that Concierge installs use cryptography of their own. Juju, for example, generates controller certificates during bootstrap.

Concierge doesn't encrypt anything at rest. The confidentiality of the files listed in [](#files-concierge-writes) rests on their permissions and on the host's own encryption.

See more: [Debian | SecureApt](https://wiki.debian.org/SecureApt) and [Snap | Security policies](https://snapcraft.io/docs/security-policies).

## Configuring and operating

To harden a machine provisioned by Concierge:

- Provision throwaway machines only, so that a compromised environment can be destroyed rather than cleaned. See [](explanation-prepare-restore-opposites).
- Review any `concierge.yaml` you didn't write before running it. The `extra-snaps`, `extra-debs`, and `model-defaults` keys all cause privileged actions.
- Restrict the permissions of any credentials file you pass with `--google-credential-file`. Concierge reads it and doesn't modify it.
- Harden the installed products themselves. Concierge doesn't harden Juju, LXD, or Kubernetes for you.

Concierge resolves settings from the environment first, then command-line flags, then the config file or preset. An environment variable therefore overrides a flag, which is the opposite of what you may expect. See {ref}`reference-environment-variables`.

The image registry password in a config file expands environment variables, so `password: ${REGISTRY_PASSWORD}` reads the value at run time. Prefer that to writing the password into the file.

See also: [Juju | Harden your deployment](https://documentation.ubuntu.com/juju/3.6/howto/manage-your-juju-deployment/harden-your-juju-deployment/), [Canonical K8s | Hardening guide](https://documentation.ubuntu.com/canonical-kubernetes/release-1.32/snap/howto/security/hardening/), and [LXD | Security](https://documentation.ubuntu.com/lxd/stable-5.21/explanation/security/).

## Logging and monitoring

Concierge writes human-readable progress to standard error, controlled by `--verbose` and `--trace`. Separately, it emits structured security events to the system journal under the `concierge` identifier, so the audit trail doesn't depend on the verbosity you happened to choose.

The events follow the [OWASP Application Logging Vocabulary](https://cheatsheetseries.owasp.org/cheatsheets/Logging_Vocabulary_Cheat_Sheet.html):

| Event | Emitted when |
| :--- | :--- |
| `sys_startup` | `concierge prepare` begins provisioning |
| `sys_shutdown` | `concierge restore` begins decommissioning |
| `authz_admin` | A privileged command runs, or a credentials file is written |
| `privilege_permissions_changed` | Concierge changes the ownership of a file or directory |

Each event is a JSON object. To read them:

```bash
journalctl -t concierge -o cat | jq .
```

Read-only checks are not logged, so the trail covers the actions that changed the machine.

The events record the commands Concierge ran. If you pass a secret as an argument in `extra-snaps`, `model-defaults`, or a similar key, that argument appears in the journal and in `--trace` output. Pass secrets through environment variables or a credentials file instead.

## Decommissioning

`concierge restore` destroys the Juju controllers it bootstrapped, removes the snaps and packages it installed, and deletes `~/.local/share/juju` and `~/.kube`.

Two things are left behind, and you should remove them yourself if the machine is not being destroyed:

- `~/.cache/concierge/concierge.yaml`, the cached runtime configuration. Concierge reads it during restore and doesn't delete it afterwards. It contains the image registry password if you configured one.
- Any credentials file you supplied with `--google-credential-file`.

Restore removes what the configuration says `prepare` would have installed, not what it observes on the machine. If a snap was already present before you ran `prepare`, restore removes it anyway.

To remove Concierge itself, run `sudo snap remove concierge`.

## Security lifecycle

Concierge is distributed as the [`concierge` snap](https://snapcraft.io/concierge) and as source releases on GitHub.

Install the snap to get security updates automatically, delivered by snap refresh. To schedule or defer refreshes, see the [snap documentation](https://snapcraft.io/docs/managing-updates), bearing in mind that delaying a refresh delays security fixes.

Security updates are released for all major versions that have had a release in the last year, as stated in [SECURITY.md](https://github.com/canonical/concierge/blob/main/SECURITY.md). A major version with no release for over a year is end of life.

To check which version you are running:

```bash
concierge version
```

## Reporting vulnerabilities

To report a security issue, follow the instructions in [SECURITY.md](https://github.com/canonical/concierge/blob/main/SECURITY.md).

The preferred channel is a private [GitHub security advisory](https://github.com/canonical/concierge/security/advisories/new). You can also email `security@ubuntu.com`. To encrypt your email, follow [Canonical's reporting instructions](https://ubuntu.com/security/disclosure-policy#contact-us).

Known vulnerabilities are published as [GitHub security advisories](https://github.com/canonical/concierge/security/advisories) for this repository.

The [Ubuntu Security disclosure and embargo policy](https://ubuntu.com/security/disclosure-policy) describes what you can expect after you report an issue.
