# Checkpoint: Topic 9 — SP Resource Commands

- **Branch:** `topic-9-sp-resource-commands`
- **Base:** `topic-9-sp-resource-plans` (commit `62cdb9b`)
- **Date:** 2026-03-24
- **Status:** Complete

---

## Scope

Topic 9 implements the `dcm sp resource` command group with read-only subcommands (`list` and `get`) per spec section 4.9. SP resources are service type instances exposed by the SP Resource Manager API in control-plane. The CLI provides read-only access to these resources via the generated SP resource manager client.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-SPR-010 | `dcm sp resource list` with `--provider`, `--page-size`, `--page-token` flags | Done |
| REQ-SPR-020 | Display SP resources in configured output format | Done |
| REQ-SPR-030 | `dcm sp resource get INSTANCE_ID` | Done |
| REQ-SPR-040 | Missing `INSTANCE_ID` → usage error (exit code 2) | Done |
| REQ-SPR-050 | All commands use generated SP Resource Manager client | Done |

### Tests Implemented (10 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U121 | List SP resources — GET `/api/v1alpha1/service-type-instances`, displays results | Pass |
| TC-U122 | List with `--page-size 5` — passes `max_page_size=5` query parameter | Pass |
| TC-U122 | List with `--page-token abc123` — passes `page_token=abc123` query parameter | Pass |
| TC-U123 | List with `--provider kubevirt-123` — passes `provider=kubevirt-123` query parameter | Pass |
| TC-U124 | Get SP resource — GET `/api/v1alpha1/service-type-instances/my-instance` | Pass |
| TC-U125 | Get without INSTANCE_ID → UsageError (exit code 2) | Pass |
| TC-U126 | Empty list — table shows headers only; JSON shows empty `results` array | Pass |
| TC-U127 | Get non-existent SP resource — 404 RFC 7807 error formatted to stderr | Pass |
| TC-U128 | Table output columns: ID, PROVIDER, STATUS, CREATED | Pass |
| TC-U129 | SP command registers `resource` subcommand | Pass |

---

## Files Created / Modified

| File | Change | Purpose |
|------|--------|---------|
| `go.mod` / `go.sum` | Modified | Added `github.com/dcm-project/control-plane` dependency (SP resource manager client) |
| `internal/commands/helpers.go` | Modified | Added `newSPResourceClient` using the resource_manager client package |
| `internal/commands/sp.go` | Created | `dcm sp` parent command group |
| `internal/commands/sp_resource.go` | Created | `list` and `get` commands with generated SP Resource Manager client |
| `internal/commands/sp_resource_test.go` | Created | 10 Ginkgo test specs with httptest-based mocking |
| `internal/commands/root.go` | Modified | Registered `newSPCommand()` alongside policy, catalog, version |
| `internal/commands/root_test.go` | Modified | Added TC-U129 (sp subcommand registration), sp resource get usage error entry, updated TC-U019 to include `sp` |
| `.ai/checkpoints/topic-9-sp-resource-commands.md` | Created | This checkpoint |

---

## Key Design Decisions

1. **Generated client from control-plane** — Per REQ-SPR-050, all SP resource operations use the oapi-codegen generated client from `github.com/dcm-project/control-plane/pkg/sp/client/resource_manager`. The `newSPResourceClient` function follows the same pattern as `newPolicyClient` and `newCatalogClient`, using `sprmclient.NewClient(apiBaseURL(cfg), sprmclient.WithHTTPClient(httpClient))`.

2. **Separate API type import** — SP resource API types live in `github.com/dcm-project/control-plane/api/sp/v1alpha1/resource_manager`, imported as `sprmapi` for `ListInstancesParams`.

3. **Table columns** — ID, PROVIDER, STATUS, CREATED per spec section 4.9. Fields map to `id`, `provider_name`, `status`, `create_time` from the `ServiceTypeInstance` type.

4. **List response uses `instances` field** — Unlike the catalog API which uses `results`, the SP resource manager's `ServiceTypeInstanceList` type uses `instances` for the array and `next_page_token` for pagination. The formatter re-wraps this as `results` for consistent JSON/YAML output.

5. **`MaxPageSize` type difference** — The SP resource manager client uses `*int` for `MaxPageSize` (not `*int32` like the catalog client), so the `--page-size` flag value is converted from `int32` to `int`.

6. **`--provider` filter** — The `ListInstancesParams` includes a `Provider` field passed as the `provider` query parameter, matching the spec's REQ-SPR-010.

7. **Same patterns as previous command groups** — List and get follow the identical patterns established in Topics 4–7: generated client usage, `handleErrorResponse` for errors, `newFormatter` for output, `connectionError` for connection failures.

---

## What's Next

All topics (1–9) are now complete. The CLI supports:
- Policy CRUD operations (Topic 4)
- Catalog service-type read operations (Topic 5)
- Catalog item operations (Topic 6)
- Catalog instance operations (Topic 7)
- Version display (Topic 8)
- SP resource read operations (Topic 9)
