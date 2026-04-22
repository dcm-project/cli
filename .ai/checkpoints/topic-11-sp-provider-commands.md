# Checkpoint: Topic 11 — SP Provider Commands

- **Branch:** `topic-11-sp-provider-plans`
- **Base:** `d076a8a` (Add SP provider commands to spec and test plan)
- **Date:** 2026-03-27
- **Status:** Complete

---

## Scope

Topic 11 implements the `dcm sp provider` command group with read-only subcommands (`list` and `get`) per spec section 4.11. Providers are service providers registered with the Service Provider Manager. The CLI provides read-only access to these resources via the top-level generated SP Manager client (`service-provider-manager/pkg/client`).

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-SPP-010 | `dcm sp provider list` with `--type`, `--page-size`, `--page-token` flags | Done |
| REQ-SPP-020 | Display SP providers in configured output format | Done |
| REQ-SPP-030 | `dcm sp provider get PROVIDER_ID` | Done |
| REQ-SPP-040 | Missing `PROVIDER_ID` → usage error (exit code 2) | Done |
| REQ-SPP-050 | All commands use generated SP Manager client | Done |

### Tests Implemented (12 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U139 | List SP providers — GET `/api/v1alpha1/providers`, displays results | Pass |
| TC-U140 | List with `--page-size 5` — passes `max_page_size=5` query parameter | Pass |
| TC-U140 | List with `--page-token abc123` — passes `page_token=abc123` query parameter | Pass |
| TC-U141 | List with `--type compute` — passes `type=compute` query parameter | Pass |
| TC-U142 | Get SP provider — GET `/api/v1alpha1/providers/kubevirt-123` | Pass |
| TC-U143 | Get without PROVIDER_ID → UsageError (exit code 2) | Pass |
| TC-U144 | Empty list — table shows headers only; JSON shows empty `results` array | Pass |
| TC-U145 | Get non-existent SP provider — 404 RFC 7807 error formatted to stderr | Pass |
| TC-U146 | Table output columns: ID, NAME, SERVICE TYPE, STATUS, HEALTH, CREATED | Pass |
| TC-U147 | SP command registers `provider` subcommand alongside `resource` | Pass |

---

## Files Created / Modified

| File | Change | Purpose |
|------|--------|---------|
| `internal/commands/sp_provider.go` | Created | `list` and `get` commands with generated SP Manager client |
| `internal/commands/sp_provider_test.go` | Created | 10 Ginkgo test specs with httptest-based mocking |
| `internal/commands/helpers.go` | Modified | Added `newSPProviderClient` using the top-level SP Manager client package |
| `internal/commands/sp.go` | Modified | Registered `newSPProviderCommand()` alongside `newSPResourceCommand()` |
| `internal/commands/root_test.go` | Modified | Updated TC-U129 to check for `provider` subcommand, added `sp provider get` usage error entry |
| `.ai/checkpoints/topic-11-sp-provider-commands.md` | Created | This checkpoint |

---

## Key Design Decisions

1. **Top-level SP Manager client** — Per REQ-SPP-050, all SP provider operations use the oapi-codegen generated client from `github.com/dcm-project/service-provider-manager/pkg/client` (not the `resource_manager` sub-package). The `newSPProviderClient` function follows the same pattern as `newSPResourceClient`.

2. **Separate API type import** — The SP Manager has its API types in `api/v1alpha1`, imported as `spmapi` for `ListProvidersParams`.

3. **Table columns** — ID, NAME, SERVICE TYPE, HEALTH, CREATED per spec section 4.11. Fields map to `id`, `name`, `service_type`, `health_status`, `create_time` from the `Provider` type. (Updated 2026-04-22: removed STATUS column after upstream API dropped the `status` field.)

4. **List response uses `providers` field** — The SP Manager's `ProviderList` type uses `providers` for the array and `next_page_token` for pagination.

5. **`--type` filter** — The `ListProvidersParams` includes a `Type` field passed as the `type` query parameter, matching REQ-SPP-010.

6. **Same patterns as SP resource commands** — List and get follow the identical patterns established in Topic 9, reusing `handleErrorResponse`, `newFormatter`, `connectionError`, and `requestContext`.
