# Checkpoint: Catalog Instance Rehydrate Command

- **Branch:** `catalog-instance-rehydrate`
- **Base:** `main` (commit `b0d7ba8`)
- **Date:** 2026-03-31
- **Status:** Complete

---

## Scope

Adds the `dcm catalog instance rehydrate` command, which triggers rehydration of
an existing catalog item instance. The command sends a POST request to
`/api/v1alpha1/catalog-item-instances/{id}:rehydrate` and displays the
rehydrated instance in the configured output format.

This feature depends on the rehydration-flow branch of the Catalog Manager fork
(`github.com/ygalblum/dcm-catalog-manager`), integrated via a `replace`
directive in `go.mod`.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-CIN-120 | `dcm catalog instance rehydrate INSTANCE_ID` triggers rehydration | Done |
| REQ-CIN-130 | Display rehydrated instance in configured output format | Done |
| REQ-CIN-110 (updated) | Missing positional arg for `rehydrate` → usage error (exit code 2) | Done |

### Tests Implemented (4 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U150 | Rehydrate instance — POST `/api/v1alpha1/catalog-item-instances/my-instance:rehydrate`, displays result | Pass |
| TC-U151 | Rehydrate without INSTANCE_ID → UsageError (exit code 2) | Pass |
| TC-U152 | Rehydrate non-existent instance — 404 RFC 7807 error formatted to stderr | Pass |
| TC-U153 | Rehydrate server error — 500 RFC 7807 error formatted to stderr | Pass |

---

## Files Created / Modified

| File | Change | Purpose |
|------|--------|---------|
| `go.mod` | Modified | Added `replace` directive to use `github.com/ygalblum/dcm-catalog-manager` fork with rehydration API |
| `go.sum` | Modified | Updated checksums for fork dependency |
| `internal/commands/catalog_instance.go` | Modified | Added `newCatalogInstanceRehydrateCommand()` and registered it in `newCatalogInstanceCommand()` |
| `internal/commands/catalog_instance_test.go` | Modified | Added 4 Ginkgo test specs for rehydrate (success, missing arg, not found, server error) |
| `.ai/specs/dcm-cli.spec.md` | Modified | Added REQ-CIN-120, REQ-CIN-130, AC-CIN-130/140/150 for rehydrate |
| `.ai/test-plans/dcm-cli-unit.test-plan.md` | Modified | Added TC-U150–TC-U153, updated coverage matrix |
| `.ai/checkpoints/catalog-instance-rehydrate.md` | Created | This checkpoint |

---

## Key Design Decisions

1. **Fork via `replace` directive** — The upstream `catalog-manager` module does not yet have the rehydrate API. A Go module `replace` directive points to the `ygalblum/dcm-catalog-manager` fork's `rehydration-flow` branch, keeping the import paths unchanged throughout the codebase.

2. **Same patterns as existing instance commands** — The rehydrate command follows the identical structure used by `get`: accepts a positional `INSTANCE_ID`, creates the catalog client, sends the request, and formats the response or error.

3. **POST with no request body** — The rehydrate API takes no request body; only the instance ID in the URL path is required. The generated client's `RehydrateCatalogItemInstance` method handles this.

4. **200 OK response** — Unlike `create` (201) or `delete` (204), the rehydrate endpoint returns 200 with the updated instance body, which is decoded and displayed via `FormatOne`.
