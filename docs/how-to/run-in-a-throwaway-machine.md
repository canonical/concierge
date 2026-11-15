---
myst:
  html_meta:
    description: How to get a throwaway Multipass or LXD virtual machine that can run Concierge's dev preset, with an optional shared project directory.
---

(how-to-run-in-a-throwaway-machine)=
# How to run Concierge in a throwaway machine

{ref}`explanation-what-is-concierge` recommends using Concierge in virtual machines, CI runners, and dedicated test hosts rather than on your daily workstation. This guide shows how to get a throwaway virtual machine with Multipass or LXD, and then prepare it with Concierge.

Concierge doesn't create the machine itself. Use whichever of these tools you already have.

## Multipass

Launch a VM and open a shell in it:

```bash
multipass launch --name concierge-dev --cpus 4 --memory 8G --disk 20G noble
multipass shell concierge-dev
```

Then {ref}`prepare the machine <how-to-run-in-a-throwaway-machine-prepare>`.

## LXD

Launch an LXD virtual machine:

```bash
lxc launch ubuntu:24.04 concierge-dev --vm -c limits.cpu=4 -c limits.memory=8GiB -d root,size=20GiB
```

Use a virtual machine, not a container. The `dev` preset installs Kubernetes, and Canonical Kubernetes [recommends a VM over a privileged container](https://documentation.ubuntu.com/canonical-kubernetes/latest/snap/howto/install/lxd/). A container also needs its own loop devices for Kubernetes storage, and on a host with many snaps installed there are often none free.

Wait for the VM to finish booting:

```bash
lxc exec concierge-dev -- cloud-init status --wait
```

If this reports that the LXD VM agent isn't running, wait a few seconds and run it again.

### Share a project directory

This step is optional. Skip it if you don't need files from the host inside the VM.

Files in a shared directory keep their host owner. If your user ID on the host isn't 1000, change the `ubuntu` user in the VM to match your host user. Do this before you open a shell as `ubuntu`:

```bash
lxc exec concierge-dev -- usermod -u "$(id -u)" ubuntu
lxc exec concierge-dev -- groupmod -g "$(id -g)" ubuntu
lxc exec concierge-dev -- chown -R "$(id -u):$(id -g)" /home/ubuntu
```

Then share the current directory into the VM:

```bash
lxc config device add concierge-dev project disk source="$(pwd)" path=/home/ubuntu/project
```

### Open a shell

Open a shell in the VM as the `ubuntu` user:

```bash
lxc exec concierge-dev -- su -l ubuntu
```

(how-to-run-in-a-throwaway-machine-prepare)=
## Prepare the machine

Inside the VM, install Concierge and run it as usual:

```bash
sudo snap install --classic concierge
sudo concierge prepare -p dev
```

See {ref}`how-to-set-up-a-machine` for other presets, and {ref}`reference-presets` for what each one installs.

When you've finished with the machine, delete it with `multipass delete concierge-dev` or `lxc delete --force concierge-dev`.
