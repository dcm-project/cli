# Checkpoint: Topic 2 — Configuration Management

- **Branch:** `topic-2-configuration`
- **Base:** `topic-1-cli-framework` (commit `5a59418`)
- **Date:** 2026-03-12
- **Status:** Complete (uncommitted)

---

## Scope

Topic 2 implements configuration management per spec section 4.2. It provides Viper-based configuration loading with file persistence, environment variable overrides, and CLI flag overrides following the required precedence order.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-CFG-010 | Load config from `~/.dcm/config.yaml` by default | Done |
| REQ-CFG-020 | Config path overridable via `--config` flag or `DCM_CONFIG` env var | Done |
| REQ-CFG-030 | Support all `DCM_*` environment variables | Done |
| REQ-CFG-040 | Precedence: CLI flags > env vars > config file > defaults | Done |
| REQ-CFG-050 | Built-in defaults for all config keys | Done |
| REQ-CFG-060 | Uses Viper for configuration management | Done |
| REQ-CFG-070 | Missing config file does not cause failure | Done |

### Tests Implemented (14 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U001 | Config file loading (`api-gateway-url` from YAML) | Pass |
| TC-U002 | Env var overrides config file value | Pass |
| TC-U003 | CLI flag overrides env var and config file | Pass |
| TC-U004 | Built-in defaults for all 7 config fields | Pass |
| TC-U005 | Missing config file does not cause failure | Pass |
| TC-U006 | Custom config file path via `--config` flag | Pass |
| TC-U007 | Custom config file path via `DCM_CONFIG` env var | Pass |
| TC-U008 | All 7 environment variables supported (table-driven) | Pass |

---

## Files Created

| File | Purpose |
|------|---------|
| `internal/config/config.go` | `Config` struct, `Load()` with Viper, context helpers (`WithConfig`, `FromContext`, `FromCommand`) |
| `internal/config/config_suite_test.go` | Ginkgo test suite bootstrap |
| `internal/config/config_test.go` | Tests for TC-U001 through TC-U008 |

## Files Modified

| File | Change |
|------|--------|
| `internal/commands/root.go` | Added `PersistentPreRunE` to load config and store in command context |
| `go.mod` / `go.sum` | Added `github.com/spf13/viper` dependency |

---

## Key Design Decisions

1. **`config.Load(cmd *cobra.Command)`** — Takes the cobra command to inspect parsed flags. Only flags marked as `Changed` (explicitly set by user) are bound to Viper, preventing flag defaults from overriding env vars or config file values.

2. **Context-based config propagation** — The root command's `PersistentPreRunE` calls `config.Load`, then stores the result in the command context via `config.WithConfig`. Subcommands retrieve it with `config.FromCommand(cmd)`.

3. **Config file path resolution** — Checks `--config` flag first (if changed), then `DCM_CONFIG` env var, then falls back to `~/.dcm/config.yaml`.

4. **Flag-to-config key mapping** — The `--output` flag maps to config key `output-format` via an explicit `flagToKey` map in `bindFlags`.

5. **Missing config file tolerance** — `viper.ReadInConfig` errors are suppressed for `ConfigFileNotFoundError` and `os.IsNotExist`, satisfying REQ-CFG-070.

6. **Test isolation** — Tests clear all `DCM_*` env vars in `BeforeEach`, use `GinkgoT().Setenv()` for auto-restore, and point `--config` to temp files to avoid loading the developer's `~/.dcm/config.yaml`.

---

## What's Next

- **Topic 3: Output Formatting** — Formatter interface, table/JSON/YAML rendering (independent of Topic 2)
- **Topic 8: Version Command** — Depends only on Topic 1 (already stubbed)
- **Topics 4–7** — Command implementations (depend on Topics 1, 2, 3)
