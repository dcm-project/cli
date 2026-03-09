# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project Overview

DCM CLI (`dcm`) is a Go-based command-line tool for interacting with the DCM (Data Center Management) control plane. It communicates through the API Gateway (KrakenD on port 9080) to reach the PolicyManager and CatalogManager backends. The CLI uses generated clients from `policy-manager/pkg/client` and `catalog-manager/pkg/client` (oapi-codegen generated) as Go module dependencies.

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
  - `policy.go`: Policy CRUD commands
  - `catalog_service_type.go`: Service type list/get commands
  - `catalog_item.go`: Catalog item CRUD commands
  - `catalog_instance.go`: Catalog instance create/list/get/delete commands
  - `version.go`: Version display command

- **internal/version/**: Build-time version info injected via ldflags

- **test/e2e/**: E2E tests with `e2e` build tag (`//go:build e2e`)

- **tools.go**: Build tool dependencies (ginkgo)

## Testing

The project uses Ginkgo as the test framework with Gomega matchers. HTTP-level mocking uses `net/http/httptest`.

E2E tests live under `test/e2e/` and use the `e2e` build tag (`//go:build e2e`). They require a live DCM stack with `DCM_API_GATEWAY_URL` set.

## Key Conventions

1. **Cobra commands**: Each resource group (policy, catalog service-type, catalog item, catalog instance) has its own file with create/list/get/update/delete subcommands.

2. **Generated clients**: Import `github.com/dcm-project/policy-manager/pkg/client` and `github.com/dcm-project/catalog-manager/pkg/client`. No hand-written HTTP client code.

3. **Configuration precedence**: CLI flags > environment variables (`DCM_API_GATEWAY_URL`, `DCM_OUTPUT_FORMAT`, `DCM_TIMEOUT`, `DCM_CONFIG`) > config file (`~/.dcm/config.yaml`) > built-in defaults.

4. **Output formatting**: All commands support `--output/-o` flag with `table` (default), `json`, and `yaml` formats.

5. **Input files**: Resource creation and updates use `--from-file` flag accepting YAML or JSON files.

6. **Error handling**: API errors follow RFC 7807 Problem Details format. Exit code 0 for success, 1 for runtime errors, 2 for usage errors.

7. **Version injection**: Build-time ldflags set `internal/version.Version`, `internal/version.Commit`, `internal/version.BuildTime`.

8. **Commit conventions**: All commit messages must include a `Co-Authored-By:` line. The `git commit` command must always use the `--signoff` flag (e.g., `git commit --signoff`).
