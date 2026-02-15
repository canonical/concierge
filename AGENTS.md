# AGENTS.md

This file provides guidance to AI agents when working with code in this repository.

## Project Overview

`concierge` is a Go utility for provisioning charm development and testing machines. It installs "craft" tools (charmcraft, snapcraft, rockcraft), configures providers (LXD, MicroK8s, K8s, Google Cloud), and bootstraps Juju controllers onto those providers.

## Build and Test Commands

```bash
# Build the binary
go build

# Build a snapshot release with goreleaser (output in ./dist)
goreleaser build --clean --snapshot

# Run directly
go run .

# Run unit tests
go test ./...

# Run all integration tests (requires LXD)
spread -v lxd:

# Run specific integration test
spread -v lxd:ubuntu-24.04:tests/juju-model-defaults

# Run integration tests on local machine
spread -v github-ci:
```

Note: The binary must be run with `sudo` for most operations since it installs system packages and configures providers.

## Architecture

### Core Execution Flow

1. **Command Layer** (`cmd/`): Cobra commands (`prepare`, `restore`, `status`) parse flags and load configuration
2. **Manager** (`internal/concierge/manager.go`): Orchestrates the overall execution flow
3. **Plan** (`internal/concierge/plan.go`): Represents the complete set of actions to execute
4. **Handlers**: Execute actions concurrently using errgroups

### Package Structure

- **`cmd/`**: Command-line interface definitions
  - `prepare.go`: Provisions the machine according to config
  - `restore.go`: Reverses the provisioning process
  - `status.go`: Reports concierge status on the machine

- **`internal/concierge/`**: Core orchestration logic
  - `manager.go`: Creates plans, records runtime config to `~/.cache/concierge/concierge.yaml`
  - `plan.go`: Builds execution plan from config, runs providers/packages/juju in sequence
  - `plan_validators.go`: Validates plans before execution

- **`internal/config/`**: Configuration management
  - `config.go`: Loads config from files, presets, or flags
  - `presets.go`: Defines built-in presets (`dev`, `k8s`, `microk8s`, `machine`, `crafts`)
  - `overrides.go`: Handles CLI flags and environment variable overrides

- **`internal/providers/`**: Provider implementations
  - `providers.go`: Provider interface definition
  - Each provider (LXD, MicroK8s, K8s, Google) implements: `Prepare()`, `Restore()`, `Bootstrap()`, `CloudName()`, `Credentials()`, `ModelDefaults()`, `BootstrapConstraints()`

- **`internal/packages/`**: Package handlers
  - `snap_handler.go`: Installs/removes snaps
  - `deb_handler.go`: Installs/removes apt packages

- **`internal/juju/`**: Juju controller management
  - Bootstraps Juju controllers onto configured providers
  - Applies model-defaults and bootstrap-constraints

- **`internal/system/`**: System abstraction layer
  - `interface.go`: Worker interface for system operations
  - `runner.go`: Executes shell commands with retries and locking
  - `snap.go`: Snap-specific utilities

### Key Design Patterns

1. **Action Pattern**: Operations implement `Prepare()` and `Restore()` methods. The `DoAction()` function calls the appropriate method based on the action string.

2. **Concurrent Execution**: Plan execution uses `errgroup` to run independent operations concurrently:
   - Snaps and Debs are installed in parallel
   - Providers are prepared/restored in parallel
   - Juju bootstrap runs after all providers are ready

3. **Worker Interface**: All system operations go through the `system.Worker` interface, enabling:
   - Testability via mock implementations
   - Consistent command execution with options (e.g. `system.Exclusive()`, `system.WithRetries(d)`)
   - Safe file operations via helper functions (e.g. `system.WriteHomeDirFile`, `system.ReadHomeDirFile`)

4. **Runtime Config Caching**: During `prepare`, the merged configuration (including all overrides) is saved to `~/.cache/concierge/concierge.yaml`. The `restore` command reads this file to undo exactly what was provisioned.

5. **Channel Overrides**: Snap channels specified in config can be overridden by CLI flags or environment variables (e.g., `--juju-channel` or `CONCIERGE_JUJU_CHANNEL`).

## Configuration Priority

Configuration is loaded in this order (later overrides earlier):
1. Preset (if specified with `-p`)
2. Config file (`concierge.yaml` or path from `-c`)
3. Environment variables (e.g., `CONCIERGE_JUJU_CHANNEL`)
4. CLI flags (e.g., `--juju-channel`)

## Testing Strategy

- **Unit Tests**: Standard Go tests in `*_test.go` files. Most business logic is tested via unit tests.
- **Integration Tests**: The `spread` framework runs full end-to-end tests in LXD VMs or on pre-provisioned machines. Tests are in the `tests/` directory.
- **Mock System**: `internal/system/mock_system.go` provides test doubles for system operations.

## Adding New Providers

To add a new provider:
1. Create a new file in `internal/providers/` (e.g., `newprovider.go`)
2. Implement the `Provider` interface with all required methods
3. Add the provider name to `SupportedProviders` in `providers.go`
4. Update `NewProvider()` factory function to instantiate your provider
5. Add provider configuration to `internal/config/config.go` struct
6. Add integration tests in `tests/`
