# Contributing to and developing Concierge

We welcome contributions to Concierge!

Before working on changes, please consider [opening an issue](https://github.com/canonical/concierge/issues) explaining your use case. If you would like to chat with us about your use cases or proposed implementation, you can reach us on [Matrix](https://matrix.to/#/#charmhub-charmdev:ubuntu.com) or [Discourse](https://discourse.charmhub.io/).

## Getting started

This project uses [goreleaser](https://goreleaser.com/) to build and release, and `spread` for
integration testing,

You can get started by just using Go, or with `goreleaser`:

```shell
# Clone the repository
git clone https://github.com/canonical/concierge
cd concierge

# Build/run with Go
go run .

# Run the unit tests
go test ./...

# Build a snapshot release with goreleaser (output in ./dist)
goreleaser build --clean --snapshot
```

## Testing

Most of the code within tries to call a shell command, or manipulate the system in some way, which
makes unit testing much more awkward. As a result, the majority of the testing is done with
[`spread`](https://github.com/canonical/spread).

Currently, there are two supported backends - tests can either be run in LXD virtual machines, or
on a pre-provisioned server (such as a GitHub Actions runner or development VM).

To show the available integration tests, you can:

```bash
$ spread -list lxd:
lxd:ubuntu-24.04:tests/extra-debs
lxd:ubuntu-24.04:tests/extra-packages-config-file
lxd:ubuntu-24.04:tests/extra-snaps
# ...
```

From there, you can either run all of the tests, or a selection:

```bash
# Run all of the tests
$ spread -v lxd:
# Run a particular test
$ spread -v lxd:ubuntu-24.04:tests/juju-model-defaults
```

To run any of the tests on a locally provisioned machine, use the `github-ci` backend, for example:

```bash
# List available tests
$ spread --list github-ci:
# Run all of the tests
$ spread -v github-ci:
# Run a particular test
$ spread -v github-ci:ubuntu-24.04:tests/juju-model-defaults
```

Proposed changes should include tests: almost always spread tests, and where possible also unit tests.

## Pull requests

Changes are proposed as [pull requests on GitHub](https://github.com/canonical/concierge/pulls).

Pull requests should have a short title that follows the [conventional commit style](https://www.conventionalcommits.org/en/) using one of these types:

- chore
- ci
- docs
- feat
- fix
- perf
- refactor
- revert
- test

Some examples:

- feat: add the ability to bootstrap EKS
- fix!: correct the type of the channel argument
- docs: clarify how to use custom configuration files

We consider Concierge too small a project to use scopes, so we don't use them.

Note that the commit messages to the PR's branch do not need to follow the conventional commit format, as these will be squashed into a single commit to `main` using the PR title as the commit message.

To help us review your changes, please rebase your pull request onto the `main` branch before you request a review. If you need to bring in the latest changes from `main` after the review has started, please use a merge commit.

## AI

You're welcome to submit pull requests that are partly or entirely generated using generative AI tools. However, you must review the code yourself before moving the PR out of draft -- by submitting the PR, you are claiming personal responsibility for its quality and suitability. If you are not capable of reviewing the PR (for example, if you are not fluent in Go, or are not familiar with Concierge), please do not submit the PR (maybe you'd like to open an issue instead). PRs that are clearly (co-)authored by tools will be closed without review unless there is a human author that claims responsibility for the PR.

Please do not use tools (such as GitHub Copilot) to provide PR reviews. The Charm Tech team also has access to these tools, and will use them when appropriate.

## Creating a release

To release, simply create a new release in GitHub.

1. [Draft a new GitHub release](https://github.com/canonical/concierge/releases/new)
2. Enter the version tag (for example `v1.28.0`) and select "Create new tag: on publish".
3. Enter a release title: include the version tag and a short summary of the release.
4. Write release notes - start with the draft provided by GitHub, drop the `by @author` credit for anyone in the Charm Tech team (including Copilot and other AI users), and include a short summary of the new features and bug fixes at the top. Leave the link to the full list of commits and any acknowledgement of new contributors.
5. Click "Publish release".
6. Monitor the release [GitHub Action](https://github.com/canonical/concierge/actions) and check that the [snap](https://snapcraft.io/concierge) is uploaded correctly (it will have been published to all risks, including `stable`)
7. Run the appropriate security scan and SBOM generation, as described in the Canonical library's SSDLC process. Upload artifacts to the [SSDLC Concierge folder in Drive](https://drive.google.com/drive/folders/1RtAn7x0EX97C6eV66xs74Pwth3KW7NHI). Open the artifact and verify that the security scan has not found any vulnerabilities.
