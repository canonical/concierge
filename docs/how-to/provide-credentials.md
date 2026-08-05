---
myst:
  html_meta:
    description: How to give Concierge the credentials it needs to bootstrap Juju controllers on clouds like Google, AWS, and Azure using the credentials-file field.
---

(how-to-provide-credentials)=
# Provide cloud credentials

Some Juju providers — such as Google, AWS, and Azure — require credentials before Concierge can bootstrap a controller on them. Concierge accepts these through the `credentials-file` field on the provider.

Providers with built-in Juju credentials (LXD, MicroK8s, K8s) do not need this.

## Expected file format

Concierge expects the file to contain **only** the credential body, without the surrounding `credentials:` / cloud / credential-name keys that Juju uses in `~/.local/share/juju/credentials.yaml`.

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

## Extract from existing Juju credentials

If you already have credentials in `~/.local/share/juju/credentials.yaml`, extract the block you need with `yq`:

```bash
yq -r '.credentials.google.mycred' \
  ~/.local/share/juju/credentials.yaml > google-creds.yaml
```

## Reference the file from your config

```yaml
providers:
  google:
    enable: true
    bootstrap: true
    credentials-file: /home/ubuntu/google-creds.yaml
```

You can also point Concierge at the file from the command line:

```bash
sudo concierge prepare -c concierge.yaml \
  --google-credential-file /home/ubuntu/google-creds.yaml
```

Or through an environment variable:

```bash
export CONCIERGE_GOOGLE_CREDENTIAL_FILE=/home/ubuntu/google-creds.yaml
sudo concierge prepare -c concierge.yaml
```
