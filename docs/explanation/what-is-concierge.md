---
myst:
  html_meta:
    description: What Concierge is designed to do — make charm-development machine setup declarative and reproducible — what it deliberately isn't, and why restore is a strict opposite of prepare.
---

(explanation-what-is-concierge)=
# What Concierge is for

Charm development needs a familiar-looking machine: the right craft tools (`charmcraft`, `snapcraft`, `rockcraft`), the right providers (LXD, Kubernetes), and a Juju controller bootstrapped and ready to use. Getting a machine into that shape by hand takes many commands and a lot of tacit knowledge, and every developer's machine ends up subtly different.

Concierge exists to make that setup **declarative** and **reproducible**. You describe the machine you want in a YAML file — or pick one of the {ref}`built-in presets <reference-presets>` — and Concierge does the work.

## What Concierge is not

Concierge is a **provisioner**, not a runtime. Once it has prepared a machine, you interact with the tools it installed (Juju, Charmcraft, snapd, …) directly. Concierge doesn't sit in the loop.

Concierge is also not a charm-development tutorial. Learning how to develop charms is covered elsewhere, such as in the {external+ops:doc}`Ops documentation <index>`.

## Why declarative

A `concierge.yaml` file captures exactly what a machine needs to be a charm-development environment, in a form that can be committed to version control, diffed, and shared between developers and CI.

(explanation-prepare-restore-opposites)=
## `prepare` and `restore` are opposites, not deltas

Concierge has two mirror-image commands: `prepare` provisions the machine according to your configuration; `restore` undoes what `prepare` did.

`concierge restore` does not observe the machine and revert changes it can detect. It computes what `prepare` **would install** from the same configuration, and removes exactly that set. This makes restore predictable and cheap, but it has a consequence:

```{warning}
If the machine already had one of Concierge's snaps, packages, or configuration files before you ran `prepare`, `restore` will remove it anyway.
```

Concierge doesn't attempt to restore to a snapshot of the machine's pre-existing state.

## Where Concierge fits

Because restore is all-or-nothing, Concierge is intended for **throwaway** machines. Use Concierge in virtual machines, CI runners, and dedicated test hosts. Don't use Concierge in your daily workstation.

Concierge is a natural fit for CI pipelines, which need to leave the runner in a clean state between jobs.
