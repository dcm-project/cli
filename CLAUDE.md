# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DCM CLI (`dcm`) is a Go-based command-line tool for interacting with the DCM (Data Center Management) control plane. It communicates directly with the control-plane monolith on port 8080. The CLI uses oapi-codegen generated clients from the [control-plane](https://github.com/dcm-project/control-plane/tree/main/pkg) repo as a Go module dependency.

Generated client packages:

- [pkg/policy/client](https://github.com/dcm-project/control-plane/tree/main/pkg/policy/client) — Policy Manager
- [pkg/catalog/client](https://github.com/dcm-project/control-plane/tree/main/pkg/catalog/client) — Catalog Manager
- [pkg/sp/client/resource_manager](https://github.com/dcm-project/control-plane/tree/main/pkg/sp/client/resource_manager) — SP Resource Manager
- [pkg/sp/client/provider](https://github.com/dcm-project/control-plane/tree/main/pkg/sp/client/provider) — SP Manager

## Build and Development Commands

```bash
# Build the binary
make build

# Run tests
make test

# Run a single test
go test -run TestName ./path/to/package

# Format code
make fmt

# Vet code
make vet

# Run linter
make lint

# Clean build artifacts
make clean

# Tidy dependencies
make tidy

# Run E2E tests (requires live DCM stack)
make test-e2e
```

## Architecture

### Directory Structure

- **cmd/dcm/**: Main entry point
  - `main.go`: Bootstrap and root command execution

- **internal/config/**: Configuration loading/saving
  - Manages `~/.dcm/config.yaml`, env vars, and flag overrides
  - Precedence: flags > env vars > config file > defaults

- **internal/output/**: Output formatting
  - Supports table, JSON, and YAML output formats
  - Implements `Formatter` interface

- **internal/commands/**: Cobra command definitions
  - `root.go`: Root command with global flags
  - `helpers.go`: Client constructors, HTTP/TLS helpers, input file parsing
  - `policy.go`: Policy CRUD commands
  - `catalog_service_type.go`: Service type list/get commands
  - `catalog_item.go`: Catalog item create/list/get/delete commands
  - `catalog_instance.go`: Catalog instance create/list/get/delete/rehydrate commands
  - `sp_resource.go`: SP resource list/get commands
  - `sp_provider.go`: SP provider list/get commands
  - `completion.go`: Shell completion
  - `version.go`: Version display command

- **internal/version/**: Build-time version info injected via ldflags

- **test/e2e/**: E2E tests with `e2e` build tag (`//go:build e2e`)

- **tools.go**: Build tool dependencies (ginkgo)

## Testing

The project uses Ginkgo as the test framework with Gomega matchers. HTTP-level mocking uses `net/http/httptest`.

E2E tests live under `test/e2e/` and use the `e2e` build tag (`//go:build e2e`). They require a live DCM stack with `DCM_CONTROL_PLANE_URL` set.

## Key Conventions

1. **Cobra commands**: Each resource group (policy, catalog service-type, catalog item, catalog instance, sp resource, sp provider) has its own file with subcommands. Policy supports create/list/get/update/delete. Catalog item and catalog instance do not support update. SP commands are read-only (list/get).

2. **Generated clients**: Import from `github.com/dcm-project/control-plane/pkg/...` (see links in Project Overview). Client constructors live in `helpers.go`. No hand-written HTTP client code.

3. **Configuration precedence**: CLI flags > environment variables (`DCM_CONTROL_PLANE_URL`, `DCM_OUTPUT_FORMAT`, `DCM_TIMEOUT`, `DCM_CONFIG`) > config file (`~/.dcm/config.yaml`) > built-in defaults.

4. **Output formatting**: All commands support `--output/-o` flag with `table` (default), `json`, and `yaml` formats.

5. **Input files**: Resource creation and updates use `--from-file` flag accepting YAML or JSON files.

6. **Error handling**: API errors follow RFC 7807 Problem Details format. Exit code 0 for success, 1 for runtime errors, 2 for usage errors.

7. **Version injection**: Build-time ldflags set `internal/version.Version`, `internal/version.Commit`, `internal/version.BuildTime`.

8. **Commit conventions**: All commit messages must include a `Co-Authored-By:` line. The `git commit` command must always use the `--signoff` flag (e.g., `git commit --signoff`).

9. **Documentation sync**: When changing behavior, flags, env vars, module paths, or architecture, update `README.md`, `CLAUDE.md`, `.ai/specs/`, and `.ai/test-plans/`. Leave `.ai/checkpoints/` unchanged — they are historical dev notes.
