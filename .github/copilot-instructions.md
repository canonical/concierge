# Copilot Code Review Instructions

**For general work context, refer to the [AGENTS.md](../AGENTS.md) file at the repository root.**

This file provides specific guidance for code review in the concierge repository.

## Code Review Focus Areas

### 1. Testing Requirements

- **Unit Tests**: All new business logic must include table-driven unit tests
  - Use `system.NewMockSystem()` for testing code that interacts with the system
  - Test happy paths AND error conditions
  - Verify command execution using `system.ExecutedCommands`
  - Example pattern: See `internal/providers/lxd_test.go`

- **Integration Tests**: Provider changes and major features require spread tests
  - Add tests in the `tests/` directory
  - Ensure tests work with both `lxd:` and `github-ci:` backends
  - Test both `prepare` and `restore` operations

### 2. Error Handling

- **Always wrap errors** with context using `fmt.Errorf("context: %w", err)`
- **Check return values**: Never ignore errors from system operations
- **Fail early**: Validate inputs before executing expensive operations
- **Log at appropriate levels**:
  - `slog.Debug()` for verbose operational details
  - `slog.Info()` for user-facing progress messages
  - `slog.Error()` for failures (though errors should typically be returned)

### 3. System Interaction Patterns

- **Use the Worker interface**: All system commands must go through `system.Worker`
  - Direct use of `exec.Command()` is prohibited
  - Use `system.NewCommand()` to construct commands
  - Prefer `system.RunMany(w, ...)` helper for sequential independent operations

- **Exclusive operations**: Use `system.RunExclusive(w, cmd)` for operations requiring locks
  - Package installations (snap, apt)
  - State-modifying operations that can conflict

### 4. Concurrency and Performance

- **Use errgroups** for concurrent operations (see `plan.go`)
- **Parallel execution**: Snaps and debs should install concurrently
- **Sequential dependencies**: Juju bootstrap must wait for providers
- **Avoid blocking**: Don't introduce unnecessary sequential execution

### 5. Configuration Management

- **Respect override priority**:
  1. CLI flags (highest)
  2. Environment variables
  3. Config file
  4. Presets (lowest)

- **Channel handling**: Always check `config.Overrides.*Channel` before `config.*Channel`
- **Validate early**: Use plan validators for configuration errors (see `plan_validators.go`)

### 6. Provider Interface Compliance

All providers must implement the complete `Provider` interface:
- `Prepare()` - Install and configure the provider
- `Restore()` - Remove the provider
- `Bootstrap()` - Whether to bootstrap Juju
- `CloudName()` - Juju cloud name
- `Credentials()` - Juju credentials map
- `ModelDefaults()` - Juju model-defaults
- `BootstrapConstraints()` - Juju bootstrap-constraints
- `Name()` - Concierge provider name

### 7. Security Considerations

- **Command injection**: Never use string concatenation to build commands
  - Use `system.NewCommand(cmd, []string{arg1, arg2})` pattern
  - Each argument must be a separate string in the slice

- **Privilege escalation**: The binary runs as root/sudo
  - Minimize operations requiring root
  - Use `system.User()` to get the non-root user for usermod operations
  - Be cautious with file permissions and ownership

- **Credentials handling**:
  - Never log credentials or sensitive data
  - Credentials files should be read-only by owner
  - Clear credential data from memory when no longer needed

### 8. Go Style and Idioms

- **Exported vs unexported**: Only export what's necessary for the public API
- **Struct initialization**: Use named fields in struct literals
- **Interface satisfaction**: Ensure types implement interfaces at compile time
- **nil checks**: Check for nil before dereferencing pointers
- **String formatting**: Use `fmt.Sprintf()` for complex strings, avoid concatenation

### 9. Common Pitfalls

- **Snap channel changes**: Refreshing snaps to different channels may require stopping them first
  - See `lxd.go:workaroundRefresh()` for the pattern

- **Race conditions**: The runtime config is cached to `~/.cache/concierge/concierge.yaml`
  - `restore` must read this cached config, not a new config file

- **File paths**: Use `system.Worker.UserHomeDir()` instead of hardcoding `/home/user`

- **Test isolation**: Mock system must be reset between test cases

### 10. Documentation Standards

- **Public functions**: Must have godoc comments starting with the function name
- **Complex logic**: Inline comments explaining the "why", not the "what"
- **Configuration changes**: Update the README.md schema section
- **New providers**: Update the "Adding New Providers" section in AGENTS.md

## Review Checklist

When reviewing a PR, verify:

- [ ] Unit tests exist and cover new/modified code paths
- [ ] Integration tests added for provider or major feature changes
- [ ] Errors are properly wrapped with context
- [ ] All system operations use the `system.Worker` interface
- [ ] Configuration override priority is respected
- [ ] No command injection vulnerabilities
- [ ] No hardcoded file paths or user assumptions
- [ ] Exported functions have godoc comments
- [ ] Code follows Go idioms and project patterns
- [ ] Changes don't introduce unnecessary complexity
- [ ] Concurrent operations use proper synchronization
- [ ] Mock system is used correctly in tests
