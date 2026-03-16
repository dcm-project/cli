# Checkpoint: Topic 3 — Output Formatting

- **Branch:** `topic-3-output-formatting`
- **Base:** `topic-1-cli-framework` (commit `5a59418`)
- **Date:** 2026-03-12
- **Status:** Complete (pending commit)

---

## Scope

Topic 3 implements the output formatting package per spec section 4.3. It provides a `Formatter` struct that renders single resources, resource lists, status messages, and errors in table, JSON, or YAML format. Success output goes to stdout; error output goes to stderr.

No dependencies on Topic 2 (Configuration) were required.

### Requirements Addressed

| ID | Description | Status |
|----|-------------|--------|
| REQ-OUT-010 | Three output formats: `table`, `json`, `yaml` | Done |
| REQ-OUT-020 | Default output format is `table` | Done (via `ParseFormat` + default in root flag) |
| REQ-OUT-030 | Output format selectable via `--output`/`-o` flag | Done (flag exists from Topic 1; `ParseFormat` validates) |
| REQ-OUT-050 | Table output with fixed column headers per resource type | Done (`TableDef` with `Headers` and `RowFunc`) |
| REQ-OUT-060 | JSON output produces valid JSON | Done |
| REQ-OUT-070 | YAML output produces valid YAML | Done |
| REQ-OUT-080 | JSON/YAML list output includes `next_page_token` when present | Done |
| REQ-OUT-090 | Table list output shows pagination hint when `next_page_token` present | Done |
| REQ-OUT-100 | Invalid output format values rejected | Done (`ParseFormat` returns error) |
| REQ-OUT-110 | Success output to stdout, error output to stderr | Done (separate `out`/`errOut` writers) |
| REQ-OUT-120 | `FormatError` renders API errors per format | Done |

### Tests Implemented (18 specs)

| TC ID | Description | Status |
|-------|-------------|--------|
| TC-U009 | Table output for single resource with headers and values | Pass |
| TC-U010 | Table output for 3-resource list, no pagination hint | Pass |
| TC-U011 | JSON output is valid, parseable JSON with correct fields | Pass |
| TC-U012 | YAML output is valid, parseable YAML with correct fields | Pass |
| TC-U013 | Table list output shows pagination hint with token | Pass |
| TC-U014 | JSON list output includes `next_page_token` | Pass |
| TC-U015 | YAML list output includes `next_page_token` | Pass |
| TC-U016 | No pagination hint when `nextPageToken` is empty | Pass |
| TC-U017 | `ParseFormat("invalid")` returns error | Pass |
| TC-U018 | `FormatMessage` writes to stdout, nothing to stderr | Pass |
| TC-U116 | `FormatError` writes to stderr, stdout is empty | Pass |
| TC-U117 | `FormatError` table format: `Error: TYPE - TITLE`, `Status`, `Detail` | Pass |
| TC-U118 | `FormatError` JSON format: full Problem Details object to stderr | Pass |
| TC-U119 | `FormatError` YAML format: full Problem Details object to stderr | Pass |
| — | Valid format strings accepted by `ParseFormat` | Pass |
| — | Empty table list renders headers only | Pass |
| — | Empty JSON list renders `[]` results array | Pass |
| — | Empty YAML list renders empty results sequence | Pass |

---

## Files Created

| File | Purpose |
|------|---------|
| `internal/output/formatter.go` | `Formatter` struct, `Format` type, `ProblemDetail`, `TableDef`, `ListResponse`, `ParseFormat` |
| `internal/output/formatter_test.go` | Ginkgo tests for TC-U009–TC-U018, TC-U116–TC-U119, plus edge cases |
| `internal/output/output_suite_test.go` | Ginkgo test suite bootstrap |

---

## Key Design Decisions

1. **Concrete struct, not interface** — `Formatter` is a struct rather than an interface. Commands receive a `*Formatter` directly. This keeps the API simple; an interface can be introduced later if needed for testing command logic.

2. **Separate `out` and `errOut` writers** — The constructor takes two `io.Writer` parameters. `FormatOne`, `FormatList`, and `FormatMessage` write to `out` (stdout). `FormatError` writes to `errOut` (stderr). This satisfies REQ-OUT-110.

3. **`TableDef` injection** — Table column layout is defined per resource type via `TableDef` (headers + row extraction function). The output package is generic; resource-specific table definitions will be provided by command implementations in Topics 4–7.

4. **`command` parameter for pagination hints** — The `Formatter` receives the base command string (e.g., `"policy list --page-size 2"`) so pagination hints render as `Next page: dcm <command> --page-token <token>`.

5. **`ParseFormat` for validation** — A standalone function validates format strings and returns a typed `Format`. Commands will call this to convert the `--output` flag value, returning a usage error on invalid input (REQ-OUT-100).

6. **Nil-safe empty lists** — `FormatList` normalises `nil` resource slices to empty `[]any{}` so JSON/YAML always render `"results": []` rather than `null`.

---

## What's Next

- **Topic 2: Configuration Management** — In progress on separate branch `topic-2-configuration`
- **Topic 8: Version Command** — Depends only on Topic 1
- **Topics 4–7: Command implementations** — Depend on Topics 1, 2, 3; will wire `Formatter` into command `RunE` functions with resource-specific `TableDef` definitions
- **TC-U116 full verification** — Command-level test (policy get → mock 404 → stderr output) will be implemented in Topic 4
