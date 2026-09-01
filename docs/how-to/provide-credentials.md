---
myst:
  html_meta:
    description: How to give Concierge the credentials it needs to bootstrap Juju controllers on clouds like Google, AWS, and Azure using the credentials-file field.
---

(how-to-provide-credentials)=
# How to provide cloud credentials

Some Juju providers — such as Google, AWS, and Azure — require credentials before Concierge can bootstrap a controller on them. Concierge accepts these through the `credentials-file` field on the provider.

You don't need to provide credentials for LXD, Canonical Kubernetes, or MicroK8s.

## Expected file format

Concierge expects the file to contain **only** the credential body, without the three enclosing keys (`credentials:`, the cloud name, and the credential name) that Juju uses in `~/.local/share/juju/credentials.yaml`.

For example, a Google credential file:

```yaml
auth-type: oauth2
client-email: juju-gce-1-sa@example.iam.gserviceaccount.com
client-id: "1234567891234"
private-key: |
  -----BEGIN PRIVATE KEY-----
  deadbeef
  -----END PRIVATE KEY-----
project-id: example
```

## Extract credentials from Juju

If you already have credentials in `~/.local/share/juju/credentials.yaml`, extract the block you need with `yq`:

```bash
yq -r '.credentials.google.mycred' \
  ~/.local/share/juju/credentials.yaml > google-creds.yaml
```

## Provide credentials to Concierge

In your config:

```yaml
providers:
  google:
    enable: true
    bootstrap: true
    credentials-file: /home/ubuntu/google-creds.yaml
```

Or on the command line:

```bash
sudo concierge prepare -c concierge.yaml \
  --google-credential-file /home/ubuntu/google-creds.yaml
```

Or through an environment variable:

```bash
export CONCIERGE_GOOGLE_CREDENTIAL_FILE=/home/ubuntu/google-creds.yaml
sudo concierge prepare -c concierge.yaml
```
