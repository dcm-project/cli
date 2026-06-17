# Checkpoint: SP Resource — show-deleted Support

- **Branch:** `add-show-deleted-sp-resource`
- **Base:** `main` (commit `3f7b012`)
- **Date:** 2026-04-09
- **Status:** Complete

---

## Scope

Adds `--show-deleted` flag support to `dcm sp resource list` and
`dcm sp resource get` commands. When set, the flag passes `show_deleted=true` as
a query parameter to the Service Provider Resource Manager API and adds a
`DELETION STATUS` column to the table output showing the `deletion_status` field
from the response.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-SPR-010 (updated) | Added `--show-deleted` to list flags | Done |
| REQ-SPR-035 (new) | `dcm sp resource get` supports optional `--show-deleted` flag | Done |
| REQ-SPR-060 (new) | `--show-deleted` passes `show_deleted=true` query parameter on list and get | Done |
| REQ-SPR-070 (new) | `--show-deleted` adds `DELETION STATUS` column to table output | Done |

### Tests Implemented (4 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U154 | List with `--show-deleted` — passes `show_deleted=true`, displays `DELETION STATUS` column with `PENDING` | Pass |
| TC-U155 | List without `--show-deleted` — does not send `show_deleted`, no `DELETION STATUS` column | Pass |
| TC-U156 | Get with `--show-deleted` — passes `show_deleted=true`, displays `DELETION STATUS` column with `PENDING` | Pass |
| TC-U157 | Get without `--show-deleted` — does not send `show_deleted`, no `DELETION STATUS` column | Pass |

---

## Files Created / Modified

| File | Change | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Modified | Bumped control-plane dependency for `ShowDeleted` on `ListInstancesParams` and `GetInstanceParams` |
| `internal/commands/helpers.go` | Modified | SP provider client import: `control-plane/pkg/sp/client/provider` |
| `internal/commands/sp_provider.go` | Modified | SP provider API types: `control-plane/api/sp/v1alpha1/provider` |
| `internal/commands/sp_resource.go` | Modified | Added `spResourceWithDeletedTableDef`, `--show-deleted` flag on list and get, conditional table def selection, `ShowDeleted` param passing, updated `GetInstance` call to include `GetInstanceParams` |
| `internal/commands/sp_resource_test.go` | Modified | Added `sampleDeletedSPResourceResponse()` helper and 4 new test specs |
| `.ai/specs/dcm-cli.spec.md` | Modified | Added REQ-SPR-035/060/070, AC-SPR-035/036/045/046, updated table output section |
| `.ai/test-plans/dcm-cli-unit.test-plan.md` | Modified | Added TC-U154–TC-U157, updated coverage matrix and totals |
| `.ai/checkpoints/sp-resource-show-deleted.md` | Created | This checkpoint |

---

## Key Design Decisions

1. **Control-plane API update** — `ShowDeleted` on `ListInstancesParams`, the `GetInstanceParams` type (with `ShowDeleted`), and the updated `GetInstance` signature come from the control-plane SP resource manager OpenAPI. The CLI pins `github.com/dcm-project/control-plane` accordingly.

2. **SP client package layout** — Provider and resource manager clients live in separate control-plane packages: `pkg/sp/client/provider` and `pkg/sp/client/resource_manager`, with matching API type paths under `api/sp/v1alpha1/`.

3. **Conditional table definition** — Rather than always showing the `DELETION STATUS` column (which would be empty for most use cases), two table definitions are used: `spResourceTableDef` (default 4 columns) and `spResourceWithDeletedTableDef` (5 columns including `DELETION STATUS`). The flag value selects which definition is passed to the formatter.

4. **Flag defaults to false** — `--show-deleted` defaults to `false`, matching the API default. The query parameter is only sent when the flag is explicitly set to `true`, keeping default requests unchanged.
