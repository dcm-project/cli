# Checkpoint: Topic 1 — CLI Framework & Entry Point

- **Branch:** `topic-1-cli-framework`
- **Commit:** `781a76b`
- **Date:** 2026-03-12
- **Status:** Complete

---

## Scope

Topic 1 implements the foundational CLI structure per spec section 4.1. It provides the Cobra-based command tree, global flags, entry point, version package, Makefile, and exit code handling. All subcommands are stubs — actual logic is deferred to Topics 4–8.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-CLI-020 | Root command `dcm` with global flags | Done |
| REQ-CLI-030 | Subcommand groups: `policy`, `catalog`, `version` | Done |
| REQ-CLI-040 | `catalog` subgroups: `service-type`, `item`, `instance` | Done |
| REQ-CLI-050 | Global flags: `--api-gateway-url`, `--output`/`-o`, `--timeout`, `--config`, `--tls-ca-cert`, `--tls-client-cert`, `--tls-client-key`, `--tls-skip-verify` | Done |
| REQ-CLI-060 | Exit codes: 0 success, 1 runtime, 2 usage | Done |
| REQ-CLI-070 | Entry point in `cmd/dcm/main.go` | Done |

### Tests Implemented (14 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U019 | Root command lists `policy`, `catalog`, `version` in help | Pass |
| TC-U020 | `catalog` lists `service-type`, `item`, `instance` in help | Pass |
| TC-U021 | All 9 global flags present in help output | Pass |
| TC-U022 | Exit code 0 on successful `dcm version` | Pass |
| TC-U023 | `UsageError` on missing args (8 table-driven entries) + unknown flags | Pass |

---

## Files Created

| File | Purpose |
|------|---------|
| `Makefile` | Build targets: build, test, test-e2e, fmt, vet, lint, clean, tidy |
| `go.mod` / `go.sum` | Go module (`github.com/dcm-project/cli`) with cobra, ginkgo, gomega |
| `tools.go` | Build tool dependency (ginkgo) |
| `cmd/dcm/main.go` | Entry point — calls `commands.Execute()` |
| `internal/version/version.go` | Build-time version info (ldflags: Version, Commit, BuildTime) |
| `internal/commands/root.go` | Root command, global flags, `UsageError` type, `ExactArgs` wrapper |
| `internal/commands/catalog.go` | `catalog` parent command |
| `internal/commands/version.go` | `version` subcommand |
| `internal/commands/policy.go` | `policy` group with create/list/get/update/delete stubs |
| `internal/commands/catalog_service_type.go` | `service-type` group with list/get stubs |
| `internal/commands/catalog_item.go` | `item` group with create/list/get/delete stubs |
| `internal/commands/catalog_instance.go` | `instance` group with create/list/get/delete stubs |
| `internal/commands/commands_suite_test.go` | Ginkgo test suite bootstrap |
| `internal/commands/root_test.go` | Tests for TC-U019 through TC-U023 |
| `.gitignore` | Ignores build artifacts, IDE files, OS files, Go test/tool outputs |
| `.gitattributes` | Collapses generated files (`go.sum`, `go.mod`) in GitHub diffs |
| `.golangci.yml` | golangci-lint configuration aligned with dcm-catalog-manager |
| `.github/workflows/ci.yaml` | CI workflow for running tests (uses shared workflows) |
| `.github/workflows/lint.yaml` | Lint workflow using golangci-lint (uses shared workflows) |
| `.github/workflows/check-clean-commits.yaml` | Clean commit check workflow (uses shared workflows) |

---

## Key Design Decisions

1. **`UsageError` wrapper type** — Cobra doesn't distinguish usage vs runtime errors in `Execute()` return values. A custom `UsageError` type wraps errors that should exit with code 2.

2. **`ExactArgs` wrapper** — Replaces `cobra.ExactArgs` to wrap arg validation errors as `UsageError`. Used by all commands requiring positional arguments.

3. **`SetFlagErrorFunc`** — Wraps flag parsing errors as `UsageError` on the root command.

4. **Shell completion disabled** — `CompletionOptions.DisableDefaultCmd = true` since autocompletion is out of scope for v1alpha1.

5. **Stubs only** — All subcommand `RunE` functions return `nil`. Configuration (Topic 2), output formatting (Topic 3), and actual command logic (Topics 4–8) are not implemented.

6. **golangci-lint** — Configuration aligned with dcm-catalog-manager. Lint issues fixed: unchecked fmt return values, unused parameters, missing package comments, gofumpt formatting.

7. **GitHub CI** — Three workflows (ci, lint, check-clean-commits) using shared workflows from `dcm-project/shared-workflows`, matching the pattern used by other DCM repositories.

---

## What's Next

Topics 1, 2, and 3 are independent. The next topics to implement are:

- **Topic 2: Configuration Management** — Viper integration, config file loading, env vars, precedence
- **Topic 3: Output Formatting** — Formatter interface, table/JSON/YAML rendering
- **Topic 8: Version Command** — Depends only on Topic 1 (already stubbed, needs full tests)
- **Topics 4–7** — Command implementations (depend on Topics 1, 2, 3)
