# Checkpoint: Topic 4 — Policy Commands

- **Branch:** `topic-4-policy-commands`
- **Base:** `baseline` (commit `794930d`)
- **Date:** 2026-03-12
- **Status:** Complete

---

## Scope

Topic 4 implements the five policy CRUD commands (create, list, get, update, delete) per spec section 4.4, along with shared command helpers extracted into `helpers.go` for reuse by Topics 5-7.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-POL-010 | `dcm policy create --from-file FILE` | Done |
| REQ-POL-020 | `dcm policy create --id ID` optional client-specified ID | Done |
| REQ-POL-030 | `dcm policy list` with filter, order-by, pagination flags | Done |
| REQ-POL-040 | `dcm policy get POLICY_ID` | Done |
| REQ-POL-050 | `dcm policy update POLICY_ID --from-file FILE` (PATCH) | Done |
| REQ-POL-060 | `dcm policy delete POLICY_ID` | Done |
| REQ-XC-CLI-010 | Exit code 0 for success | Done |
| REQ-XC-CLI-020 | Exit code 1 for runtime errors | Done |
| REQ-XC-CLI-030 | Exit code 2 for usage errors | Done |
| REQ-XC-CLI-040 | Context-based request timeout | Done |
| REQ-ERR-010 | RFC 7807 Problem Details error formatting | Done |
| REQ-INP-010 | YAML/JSON input file parsing | Done |

### Tests Implemented (32 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U026 | `policy create --from-file policy.yaml` sends POST, returns 201 | Pass |
| TC-U027 | `policy create` without `--from-file` exits code 2 | Pass |
| TC-U028 | `policy create --from-file` with nonexistent file returns error | Pass |
| TC-U029 | `policy create --from-file bad.yaml` with invalid YAML returns error | Pass |
| TC-U030 | `policy create --id custom-id` passes `?id=custom-id` query param | Pass |
| TC-U031 | `policy create` with 409 RFC 7807 error formats to stderr | Pass |
| TC-U032 | `policy list` sends GET, returns table with 2 policies | Pass |
| TC-U033 | `policy list --filter` passes filter query parameter | Pass |
| TC-U034 | `policy list --order-by` passes order_by query parameter | Pass |
| TC-U035 | `policy list --page-size --page-token` passes pagination params | Pass |
| TC-U036 | `policy list` with next_page_token shows pagination hint | Pass |
| TC-U037 | `policy list -o json` returns JSON with results array | Pass |
| TC-U038 | `policy get POLICY_ID` sends GET to correct path | Pass |
| TC-U039 | `policy get` without ID exits code 2 | Pass |
| TC-U040 | `policy update POLICY_ID --from-file` sends PATCH | Pass |
| TC-U041 | `policy update` without `--from-file` exits code 2 | Pass |
| TC-U062 | `policy delete POLICY_ID` sends DELETE, returns success message | Pass |
| TC-U063 | `policy delete` without ID exits code 2 | Pass |
| TC-U080 | Connection refused returns user-friendly error with gateway URL | Pass |
| TC-U081 | Request timeout returns timeout-specific error message | Pass |
| TC-U082 | 404 RFC 7807 error formatted to stderr, exit code 1 | Pass |
| TC-U083 | Non-RFC-7807 error body returns `HTTP <status>: <body>` | Pass |
| TC-U084 | `--from-file` with JSON input sends correct request body | Pass |
| TC-U085 | `--from-file` with YAML list (non-object) returns parse error | Pass |
| TC-U100 | `policy list -o json` returns valid JSON structure | Pass |
| TC-U101 | `policy list -o yaml` returns valid YAML structure | Pass |
| TC-U102 | `policy get -o json` returns valid JSON | Pass |
| TC-U103 | `policy get -o yaml` returns valid YAML | Pass |
| TC-U104 | `policy create -o json` returns valid JSON | Pass |
| TC-U116 | Error response formatted to stderr per output format | Pass |
| TC-U120 | Empty policy list returns headers only (table) / empty array (JSON) | Pass |
| — | `policy list` with `--page-size` includes it in pagination hint | Pass |

---

## Files Created / Modified

| File | Change | Purpose |
|------|--------|---------|
| `internal/commands/helpers.go` | Created | Shared utilities: `FormattedError`, `newFormatter`, `buildHTTPClient`, `apiBaseURL`, `parseInputFile`, `parseInputFileAs` (generic typed variant), `handleErrorResponse`, `requestContext`, `connectionError`, `isTimeoutError`, `stringifyValue` |
| `internal/commands/policy.go` | Modified | Full CRUD implementation using generated policy-manager client |
| `internal/commands/root.go` | Modified | Added `FormattedError` handling in `Execute()`, `requiredFlagsPreRun` hook |
| `internal/commands/policy_test.go` | Created | 32 Ginkgo test specs with httptest-based mocking |

---

## Key Design Decisions

1. **Generated client from policy-manager** — Per REQ-XC-CLI-010, all policy operations use the oapi-codegen generated client from `github.com/dcm-project/policy-manager/pkg/client`. Client is instantiated via `policyclient.NewClient(apiBaseURL(cfg), policyclient.WithHTTPClient(buildHTTPClient(cfg)))`. Create and update commands use typed client methods (`CreatePolicy`, `UpdatePolicyWithApplicationMergePatchPlusJSONBody`) with typed request bodies (`CreatePolicyJSONRequestBody`, `UpdatePolicyApplicationMergePatchPlusJSONRequestBody`) for client-side payload validation against the generated schema.

2. **Shared helpers in `helpers.go`** — HTTP client construction, request context with timeout, error handling, input file parsing, and table cell extraction are shared across all command groups. Extracted into `helpers.go` so Topics 5-7 reuse them without duplication.

3. **`FormattedError` sentinel type** — When `handleErrorResponse` detects an RFC 7807 error, it formats it via `formatter.FormatError()` (writing to stderr) and returns `&FormattedError{}`. `Execute()` checks for this type and skips printing the error again, preventing double output.

4. **`MarkFlagRequired` with `requiredFlagsPreRun`** — Required flags use Cobra's `MarkFlagRequired` for help/docs. A `requiredFlagsPreRun` hook (set as `PreRunE`) calls `cmd.ValidateRequiredFlags()` early and wraps errors as `UsageError` for exit code 2. Cobra's own `ValidateRequiredFlags` call that follows is a no-op.

5. **Context timeout, not `http.Client.Timeout`** — Per REQ-XC-CLI-040, request deadlines use `context.WithTimeout` on the command context, allowing cancellation propagation through the request lifecycle.

6. **YAML-first parsing with typed unmarshalling** — `parseInputFile` uses YAML unmarshal (which also handles valid JSON) with a nil-result check to reject non-object content (arrays, scalars, empty files). `parseInputFileAs[T]` extends this by further unmarshalling the parsed map into a typed struct via JSON round-trip, enabling client-side schema validation.

7. **Pagination hint includes `--page-size`** — The list command dynamically builds the hint command string to include `--page-size` if specified. The CLI flag `--page-size` maps to the API's `max_page_size` query parameter via `ListPoliciesParams.MaxPageSize`.

8. **List response field mapping** — The API returns policies in a `policies` field (per `PolicyList` type), which the CLI re-wraps as `results` in its JSON/YAML output via the formatter's `ListResponse` struct.

---

## What's Next

- **Topic 5: Catalog Service-Type Commands** — `catalog service-type list` and `catalog service-type get`
- **Topic 6: Catalog Item Commands** — `catalog item create/list/get/delete`
- **Topic 7: Catalog Instance Commands** — `catalog instance create/list/get/delete`
- All future command topics reuse `helpers.go` utilities
