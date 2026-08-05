---
myst:
  html_meta:
    description: What Concierge is designed to do — make charm-development machine setup declarative and reproducible — what it deliberately isn't, and why restore is a strict opposite of prepare.
---

(explanation-what-is-concierge)=
# What Concierge is for

Charm development needs a familiar-looking machine: the right craft tools (`charmcraft`, `snapcraft`, `rockcraft`), the right providers (LXD, Kubernetes), and a Juju controller bootstrapped and ready to use. Getting a machine into that shape by hand takes many commands and a lot of tacit knowledge, and every developer's machine ends up subtly different.

Concierge exists to make that setup **declarative** and **reproducible**. You describe the machine you want in a YAML file — or pick one of the [built-in presets](../reference/presets) — and Concierge does the work.

## What Concierge is not

Concierge is a **provisioner**, not a runtime. Once it has prepared a machine, you interact with the tools it installed (Juju, Charmcraft, snapd, …) directly. Concierge doesn't sit in the loop.

Concierge is also not a **charm-development tutorial**. It gets you a machine that is ready to develop charms; learning how to develop charms happens elsewhere (see the [Ops documentation](https://canonical.com/juju/docs/ops/latest/) for that).

## Why declarative

A `concierge.yaml` file captures exactly what a machine needs to be a charm-development environment, in a form that can be committed to version control, diffed, and shared between developers and CI. Two developers using the same config get the same setup; a CI job using that config gets the same setup as a developer's laptop.

## `prepare` and `restore` are opposites, not deltas

Concierge has two mirror-image commands: `prepare` provisions the machine according to your configuration; `restore` undoes what `prepare` did.

`concierge restore` does not observe the machine and revert changes it can detect. It computes what `prepare` **would install** from the same configuration, and removes exactly that set. This makes restore predictable and cheap, but it has a consequence:

:::{warning}
If the machine already had one of Concierge's snaps, packages, or configuration files before you ran `prepare`, `restore` will remove it anyway.
:::

The alternative — a restore that tries to preserve pre-existing state — would need to snapshot every changed file, every installed package, and every Juju controller before touching them, and remember which of them existed beforehand. That's a substantial amount of machinery to build and maintain, and it hides bugs when the snapshot and reality drift apart.

## Where Concierge fits

Because restore is all-or-nothing, Concierge is intended for **throwaway** machines — VMs, CI runners, dedicated test hosts — not for the workstation you use for everything else. That constraint is also what makes Concierge a natural fit for CI pipelines, which need to leave the runner in a clean state between jobs.
