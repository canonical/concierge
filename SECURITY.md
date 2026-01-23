# Security policy

## Supported versions

Security updates will be released for all major versions that have had releases in the last year.

## Reporting a vulnerability

Please provide a description of the issue, the steps you took to
create the issue, affected versions, and, if known, mitigations for
the issue.

The easiest way to report a security issue is through
[GitHub's security advisory for this project](https://github.com/canonical/concierge/security/advisories/new). See
[Privately reporting a security
vulnerability](https://docs.github.com/en/code-security/security-advisories/guidance-on-reporting-and-writing/privately-reporting-a-security-vulnerability)
for instructions on reporting using GitHub's security advisory feature.

The Concierge GitHub admins will be notified of the issue and will work with you
to determine whether the issue qualifies as a security issue and, if so, in
which component. We will then figure out a fix, get a CVE assigned, and coordinate
the release of the fix.

You may also send email to security@ubuntu.com. Email may optionally be
encrypted to OpenPGP key
[`75E1 451E 529B 51E1 9006 CD5E 91EC 85F1 DA9A 776D`](https://keyserver.ubuntu.com/pks/lookup?op=get&search=0x75e1451e529b51e19006cd5e91ec85f1da9a776d)

If you have a deadline for public disclosure, please let us know.
Our vulnerability management team intends to respond within 3 working
days of your report. This project aims to resolve all vulnerabilities
within 90 days.

The [Ubuntu Security disclosure and embargo
policy](https://ubuntu.com/security/disclosure-policy) contains more
information about what you can expect when you contact us, and what we
expect from you.

## Cryptographic technology

Concierge uses cryptographic technology to securely download Snaps and Debian packages from the Ubuntu archive to install. Some of those tools, such as Juju, will in turn use crytographic technology to securely download images and other data needed to initialise.

Concierge uses `apt` to install Debian packages, and Snap (via the [snapcore/snapd library](https://github.com/canonical/snapd))) to install Snaps.

> See more:
>  - [Debian | SecureApt](https://wiki.debian.org/SecureApt)
>  - [Ubuntu Community | SecureApt](https://help.ubuntu.com/community/SecureApt)
>  - [Snap | Cryptography](https://snapcraft.io/docs/security-policies#p-2741-cryptography)

## Hardening

No additional steps are required to harden your system when using Concierge.

> See also:
>  - [Juju | Harden your deployment](https://documentation.ubuntu.com/juju/3.6/howto/manage-your-deployment/#harden-your-deployment)
>  - [MicroK8s | CIS cluster hardening](https://microk8s.io/docs/cis-compliance)
>  - [Canonical K8s | Hardening guide](https://documentation.ubuntu.com/canonical-kubernetes/release-1.32/snap/howto/security/hardening/)

## Risks

Concierge does not add any risks over manually installing and configuring the Snaps and other packages included in the presets. However, users should be familiar with the security of each of the installed products.

> See also:
>  - [Juju Security](https://documentation.ubuntu.com/juju/3.6/explanation/juju-security/)
>  - [LXD Security](https://documentation.ubuntu.com/lxd/stable-5.21/explanation/security/)
>  - [Canonical K8s Security](https://documentation.ubuntu.com/canonical-kubernetes/release-1.32/snap/howto/security/)


## Good practice

If you are [providing credentials to Concierge](https://github.com/canonical/concierge/?tab=readme-ov-file#providing-credentials-files) for clouds, ensure that these are stored securely.
