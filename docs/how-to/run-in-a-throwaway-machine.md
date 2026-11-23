---
myst:
  html_meta:
    description: How to get a throwaway Multipass or LXD virtual machine that can run Concierge's dev preset, with an optional shared project directory.
---

(how-to-run-in-a-throwaway-machine)=
# How to run Concierge in a throwaway machine

{ref}`explanation-what-is-concierge` recommends using Concierge in virtual machines, CI runners, and dedicated test hosts rather than on your daily workstation. This guide shows how to get a throwaway virtual machine with Multipass or LXD, and then prepare it with Concierge.

Concierge doesn't create the machine itself. Use whichever of these tools you already have.

The commands below give the machine 4 CPUs, 8 GiB of memory, and 20 GiB of disk. This is enough to prepare the machine and deploy a small charm. Give the VM more memory if you'll deploy anything heavier.

## Multipass

Launch a VM and open a shell in it:

```bash
multipass launch --name concierge-dev --cpus 4 --memory 8G --disk 20G noble
multipass shell concierge-dev
```

Optionally, share the current directory into the VM. Run this on the host:

```bash
multipass mount . concierge-dev:/home/ubuntu/project
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

(how-to-run-in-a-throwaway-machine-share)=
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

## LXD container

An LXD container starts faster than a VM and uses less memory, but Kubernetes needs access to the host that LXD doesn't give a container by default. The container below is privileged and runs without AppArmor confinement. Root in this container is root on the host, so it doesn't isolate your workstation the way a VM does.

Launch the container:

```bash
lxc launch ubuntu:24.04 concierge-dev \
  -c limits.cpu=4 -c limits.memory=8GiB \
  -c security.nesting=true -c security.privileged=true \
  -c linux.kernel_modules=br_netfilter,ip6_tables,ip6table_raw,ip_tables,iptable_raw,ip_vs,ip_vs_rr,ip_vs_sh,ip_vs_wrr,netlink_diag,nf_conntrack,nf_nat,overlay,vxlan,xt_socket \
  -c raw.lxc='lxc.apparmor.profile=unconfined
lxc.mount.auto=proc:rw sys:mixed
lxc.cgroup2.devices.allow=a
lxc.cap.drop='
```

Pass `/dev/kmsg` through to the container. The Kubernetes node agent reads it and won't start without it:

```bash
lxc config device add concierge-dev kmsg unix-char source=/dev/kmsg path=/dev/kmsg
```

Give the container loop devices. Kubernetes storage attaches each volume as a loop device, and a container has none of its own:

```bash
lxc config device add concierge-dev loop-control unix-char source=/dev/loop-control path=/dev/loop-control
for i in $(seq 0 127); do
  lxc config device add concierge-dev "loop$i" unix-block path="/dev/loop$i" major=7 minor="$i"
done
```

The loop devices are shared with the host, and every snap on the host uses one. If `losetup --list | wc -l` on the host is close to 128, pass a higher range.

Wait for the container to finish booting, then bring its packages up to date:

```bash
lxc exec concierge-dev -- cloud-init status --wait
lxc exec concierge-dev -- sh -c 'apt-get update && apt-get -y upgrade'
```

If a package upgrade restarts snapd while Concierge is installing snaps, the snaps installed after the restart can fail to start. Upgrading first avoids this.

To share a project directory, follow {ref}`the LXD VM steps <how-to-run-in-a-throwaway-machine-share>`. They work the same way for a container. Then open a shell as the `ubuntu` user:

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

When you've finished with the machine, delete it with `multipass delete --purge concierge-dev` or `lxc delete --force concierge-dev`.
