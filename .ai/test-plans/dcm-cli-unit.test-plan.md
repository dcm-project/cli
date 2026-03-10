# Test Plan: DCM CLI — Unit Tests

## Overview

- **Related Spec:** .ai/specs/dcm-cli.spec.md
- **Related Plan:** .ai/plan/dcm-cli.plan.md
- **Related Requirements:** REQ-CLI-010–070, REQ-CFG-010–070, REQ-OUT-010–100, REQ-POL-010–130, REQ-CST-010–050, REQ-CIT-010–130, REQ-CIN-010–110, REQ-VER-010–030, REQ-XC-ERR-010–060, REQ-XC-INP-010–030, REQ-XC-CLI-010–050, REQ-XC-PAG-010–030, REQ-XC-TLS-010–080
- **Framework:** Ginkgo v2 + Gomega
- **Created:** 2026-03-09

Unit tests verify individual components in isolation. All external dependencies
(API Gateway, Policy Manager, Catalog Manager) are replaced with
`net/http/httptest` servers. Tests use direct command execution via Cobra's
`Execute()` with captured stdout/stderr, and direct function calls for pure
logic (config loading, output formatting, file parsing).

### Utility Test Case Approach

Utility and helper functions (input file parsing, error formatting, client
instantiation, pagination parameter passing) are **not** tested in dedicated
test classes. Instead:

- Each utility behaviour retains a **TC-ID** for requirements traceability.
- The TC-ID is **referenced** in the higher-level behavioural test(s) that
  exercise the utility transitively.
- All utility TC-IDs, their descriptions, and cross-references are collected in
  the [Utility Test Case Index](#utility-test-case-index) at the end of this
  document.

---

## 1 · Configuration

> **Suggested Ginkgo structure:** `Describe("Configuration")`

### TC-U001: Load configuration from config file

- **Requirement:** REQ-CFG-010
- **Priority:** High
- **Type:** Unit
- **Given:** A config file exists at a temporary path with `api-gateway-url: http://custom:9080`
- **When:** Config is loaded with `--config` pointing to that file
- **Then:** The loaded config has `APIGatewayURL = "http://custom:9080"`

### TC-U002: Environment variable overrides config file

- **Requirement:** REQ-CFG-030, REQ-CFG-040
- **Priority:** High
- **Type:** Unit
- **Given:** A config file has `api-gateway-url: http://file:9080` AND `DCM_API_GATEWAY_URL=http://env:9080` is set
- **When:** Config is loaded without `--api-gateway-url` flag
- **Then:** The loaded config has `APIGatewayURL = "http://env:9080"`

### TC-U003: CLI flag overrides environment variable and config file

- **Requirement:** REQ-CFG-040
- **Priority:** High
- **Type:** Unit
- **Given:** `DCM_API_GATEWAY_URL=http://env:9080` is set AND config file has `api-gateway-url: http://file:9080`
- **When:** Config is loaded with `--api-gateway-url http://flag:9080`
- **Then:** The loaded config has `APIGatewayURL = "http://flag:9080"`

### TC-U004: Default values applied when no config specified

- **Requirement:** REQ-CFG-050
- **Priority:** High
- **Type:** Unit
- **Given:** No config file exists AND no environment variables are set AND no flags are provided
- **When:** Config is loaded
- **Then:** `APIGatewayURL` defaults to `"http://localhost:9080"` AND `OutputFormat` defaults to `"table"` AND `Timeout` defaults to `30` AND `TLSCACert` defaults to `""` AND `TLSClientCert` defaults to `""` AND `TLSClientKey` defaults to `""` AND `TLSSkipVerify` defaults to `false`

### TC-U005: Missing config file does not cause failure

- **Requirement:** REQ-CFG-070
- **Priority:** High
- **Type:** Unit
- **Given:** No config file exists at the default path `~/.dcm/config.yaml`
- **When:** Config is loaded
- **Then:** No error is returned AND built-in defaults are used

### TC-U006: Custom config file path via --config flag

- **Requirement:** REQ-CFG-020
- **Priority:** High
- **Type:** Unit
- **Given:** A config file exists at `/tmp/dcm-test.yaml` with `timeout: 60`
- **When:** Config is loaded with `--config /tmp/dcm-test.yaml`
- **Then:** The loaded config has `Timeout = 60`

### TC-U007: Custom config file path via DCM_CONFIG environment variable

- **Requirement:** REQ-CFG-020
- **Priority:** Medium
- **Type:** Unit
- **Given:** A config file exists at `/tmp/dcm-test.yaml` AND `DCM_CONFIG=/tmp/dcm-test.yaml` is set
- **When:** Config is loaded without `--config` flag
- **Then:** Configuration is loaded from `/tmp/dcm-test.yaml`

### TC-U008: All environment variables are supported

- **Requirement:** REQ-CFG-030
- **Priority:** High
- **Type:** Unit (table-driven)
- **Given:** Each environment variable is set individually:

  | Environment Variable   | Value      | Expected Config Field |
  |------------------------|------------|-----------------------|
  | `DCM_API_GATEWAY_URL`  | `http://e:9080` | `APIGatewayURL`  |
  | `DCM_OUTPUT_FORMAT`    | `json`     | `OutputFormat`        |
  | `DCM_TIMEOUT`          | `60`       | `Timeout`             |
  | `DCM_TLS_CA_CERT`      | `/path/ca.pem` | `TLSCACert`      |
  | `DCM_TLS_CLIENT_CERT`  | `/path/cert.pem` | `TLSClientCert` |
  | `DCM_TLS_CLIENT_KEY`   | `/path/key.pem` | `TLSClientKey`   |
  | `DCM_TLS_SKIP_VERIFY`  | `true`     | `TLSSkipVerify`       |

- **When:** Config is loaded
- **Then:** Each config field matches the environment variable value

---

## 2 · Output Formatting

> **Suggested Ginkgo structure:** `Describe("Output Formatting")` with nested
> `Describe` per format and `Context` per scenario.

### TC-U009: Table output for single resource

- **Requirement:** REQ-OUT-050
- **Priority:** High
- **Type:** Unit
- **Given:** A single policy resource with known field values
- **When:** `FormatOne` is called with format `table`
- **Then:** The output contains column headers AND the resource values in tabular format

### TC-U010: Table output for resource list

- **Requirement:** REQ-OUT-050
- **Priority:** High
- **Type:** Unit
- **Given:** A list of 3 policy resources
- **When:** `FormatList` is called with format `table` and empty `nextPageToken`
- **Then:** The output contains column headers AND 3 data rows AND no pagination hint

### TC-U011: JSON output produces valid JSON

- **Requirement:** REQ-OUT-060
- **Priority:** High
- **Type:** Unit
- **Given:** A single resource
- **When:** `FormatOne` is called with format `json`
- **Then:** The output is valid, parseable JSON containing the resource fields

### TC-U012: YAML output produces valid YAML

- **Requirement:** REQ-OUT-070
- **Priority:** High
- **Type:** Unit
- **Given:** A single resource
- **When:** `FormatOne` is called with format `yaml`
- **Then:** The output is valid, parseable YAML containing the resource fields

### TC-U013: Pagination hint in table output

- **Requirement:** REQ-OUT-090
- **Priority:** High
- **Type:** Unit
- **Given:** A list response with `nextPageToken = "eyJvZmZzZXQiOjJ9"`
- **When:** `FormatList` is called with format `table`
- **Then:** The output includes a line showing the command to fetch the next page (e.g., `Next page: dcm policy list --page-size 2 --page-token eyJvZmZzZXQiOjJ9`)

### TC-U014: Pagination token in JSON output

- **Requirement:** REQ-OUT-080
- **Priority:** High
- **Type:** Unit
- **Given:** A list response with `nextPageToken = "abc123"`
- **When:** `FormatList` is called with format `json`
- **Then:** The JSON output includes `"next_page_token": "abc123"`

### TC-U015: Pagination token in YAML output

- **Requirement:** REQ-OUT-080
- **Priority:** High
- **Type:** Unit
- **Given:** A list response with `nextPageToken = "abc123"`
- **When:** `FormatList` is called with format `yaml`
- **Then:** The YAML output includes `next_page_token: abc123`

### TC-U016: No pagination hint when nextPageToken is empty

- **Requirement:** REQ-OUT-090
- **Priority:** Medium
- **Type:** Unit
- **Given:** A list response with no `nextPageToken`
- **When:** `FormatList` is called with format `table`
- **Then:** No pagination hint line is displayed

### TC-U017: Invalid output format rejected

- **Requirement:** REQ-OUT-100
- **Priority:** High
- **Type:** Unit
- **Given:** `--output invalid` is provided
- **When:** The CLI processes the flag
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U018: FormatMessage displays status message

- **Requirement:** REQ-OUT-040
- **Priority:** High
- **Type:** Unit
- **Given:** A status message `Policy "my-policy" deleted successfully.`
- **When:** `FormatMessage` is called
- **Then:** The message is written to stdout exactly as provided

---

## 3 · CLI Framework & Root Command

> **Suggested Ginkgo structure:** `Describe("Root Command")`

### TC-U019: Root command registers all subcommands

- **Requirement:** REQ-CLI-030
- **Priority:** High
- **Type:** Unit
- **Given:** The root command is created via `NewRootCommand()`
- **When:** `dcm --help` is executed
- **Then:** Subcommands `policy`, `catalog`, and `version` are listed in the help output

### TC-U020: Catalog command registers subcommand groups

- **Requirement:** REQ-CLI-040
- **Priority:** High
- **Type:** Unit
- **Given:** The root command is created
- **When:** `dcm catalog --help` is executed
- **Then:** Subcommands `service-type`, `item`, and `instance` are listed

### TC-U021: Global flags are registered

- **Requirement:** REQ-CLI-050
- **Priority:** High
- **Type:** Unit
- **Given:** The root command is created
- **When:** `dcm --help` is executed
- **Then:** Flags `--api-gateway-url`, `--output`/`-o`, `--timeout`, `--config`, `--tls-ca-cert`, `--tls-client-cert`, `--tls-client-key`, and `--tls-skip-verify` are listed

### TC-U022: Exit code 0 on success

- **Requirement:** REQ-CLI-060
- **Priority:** High
- **Type:** Unit
- **Given:** A command completes successfully (e.g., `dcm version`)
- **When:** The process exits
- **Then:** The exit code is 0

### TC-U023: Exit code 2 on usage error

- **Requirement:** REQ-CLI-060
- **Priority:** High
- **Type:** Unit
- **Given:** A command is invoked with missing required arguments (e.g., `dcm policy get` without POLICY_ID)
- **When:** The process exits
- **Then:** The exit code is 2

---

## 4 · Version Command

> **Suggested Ginkgo structure:** `Describe("Version Command")`

### TC-U024: Version displays all build information

- **Requirement:** REQ-VER-010
- **Priority:** High
- **Type:** Unit
- **Given:** Version info is set to `Version="1.0.0"`, `Commit="abc1234"`, `BuildTime="2026-03-09T10:00:00Z"`
- **When:** `dcm version` is executed
- **Then:** The output contains:
  - `dcm version 1.0.0`
  - `commit: abc1234`
  - `built: 2026-03-09T10:00:00Z`
  - `go:` followed by the Go version

### TC-U025: Default version values when not built with ldflags

- **Requirement:** REQ-VER-030
- **Priority:** High
- **Type:** Unit
- **Given:** No ldflags are set (default values)
- **When:** `dcm version` is executed
- **Then:** Version is `dev`, commit is `unknown`, build time is `unknown`

---

## 5 · Policy Commands

> **Suggested Ginkgo structure:** `Describe("Policy Commands")` with nested
> `Describe` per subcommand. All tests use `net/http/httptest` to mock the
> generated client's HTTP calls.

### TC-U026: Create policy from YAML file

- **Requirement:** REQ-POL-010, REQ-POL-030, REQ-POL-110
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U060 (YAML file parsing), TC-U066 (generated Policy Manager client usage)
- **Given:** A valid policy YAML file AND a mock server returning 201 with a policy response
- **When:** `dcm policy create --from-file policy.yaml` is executed
- **Then:** A POST request is sent to `/api/v1alpha1/policies` AND the created policy is displayed in the configured output format

### TC-U027: Create policy from JSON file

- **Requirement:** REQ-POL-010
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U061 (JSON file parsing)
- **Given:** A valid policy JSON file AND a mock server returning 201
- **When:** `dcm policy create --from-file policy.json` is executed
- **Then:** The policy is created successfully

### TC-U028: Create policy with client-specified ID

- **Requirement:** REQ-POL-020
- **Priority:** High
- **Type:** Unit
- **Given:** A valid policy file AND a mock server
- **When:** `dcm policy create --from-file policy.yaml --id my-policy` is executed
- **Then:** The POST request includes `?id=my-policy` as a query parameter

### TC-U029: Create policy without --from-file fails

- **Requirement:** REQ-POL-120
- **Priority:** High
- **Type:** Unit
- **Given:** No `--from-file` flag is provided
- **When:** `dcm policy create` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U030: List policies

- **Requirement:** REQ-POL-040, REQ-POL-050
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U066 (generated Policy Manager client usage)
- **Given:** A mock server returning 200 with a list of policies
- **When:** `dcm policy list` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/policies` AND policies are displayed in the configured output format

### TC-U031: List policies with filter

- **Requirement:** REQ-POL-040
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server
- **When:** `dcm policy list --filter "policy_type='GLOBAL'"` is executed
- **Then:** The GET request includes the filter as a query parameter

### TC-U032: List policies with order-by

- **Requirement:** REQ-POL-040
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server
- **When:** `dcm policy list --order-by "priority asc"` is executed
- **Then:** The GET request includes the `order_by` as a query parameter

### TC-U033: List policies with pagination

- **Requirement:** REQ-POL-040
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U069 (pagination flags present), TC-U070 (pagination parameter pass-through)
- **Given:** A mock server
- **When:** `dcm policy list --page-size 10 --page-token abc123` is executed
- **Then:** The GET request includes `page_size=10` and `page_token=abc123` as query parameters

### TC-U034: Get policy

- **Requirement:** REQ-POL-060
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 200 with a policy for id `my-policy`
- **When:** `dcm policy get my-policy` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/policies/my-policy` AND the policy is displayed

### TC-U035: Get policy without POLICY_ID fails

- **Requirement:** REQ-POL-130
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm policy get` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U036: Update policy

- **Requirement:** REQ-POL-070, REQ-POL-080
- **Priority:** High
- **Type:** Unit
- **Given:** A valid patch YAML file AND a mock server returning 200 with the updated policy
- **When:** `dcm policy update my-policy --from-file patch.yaml` is executed
- **Then:** A PATCH request is sent to `/api/v1alpha1/policies/my-policy` AND the updated policy is displayed

### TC-U037: Update policy without --from-file fails

- **Requirement:** REQ-POL-120
- **Priority:** High
- **Type:** Unit
- **Given:** No `--from-file` flag is provided
- **When:** `dcm policy update my-policy` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U038: Update policy without POLICY_ID fails

- **Requirement:** REQ-POL-130
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm policy update` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U039: Delete policy

- **Requirement:** REQ-POL-090, REQ-POL-100
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 204 for delete
- **When:** `dcm policy delete my-policy` is executed
- **Then:** A DELETE request is sent to `/api/v1alpha1/policies/my-policy` AND the message `Policy "my-policy" deleted successfully.` is displayed

### TC-U040: Delete policy without POLICY_ID fails

- **Requirement:** REQ-POL-130
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm policy delete` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U041: Policy table output columns

- **Requirement:** REQ-OUT-050
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning a policy with all fields populated
- **When:** `dcm policy get my-policy` is executed with `--output table`
- **Then:** The table output includes columns: ID, DISPLAY NAME, TYPE, PRIORITY, ENABLED, CREATED

---

## 6 · Catalog Service-Type Commands

> **Suggested Ginkgo structure:** `Describe("Catalog Service-Type Commands")`
> with nested `Describe` per subcommand.

### TC-U042: List service types

- **Requirement:** REQ-CST-010, REQ-CST-020
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U067 (generated Catalog Manager client usage)
- **Given:** A mock server returning 200 with a list of service types
- **When:** `dcm catalog service-type list` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/service-types` AND the service types are displayed

### TC-U043: List service types with pagination

- **Requirement:** REQ-CST-010
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U069 (pagination flags present)
- **Given:** A mock server
- **When:** `dcm catalog service-type list --page-size 5` is executed
- **Then:** The GET request includes `page_size=5` as a query parameter

### TC-U044: Get service type

- **Requirement:** REQ-CST-030
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 200 with a service type
- **When:** `dcm catalog service-type get my-service-type` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/service-types/my-service-type` AND the service type is displayed

### TC-U045: Get service type without SERVICE_TYPE_ID fails

- **Requirement:** REQ-CST-040
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm catalog service-type get` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

---

## 7 · Catalog Item Commands

> **Suggested Ginkgo structure:** `Describe("Catalog Item Commands")` with
> nested `Describe` per subcommand.

### TC-U046: Create catalog item from file

- **Requirement:** REQ-CIT-010, REQ-CIT-030
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U067 (generated Catalog Manager client usage)
- **Given:** A valid catalog item YAML file AND a mock server returning 201
- **When:** `dcm catalog item create --from-file item.yaml` is executed
- **Then:** A POST request is sent to `/api/v1alpha1/catalog-items` AND the created catalog item is displayed

### TC-U047: Create catalog item with client-specified ID

- **Requirement:** REQ-CIT-020
- **Priority:** High
- **Type:** Unit
- **Given:** A valid catalog item file AND a mock server
- **When:** `dcm catalog item create --from-file item.yaml --id my-catalog-item` is executed
- **Then:** The POST request includes `?id=my-catalog-item` as a query parameter

### TC-U048: Create catalog item without --from-file fails

- **Requirement:** REQ-CIT-120
- **Priority:** High
- **Type:** Unit
- **Given:** No `--from-file` flag is provided
- **When:** `dcm catalog item create` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U049: List catalog items

- **Requirement:** REQ-CIT-040, REQ-CIT-050
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 200 with a list of catalog items
- **When:** `dcm catalog item list` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/catalog-items` AND the catalog items are displayed

### TC-U050: List catalog items with service-type filter

- **Requirement:** REQ-CIT-040
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server
- **When:** `dcm catalog item list --service-type container` is executed
- **Then:** The GET request includes the service type as a query parameter

### TC-U051: Get catalog item

- **Requirement:** REQ-CIT-060
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 200 with a catalog item
- **When:** `dcm catalog item get my-catalog-item` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/catalog-items/my-catalog-item` AND the catalog item is displayed

### TC-U052: Get catalog item without CATALOG_ITEM_ID fails

- **Requirement:** REQ-CIT-130
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm catalog item get` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U053: Update catalog item

- **Requirement:** REQ-CIT-070, REQ-CIT-080
- **Priority:** High
- **Type:** Unit
- **Given:** A valid patch file AND a mock server returning 200
- **When:** `dcm catalog item update my-catalog-item --from-file patch.yaml` is executed
- **Then:** A PATCH request is sent to `/api/v1alpha1/catalog-items/my-catalog-item` AND the updated item is displayed

### TC-U054: Update catalog item without --from-file fails

- **Requirement:** REQ-CIT-120
- **Priority:** High
- **Type:** Unit
- **Given:** No `--from-file` flag is provided
- **When:** `dcm catalog item update my-catalog-item` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U055: Delete catalog item

- **Requirement:** REQ-CIT-090, REQ-CIT-100
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 204 for delete
- **When:** `dcm catalog item delete my-catalog-item` is executed
- **Then:** A DELETE request is sent to `/api/v1alpha1/catalog-items/my-catalog-item` AND a success message is displayed: `Catalog item "my-catalog-item" deleted successfully.`

### TC-U056: Delete catalog item without CATALOG_ITEM_ID fails

- **Requirement:** REQ-CIT-130
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm catalog item delete` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U057: Catalog item table output columns

- **Requirement:** REQ-OUT-050
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning a catalog item with all fields populated
- **When:** `dcm catalog item get my-catalog-item` is executed with `--output table`
- **Then:** The table output includes columns: ID, UID, DISPLAY NAME, CREATED

---

## 8 · Catalog Instance Commands

> **Suggested Ginkgo structure:** `Describe("Catalog Instance Commands")` with
> nested `Describe` per subcommand.

### TC-U058: Create instance from file

- **Requirement:** REQ-CIN-010, REQ-CIN-030
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U067 (generated Catalog Manager client usage)
- **Given:** A valid instance YAML file AND a mock server returning 201
- **When:** `dcm catalog instance create --from-file instance.yaml` is executed
- **Then:** A POST request is sent to `/api/v1alpha1/catalog-item-instances` AND the created instance is displayed

### TC-U059: Create instance with client-specified ID

- **Requirement:** REQ-CIN-020
- **Priority:** High
- **Type:** Unit
- **Given:** A valid instance file AND a mock server
- **When:** `dcm catalog instance create --from-file instance.yaml --id my-instance` is executed
- **Then:** The POST request includes `?id=my-instance` as a query parameter

### TC-U072: Create instance without --from-file fails

- **Requirement:** REQ-CIN-100
- **Priority:** High
- **Type:** Unit
- **Given:** No `--from-file` flag is provided
- **When:** `dcm catalog instance create` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U073: List instances

- **Requirement:** REQ-CIN-040, REQ-CIN-050
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 200 with a list of instances
- **When:** `dcm catalog instance list` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/catalog-item-instances` AND the instances are displayed

### TC-U074: List instances with pagination

- **Requirement:** REQ-CIN-040
- **Priority:** High
- **Type:** Unit
- **Transitively covers:** TC-U069 (pagination flags present)
- **Given:** A mock server
- **When:** `dcm catalog instance list --page-size 10` is executed
- **Then:** The GET request includes `page_size=10` as a query parameter

### TC-U075: Get instance

- **Requirement:** REQ-CIN-060
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 200 with an instance
- **When:** `dcm catalog instance get my-instance` is executed
- **Then:** A GET request is sent to `/api/v1alpha1/catalog-item-instances/my-instance` AND the instance is displayed

### TC-U076: Get instance without INSTANCE_ID fails

- **Requirement:** REQ-CIN-110
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm catalog instance get` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U077: Delete instance

- **Requirement:** REQ-CIN-070, REQ-CIN-080
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 204 for delete
- **When:** `dcm catalog instance delete my-instance` is executed
- **Then:** A DELETE request is sent to `/api/v1alpha1/catalog-item-instances/my-instance` AND the message `Catalog item instance "my-instance" deleted successfully.` is displayed

### TC-U078: Delete instance without INSTANCE_ID fails

- **Requirement:** REQ-CIN-110
- **Priority:** High
- **Type:** Unit
- **Given:** No positional argument is provided
- **When:** `dcm catalog instance delete` is executed
- **Then:** The CLI exits with code 2 and displays a usage error

### TC-U079: Instance table output columns

- **Requirement:** REQ-OUT-050
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning an instance with all fields populated
- **When:** `dcm catalog instance get my-instance` is executed with `--output table`
- **Then:** The table output includes columns: ID, UID, DISPLAY NAME, CATALOG ITEM, CREATED

---

## 9 · TLS Configuration

> **Suggested Ginkgo structure:** `Describe("TLS Configuration")` with `Context`
> per scenario. Tests use `net/http/httptest` with TLS-enabled servers where
> applicable.

### TC-U088: HTTPS URL enables TLS transport

- **Requirement:** REQ-XC-TLS-010
- **Priority:** High
- **Type:** Unit
- **Given:** The API Gateway URL is `https://localhost:<port>` pointing to a TLS-enabled httptest server
- **When:** `dcm policy list --tls-skip-verify` is executed
- **Then:** The request MUST succeed over TLS AND the mock server receives the request

### TC-U089: HTTP URL skips TLS and ignores TLS flags

- **Requirement:** REQ-XC-TLS-020
- **Priority:** High
- **Type:** Unit
- **Given:** The API Gateway URL is `http://localhost:<port>` AND `--tls-skip-verify` is set AND `--tls-ca-cert /some/path` is set
- **When:** `dcm policy list` is executed against a non-TLS httptest server
- **Then:** The request MUST succeed without TLS AND TLS flags MUST be silently ignored

### TC-U090: Custom CA certificate used for verification

- **Requirement:** REQ-XC-TLS-030, REQ-XC-TLS-080
- **Priority:** High
- **Type:** Unit
- **Given:** A TLS-enabled httptest server with a self-signed certificate AND the CA cert is written to a temp file
- **When:** `dcm policy list --api-gateway-url https://... --tls-ca-cert /tmp/ca.pem` is executed
- **Then:** The TLS handshake MUST succeed using the provided CA certificate

### TC-U091: Mutual TLS with client certificate and key

- **Requirement:** REQ-XC-TLS-040
- **Priority:** High
- **Type:** Unit
- **Given:** A TLS-enabled httptest server that requires client certificates AND valid client cert/key files exist
- **When:** `dcm policy list --tls-client-cert /tmp/client.pem --tls-client-key /tmp/client-key.pem --tls-ca-cert /tmp/ca.pem` is executed
- **Then:** The TLS handshake MUST succeed with mutual authentication

### TC-U092: Skip TLS verification

- **Requirement:** REQ-XC-TLS-050
- **Priority:** High
- **Type:** Unit
- **Given:** A TLS-enabled httptest server with a self-signed certificate AND no CA cert is provided
- **When:** `dcm policy list --api-gateway-url https://... --tls-skip-verify` is executed
- **Then:** The request MUST succeed despite the untrusted certificate

### TC-U093: Incomplete mTLS config — cert without key

- **Requirement:** REQ-XC-TLS-060
- **Priority:** High
- **Type:** Unit
- **Given:** `--tls-client-cert /tmp/client.pem` is provided WITHOUT `--tls-client-key`
- **When:** The CLI validates configuration
- **Then:** The CLI MUST exit with code 2 AND display a usage error indicating both cert and key are required

### TC-U094: Incomplete mTLS config — key without cert

- **Requirement:** REQ-XC-TLS-060
- **Priority:** High
- **Type:** Unit
- **Given:** `--tls-client-key /tmp/client-key.pem` is provided WITHOUT `--tls-client-cert`
- **When:** The CLI validates configuration
- **Then:** The CLI MUST exit with code 2 AND display a usage error indicating both cert and key are required

### TC-U095: Nonexistent CA cert file

- **Requirement:** REQ-XC-TLS-070
- **Priority:** High
- **Type:** Unit
- **Given:** `--tls-ca-cert /nonexistent/ca.pem` is provided AND the API Gateway URL uses `https://`
- **When:** The CLI attempts to load the CA certificate
- **Then:** The CLI MUST exit with code 1 AND display a clear error message

### TC-U096: Nonexistent client cert file

- **Requirement:** REQ-XC-TLS-070
- **Priority:** High
- **Type:** Unit
- **Given:** `--tls-client-cert /nonexistent/cert.pem` and `--tls-client-key /tmp/valid-key.pem` are provided AND the API Gateway URL uses `https://`
- **When:** The CLI attempts to load the client certificate
- **Then:** The CLI MUST exit with code 1 AND display a clear error message

### TC-U097: Default system CA bundle used when no custom CA specified

- **Requirement:** REQ-XC-TLS-080
- **Priority:** Medium
- **Type:** Unit
- **Given:** The API Gateway URL uses `https://` AND no `--tls-ca-cert` is provided
- **When:** The TLS transport is configured
- **Then:** The system default CA bundle MUST be used (RootCAs is nil in tls.Config)

### TC-U098: TLS config loaded from config file

- **Requirement:** REQ-CFG-010, REQ-XC-TLS-030
- **Priority:** Medium
- **Type:** Unit
- **Given:** A config file exists with `tls-ca-cert: /path/to/ca.pem` and `tls-skip-verify: true`
- **When:** Config is loaded
- **Then:** `TLSCACert` is `"/path/to/ca.pem"` AND `TLSSkipVerify` is `true`

### TC-U099: TLS environment variables override config file

- **Requirement:** REQ-CFG-030, REQ-CFG-040
- **Priority:** Medium
- **Type:** Unit
- **Given:** A config file has `tls-skip-verify: false` AND `DCM_TLS_SKIP_VERIFY=true` is set
- **When:** Config is loaded
- **Then:** `TLSSkipVerify` is `true`

---

## 10 · Error Handling

> **Suggested Ginkgo structure:** `Describe("Error Handling")` with `Context`
> per error type. Tests exercise error paths through command execution with
> mock servers returning error responses. TLS-related errors are covered in
> section 9 (TLS Configuration).

### TC-U080: API error displayed in table format

- **Requirement:** REQ-XC-ERR-010, REQ-XC-ERR-020
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 404 with RFC 7807 body: `{"type":"NOT_FOUND","status":404,"title":"Policy \"nonexistent\" not found.","detail":"The requested policy resource does not exist."}`
- **When:** `dcm policy get nonexistent` is executed with `--output table`
- **Then:** The CLI displays:
  ```
  Error: NOT_FOUND - Policy "nonexistent" not found.
    Status: 404
    Detail: The requested policy resource does not exist.
  ```
- **And** the CLI exits with code 1

### TC-U081: API error displayed in JSON format

- **Requirement:** REQ-XC-ERR-030
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 404 with RFC 7807 body
- **When:** `dcm policy get nonexistent -o json` is executed
- **Then:** The full Problem Details JSON object is printed to stderr

### TC-U082: API error displayed in YAML format

- **Requirement:** REQ-XC-ERR-030
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning 404 with RFC 7807 body
- **When:** `dcm policy get nonexistent -o yaml` is executed
- **Then:** The full Problem Details YAML object is printed to stderr

### TC-U083: Connection error displays clear message

- **Requirement:** REQ-XC-ERR-040
- **Priority:** High
- **Type:** Unit
- **Given:** The API Gateway URL points to a closed/non-existent server
- **When:** `dcm policy list` is executed
- **Then:** The CLI displays a connection error message AND exits with code 1

### TC-U084: Timeout error displays clear message

- **Requirement:** REQ-XC-ERR-050
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server that delays longer than the configured timeout AND `--timeout 1` is set
- **When:** `dcm policy list` is executed
- **Then:** The CLI displays a timeout error message AND exits with code 1

### TC-U085: Exit code 1 on runtime error

- **Requirement:** REQ-CLI-060, REQ-XC-ERR-060
- **Priority:** High
- **Type:** Unit
- **Given:** A mock server returning a 500 error response
- **When:** Any command is executed
- **Then:** The CLI exits with code 1

---

## Utility Test Case Index

Utility and helper functions are tested **transitively** through the
behavioural tests listed above and in the E2E test plan. Each utility
TC-ID is preserved for requirements traceability but does **not** map to a
dedicated test class or `Describe` block.

### Input File Parsing

#### TC-U060: YAML file correctly parsed

- **Requirement:** REQ-XC-INP-010, REQ-XC-INP-020
- **Priority:** High
- **Type:** Unit
- **Given:** A valid YAML file is provided via `--from-file`
- **When:** The file is parsed
- **Then:** The content is correctly deserialized to a request body
- **Referenced by:** TC-U026 (create policy from YAML)

#### TC-U061: JSON file correctly parsed

- **Requirement:** REQ-XC-INP-010, REQ-XC-INP-020
- **Priority:** High
- **Type:** Unit
- **Given:** A valid JSON file is provided via `--from-file`
- **When:** The file is parsed
- **Then:** The content is correctly deserialized to a request body
- **Referenced by:** TC-U027 (create policy from JSON)

#### TC-U062: Invalid file produces error

- **Requirement:** REQ-XC-INP-030
- **Priority:** High
- **Type:** Unit
- **Given:** The file specified by `--from-file` does not exist
- **When:** The CLI attempts to read it
- **Then:** A clear error message is displayed AND the CLI exits with code 1
- **Referenced by:** TC-U026 (tested as negative case alongside create policy)

#### TC-U063: Unreadable file content produces error

- **Requirement:** REQ-XC-INP-030
- **Priority:** High
- **Type:** Unit
- **Given:** The file specified by `--from-file` contains invalid/unparseable content
- **When:** The CLI attempts to parse it
- **Then:** A clear error message is displayed AND the CLI exits with code 1
- **Referenced by:** TC-U026 (tested as negative case alongside create policy)

### Generated Client Usage

#### TC-U064: Policy Manager client instantiated with correct URL

- **Requirement:** REQ-XC-CLI-010, REQ-XC-CLI-030
- **Priority:** High
- **Type:** Unit
- **Given:** The API Gateway URL is `http://localhost:9080`
- **When:** The Policy Manager client is created
- **Then:** The client base URL is `http://localhost:9080/api/v1alpha1`
- **Referenced by:** TC-U026 (create policy verifies request goes to correct URL path)

#### TC-U065: Catalog Manager client instantiated with correct URL

- **Requirement:** REQ-XC-CLI-020, REQ-XC-CLI-030
- **Priority:** High
- **Type:** Unit
- **Given:** The API Gateway URL is `http://localhost:9080`
- **When:** The Catalog Manager client is created
- **Then:** The client base URL is `http://localhost:9080/api/v1alpha1`
- **Referenced by:** TC-U042 (list service types verifies request goes to correct URL path)

#### TC-U066: Policy Manager generated client used for policy operations

- **Requirement:** REQ-XC-CLI-010, REQ-POL-110
- **Priority:** High
- **Type:** Unit (structural)
- **Given:** Any policy command is invoked
- **When:** The command communicates with the API
- **Then:** The generated Policy Manager client is used (verified by mock server receiving correctly structured requests)
- **Referenced by:** TC-U026 (create), TC-U030 (list), TC-U034 (get), TC-U036 (update), TC-U039 (delete)

#### TC-U067: Catalog Manager generated client used for catalog operations

- **Requirement:** REQ-XC-CLI-020, REQ-CST-050, REQ-CIT-110, REQ-CIN-090
- **Priority:** High
- **Type:** Unit (structural)
- **Given:** Any catalog command is invoked
- **When:** The command communicates with the API
- **Then:** The generated Catalog Manager client is used (verified by mock server receiving correctly structured requests)
- **Referenced by:** TC-U042 (service-type list), TC-U044 (service-type get), TC-U046 (item create), TC-U058 (instance create)

#### TC-U068: Request timeout applied to HTTP requests

- **Requirement:** REQ-XC-CLI-040
- **Priority:** High
- **Type:** Unit
- **Given:** The timeout is configured to 30 seconds
- **When:** A request is made via the generated client
- **Then:** The request context has a 30-second deadline
- **Referenced by:** TC-U084 (timeout error test exercises this path)

### Pagination

#### TC-U069: Pagination flags available on all list commands

- **Requirement:** REQ-XC-PAG-010
- **Priority:** High
- **Type:** Unit
- **Given:** Any list command (`policy list`, `catalog service-type list`, `catalog item list`, `catalog instance list`)
- **When:** `--help` is displayed
- **Then:** `--page-size` and `--page-token` flags are listed
- **Referenced by:** TC-U033 (policy list pagination), TC-U043 (service-type list pagination), TC-U074 (instance list pagination)

#### TC-U070: Pagination parameters passed as query parameters

- **Requirement:** REQ-XC-PAG-020
- **Priority:** High
- **Type:** Unit
- **Given:** `dcm policy list --page-size 10 --page-token abc123` is invoked
- **When:** The request is sent
- **Then:** The query parameters include `page_size=10` and `page_token=abc123`
- **Referenced by:** TC-U033 (policy list with pagination)

#### TC-U071: next_page_token surfaced according to output format

- **Requirement:** REQ-XC-PAG-030
- **Priority:** High
- **Type:** Unit
- **Given:** A list response includes `next_page_token`
- **When:** The response is formatted
- **Then:** Table output shows a pagination hint AND JSON/YAML output includes the token in the response object
- **Referenced by:** TC-U013 (table pagination hint), TC-U014 (JSON pagination token), TC-U015 (YAML pagination token)

### Cobra Framework

#### TC-U086: Cobra framework used

- **Requirement:** REQ-CLI-010
- **Priority:** High
- **Type:** Unit (structural)
- **Given:** The CLI binary is built
- **When:** `dcm --help` is invoked
- **Then:** Cobra-generated help text is displayed
- **Referenced by:** TC-U019 (root command help output)

#### TC-U087: Viper used for configuration management

- **Requirement:** REQ-CFG-060
- **Priority:** High
- **Type:** Unit (structural)
- **Given:** The configuration package imports and uses Viper
- **When:** Configuration is loaded
- **Then:** Viper is used for config resolution with file, env, and flag binding
- **Referenced by:** TC-U001–TC-U008 (configuration tests exercise Viper indirectly)

---

## Coverage Matrix

| Requirement     | Test Cases                                          | Status  |
|-----------------|-----------------------------------------------------|---------|
| REQ-CLI-010     | TC-U086 (via TC-U019)                               | Covered |
| REQ-CLI-020     | TC-U019, TC-U021                                    | Covered |
| REQ-CLI-030     | TC-U019                                             | Covered |
| REQ-CLI-040     | TC-U020                                             | Covered |
| REQ-CLI-050     | TC-U021                                             | Covered |
| REQ-CLI-060     | TC-U022, TC-U023, TC-U085                           | Covered |
| REQ-CLI-070     | TC-U022 (entry point bootstraps and executes)        | Covered |
| REQ-CFG-010     | TC-U001                                             | Covered |
| REQ-CFG-020     | TC-U006, TC-U007                                    | Covered |
| REQ-CFG-030     | TC-U002, TC-U008                                    | Covered |
| REQ-CFG-040     | TC-U002, TC-U003                                    | Covered |
| REQ-CFG-050     | TC-U004                                             | Covered |
| REQ-CFG-060     | TC-U087 (via TC-U001–TC-U008)                       | Covered |
| REQ-CFG-070     | TC-U005                                             | Covered |
| REQ-OUT-010     | TC-U009, TC-U011, TC-U012                           | Covered |
| REQ-OUT-020     | TC-U009 (table is default)                           | Covered |
| REQ-OUT-030     | TC-U017                                             | Covered |
| REQ-OUT-040     | TC-U009, TC-U010, TC-U018                           | Covered |
| REQ-OUT-050     | TC-U009, TC-U010, TC-U041, TC-U057, TC-U079         | Covered |
| REQ-OUT-060     | TC-U011                                             | Covered |
| REQ-OUT-070     | TC-U012                                             | Covered |
| REQ-OUT-080     | TC-U014, TC-U015                                    | Covered |
| REQ-OUT-090     | TC-U013, TC-U016                                    | Covered |
| REQ-OUT-100     | TC-U017                                             | Covered |
| REQ-POL-010     | TC-U026, TC-U027                                    | Covered |
| REQ-POL-020     | TC-U028                                             | Covered |
| REQ-POL-030     | TC-U026                                             | Covered |
| REQ-POL-040     | TC-U030, TC-U031, TC-U032, TC-U033                  | Covered |
| REQ-POL-050     | TC-U030                                             | Covered |
| REQ-POL-060     | TC-U034                                             | Covered |
| REQ-POL-070     | TC-U036                                             | Covered |
| REQ-POL-080     | TC-U036                                             | Covered |
| REQ-POL-090     | TC-U039                                             | Covered |
| REQ-POL-100     | TC-U039                                             | Covered |
| REQ-POL-110     | TC-U066 (via TC-U026, TC-U030, TC-U034, TC-U036, TC-U039) | Covered |
| REQ-POL-120     | TC-U029, TC-U037                                    | Covered |
| REQ-POL-130     | TC-U035, TC-U038, TC-U040                           | Covered |
| REQ-CST-010     | TC-U042, TC-U043                                    | Covered |
| REQ-CST-020     | TC-U042                                             | Covered |
| REQ-CST-030     | TC-U044                                             | Covered |
| REQ-CST-040     | TC-U045                                             | Covered |
| REQ-CST-050     | TC-U067 (via TC-U042, TC-U044)                      | Covered |
| REQ-CIT-010     | TC-U046                                             | Covered |
| REQ-CIT-020     | TC-U047                                             | Covered |
| REQ-CIT-030     | TC-U046                                             | Covered |
| REQ-CIT-040     | TC-U049, TC-U050                                    | Covered |
| REQ-CIT-050     | TC-U049                                             | Covered |
| REQ-CIT-060     | TC-U051                                             | Covered |
| REQ-CIT-070     | TC-U053                                             | Covered |
| REQ-CIT-080     | TC-U053                                             | Covered |
| REQ-CIT-090     | TC-U055                                             | Covered |
| REQ-CIT-100     | TC-U055                                             | Covered |
| REQ-CIT-110     | TC-U067 (via TC-U046, TC-U049, TC-U051, TC-U053, TC-U055) | Covered |
| REQ-CIT-120     | TC-U048, TC-U054                                    | Covered |
| REQ-CIT-130     | TC-U052, TC-U056                                    | Covered |
| REQ-CIN-010     | TC-U058                                             | Covered |
| REQ-CIN-020     | TC-U059                                             | Covered |
| REQ-CIN-030     | TC-U058                                             | Covered |
| REQ-CIN-040     | TC-U073, TC-U074                                    | Covered |
| REQ-CIN-050     | TC-U073                                             | Covered |
| REQ-CIN-060     | TC-U075                                             | Covered |
| REQ-CIN-070     | TC-U077                                             | Covered |
| REQ-CIN-080     | TC-U077                                             | Covered |
| REQ-CIN-090     | TC-U067 (via TC-U058, TC-U073, TC-U075, TC-U077)    | Covered |
| REQ-CIN-100     | TC-U072                                             | Covered |
| REQ-CIN-110     | TC-U076, TC-U078                                    | Covered |
| REQ-VER-010     | TC-U024                                             | Covered |
| REQ-VER-020     | TC-U024                                             | Covered |
| REQ-VER-030     | TC-U025                                             | Covered |
| REQ-XC-ERR-010  | TC-U080                                             | Covered |
| REQ-XC-ERR-020  | TC-U080                                             | Covered |
| REQ-XC-ERR-030  | TC-U081, TC-U082                                    | Covered |
| REQ-XC-ERR-040  | TC-U083                                             | Covered |
| REQ-XC-ERR-050  | TC-U084                                             | Covered |
| REQ-XC-ERR-060  | TC-U085                                             | Covered |
| REQ-XC-INP-010  | TC-U060 (via TC-U026), TC-U061 (via TC-U027)         | Covered |
| REQ-XC-INP-020  | TC-U060 (via TC-U026), TC-U061 (via TC-U027)         | Covered |
| REQ-XC-INP-030  | TC-U062 (via TC-U026), TC-U063 (via TC-U026)         | Covered |
| REQ-XC-CLI-010  | TC-U064 (via TC-U026), TC-U066 (via TC-U026/U030/U034/U036/U039) | Covered |
| REQ-XC-CLI-020  | TC-U065 (via TC-U042), TC-U067 (via TC-U042/U044/U046/U058) | Covered |
| REQ-XC-CLI-030  | TC-U064 (via TC-U026), TC-U065 (via TC-U042)         | Covered |
| REQ-XC-CLI-040  | TC-U068 (via TC-U084)                               | Covered |
| REQ-XC-CLI-050  | TC-U088, TC-U090, TC-U091                            | Covered |
| REQ-XC-PAG-010  | TC-U069 (via TC-U033, TC-U043, TC-U074)              | Covered |
| REQ-XC-PAG-020  | TC-U070 (via TC-U033)                               | Covered |
| REQ-XC-PAG-030  | TC-U071 (via TC-U013, TC-U014, TC-U015)              | Covered |
| REQ-XC-TLS-010  | TC-U088                                             | Covered |
| REQ-XC-TLS-020  | TC-U089                                             | Covered |
| REQ-XC-TLS-030  | TC-U090                                             | Covered |
| REQ-XC-TLS-040  | TC-U091                                             | Covered |
| REQ-XC-TLS-050  | TC-U092                                             | Covered |
| REQ-XC-TLS-060  | TC-U093, TC-U094                                    | Covered |
| REQ-XC-TLS-070  | TC-U095, TC-U096                                    | Covered |
| REQ-XC-TLS-080  | TC-U090, TC-U097                                    | Covered |

**Total:** 80 test case IDs — 58 in behavioural test classes, 22 in the utility
index (tested transitively through higher-level behavioural tests).

---

## Notes

- **Table-driven tests:** TC-U008 (environment variables) should be implemented as Ginkgo `DescribeTable` / `Entry` for conciseness.
- **Mock HTTP servers:** Command tests (TC-U026–TC-U085) use `net/http/httptest.NewServer` to mock generated client HTTP calls. The mock server validates request method, path, and query parameters, then returns canned responses.
- **Cobra command testing:** Tests create the root command via `NewRootCommand()`, set args via `cmd.SetArgs()`, and capture output via `cmd.SetOut()` / `cmd.SetErr()`. Exit codes are verified by inspecting the error returned from `cmd.Execute()`.
- **Temp files:** Tests requiring `--from-file` create temporary YAML/JSON files using `os.CreateTemp` and clean up via `t.Cleanup` or Ginkgo's `DeferCleanup`.
- **Environment isolation:** Tests that set environment variables must save and restore the original values (or use `t.Setenv` / Ginkgo's `DeferCleanup`) to avoid polluting other tests.
- **Config file isolation:** Configuration tests must use `--config` pointing to temp files or unset `DCM_CONFIG` to avoid loading the developer's actual `~/.dcm/config.yaml`.
