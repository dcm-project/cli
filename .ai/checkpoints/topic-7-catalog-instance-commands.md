# Checkpoint: Topic 7 — Catalog Instance Commands

- **Branch:** `topic-7-catalog-instance-commands`
- **Base:** `topic-6-catalog-item-commands` (commit `86a6fec`)
- **Commit:** `a4e7ec7`
- **Date:** 2026-03-13
- **Status:** Complete

---

## Scope

Topic 7 implements the `dcm catalog instance` command group with subcommands (`create`, `list`, `get`, `delete`) per spec section 4.7. Instances represent deployed catalog items. No update operation is supported for instances in v1alpha1. All commands use the generated Catalog Manager client.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-CIN-010 | `dcm catalog instance create` from YAML/JSON file via `--from-file` | Done |
| REQ-CIN-020 | Optional `--id` flag for client-specified instance ID | Done |
| REQ-CIN-030 | Display created instance in configured output format | Done |
| REQ-CIN-040 | `dcm catalog instance list` with `--catalog-item-id`, `--page-size`, `--page-token` | Done |
| REQ-CIN-050 | Display instances in configured output format | Done |
| REQ-CIN-060 | `dcm catalog instance get INSTANCE_ID` | Done |
| REQ-CIN-070 | `dcm catalog instance delete INSTANCE_ID` | Done |
| REQ-CIN-080 | Delete success message format: `Catalog item instance "<id>" deleted successfully.` | Done |
| REQ-CIN-090 | All commands use generated Catalog Manager client | Done |
| REQ-CIN-100 | `--from-file` required for `create` | Done |
| REQ-CIN-110 | Missing positional args → usage error (exit code 2) | Done |

### Tests Implemented (15 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U058 | Create instance from YAML file — POST `/api/v1alpha1/catalog-item-instances` | Pass |
| TC-U059 | Create with `--id my-instance` — passes `id=my-instance` query parameter | Pass |
| TC-U072 | Create without `--from-file` → UsageError (exit code 2) | Pass |
| TC-U073 | List instances — GET `/api/v1alpha1/catalog-item-instances`, displays results | Pass |
| TC-U074 | List with `--page-size 10` — passes `max_page_size=10` query parameter | Pass |
| TC-U075 | Get instance — GET `/api/v1alpha1/catalog-item-instances/my-instance` | Pass |
| TC-U076 | Get without INSTANCE_ID → UsageError (exit code 2) | Pass |
| TC-U077 | Delete instance — DELETE, displays success message | Pass |
| TC-U078 | Delete without INSTANCE_ID → UsageError (exit code 2) | Pass |
| TC-U079 | Table output columns: UID, DISPLAY NAME, CATALOG ITEM, RESOURCE ID, CREATED | Pass |
| TC-U112 | Empty list — table shows headers only; JSON shows empty `results` array | Pass |
| TC-U113 | Get non-existent instance — 404 RFC 7807 error formatted to stderr | Pass |
| TC-U114 | Delete non-existent instance — 404 RFC 7807 error formatted to stderr | Pass |
| TC-U115 | Create server error — 500 RFC 7807 error formatted to stderr | Pass |
| (extra) | List with `--catalog-item-id` — passes `catalog_item_id` query parameter | Pass |

---

## Files Created / Modified

| File | Change | Purpose |
|------|--------|---------|
| `internal/commands/catalog_instance.go` | Modified | Full implementation of `create`, `list`, `get`, `delete` commands with generated Catalog Manager client (added ~201 lines to the stub) |
| `internal/commands/catalog_instance_test.go` | Created | 15 Ginkgo test specs with httptest-based mocking |
| `.ai/checkpoints/topic-7-catalog-instance-commands.md` | Created | This checkpoint |

---

## Key Design Decisions

1. **Same patterns as previous command groups** — Create, list, get, and delete follow the identical patterns established in Topics 4, 5, and 6: generated client usage, `handleErrorResponse` for errors, `newFormatter` for output, `parseInputFileAs` for input parsing, `connectionError` for connection failures.

2. **Reuses `newCatalogClient` from helpers.go** — Per Topic 5's design, the catalog client constructor is shared across all catalog commands (service-type, item, instance).

3. **Table columns** — UID, DISPLAY NAME, CATALOG ITEM, RESOURCE ID, CREATED. The catalog item ID and resource ID are extracted from `spec.catalog_item_id` and `spec.resource_id` via the nested map access pattern.

4. **List response uses `results` field** — Consistent with the Catalog Manager's response format, same as service-type and catalog item lists.

5. **`--catalog-item-id` filter** — The `ListCatalogItemInstancesParams` includes a `CatalogItemId` field passed as the `catalog_item_id` query parameter. This goes beyond the spec's `--page-size`/`--page-token` to provide useful instance filtering.

6. **No update command** — Per spec section 4.7, no update operation is supported for instances in v1alpha1.

7. **Pagination parameter naming** — The Catalog Manager API uses `max_page_size` (not `page_size`) as the query parameter name, matching the generated client's `MaxPageSize` field.

---

## What's Next

- **Topic 8: Version Command** — Full tests (depends only on Topic 1; command already stubbed)
