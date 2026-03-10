# Plan: DCM CLI

## 1. Overview

The DCM CLI (`dcm`) is a Go-based command-line tool for interacting with the
DCM (Data Center Management) control plane. It communicates exclusively through
the API Gateway (KrakenD on port 9080) to reach the Policy Manager and Catalog
Manager backends. The CLI uses generated clients from `policy-manager/pkg/client`
and `catalog-manager/pkg/client` (oapi-codegen generated) as Go module
dependencies.

**Version scope (v1alpha1):**

- Policy CRUD operations (create, list, get, update, delete)
- Service type read operations (list, get)
- Catalog item CRUD operations (create, list, get, update, delete)
- Catalog item instance operations (create, list, get, delete)
- Version display
- Output formatting (table, JSON, YAML)
- Configuration via file, environment variables, and flags
- Pagination support for list operations
- TLS support with custom CA certificates, client certificates (mTLS), and skip-verify
- Container image for distribution

**Out of scope (v1alpha1):**

- Authentication and authorization (no auth in v1alpha1 API Gateway)
- Interactive/wizard-style resource creation
- Watch/streaming operations
- Shell autocompletion generation
- Plugin/extension system
- Offline mode or local caching
- Bulk operations
- Resource diff/dry-run
- Health check command for API Gateway connectivity

**Reference documents:**

- [DCM CLI Specification](.ai/specs/dcm-cli.spec.md)
- Policy Manager OpenAPI: `api/v1alpha1/openapi.yaml` in dcm-policy-manager
- Catalog Manager OpenAPI: `api/v1alpha1/openapi.yaml` in dcm-catalog-manager
- [AEP Standards](https://aep.dev/) - API Enhancement Proposals
- RFC 7807 - Problem Details for HTTP APIs
- RFC 7396 - JSON Merge Patch

---

## 2. Architecture

```
┌─────────┐              ┌──────────────────┐       ┌──────────────────┐
│         │              │                  │       │ Policy Manager   │
│  dcm    │─────────────▶│  API Gateway     │──────▶│ (port 8080)      │
│  CLI    │ HTTP / HTTPS │  (KrakenD 9080)  │       └──────────────────┘
│         │              │                  │       ┌──────────────────┐
└─────────┘              │                  │──────▶│ Catalog Manager  │
                         └──────────────────┘       │ (port 8080)      │
                                                    └──────────────────┘
```

```
dcm-cli/
├── cmd/dcm/main.go                    ← Entry point, root command setup
├── internal/
│   ├── config/                        ← Configuration loading/saving
│   ├── output/                        ← Output formatting (table/json/yaml)
│   ├── commands/                      ← Cobra command definitions
│   │   ├── root.go                    ← Root command, global flags
│   │   ├── version.go                 ← Version command
│   │   ├── policy.go                  ← Policy command group
│   │   ├── catalog_service_type.go    ← Service-type command group
│   │   ├── catalog_item.go            ← Catalog item command group
│   │   └── catalog_instance.go        ← Catalog instance command group
│   └── version/                       ← Build-time version info
├── test/e2e/                          ← E2E tests (build tag: e2e)
├── Makefile
├── Containerfile
├── go.mod
└── tools.go
```

---

## 3. Topic Dependency Graph

| # | Topic                          | Prefix | Depends On |
|---|--------------------------------|--------|------------|
| 1 | CLI Framework & Entry Point    | CLI    | -          |
| 2 | Configuration Management       | CFG    | -          |
| 3 | Output Formatting              | OUT    | -          |
| 4 | Policy Commands                | POL    | 1, 2, 3    |
| 5 | Catalog Service-Type Commands  | CST    | 1, 2, 3    |
| 6 | Catalog Item Commands          | CIT    | 1, 2, 3    |
| 7 | Catalog Instance Commands      | CIN    | 1, 2, 3    |
| 8 | Version Command                | VER    | 1          |

```
Topic 1: CLI Framework           (independent)
Topic 2: Configuration           (independent)
Topic 3: Output Formatting       (independent)
  |         |         |
  +---------+---------+---> Topic 4: Policy Commands         (depends on 1, 2, 3)
  +---------+---------+---> Topic 5: Service-Type Commands   (depends on 1, 2, 3)
  +---------+---------+---> Topic 6: Catalog Item Commands   (depends on 1, 2, 3)
  +---------+---------+---> Topic 7: Catalog Instance Cmds   (depends on 1, 2, 3)
  |
  +-----------------------> Topic 8: Version Command         (depends on 1)
```

Topics 1, 2, and 3 can be delivered in parallel. Topics 4-7 depend on all
three foundation topics. Topic 8 depends only on Topic 1.

> **Note:** Command tests use `net/http/httptest` to mock generated client HTTP
> calls. No real API Gateway is needed for unit tests.

---

## 4. Topic Specifications

### 4.1 CLI Framework & Entry Point

#### Overview

Foundation layer: Cobra-based CLI with root command, global flag registration,
and entry point in `cmd/dcm/main.go`. Wires together configuration, output
formatting, and all command groups.

Out of scope: shell autocompletion, plugin system, interactive prompts.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-CLI-010 | The CLI MUST use Cobra as the command framework | MUST | |
| REQ-CLI-020 | The CLI MUST define a root command `dcm` with global flags | MUST | |
| REQ-CLI-030 | The root command MUST register all subcommand groups: `policy`, `catalog`, `version` | MUST | |
| REQ-CLI-040 | The `catalog` command MUST register subcommand groups: `service-type`, `item`, `instance` | MUST | |
| REQ-CLI-050 | Global flags MUST include `--api-gateway-url`, `--output`/`-o`, `--timeout`, `--config`, `--tls-ca-cert`, `--tls-client-cert`, `--tls-client-key`, `--tls-skip-verify` | MUST | |
| REQ-CLI-060 | The CLI MUST exit with code 0 on success, 1 on runtime errors, 2 on usage errors | MUST | |
| REQ-CLI-070 | The entry point (`cmd/dcm/main.go`) MUST bootstrap the root command and execute it | MUST | |

#### Acceptance Criteria

##### AC-CLI-010: Cobra framework usage

- **Validates:** REQ-CLI-010
- **Given** the CLI binary is built
- **When** `dcm --help` is invoked
- **Then** Cobra-generated help text MUST be displayed

##### AC-CLI-020: Root command with global flags

- **Validates:** REQ-CLI-020, REQ-CLI-050
- **Given** the CLI is invoked
- **When** `dcm --help` is run
- **Then** the global flags `--api-gateway-url`, `--output`/`-o`, `--timeout`, `--config`, `--tls-ca-cert`, `--tls-client-cert`, `--tls-client-key`, and `--tls-skip-verify` MUST be listed

##### AC-CLI-030: Subcommand registration

- **Validates:** REQ-CLI-030, REQ-CLI-040
- **Given** the CLI is invoked
- **When** `dcm --help` is run
- **Then** subcommands `policy`, `catalog`, and `version` MUST be listed
- **And** `dcm catalog --help` MUST list `service-type`, `item`, and `instance`

##### AC-CLI-040: Exit code on success

- **Validates:** REQ-CLI-060
- **Given** a command completes successfully
- **When** the process exits
- **Then** the exit code MUST be 0

##### AC-CLI-050: Exit code on runtime error

- **Validates:** REQ-CLI-060
- **Given** a command encounters a runtime error (e.g., API error, connection error)
- **When** the process exits
- **Then** the exit code MUST be 1

##### AC-CLI-060: Exit code on usage error

- **Validates:** REQ-CLI-060
- **Given** a command is invoked with invalid flags or missing arguments
- **When** the process exits
- **Then** the exit code MUST be 2

##### AC-CLI-070: Entry point execution

- **Validates:** REQ-CLI-070
- **Given** `cmd/dcm/main.go` is the entry point
- **When** the binary is executed
- **Then** the root command MUST be created and executed

#### Dependencies

None - independently deliverable.

---

### 4.2 Configuration Management

#### Overview

Manages CLI configuration with file persistence, environment variable overrides,
and command-line flag overrides. Uses Viper for configuration resolution.

Out of scope: config file creation wizard, config validation command,
profile/context support.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-CFG-010 | The CLI MUST load configuration from the config file at `~/.dcm/config.yaml` by default | MUST | |
| REQ-CFG-020 | The config file path MUST be overridable via `--config` flag or `DCM_CONFIG` environment variable | MUST | |
| REQ-CFG-030 | The CLI MUST support environment variables: `DCM_API_GATEWAY_URL`, `DCM_OUTPUT_FORMAT`, `DCM_TIMEOUT`, `DCM_CONFIG`, `DCM_TLS_CA_CERT`, `DCM_TLS_CLIENT_CERT`, `DCM_TLS_CLIENT_KEY`, `DCM_TLS_SKIP_VERIFY` | MUST | |
| REQ-CFG-040 | Configuration precedence MUST be: CLI flags > environment variables > config file > built-in defaults | MUST | |
| REQ-CFG-050 | Built-in defaults MUST be: `api-gateway-url=http://localhost:9080`, `output-format=table`, `timeout=30`, `tls-ca-cert=""`, `tls-client-cert=""`, `tls-client-key=""`, `tls-skip-verify=false` | MUST | |
| REQ-CFG-060 | The CLI MUST use Viper for configuration management | MUST | |
| REQ-CFG-070 | The CLI MUST NOT fail if the config file does not exist; defaults MUST be used | MUST | |

#### Configuration Reference

| Config Key | Env Var | Flag | Default | Description |
|------------|---------|------|---------|-------------|
| api-gateway-url | DCM_API_GATEWAY_URL | --api-gateway-url | http://localhost:9080 | API Gateway base URL |
| output-format | DCM_OUTPUT_FORMAT | --output / -o | table | Output format (table, json, yaml) |
| timeout | DCM_TIMEOUT | --timeout | 30 | Request timeout in seconds |
| - | DCM_CONFIG | --config | ~/.dcm/config.yaml | Config file path |
| tls-ca-cert | DCM_TLS_CA_CERT | --tls-ca-cert | (empty) | Path to CA certificate file for TLS verification |
| tls-client-cert | DCM_TLS_CLIENT_CERT | --tls-client-cert | (empty) | Path to client certificate file for mTLS |
| tls-client-key | DCM_TLS_CLIENT_KEY | --tls-client-key | (empty) | Path to client private key file for mTLS |
| tls-skip-verify | DCM_TLS_SKIP_VERIFY | --tls-skip-verify | false | Skip TLS certificate verification |

#### Acceptance Criteria

##### AC-CFG-010: Config file loading

- **Validates:** REQ-CFG-010
- **Given** a config file exists at `~/.dcm/config.yaml` with `api-gateway-url: http://custom:9080`
- **When** the CLI is invoked without flags or env vars
- **Then** the API Gateway URL MUST be `http://custom:9080`

##### AC-CFG-020: Custom config file path

- **Validates:** REQ-CFG-020
- **Given** a config file exists at `/tmp/dcm.yaml`
- **When** `dcm --config /tmp/dcm.yaml policy list` is invoked
- **Then** configuration MUST be loaded from `/tmp/dcm.yaml`

##### AC-CFG-030: Environment variable override

- **Validates:** REQ-CFG-030
- **Given** `DCM_API_GATEWAY_URL=http://env:9080` is set
- **And** the config file has `api-gateway-url: http://file:9080`
- **When** the CLI is invoked without `--api-gateway-url`
- **Then** the API Gateway URL MUST be `http://env:9080`

##### AC-CFG-040: CLI flag override

- **Validates:** REQ-CFG-040
- **Given** `DCM_API_GATEWAY_URL=http://env:9080` is set
- **And** the config file has `api-gateway-url: http://file:9080`
- **When** `dcm --api-gateway-url http://flag:9080 policy list` is invoked
- **Then** the API Gateway URL MUST be `http://flag:9080`

##### AC-CFG-050: Built-in defaults

- **Validates:** REQ-CFG-050
- **Given** no config file exists and no env vars are set
- **When** the CLI resolves configuration
- **Then** `api-gateway-url` MUST be `http://localhost:9080`
- **And** `output-format` MUST be `table`
- **And** `timeout` MUST be `30`
- **And** `tls-ca-cert` MUST be `""`
- **And** `tls-client-cert` MUST be `""`
- **And** `tls-client-key` MUST be `""`
- **And** `tls-skip-verify` MUST be `false`

##### AC-CFG-060: Missing config file

- **Validates:** REQ-CFG-070
- **Given** no config file exists at `~/.dcm/config.yaml`
- **When** the CLI is invoked
- **Then** the CLI MUST NOT fail
- **And** built-in defaults MUST be used

#### Dependencies

None - independently deliverable.

---

### 4.3 Output Formatting

#### Overview

Formats API responses for display in table, JSON, or YAML formats. Provides a
`Formatter` interface used by all commands to render single resources, resource
lists, and status messages.

Out of scope: color output, custom column selection, template-based formatting.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-OUT-010 | The CLI MUST support three output formats: `table`, `json`, `yaml` | MUST | |
| REQ-OUT-020 | The default output format MUST be `table` | MUST | |
| REQ-OUT-030 | The output format MUST be selectable via `--output`/`-o` flag | MUST | |
| REQ-OUT-040 | The `Formatter` interface MUST support `FormatOne` (single resource), `FormatList` (resource list with pagination), and `FormatMessage` (status message) | MUST | |
| REQ-OUT-050 | Table output for list commands MUST display resources in a tabular format with column headers | MUST | |
| REQ-OUT-060 | JSON output MUST produce valid, parseable JSON | MUST | |
| REQ-OUT-070 | YAML output MUST produce valid, parseable YAML | MUST | |
| REQ-OUT-080 | For JSON/YAML list output, `next_page_token` MUST be included in the response object when present | MUST | |
| REQ-OUT-090 | For table list output, when `next_page_token` is present, a pagination hint MUST be displayed showing the next command to run | MUST | |
| REQ-OUT-100 | The CLI MUST reject invalid output format values with a usage error | MUST | |

#### Acceptance Criteria

##### AC-OUT-010: Table output for single resource

- **Validates:** REQ-OUT-050
- **Given** a single resource is returned from the API
- **When** the output format is `table`
- **Then** the resource MUST be displayed in tabular format with column headers

##### AC-OUT-020: JSON output

- **Validates:** REQ-OUT-060
- **Given** a resource is returned from the API
- **When** the output format is `json`
- **Then** valid JSON MUST be printed to stdout

##### AC-OUT-030: YAML output

- **Validates:** REQ-OUT-070
- **Given** a resource is returned from the API
- **When** the output format is `yaml`
- **Then** valid YAML MUST be printed to stdout

##### AC-OUT-040: Pagination hint in table output

- **Validates:** REQ-OUT-090
- **Given** a list response includes `next_page_token`
- **When** the output format is `table`
- **Then** a line MUST be displayed showing the command to fetch the next page
- **Example:** `Next page: dcm policy list --page-size 2 --page-token eyJvZmZzZXQiOjJ9`

##### AC-OUT-050: Pagination token in JSON/YAML output

- **Validates:** REQ-OUT-080
- **Given** a list response includes `next_page_token`
- **When** the output format is `json` or `yaml`
- **Then** `next_page_token` MUST be included in the output object

##### AC-OUT-060: Invalid output format

- **Validates:** REQ-OUT-100
- **Given** `--output invalid` is provided
- **When** the CLI processes the flag
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-OUT-070: Status message formatting

- **Validates:** REQ-OUT-040
- **Given** a delete command succeeds
- **When** `FormatMessage` is called
- **Then** the message MUST be displayed to stdout (e.g., `Policy "my-policy" deleted successfully.`)

#### Dependencies

None - independently deliverable.

---

### 4.4 Policy Commands

#### Overview

Implement the `dcm policy` command group with CRUD subcommands: `create`,
`list`, `get`, `update`, `delete`. Each command uses the generated Policy
Manager client to communicate with the API Gateway.

Out of scope: policy validation/dry-run, policy diff, bulk policy operations.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-POL-010 | `dcm policy create` MUST create a policy from a YAML or JSON file specified by `--from-file` | MUST | |
| REQ-POL-020 | `dcm policy create` MUST support an optional `--id` flag for client-specified policy ID | MUST | |
| REQ-POL-030 | `dcm policy create` MUST display the created policy in the configured output format | MUST | |
| REQ-POL-040 | `dcm policy list` MUST list policies with optional `--filter`, `--order-by`, `--page-size`, `--page-token` flags | MUST | |
| REQ-POL-050 | `dcm policy list` MUST display policies in the configured output format | MUST | |
| REQ-POL-060 | `dcm policy get` MUST accept a `POLICY_ID` positional argument and display the policy | MUST | |
| REQ-POL-070 | `dcm policy update` MUST accept a `POLICY_ID` positional argument and `--from-file` flag with a patch file (JSON Merge Patch - RFC 7396) | MUST | |
| REQ-POL-080 | `dcm policy update` MUST display the updated policy in the configured output format | MUST | |
| REQ-POL-090 | `dcm policy delete` MUST accept a `POLICY_ID` positional argument and delete the policy | MUST | |
| REQ-POL-100 | `dcm policy delete` MUST display a success message on deletion | MUST | |
| REQ-POL-110 | `dcm policy create` and `dcm policy update` MUST use the generated Policy Manager client | MUST | |
| REQ-POL-120 | `--from-file` MUST be required for `create` and `update` commands | MUST | |
| REQ-POL-130 | Missing `POLICY_ID` argument for `get`, `update`, `delete` MUST result in a usage error (exit code 2) | MUST | |

#### Table Output Columns

```
ID          DISPLAY NAME        TYPE    PRIORITY  ENABLED  CREATED
my-policy   Require CPU Limits  GLOBAL  100       true     2026-03-09T10:00:00Z
```

#### Acceptance Criteria

##### AC-POL-010: Create policy from file

- **Validates:** REQ-POL-010, REQ-POL-030
- **Given** a valid policy YAML file `policy.yaml`
- **When** `dcm policy create --from-file policy.yaml` is invoked
- **Then** a POST request MUST be sent to `/api/v1alpha1/policies`
- **And** the created policy MUST be displayed in the configured output format

##### AC-POL-020: Create policy with client-specified ID

- **Validates:** REQ-POL-020
- **Given** a valid policy file
- **When** `dcm policy create --from-file policy.yaml --id my-policy` is invoked
- **Then** the POST request MUST include `?id=my-policy` as a query parameter

##### AC-POL-030: Create policy from JSON file

- **Validates:** REQ-POL-010
- **Given** a valid policy JSON file `policy.json`
- **When** `dcm policy create --from-file policy.json` is invoked
- **Then** the policy MUST be created successfully

##### AC-POL-040: List policies

- **Validates:** REQ-POL-040, REQ-POL-050
- **Given** policies exist in the system
- **When** `dcm policy list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/policies`
- **And** the policies MUST be displayed in the configured output format

##### AC-POL-050: List policies with filter

- **Validates:** REQ-POL-040
- **Given** policies exist in the system
- **When** `dcm policy list --filter "policy_type='GLOBAL'"` is invoked
- **Then** the GET request MUST include the filter as a query parameter

##### AC-POL-060: List policies with ordering

- **Validates:** REQ-POL-040
- **Given** policies exist in the system
- **When** `dcm policy list --order-by "priority asc"` is invoked
- **Then** the GET request MUST include the order_by as a query parameter

##### AC-POL-070: List policies with pagination

- **Validates:** REQ-POL-040
- **Given** policies exist in the system
- **When** `dcm policy list --page-size 10` is invoked
- **Then** the GET request MUST include `page_size=10` as a query parameter

##### AC-POL-080: Get policy

- **Validates:** REQ-POL-060
- **Given** a policy with ID `my-policy` exists
- **When** `dcm policy get my-policy` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/policies/my-policy`
- **And** the policy MUST be displayed in the configured output format

##### AC-POL-090: Update policy

- **Validates:** REQ-POL-070, REQ-POL-080
- **Given** a policy with ID `my-policy` exists
- **And** a valid patch file `patch.yaml`
- **When** `dcm policy update my-policy --from-file patch.yaml` is invoked
- **Then** a PATCH request MUST be sent to `/api/v1alpha1/policies/my-policy`
- **And** the updated policy MUST be displayed in the configured output format

##### AC-POL-100: Delete policy

- **Validates:** REQ-POL-090, REQ-POL-100
- **Given** a policy with ID `my-policy` exists
- **When** `dcm policy delete my-policy` is invoked
- **Then** a DELETE request MUST be sent to `/api/v1alpha1/policies/my-policy`
- **And** the message `Policy "my-policy" deleted successfully.` MUST be displayed

##### AC-POL-110: Create without --from-file

- **Validates:** REQ-POL-120
- **Given** no `--from-file` flag is provided
- **When** `dcm policy create` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-POL-120: Get without POLICY_ID

- **Validates:** REQ-POL-130
- **Given** no positional argument is provided
- **When** `dcm policy get` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-POL-140: List policies returns empty list

- **Validates:** REQ-POL-040, REQ-POL-050
- **Given** no policies exist in the system
- **When** `dcm policy list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/policies`
- **And** an empty result MUST be displayed (empty table with headers only, or empty JSON array/YAML list)

##### AC-POL-150: Get non-existent policy

- **Validates:** REQ-POL-060, REQ-XC-ERR-010
- **Given** no policy with ID `nonexistent` exists
- **When** `dcm policy get nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-POL-160: Update non-existent policy

- **Validates:** REQ-POL-070, REQ-XC-ERR-010
- **Given** no policy with ID `nonexistent` exists
- **And** a valid patch file `patch.yaml`
- **When** `dcm policy update nonexistent --from-file patch.yaml` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-POL-170: Delete non-existent policy

- **Validates:** REQ-POL-090, REQ-XC-ERR-010
- **Given** no policy with ID `nonexistent` exists
- **When** `dcm policy delete nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-POL-180: Create policy server error

- **Validates:** REQ-POL-010, REQ-XC-ERR-010
- **Given** a valid policy file `policy.yaml`
- **When** `dcm policy create --from-file policy.yaml` is invoked
- **And** the API returns a server-side error (e.g., 500 Internal Server Error or 409 Conflict) with RFC 7807 body
- **Then** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-POL-130: Generated client usage

- **Validates:** REQ-POL-110
- **Given** any policy command is invoked
- **When** the command communicates with the API
- **Then** the generated Policy Manager client MUST be used (no hand-written HTTP calls)

#### Dependencies

Depends on Topic 1 (CLI Framework), Topic 2 (Configuration), Topic 3 (Output
Formatting).

---

### 4.5 Catalog Service-Type Commands

#### Overview

Implement the `dcm catalog service-type` command group with read-only
subcommands: `list` and `get`. Service types are managed by the Catalog Manager
and are not user-creatable via the CLI.

Out of scope: service-type create/update/delete (managed by system).

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-CST-010 | `dcm catalog service-type list` MUST list service types with optional `--page-size`, `--page-token` flags | MUST | |
| REQ-CST-020 | `dcm catalog service-type list` MUST display service types in the configured output format | MUST | |
| REQ-CST-030 | `dcm catalog service-type get` MUST accept a `SERVICE_TYPE_ID` positional argument and display the service type | MUST | |
| REQ-CST-040 | Missing `SERVICE_TYPE_ID` argument for `get` MUST result in a usage error (exit code 2) | MUST | |
| REQ-CST-050 | All service-type commands MUST use the generated Catalog Manager client | MUST | |

#### Acceptance Criteria

##### AC-CST-010: List service types

- **Validates:** REQ-CST-010, REQ-CST-020
- **Given** service types exist in the system
- **When** `dcm catalog service-type list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/service-types`
- **And** the service types MUST be displayed in the configured output format

##### AC-CST-020: List service types with pagination

- **Validates:** REQ-CST-010
- **Given** service types exist in the system
- **When** `dcm catalog service-type list --page-size 5` is invoked
- **Then** the GET request MUST include `page_size=5` as a query parameter

##### AC-CST-030: Get service type

- **Validates:** REQ-CST-030
- **Given** a service type with ID `my-service-type` exists
- **When** `dcm catalog service-type get my-service-type` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/service-types/my-service-type`
- **And** the service type MUST be displayed in the configured output format

##### AC-CST-040: Get without SERVICE_TYPE_ID

- **Validates:** REQ-CST-040
- **Given** no positional argument is provided
- **When** `dcm catalog service-type get` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-CST-060: List service types returns empty list

- **Validates:** REQ-CST-010, REQ-CST-020
- **Given** no service types exist in the system
- **When** `dcm catalog service-type list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/service-types`
- **And** an empty result MUST be displayed (empty table with headers only, or empty JSON array/YAML list)

##### AC-CST-070: Get non-existent service type

- **Validates:** REQ-CST-030, REQ-XC-ERR-010
- **Given** no service type with ID `nonexistent` exists
- **When** `dcm catalog service-type get nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-CST-050: Generated client usage

- **Validates:** REQ-CST-050
- **Given** any service-type command is invoked
- **When** the command communicates with the API
- **Then** the generated Catalog Manager client MUST be used

#### Dependencies

Depends on Topic 1 (CLI Framework), Topic 2 (Configuration), Topic 3 (Output
Formatting).

---

### 4.6 Catalog Item Commands

#### Overview

Implement the `dcm catalog item` command group with CRUD subcommands: `create`,
`list`, `get`, `update`, `delete`. Each command uses the generated Catalog
Manager client.

Out of scope: catalog item validation, catalog item versioning.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-CIT-010 | `dcm catalog item create` MUST create a catalog item from a YAML or JSON file specified by `--from-file` | MUST | |
| REQ-CIT-020 | `dcm catalog item create` MUST support an optional `--id` flag for client-specified catalog item ID | MUST | |
| REQ-CIT-030 | `dcm catalog item create` MUST display the created catalog item in the configured output format | MUST | |
| REQ-CIT-040 | `dcm catalog item list` MUST list catalog items with optional `--service-type`, `--page-size`, `--page-token` flags | MUST | |
| REQ-CIT-050 | `dcm catalog item list` MUST display catalog items in the configured output format | MUST | |
| REQ-CIT-060 | `dcm catalog item get` MUST accept a `CATALOG_ITEM_ID` positional argument and display the catalog item | MUST | |
| REQ-CIT-070 | `dcm catalog item update` MUST accept a `CATALOG_ITEM_ID` positional argument and `--from-file` flag with a patch file (JSON Merge Patch) | MUST | |
| REQ-CIT-080 | `dcm catalog item update` MUST display the updated catalog item in the configured output format | MUST | |
| REQ-CIT-090 | `dcm catalog item delete` MUST accept a `CATALOG_ITEM_ID` positional argument and delete the catalog item | MUST | |
| REQ-CIT-100 | `dcm catalog item delete` MUST display a success message on deletion | MUST | |
| REQ-CIT-110 | All catalog item commands MUST use the generated Catalog Manager client | MUST | |
| REQ-CIT-120 | `--from-file` MUST be required for `create` and `update` commands | MUST | |
| REQ-CIT-130 | Missing positional arguments for `get`, `update`, `delete` MUST result in a usage error (exit code 2) | MUST | |

#### Table Output Columns

```
ID                UID                                   DISPLAY NAME      CREATED
my-catalog-item   b2c3d4e5-f6a7-8901-bcde-f12345678901  Small Container   2026-03-09T10:00:00Z
```

#### Acceptance Criteria

##### AC-CIT-010: Create catalog item from file

- **Validates:** REQ-CIT-010, REQ-CIT-030
- **Given** a valid catalog item YAML file `item.yaml`
- **When** `dcm catalog item create --from-file item.yaml` is invoked
- **Then** a POST request MUST be sent to `/api/v1alpha1/catalog-items`
- **And** the created catalog item MUST be displayed in the configured output format

##### AC-CIT-020: Create catalog item with client-specified ID

- **Validates:** REQ-CIT-020
- **Given** a valid catalog item file
- **When** `dcm catalog item create --from-file item.yaml --id my-catalog-item` is invoked
- **Then** the POST request MUST include `?id=my-catalog-item` as a query parameter

##### AC-CIT-030: List catalog items

- **Validates:** REQ-CIT-040, REQ-CIT-050
- **Given** catalog items exist in the system
- **When** `dcm catalog item list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/catalog-items`
- **And** the catalog items MUST be displayed in the configured output format

##### AC-CIT-040: List catalog items with service-type filter

- **Validates:** REQ-CIT-040
- **Given** catalog items exist in the system
- **When** `dcm catalog item list --service-type container` is invoked
- **Then** the GET request MUST include the service type as a query parameter

##### AC-CIT-050: Get catalog item

- **Validates:** REQ-CIT-060
- **Given** a catalog item with ID `my-catalog-item` exists
- **When** `dcm catalog item get my-catalog-item` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/catalog-items/my-catalog-item`
- **And** the catalog item MUST be displayed in the configured output format

##### AC-CIT-060: Update catalog item

- **Validates:** REQ-CIT-070, REQ-CIT-080
- **Given** a catalog item with ID `my-catalog-item` exists
- **And** a valid patch file `patch.yaml`
- **When** `dcm catalog item update my-catalog-item --from-file patch.yaml` is invoked
- **Then** a PATCH request MUST be sent to `/api/v1alpha1/catalog-items/my-catalog-item`
- **And** the updated catalog item MUST be displayed in the configured output format

##### AC-CIT-070: Delete catalog item

- **Validates:** REQ-CIT-090, REQ-CIT-100
- **Given** a catalog item with ID `my-catalog-item` exists
- **When** `dcm catalog item delete my-catalog-item` is invoked
- **Then** a DELETE request MUST be sent to `/api/v1alpha1/catalog-items/my-catalog-item`
- **And** a success message MUST be displayed

##### AC-CIT-080: Create without --from-file

- **Validates:** REQ-CIT-120
- **Given** no `--from-file` flag is provided
- **When** `dcm catalog item create` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-CIT-090: Get without CATALOG_ITEM_ID

- **Validates:** REQ-CIT-130
- **Given** no positional argument is provided
- **When** `dcm catalog item get` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-CIT-100: List catalog items returns empty list

- **Validates:** REQ-CIT-040, REQ-CIT-050
- **Given** no catalog items exist in the system
- **When** `dcm catalog item list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/catalog-items`
- **And** an empty result MUST be displayed (empty table with headers only, or empty JSON array/YAML list)

##### AC-CIT-110: Get non-existent catalog item

- **Validates:** REQ-CIT-060, REQ-XC-ERR-010
- **Given** no catalog item with ID `nonexistent` exists
- **When** `dcm catalog item get nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-CIT-120: Update non-existent catalog item

- **Validates:** REQ-CIT-070, REQ-XC-ERR-010
- **Given** no catalog item with ID `nonexistent` exists
- **And** a valid patch file `patch.yaml`
- **When** `dcm catalog item update nonexistent --from-file patch.yaml` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-CIT-130: Delete non-existent catalog item

- **Validates:** REQ-CIT-090, REQ-XC-ERR-010
- **Given** no catalog item with ID `nonexistent` exists
- **When** `dcm catalog item delete nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-CIT-140: Create catalog item server error

- **Validates:** REQ-CIT-010, REQ-XC-ERR-010
- **Given** a valid catalog item file `item.yaml`
- **When** `dcm catalog item create --from-file item.yaml` is invoked
- **And** the API returns a server-side error (e.g., 500 Internal Server Error or 409 Conflict) with RFC 7807 body
- **Then** the CLI MUST display the error in the configured output format and exit with code 1

#### Dependencies

Depends on Topic 1 (CLI Framework), Topic 2 (Configuration), Topic 3 (Output
Formatting).

---

### 4.7 Catalog Instance Commands

#### Overview

Implement the `dcm catalog instance` command group with subcommands: `create`,
`list`, `get`, `delete`. Instances represent deployed catalog items. No update
operation is supported for instances in v1alpha1.

Out of scope: instance update/day-2 operations, instance status watching,
instance logs.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-CIN-010 | `dcm catalog instance create` MUST create an instance from a YAML or JSON file specified by `--from-file` | MUST | |
| REQ-CIN-020 | `dcm catalog instance create` MUST support an optional `--id` flag for client-specified instance ID | MUST | |
| REQ-CIN-030 | `dcm catalog instance create` MUST display the created instance in the configured output format | MUST | |
| REQ-CIN-040 | `dcm catalog instance list` MUST list instances with optional `--page-size`, `--page-token` flags | MUST | |
| REQ-CIN-050 | `dcm catalog instance list` MUST display instances in the configured output format | MUST | |
| REQ-CIN-060 | `dcm catalog instance get` MUST accept an `INSTANCE_ID` positional argument and display the instance | MUST | |
| REQ-CIN-070 | `dcm catalog instance delete` MUST accept an `INSTANCE_ID` positional argument and delete the instance | MUST | |
| REQ-CIN-080 | `dcm catalog instance delete` MUST display a success message on deletion | MUST | |
| REQ-CIN-090 | All catalog instance commands MUST use the generated Catalog Manager client | MUST | |
| REQ-CIN-100 | `--from-file` MUST be required for `create` | MUST | |
| REQ-CIN-110 | Missing positional arguments for `get`, `delete` MUST result in a usage error (exit code 2) | MUST | |

#### Table Output Columns

```
ID            UID                                   DISPLAY NAME      CATALOG ITEM      CREATED
my-instance   c3d4e5f6-a7b8-9012-cdef-123456789012  My App Instance   my-catalog-item   2026-03-09T10:00:00Z
```

#### Acceptance Criteria

##### AC-CIN-010: Create instance from file

- **Validates:** REQ-CIN-010, REQ-CIN-030
- **Given** a valid instance YAML file `instance.yaml`
- **When** `dcm catalog instance create --from-file instance.yaml` is invoked
- **Then** a POST request MUST be sent to `/api/v1alpha1/catalog-item-instances`
- **And** the created instance MUST be displayed in the configured output format

##### AC-CIN-020: Create instance with client-specified ID

- **Validates:** REQ-CIN-020
- **Given** a valid instance file
- **When** `dcm catalog instance create --from-file instance.yaml --id my-instance` is invoked
- **Then** the POST request MUST include `?id=my-instance` as a query parameter

##### AC-CIN-030: List instances

- **Validates:** REQ-CIN-040, REQ-CIN-050
- **Given** instances exist in the system
- **When** `dcm catalog instance list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/catalog-item-instances`
- **And** the instances MUST be displayed in the configured output format

##### AC-CIN-040: List instances with pagination

- **Validates:** REQ-CIN-040
- **Given** instances exist in the system
- **When** `dcm catalog instance list --page-size 10` is invoked
- **Then** the GET request MUST include `page_size=10` as a query parameter

##### AC-CIN-050: Get instance

- **Validates:** REQ-CIN-060
- **Given** an instance with ID `my-instance` exists
- **When** `dcm catalog instance get my-instance` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/catalog-item-instances/my-instance`
- **And** the instance MUST be displayed in the configured output format

##### AC-CIN-060: Delete instance

- **Validates:** REQ-CIN-070, REQ-CIN-080
- **Given** an instance with ID `my-instance` exists
- **When** `dcm catalog instance delete my-instance` is invoked
- **Then** a DELETE request MUST be sent to `/api/v1alpha1/catalog-item-instances/my-instance`
- **And** a success message MUST be displayed

##### AC-CIN-070: Create without --from-file

- **Validates:** REQ-CIN-100
- **Given** no `--from-file` flag is provided
- **When** `dcm catalog instance create` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-CIN-080: Get without INSTANCE_ID

- **Validates:** REQ-CIN-110
- **Given** no positional argument is provided
- **When** `dcm catalog instance get` is invoked
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-CIN-090: List instances returns empty list

- **Validates:** REQ-CIN-040, REQ-CIN-050
- **Given** no instances exist in the system
- **When** `dcm catalog instance list` is invoked
- **Then** a GET request MUST be sent to `/api/v1alpha1/catalog-item-instances`
- **And** an empty result MUST be displayed (empty table with headers only, or empty JSON array/YAML list)

##### AC-CIN-100: Get non-existent instance

- **Validates:** REQ-CIN-060, REQ-XC-ERR-010
- **Given** no instance with ID `nonexistent` exists
- **When** `dcm catalog instance get nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-CIN-110: Delete non-existent instance

- **Validates:** REQ-CIN-070, REQ-XC-ERR-010
- **Given** no instance with ID `nonexistent` exists
- **When** `dcm catalog instance delete nonexistent` is invoked
- **Then** the API returns a 404 with RFC 7807 body
- **And** the CLI MUST display the error in the configured output format and exit with code 1

##### AC-CIN-120: Create instance server error

- **Validates:** REQ-CIN-010, REQ-XC-ERR-010
- **Given** a valid instance file `instance.yaml`
- **When** `dcm catalog instance create --from-file instance.yaml` is invoked
- **And** the API returns a server-side error (e.g., 500 Internal Server Error or 409 Conflict) with RFC 7807 body
- **Then** the CLI MUST display the error in the configured output format and exit with code 1

#### Dependencies

Depends on Topic 1 (CLI Framework), Topic 2 (Configuration), Topic 3 (Output
Formatting).

---

### 4.8 Version Command

#### Overview

Implement the `dcm version` command to display CLI version and build
information. Version info is injected at build time via ldflags.

Out of scope: update check, changelog display.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-VER-010 | `dcm version` MUST display the CLI version, commit hash, build time, and Go version | MUST | |
| REQ-VER-020 | Version, commit, and build time MUST be injected at build time via ldflags | MUST | |
| REQ-VER-030 | When not built with ldflags, version MUST default to `dev`, commit to `unknown`, build time to `unknown` | MUST | |

#### Acceptance Criteria

##### AC-VER-010: Version display

- **Validates:** REQ-VER-010
- **Given** the CLI is built with version information
- **When** `dcm version` is invoked
- **Then** the output MUST display:
  - `dcm version <VERSION>`
  - `commit: <COMMIT>`
  - `built: <BUILD_TIME>`
  - `go: <GO_VERSION>`

##### AC-VER-020: Version injection via ldflags

- **Validates:** REQ-VER-020
- **Given** the binary is built with `make build`
- **When** ldflags are set for Version, Commit, and BuildTime
- **Then** `dcm version` MUST display the injected values

##### AC-VER-030: Default version values

- **Validates:** REQ-VER-030
- **Given** the binary is built without ldflags (e.g., `go build ./cmd/dcm`)
- **When** `dcm version` is invoked
- **Then** version MUST be `dev`, commit MUST be `unknown`, build time MUST be `unknown`

#### Dependencies

Depends on Topic 1 (CLI Framework).

---

## 5. Cross-Cutting Concerns

### 5.1 Error Handling

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-XC-ERR-010 | API errors MUST be parsed from RFC 7807 Problem Details format and displayed in a human-readable format | MUST | |
| REQ-XC-ERR-020 | For table output, API errors MUST display: `Error: <TYPE> - <TITLE>`, `Status: <STATUS>`, `Detail: <DETAIL>` | MUST | |
| REQ-XC-ERR-030 | For JSON/YAML output, API errors MUST display the full Problem Details object | MUST | |
| REQ-XC-ERR-040 | Connection errors (cannot reach API Gateway) MUST be displayed with a clear error message and exit code 1 | MUST | |
| REQ-XC-ERR-050 | Timeout errors MUST be displayed with a clear error message and exit code 1 | MUST | |
| REQ-XC-ERR-060 | Configuration errors MUST result in exit code 1 | MUST | |

#### Acceptance Criteria

##### AC-XC-ERR-010: API error display (table)

- **Validates:** REQ-XC-ERR-010, REQ-XC-ERR-020
- **Given** the API returns a 404 Not Found with RFC 7807 body
- **When** the output format is `table`
- **Then** the CLI MUST display:
  ```
  Error: NOT_FOUND - Policy "nonexistent" not found.
    Status: 404
    Detail: The requested policy resource does not exist.
  ```

##### AC-XC-ERR-020: API error display (JSON)

- **Validates:** REQ-XC-ERR-030
- **Given** the API returns a 404 Not Found with RFC 7807 body
- **When** the output format is `json`
- **Then** the full Problem Details JSON object MUST be printed

##### AC-XC-ERR-030: Connection error

- **Validates:** REQ-XC-ERR-040
- **Given** the API Gateway is unreachable
- **When** any command is invoked
- **Then** the CLI MUST display a connection error message
- **And** exit with code 1

##### AC-XC-ERR-040: Timeout error

- **Validates:** REQ-XC-ERR-050
- **Given** a request exceeds the configured timeout
- **When** the timeout is reached
- **Then** the CLI MUST display a timeout error message
- **And** exit with code 1

### 5.2 Input File Parsing

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-XC-INP-010 | The `--from-file` flag MUST accept both YAML and JSON files | MUST | |
| REQ-XC-INP-020 | The CLI MUST detect the file format based on content (not file extension) | MUST | |
| REQ-XC-INP-030 | Invalid or unreadable files MUST result in a clear error message and exit code 1 | MUST | |

#### Acceptance Criteria

##### AC-XC-INP-010: YAML file parsing

- **Validates:** REQ-XC-INP-010
- **Given** a valid YAML file is provided via `--from-file`
- **When** the file is parsed
- **Then** the content MUST be correctly deserialized

##### AC-XC-INP-020: JSON file parsing

- **Validates:** REQ-XC-INP-010
- **Given** a valid JSON file is provided via `--from-file`
- **When** the file is parsed
- **Then** the content MUST be correctly deserialized

##### AC-XC-INP-030: Invalid file handling

- **Validates:** REQ-XC-INP-030
- **Given** the file specified by `--from-file` does not exist or contains invalid content
- **When** the CLI attempts to read it
- **Then** a clear error message MUST be displayed
- **And** the CLI MUST exit with code 1

### 5.3 Generated Client Usage

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-XC-CLI-010 | The CLI MUST use the generated Policy Manager client (`github.com/dcm-project/policy-manager/pkg/client`) for all policy operations | MUST | |
| REQ-XC-CLI-020 | The CLI MUST use the generated Catalog Manager client (`github.com/dcm-project/catalog-manager/pkg/client`) for all catalog operations | MUST | |
| REQ-XC-CLI-030 | Both clients MUST be instantiated with the API Gateway URL appended with `/api/v1alpha1` | MUST | |
| REQ-XC-CLI-040 | Both clients MUST respect the configured request timeout | MUST | |
| REQ-XC-CLI-050 | Both clients MUST use a custom HTTP client with TLS transport when the API Gateway URL uses `https://` | MUST | |

#### Acceptance Criteria

##### AC-XC-CLI-010: Client instantiation

- **Validates:** REQ-XC-CLI-030
- **Given** the API Gateway URL is `http://localhost:9080`
- **When** the generated clients are created
- **Then** the Policy Manager client MUST be created with `http://localhost:9080/api/v1alpha1`
- **And** the Catalog Manager client MUST be created with `http://localhost:9080/api/v1alpha1`

##### AC-XC-CLI-020: Request timeout

- **Validates:** REQ-XC-CLI-040
- **Given** the timeout is configured to 30 seconds
- **When** a request is made via the generated client
- **Then** the request context MUST have a 30-second deadline

##### AC-XC-CLI-030: TLS transport for HTTPS

- **Validates:** REQ-XC-CLI-050
- **Given** the API Gateway URL is `https://gateway.example.com:9443`
- **When** the generated clients are created
- **Then** a custom HTTP client with TLS transport MUST be passed via `WithHTTPClient`

### 5.4 Pagination

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-XC-PAG-010 | All list commands MUST support `--page-size` and `--page-token` flags | MUST | |
| REQ-XC-PAG-020 | Pagination parameters MUST be passed as query parameters to the API | MUST | |
| REQ-XC-PAG-030 | When a response includes `next_page_token`, it MUST be surfaced to the user according to the output format rules (REQ-OUT-080, REQ-OUT-090) | MUST | |

#### Acceptance Criteria

##### AC-XC-PAG-010: Pagination flags

- **Validates:** REQ-XC-PAG-010
- **Given** any list command
- **When** `--help` is displayed
- **Then** `--page-size` and `--page-token` flags MUST be listed

##### AC-XC-PAG-020: Pagination pass-through

- **Validates:** REQ-XC-PAG-020
- **Given** `dcm policy list --page-size 10 --page-token abc123` is invoked
- **When** the request is sent
- **Then** the query parameters MUST include `page_size=10` and `page_token=abc123`

### 5.5 TLS Configuration

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-XC-TLS-010 | When the API Gateway URL uses `https://`, the CLI MUST establish a TLS connection | MUST | |
| REQ-XC-TLS-020 | When the API Gateway URL uses `http://`, the CLI MUST NOT use TLS and MUST silently ignore TLS-related flags/config | MUST | |
| REQ-XC-TLS-030 | The CLI MUST support a `--tls-ca-cert` flag to specify a custom CA certificate for server verification | MUST | |
| REQ-XC-TLS-040 | The CLI MUST support `--tls-client-cert` and `--tls-client-key` flags for mutual TLS (mTLS) | MUST | |
| REQ-XC-TLS-050 | The CLI MUST support a `--tls-skip-verify` flag to skip server certificate verification | MUST | |
| REQ-XC-TLS-060 | If `--tls-client-cert` is provided without `--tls-client-key` (or vice versa), the CLI MUST exit with code 2 | MUST | |
| REQ-XC-TLS-070 | If a specified CA cert, client cert, or client key file does not exist or is unreadable, the CLI MUST exit with code 1 with a clear error message | MUST | |
| REQ-XC-TLS-080 | When no custom CA cert is provided and the URL uses `https://`, the system default CA bundle MUST be used | MUST | |

#### Acceptance Criteria

##### AC-XC-TLS-010: HTTPS triggers TLS

- **Validates:** REQ-XC-TLS-010
- **Given** the API Gateway URL is `https://gateway.example.com:9443`
- **When** any command makes a request
- **Then** the HTTP client MUST use a TLS transport

##### AC-XC-TLS-020: HTTP skips TLS

- **Validates:** REQ-XC-TLS-020
- **Given** the API Gateway URL is `http://localhost:9080`
- **And** `--tls-skip-verify` or other TLS flags are set
- **When** any command makes a request
- **Then** TLS MUST NOT be used and TLS flags MUST be silently ignored

##### AC-XC-TLS-030: Custom CA certificate

- **Validates:** REQ-XC-TLS-030, REQ-XC-TLS-080
- **Given** the API Gateway URL is `https://gateway.example.com:9443`
- **And** `--tls-ca-cert /path/to/ca.pem` is provided
- **When** the TLS connection is established
- **Then** the custom CA certificate MUST be used to verify the server

##### AC-XC-TLS-040: Mutual TLS with client certificate

- **Validates:** REQ-XC-TLS-040
- **Given** the API Gateway URL is `https://gateway.example.com:9443`
- **And** `--tls-client-cert /path/to/cert.pem` and `--tls-client-key /path/to/key.pem` are provided
- **When** the TLS connection is established
- **Then** the client certificate and key MUST be used for mutual TLS

##### AC-XC-TLS-050: Skip TLS verification

- **Validates:** REQ-XC-TLS-050
- **Given** the API Gateway URL is `https://gateway.example.com:9443`
- **And** `--tls-skip-verify` is set
- **When** the TLS connection is established
- **Then** server certificate verification MUST be skipped

##### AC-XC-TLS-060: Incomplete mTLS configuration

- **Validates:** REQ-XC-TLS-060
- **Given** `--tls-client-cert` is provided without `--tls-client-key` (or vice versa)
- **When** the CLI validates configuration
- **Then** the CLI MUST exit with code 2 and display a usage error

##### AC-XC-TLS-070: Invalid TLS file path

- **Validates:** REQ-XC-TLS-070
- **Given** `--tls-ca-cert /nonexistent/ca.pem` is provided
- **When** the CLI attempts to load the certificate
- **Then** the CLI MUST exit with code 1 with a clear error message

---

## 6. Consolidated Configuration Reference

| Config Key | Env Var | Flag | Default | Required | Topic |
|------------|---------|------|---------|----------|-------|
| api-gateway-url | DCM_API_GATEWAY_URL | --api-gateway-url | http://localhost:9080 | No | 2 |
| output-format | DCM_OUTPUT_FORMAT | --output / -o | table | No | 2 |
| timeout | DCM_TIMEOUT | --timeout | 30 | No | 2 |
| - | DCM_CONFIG | --config | ~/.dcm/config.yaml | No | 2 |
| tls-ca-cert | DCM_TLS_CA_CERT | --tls-ca-cert | (empty) | No | 2 |
| tls-client-cert | DCM_TLS_CLIENT_CERT | --tls-client-cert | (empty) | No | 2 |
| tls-client-key | DCM_TLS_CLIENT_KEY | --tls-client-key | (empty) | No | 2 |
| tls-skip-verify | DCM_TLS_SKIP_VERIFY | --tls-skip-verify | false | No | 2 |

---

## 7. Design Decisions

### DD-010: Generated clients over hand-written HTTP

**Decision:** Use oapi-codegen generated clients from `policy-manager/pkg/client`
and `catalog-manager/pkg/client` instead of hand-writing HTTP client code.

**Rationale:** Generated clients guarantee API contract conformance, reduce
boilerplate, and evolve with the OpenAPI specs. The CLI is a thin wrapper around
these clients.

**Related requirements:** REQ-XC-CLI-010, REQ-XC-CLI-020

### DD-020: Cobra + Viper for CLI framework

**Decision:** Use Cobra for command structure and Viper for configuration
management.

**Rationale:** Industry-standard Go CLI stack. Cobra provides command parsing,
help generation, and argument validation. Viper provides layered configuration
with file, environment, and flag support matching the required precedence order.

**Related requirements:** REQ-CLI-010, REQ-CFG-060

### DD-030: Single API Gateway endpoint

**Decision:** All CLI commands communicate through a single API Gateway URL.
The CLI does not connect to backend services directly.

**Rationale:** Simplifies configuration (single URL), enables the gateway to
handle routing, load balancing, and future cross-cutting concerns (auth, rate
limiting). Matches the system architecture.

**Related requirements:** REQ-XC-CLI-030

### DD-040: File-based resource input

**Decision:** Resource creation and updates use `--from-file` with YAML/JSON
files rather than inline flags for each field.

**Rationale:** Resource bodies are complex structured objects (nested fields,
arrays). File-based input avoids an explosion of flags, supports copy-paste
workflows, and enables version-controlled resource definitions.

**Related requirements:** REQ-POL-010, REQ-CIT-010, REQ-CIN-010

### DD-050: RFC 7807 error parsing

**Decision:** Parse API error responses as RFC 7807 Problem Details and display
them in a human-readable format for table output, or as raw structured data for
JSON/YAML output.

**Rationale:** All DCM backend services use RFC 7807 for error responses. Parsing
and reformatting provides a consistent, user-friendly error experience across
all commands.

**Related requirements:** REQ-XC-ERR-010, REQ-XC-ERR-020, REQ-XC-ERR-030

### DD-060: Pagination hint in table output

**Decision:** When a list response includes `next_page_token`, table output
displays a ready-to-copy command for fetching the next page.

**Rationale:** Improves UX by showing the exact command to run. Users can copy
and paste rather than manually constructing the next command with the opaque
token. JSON/YAML output includes the token in the response object for
programmatic use.

**Related requirements:** REQ-OUT-090

### DD-080: Protocol-driven TLS

**Decision:** TLS is enabled automatically when the API Gateway URL uses `https://`
and disabled when it uses `http://`. TLS flags are silently ignored for `http://` URLs.

**Rationale:** Follows the principle of least surprise — the URL scheme already
communicates intent. Users should not need to set a separate "enable TLS" flag.
Custom CA certs, client certs for mTLS, and skip-verify provide the flexibility
needed for development, staging, and production environments.

**Related requirements:** REQ-XC-TLS-010, REQ-XC-TLS-020

### DD-070: Ginkgo + Gomega for testing

**Decision:** Use Ginkgo as the test framework with Gomega matchers. HTTP-level
mocking uses `net/http/httptest`.

**Rationale:** Consistent with other DCM projects. BDD-style tests provide
readable test descriptions that map naturally to acceptance criteria. httptest
provides reliable HTTP mocking without external dependencies.

**Related requirements:** All test-related acceptance criteria

---

## 8. Spec Clarifications

### SC-001: Input file format detection

**Related requirements:** REQ-XC-INP-010, REQ-XC-INP-020

The `--from-file` flag accepts files with any extension. Format detection MUST
attempt YAML parsing first (since valid JSON is also valid YAML). The file
content, not the extension, determines the format.

### SC-002: Policy delete success message format

**Related requirements:** REQ-POL-100

The delete success message MUST follow the format:
`<ResourceType> "<resourceId>" deleted successfully.`

Examples:
- `Policy "my-policy" deleted successfully.`
- `Catalog item "my-item" deleted successfully.`
- `Catalog item instance "my-instance" deleted successfully.`

This format applies to all delete commands across resource types.

### SC-003: Table column selection per resource type

**Related requirements:** REQ-OUT-050

Each resource type has its own table column set. The columns are defined in
the spec and MUST NOT vary based on response content. If a field is absent from
a response, an empty cell MUST be displayed.

### SC-004: Timeout applies to HTTP request only

**Related requirements:** REQ-XC-CLI-040

The `--timeout` value applies to the HTTP request deadline (context timeout). It
does not include file I/O or output formatting time. File reads and output
rendering are not subject to the timeout.

---

## 9. Requirement ID Index

| Prefix | Topic | Count |
|--------|-------|-------|
| REQ-CLI-NNN | 4.1: CLI Framework & Entry Point | 7 |
| REQ-CFG-NNN | 4.2: Configuration Management | 7 |
| REQ-OUT-NNN | 4.3: Output Formatting | 10 |
| REQ-POL-NNN | 4.4: Policy Commands | 13 |
| REQ-CST-NNN | 4.5: Catalog Service-Type Commands | 5 |
| REQ-CIT-NNN | 4.6: Catalog Item Commands | 13 |
| REQ-CIN-NNN | 4.7: Catalog Instance Commands | 11 |
| REQ-VER-NNN | 4.8: Version Command | 3 |
| REQ-XC-ERR-NNN | 5.1: Error Handling | 6 |
| REQ-XC-INP-NNN | 5.2: Input File Parsing | 3 |
| REQ-XC-CLI-NNN | 5.3: Generated Client Usage | 4 |
| REQ-XC-PAG-NNN | 5.4: Pagination | 3 |
| **Total** | | **85** |
