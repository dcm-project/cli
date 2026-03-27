# DCM CLI Specification

## 1. Overview

### 1.1 Purpose

The DCM CLI (`dcm`) is the primary user-facing command-line interface for interacting with the DCM (Data Center Management) control plane. It provides commands for managing policies, service types, catalog items, catalog item instances, and service provider resources through the API Gateway.

### 1.2 Version Scope

This specification covers the `v1alpha1` API surface, matching the API Gateway route prefix `/api/v1alpha1`.

### 1.3 Reference Documents

| Document | Description |
|----------|-------------|
| Policy Manager OpenAPI | `api/v1alpha1/openapi.yaml` in dcm-policy-manager |
| Catalog Manager OpenAPI | `api/v1alpha1/openapi.yaml` in dcm-catalog-manager |
| AEP Standards | [aep.dev](https://aep.dev/) - API Enhancement Proposals |
| RFC 7807 | Problem Details for HTTP APIs |
| RFC 7396 | JSON Merge Patch |

---

## 2. Architecture

### 2.1 System Context

```
┌─────────┐              ┌──────────────────┐       ┌──────────────────┐
│         │              │                  │       ┌──────────────────┐
│  dcm    │─────────────▶│                  │──────▶│ Policy Manager   │
│  CLI    │ HTTP / HTTPS │  API Gateway     │       │ (port 8080)      │
│         │              │  (KrakenD 9080)  │       └──────────────────┘
└─────────┘              │                  │       ┌──────────────────┐
                         │                  │──────▶│ Catalog Manager  │
                         │                  │       │ (port 8080)      │
                         │                  │       └──────────────────┘
                         │                  │       ┌──────────────────┐
                         │                  │──────▶│ SP Resource Mgr  │
                         └──────────────────┘       │ (port 8080)      │
                                                    └──────────────────┘
```

The CLI communicates exclusively through the API Gateway (KrakenD on port 9080). When the API Gateway URL uses an `https://` scheme, the CLI establishes a TLS connection. When the URL uses `http://`, TLS is skipped entirely. The gateway routes requests to the appropriate backend service based on URL path:

- `/api/v1alpha1/policies/*` → Policy Manager
- `/api/v1alpha1/service-types/*` → Catalog Manager
- `/api/v1alpha1/catalog-items/*` → Catalog Manager
- `/api/v1alpha1/catalog-item-instances/*` → Catalog Manager
- `/api/v1alpha1/service-type-instances/*` → SP Resource Manager

### 2.2 Internal Architecture

```
cmd/dcm/
  main.go                    ← Entry point, root command setup

internal/
  config/                    ← Configuration loading/saving
  output/                    ← Output formatting (table/json/yaml)
  commands/
    root.go                  ← Root command, global flags
    version.go               ← Version command
    policy.go                ← Policy command group
    catalog_service_type.go  ← Catalog service-type command group
    catalog_item.go          ← Catalog item command group
    catalog_instance.go      ← Catalog instance command group
    sp.go                    ← SP parent command group
    sp_resource.go           ← SP resource command group
    completion.go            ← Shell completion command
```

### 2.3 Component Descriptions

| Component | Responsibility |
|-----------|---------------|
| `cmd/dcm/main.go` | Bootstrap, wire dependencies, execute root command |
| `internal/config` | Load config from file/env/flags with precedence |
| `internal/output` | Format responses as table, JSON, or YAML |
| `internal/commands` | Cobra command definitions, flag binding, client invocation |
| `internal/version` | Build-time version info injected via ldflags |

The CLI uses **generated clients** from `policy-manager/pkg/client`, `catalog-manager/pkg/client`, and `service-provider-manager/pkg/client/resource_manager` (oapi-codegen generated) as Go module dependencies. No hand-written HTTP client is needed.

---

## 3. Configuration

### 3.1 Configuration File

Default location: `~/.dcm/config.yaml`

```yaml
api-gateway-url: http://localhost:9080
output-format: table
timeout: 30
tls-ca-cert: ""
tls-client-cert: ""
tls-client-key: ""
tls-skip-verify: false
```

### 3.2 Environment Variables

| Variable | Description | Default |
|----------|-------------|---------|
| `DCM_API_GATEWAY_URL` | API Gateway base URL | `http://localhost:9080` |
| `DCM_OUTPUT_FORMAT` | Output format (`table`, `json`, `yaml`) | `table` |
| `DCM_TIMEOUT` | Request timeout in seconds | `30` |
| `DCM_CONFIG` | Path to config file | `~/.dcm/config.yaml` |
| `DCM_TLS_CA_CERT` | Path to CA certificate file for TLS verification | `""` |
| `DCM_TLS_CLIENT_CERT` | Path to client certificate file for mTLS | `""` |
| `DCM_TLS_CLIENT_KEY` | Path to client private key file for mTLS | `""` |
| `DCM_TLS_SKIP_VERIFY` | Skip TLS certificate verification (`true`/`false`) | `false` |

### 3.3 Precedence Order

Configuration values are resolved in the following order (highest to lowest priority):

1. **Command-line flags** (`--api-gateway-url`, `--output`, `--timeout`)
2. **Environment variables** (`DCM_API_GATEWAY_URL`, etc.)
3. **Configuration file** (`~/.dcm/config.yaml`)
4. **Built-in defaults**

### 3.4 Global Flags

These flags are available on all commands:

| Flag | Short | Description |
|------|-------|-------------|
| `--api-gateway-url` | | API Gateway URL |
| `--output` | `-o` | Output format: `table`, `json`, `yaml` |
| `--timeout` | | Request timeout in seconds |
| `--config` | | Path to config file |
| `--tls-ca-cert` | | Path to CA certificate file for TLS verification |
| `--tls-client-cert` | | Path to client certificate file for mTLS |
| `--tls-client-key` | | Path to client private key file for mTLS |
| `--tls-skip-verify` | | Skip TLS certificate verification |

---

## 4. Command Tree

### 4.1 Top-Level Structure

```
dcm
├── policy          # Policy management
│   ├── create
│   ├── list
│   ├── get
│   ├── update
│   └── delete
├── catalog
│   ├── service-type    # Service type management
│   │   ├── list
│   │   └── get
│   ├── item            # Catalog item management
│   │   ├── create
│   │   ├── list
│   │   ├── get
│   │   └── delete
│   └── instance        # Catalog item instance management
│       ├── create
│       ├── list
│       ├── get
│       └── delete
├── sp
│   └── resource       # SP resource management (read-only)
│       ├── list
│       └── get
├── completion      # Shell autocompletion
└── version         # Print version info
```

### 4.2 Policy Commands

#### `dcm policy create`

Create a new policy.

| Flag | Required | Description |
|------|----------|-------------|
| `--from-file` | Yes | Path to policy YAML/JSON file |
| `--id` | No | Client-specified policy ID (DNS-1123 format) |

```bash
# Create a policy from file
dcm policy create --from-file policy.yaml

# Create with client-specified ID
dcm policy create --from-file policy.yaml --id my-policy
```

Policy file format:

```yaml
display_name: "Require CPU Limits"
description: "Ensures all containers have CPU limits set"
policy_type: GLOBAL
label_selector:
  environment: production
priority: 100
rego_code: |
  package dcm.policy
  default allow = false
  allow { input.request.resource.limits.cpu != "" }
enabled: true
```

Example output (table):

```
ID          DISPLAY NAME        TYPE    PRIORITY  ENABLED  CREATED
my-policy   Require CPU Limits  GLOBAL  100       true     2026-03-09T10:00:00Z
```

#### `dcm policy list`

List policies with optional filtering and ordering.

| Flag | Required | Description |
|------|----------|-------------|
| `--filter` | No | CEL filter expression (e.g., `policy_type='GLOBAL'`) |
| `--order-by` | No | Order field and direction (e.g., `priority asc`) |
| `--page-size` | No | Maximum results per page |
| `--page-token` | No | Token for next page |

```bash
# List all policies
dcm policy list

# List enabled global policies ordered by priority
dcm policy list --filter "policy_type='GLOBAL' AND enabled=true" --order-by "priority asc"

# List with pagination
dcm policy list --page-size 10

# JSON output
dcm policy list -o json
```

Example output (table):

```
ID          DISPLAY NAME        TYPE    PRIORITY  ENABLED  CREATED
my-policy   Require CPU Limits  GLOBAL  100       true     2026-03-09T10:00:00Z
other-pol   Memory Limits       USER    500       true     2026-03-08T15:30:00Z
```

#### `dcm policy get`

Get a single policy by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `POLICY_ID` | Yes | Policy ID |

```bash
dcm policy get POLICY_ID

# YAML output
dcm policy get POLICY_ID -o yaml
```

#### `dcm policy update`

Update an existing policy (JSON Merge Patch - RFC 7396).

| Argument | Required | Description |
|----------|----------|-------------|
| `POLICY_ID` | Yes | Policy ID |

| Flag | Required | Description |
|------|----------|-------------|
| `--from-file` | Yes | Path to patch YAML/JSON file |

```bash
dcm policy update POLICY_ID --from-file patch.yaml
```

Patch file (only mutable fields):

```yaml
priority: 50
enabled: false
rego_code: |
  package dcm.policy
  default allow = true
```

#### `dcm policy delete`

Delete a policy by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `POLICY_ID` | Yes | Policy ID |

```bash
dcm policy delete POLICY_ID
```

Example output:

```
Policy "my-policy" deleted successfully.
```

### 4.3 Catalog Service-Type Commands

#### `dcm catalog service-type list`

List service types with optional pagination.

| Flag | Required | Description |
|------|----------|-------------|
| `--page-size` | No | Maximum results per page |
| `--page-token` | No | Token for next page |

```bash
dcm catalog service-type list
dcm catalog service-type list --page-size 5
```

#### `dcm catalog service-type get`

Get a single service type by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `SERVICE_TYPE_ID` | Yes | Service type ID |

```bash
dcm catalog service-type get SERVICE_TYPE_ID
```

### 4.4 Catalog Item Commands

#### `dcm catalog item create`

Create a new catalog item.

| Flag | Required | Description |
|------|----------|-------------|
| `--from-file` | Yes | Path to catalog item YAML/JSON file |
| `--id` | No | Client-specified catalog item ID |

```bash
dcm catalog item create --from-file item.yaml
dcm catalog item create --from-file item.yaml --id my-catalog-item
```

Catalog item file format:

```yaml
api_version: v1alpha1
display_name: "Small Container"
spec:
  service_type: container
  fields:
    - path: spec.replicas
      display_name: "Replica Count"
      editable: true
      default: "1"
      validation_schema:
        type: integer
        minimum: 1
        maximum: 10
    - path: spec.container.image
      display_name: "Container Image"
      editable: true
```

Example output (table):

```
ID                UID                                   DISPLAY NAME      CREATED
my-catalog-item   b2c3d4e5-f6a7-8901-bcde-f12345678901  Small Container   2026-03-09T10:00:00Z
```

#### `dcm catalog item list`

List catalog items with optional filtering and pagination.

| Flag | Required | Description |
|------|----------|-------------|
| `--service-type` | No | Filter by service type |
| `--page-size` | No | Maximum results per page |
| `--page-token` | No | Token for next page |

```bash
dcm catalog item list
dcm catalog item list --service-type container
```

#### `dcm catalog item get`

Get a single catalog item by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `CATALOG_ITEM_ID` | Yes | Catalog item ID |

```bash
dcm catalog item get CATALOG_ITEM_ID
dcm catalog item get CATALOG_ITEM_ID -o yaml
```

#### `dcm catalog item delete`

Delete a catalog item by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `CATALOG_ITEM_ID` | Yes | Catalog item ID |

```bash
dcm catalog item delete CATALOG_ITEM_ID
```

### 4.5 Catalog Instance Commands

#### `dcm catalog instance create`

Create a new catalog item instance.

| Flag | Required | Description |
|------|----------|-------------|
| `--from-file` | Yes | Path to instance YAML/JSON file |
| `--id` | No | Client-specified instance ID |

```bash
dcm catalog instance create --from-file instance.yaml
```

Instance file format:

```yaml
api_version: v1alpha1
display_name: "My App Instance"
spec:
  catalog_item_id: my-catalog-item
  user_values:
    - path: spec.replicas
      value: "3"
    - path: spec.container.image
      value: "nginx:latest"
```

Example output (table):

```
ID            UID                                   DISPLAY NAME      CATALOG ITEM      CREATED
my-instance   c3d4e5f6-a7b8-9012-cdef-123456789012  My App Instance   my-catalog-item   2026-03-09T10:00:00Z
```

#### `dcm catalog instance list`

List catalog item instances with optional pagination.

| Flag | Required | Description |
|------|----------|-------------|
| `--page-size` | No | Maximum results per page |
| `--page-token` | No | Token for next page |

```bash
dcm catalog instance list
dcm catalog instance list -o json
```

#### `dcm catalog instance get`

Get a single catalog item instance by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `INSTANCE_ID` | Yes | Instance ID |

```bash
dcm catalog instance get INSTANCE_ID
```

#### `dcm catalog instance delete`

Delete a catalog item instance by ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `INSTANCE_ID` | Yes | Instance ID |

```bash
dcm catalog instance delete INSTANCE_ID
```

### 4.6 SP Resource Commands

#### `dcm sp resource list`

List SP resources (service type instances) with optional filtering and pagination.

| Flag | Required | Description |
|------|----------|-------------|
| `--provider` | No | Filter by provider |
| `--page-size` | No | Maximum results per page |
| `--page-token` | No | Token for next page |

```bash
dcm sp resource list
dcm sp resource list --provider kubevirt-123
dcm sp resource list --page-size 5
```

Example output (table):

```
ID              PROVIDER        STATUS   CREATED
my-instance     kubevirt-123    ACTIVE   2026-03-09T10:00:00Z
other-instance  openstack-456   PENDING  2026-03-08T15:30:00Z
```

#### `dcm sp resource get`

Get a single SP resource by instance ID.

| Argument | Required | Description |
|----------|----------|-------------|
| `INSTANCE_ID` | Yes | Service type instance ID |

```bash
dcm sp resource get INSTANCE_ID
dcm sp resource get INSTANCE_ID -o yaml
```

### 4.7 Completion Command

#### `dcm completion`

Generate shell autocompletion scripts.

| Argument | Required | Description |
|----------|----------|-------------|
| `SHELL` | Yes | Shell type: `bash`, `zsh`, `fish`, or `powershell` |

```bash
# Bash
source <(dcm completion bash)

# Zsh
dcm completion zsh > "${fpath[1]}/_dcm"

# Fish
dcm completion fish | source

# PowerShell
dcm completion powershell | Out-String | Invoke-Expression
```

### 4.8 Version Command

#### `dcm version`

Print CLI version and build information.

```bash
dcm version
```

Example output:

```
dcm version 0.1.0
  commit: abc1234
  built:  2026-03-09T10:00:00Z
  go:     go1.25.5
```

---

## 5. Internal Components

### 5.1 `internal/config`

Manages CLI configuration with file persistence and environment/flag overrides.

```go
package config

type Config struct {
    APIGatewayURL string `yaml:"api-gateway-url" mapstructure:"api-gateway-url"`
    OutputFormat  string `yaml:"output-format" mapstructure:"output-format"`
    Timeout       int    `yaml:"timeout" mapstructure:"timeout"`
    TLSCACert     string `yaml:"tls-ca-cert" mapstructure:"tls-ca-cert"`
    TLSClientCert string `yaml:"tls-client-cert" mapstructure:"tls-client-cert"`
    TLSClientKey  string `yaml:"tls-client-key" mapstructure:"tls-client-key"`
    TLSSkipVerify bool   `yaml:"tls-skip-verify" mapstructure:"tls-skip-verify"`
}

// Load reads configuration from file, environment, and flag overrides.
func Load() (*Config, error)

```

### 5.2 `internal/output`

Formats API responses for display.

```go
package output

type Format string

const (
    FormatTable Format = "table"
    FormatJSON  Format = "json"
    FormatYAML  Format = "yaml"
)

type Formatter interface {
    // FormatOne formats a single resource.
    FormatOne(resource any) error
    // FormatList formats a list of resources with optional pagination info.
    FormatList(resources any, nextPageToken string) error
    // FormatMessage formats a simple status message.
    FormatMessage(msg string) error
}

// New creates a Formatter for the given format writing to the given writer.
func New(format Format, w io.Writer) Formatter
```

### 5.3 `internal/commands`

Cobra command definitions. Each file registers its command tree and wires generated client calls.

```go
// root.go
func NewRootCommand() *cobra.Command

// policy.go
func newPolicyCommand() *cobra.Command       // parent: dcm policy
func newPolicyCreateCommand() *cobra.Command  // dcm policy create
func newPolicyListCommand() *cobra.Command    // dcm policy list
func newPolicyGetCommand() *cobra.Command     // dcm policy get
func newPolicyUpdateCommand() *cobra.Command  // dcm policy update
func newPolicyDeleteCommand() *cobra.Command  // dcm policy delete

// catalog_service_type.go
func newCatalogServiceTypeCommand() *cobra.Command       // parent: dcm catalog service-type
func newServiceTypeListCommand() *cobra.Command
func newServiceTypeGetCommand() *cobra.Command

// catalog_item.go
func newCatalogItemCommand() *cobra.Command              // parent: dcm catalog item
func newCatalogItemCreateCommand() *cobra.Command
func newCatalogItemListCommand() *cobra.Command
func newCatalogItemGetCommand() *cobra.Command
func newCatalogItemDeleteCommand() *cobra.Command

// catalog_instance.go
func newCatalogInstanceCommand() *cobra.Command           // parent: dcm catalog instance
func newCatalogInstanceCreateCommand() *cobra.Command
func newCatalogInstanceListCommand() *cobra.Command
func newCatalogInstanceGetCommand() *cobra.Command
func newCatalogInstanceDeleteCommand() *cobra.Command

// sp.go
func newSPCommand() *cobra.Command                    // parent: dcm sp

// sp_resource.go
func newSPResourceCommand() *cobra.Command             // parent: dcm sp resource
func newSPResourceListCommand() *cobra.Command
func newSPResourceGetCommand() *cobra.Command

// completion.go
func newCompletionCommand() *cobra.Command              // dcm completion [bash|zsh|fish|powershell]
```

### 5.4 `internal/version`

Build-time version information injected via linker flags.

```go
package version

var (
    Version   = "dev"
    Commit    = "unknown"
    BuildTime = "unknown"
)

type Info struct {
    Version   string
    Commit    string
    BuildTime string
    GoVersion string
}

func Get() Info
```

### 5.5 Generated Clients (External Dependencies)

The CLI imports generated client packages as Go module dependencies:

- `github.com/dcm-project/policy-manager/pkg/client` - Policy Manager client
- `github.com/dcm-project/catalog-manager/pkg/client` - Catalog Manager client
- `github.com/dcm-project/service-provider-manager/pkg/client/resource_manager` - SP Resource Manager client

These are oapi-codegen generated clients providing typed API access. Key interfaces:

```go
// Policy Manager client (from policy-manager/pkg/client)
type ClientInterface interface {
    CreatePolicy(ctx context.Context, params *CreatePolicyParams, body CreatePolicyJSONRequestBody, ...) (*http.Response, error)
    ListPolicies(ctx context.Context, params *ListPoliciesParams, ...) (*http.Response, error)
    GetPolicy(ctx context.Context, policyId string, ...) (*http.Response, error)
    UpdatePolicy(ctx context.Context, policyId string, body UpdatePolicyJSONRequestBody, ...) (*http.Response, error)
    DeletePolicy(ctx context.Context, policyId string, ...) (*http.Response, error)
}

// Catalog Manager client (from catalog-manager/pkg/client)
type ClientInterface interface {
    CreateServiceType(ctx context.Context, params *CreateServiceTypeParams, body CreateServiceTypeJSONRequestBody, ...) (*http.Response, error)
    ListServiceTypes(ctx context.Context, params *ListServiceTypesParams, ...) (*http.Response, error)
    GetServiceType(ctx context.Context, serviceTypeId string, ...) (*http.Response, error)
    // ... similar for CatalogItem and CatalogItemInstance operations
}
```

Both clients are instantiated with the API Gateway URL and a configured HTTP client. When the API Gateway URL uses `https://`, the HTTP client is configured with a TLS transport based on the TLS settings (CA cert, client cert/key, skip verify). When the URL uses `http://`, TLS is not configured.

```go
httpClient := buildHTTPClient(cfg) // configures TLS transport when URL is https
policyClient, _ := policyclient.NewClient(cfg.APIGatewayURL + "/api/v1alpha1",
    policyclient.WithHTTPClient(httpClient))
catalogClient, _ := catalogclient.NewClient(cfg.APIGatewayURL + "/api/v1alpha1",
    catalogclient.WithHTTPClient(httpClient))
sprmClient, _ := sprmclient.NewClient(cfg.APIGatewayURL + "/api/v1alpha1",
    sprmclient.WithHTTPClient(httpClient))
```

---

## 6. Project Structure

```
dcm-cli/
├── .ai/
│   └── specs/
│       └── dcm-cli.spec.md
├── cmd/
│   └── dcm/
│       └── main.go
├── internal/
│   ├── config/
│   │   ├── config.go
│   │   └── config_test.go
│   ├── output/
│   │   ├── formatter.go
│   │   ├── table.go
│   │   ├── json.go
│   │   ├── yaml.go
│   │   └── formatter_test.go
│   ├── commands/
│   │   ├── root.go
│   │   ├── root_test.go
│   │   ├── version.go
│   │   ├── policy.go
│   │   ├── policy_test.go
│   │   ├── catalog_service_type.go
│   │   ├── catalog_service_type_test.go
│   │   ├── catalog_item.go
│   │   ├── catalog_item_test.go
│   │   ├── catalog_instance.go
│   │   ├── catalog_instance_test.go
│   │   ├── sp.go
│   │   ├── sp_resource.go
│   │   ├── sp_resource_test.go
│   │   ├── completion.go
│   │   └── completion_test.go
│   └── version/
│       └── version.go
├── test/
│   └── e2e/
│       ├── policy_test.go
│       ├── catalog_test.go
│       └── e2e_suite_test.go
├── .github/
│   └── workflows/
│       ├── ci.yaml
│       ├── lint.yaml
│       ├── check-clean-commits.yaml
│       ├── release.yaml
│       └── tag-release.yaml
├── .goreleaser.yaml
├── CLAUDE.md
├── Containerfile
├── go.mod
├── go.sum
├── LICENSE
├── Makefile
├── README.md
└── tools.go
```

---

## 7. Operational Flows

### 7.1 Command Execution Flow

```
User invokes command
  │
  ├─▶ Cobra parses flags and arguments
  │
  ├─▶ Viper resolves config (flags → env → file → defaults)
  │
  ├─▶ Build HTTP client (configure TLS transport if URL is https://)
  │
  ├─▶ Create generated client with API Gateway URL and HTTP client
  │
  ├─▶ Execute API call via generated client
  │
  ├─▶ Check response status
  │     ├─ Success → format and display response
  │     └─ Error → parse RFC 7807 problem details, display error, set exit code
  │
  └─▶ Exit
```

### 7.2 Policy CRUD Flow

```
dcm policy create --from-file policy.yaml
  │
  ├─▶ Read and parse policy.yaml (YAML or JSON)
  ├─▶ POST /api/v1alpha1/policies (with optional ?id=<id>)
  ├─▶ Display created policy
  └─▶ Exit 0

dcm policy list --filter "enabled=true" --order-by "priority asc"
  │
  ├─▶ GET /api/v1alpha1/policies?filter=enabled%3Dtrue&order_by=priority+asc
  ├─▶ Display policy table
  ├─▶ If next_page_token present, show pagination hint
  └─▶ Exit 0

dcm policy get <id>
  │
  ├─▶ GET /api/v1alpha1/policies/<id>
  ├─▶ Display policy details
  └─▶ Exit 0

dcm policy update <id> --from-file patch.yaml
  │
  ├─▶ Read and parse patch.yaml
  ├─▶ PATCH /api/v1alpha1/policies/<id>
  ├─▶ Display updated policy
  └─▶ Exit 0

dcm policy delete <id>
  │
  ├─▶ DELETE /api/v1alpha1/policies/<id>
  ├─▶ Display success message
  └─▶ Exit 0
```

### 7.3 Catalog Ordering Flow

```
dcm catalog instance create --from-file instance.yaml
  │
  ├─▶ Read and parse instance.yaml
  │     Contains: catalog_item_id, user_values
  ├─▶ POST /api/v1alpha1/catalog-item-instances
  ├─▶ Display created instance
  └─▶ Exit 0
```

### 7.4 Pagination

#### 7.4.1 Page Size

While `--page-size` is an optional parameter, services may impose a default value. Always check if
the response included `next_page_token`

#### 7.4.2 Next Page Token

When a list response includes `next_page_token`, the CLI displays it for manual follow-up:

```
dcm policy list --page-size 2
```

Output:

```
ID          DISPLAY NAME        TYPE    PRIORITY  ENABLED  CREATED
my-policy   Require CPU Limits  GLOBAL  100       true     2026-03-09T10:00:00Z
other-pol   Memory Limits       USER    500       true     2026-03-08T15:30:00Z

Next page: dcm policy list --page-size 2 --page-token eyJvZmZzZXQiOjJ9
```

For JSON/YAML output, `next_page_token` is included in the response object.

---

## 8. TLS Configuration

### 8.1 Protocol-Based TLS Behavior

TLS is determined automatically by the API Gateway URL scheme:

- **`http://`** — TLS is not used. All TLS-related flags and config are ignored.
- **`https://`** — TLS is enabled. The CLI constructs a `tls.Config` and uses it for the HTTP transport.

### 8.2 TLS Options

| Option | Description |
|--------|-------------|
| `--tls-ca-cert` | Path to a PEM-encoded CA certificate file. Used to verify the server's certificate. When not set, the system's default CA bundle is used. |
| `--tls-client-cert` | Path to a PEM-encoded client certificate file for mutual TLS (mTLS). Must be used together with `--tls-client-key`. |
| `--tls-client-key` | Path to a PEM-encoded client private key file for mTLS. Must be used together with `--tls-client-cert`. |
| `--tls-skip-verify` | When set, the CLI skips server certificate verification. **Not recommended for production.** |

### 8.3 Validation Rules

- If `--tls-client-cert` is set, `--tls-client-key` MUST also be set (and vice versa). The CLI MUST exit with code 2 if only one is provided.
- TLS flags are silently ignored when the API Gateway URL uses `http://`.
- If the CA cert, client cert, or client key file does not exist or is not readable, the CLI MUST exit with code 1 with a clear error message.

### 8.4 Configuration Precedence

TLS options follow the same precedence as other configuration:

1. CLI flags (`--tls-ca-cert`, `--tls-client-cert`, `--tls-client-key`, `--tls-skip-verify`)
2. Environment variables (`DCM_TLS_CA_CERT`, `DCM_TLS_CLIENT_CERT`, `DCM_TLS_CLIENT_KEY`, `DCM_TLS_SKIP_VERIFY`)
3. Configuration file (`tls-ca-cert`, `tls-client-cert`, `tls-client-key`, `tls-skip-verify`)
4. Built-in defaults (empty strings, `false`)

---

## 9. Error Handling

### 9.1 Error Categories

| Category | Description | Exit Code |
|----------|-------------|-----------|
| Configuration error | Invalid config file, missing required config | 1 |
| Connection error | Cannot reach API Gateway | 1 |
| API error | Backend returned an error response | 1 |
| Input error | Invalid flags, missing arguments, bad file | 2 |
| Timeout | Request exceeded timeout | 1 |

### 9.2 API Error Display

API errors follow RFC 7807 Problem Details format. The CLI parses the response and displays a human-readable error:

```
Error: NOT_FOUND - Policy "nonexistent" not found.
  Status: 404
  Detail: The requested policy resource does not exist.
```

For JSON/YAML output, the full Problem Details object is printed:

```json
{
  "type": "NOT_FOUND",
  "status": 404,
  "title": "Policy \"nonexistent\" not found.",
  "detail": "The requested policy resource does not exist."
}
```

### 9.3 Exit Codes

| Code | Meaning |
|------|---------|
| 0 | Success |
| 1 | Runtime error (API error, connection error, timeout) |
| 2 | Usage error (invalid flags, missing arguments) |

---

## 10. Build and Distribution

### 10.1 Makefile Targets

```makefile
build          # Build dcm binary to bin/dcm
test           # Run unit tests with Ginkgo
test-e2e       # Run E2E tests (requires live stack)
fmt            # Format Go code
vet            # Run go vet
lint           # Run linter
clean          # Remove build artifacts
tidy           # Run go mod tidy
```

### 10.2 Version Injection

Version information is injected at build time via ldflags:

```makefile
VERSION ?= $(shell git describe --tags --always --dirty)
COMMIT  ?= $(shell git rev-parse --short HEAD)
BUILD_TIME ?= $(shell date -u '+%Y-%m-%dT%H:%M:%SZ')

LDFLAGS = -X github.com/dcm-project/dcm-cli/internal/version.Version=$(VERSION) \
          -X github.com/dcm-project/dcm-cli/internal/version.Commit=$(COMMIT) \
          -X github.com/dcm-project/dcm-cli/internal/version.BuildTime=$(BUILD_TIME)

build:
	go build -ldflags "$(LDFLAGS)" -o bin/dcm ./cmd/dcm
```

### 10.3 Release

Releases are automated via GoReleaser and GitHub Actions. When a `v*` tag is pushed, the `tag-release` workflow runs CI tests and linting, then triggers the `release` workflow which uses GoReleaser to:

- Build `dcm` binaries for linux, darwin, and windows (amd64 and arm64)
- Inject version, commit, and build time via ldflags
- Create a GitHub release with archives and checksums
- Include the LICENSE in each archive

To create a release:

```bash
git tag v0.1.0
git push origin v0.1.0
```

### 10.4 Containerfile

Multi-stage UBI9 build matching other DCM services:

```dockerfile
# Build stage
FROM registry.access.redhat.com/ubi9/go-toolset:1.25.5 AS builder
WORKDIR /opt/app-root/src
COPY go.mod go.sum ./
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 go build -ldflags "$(LDFLAGS)" -o dcm ./cmd/dcm

# Runtime stage
FROM registry.access.redhat.com/ubi9/ubi-minimal:latest
COPY --from=builder /opt/app-root/src/dcm /usr/local/bin/dcm
USER 1001
ENTRYPOINT ["dcm"]
```

### 10.5 Go Module

```
module github.com/dcm-project/dcm-cli

go 1.25.5
```

### 10.6 Dependencies

| Dependency | Purpose |
|------------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/spf13/viper` | Configuration management |
| `gopkg.in/yaml.v3` | YAML parsing/output |
| `github.com/dcm-project/policy-manager/pkg/client` | Generated Policy Manager client |
| `github.com/dcm-project/catalog-manager/pkg/client` | Generated Catalog Manager client |
| `github.com/dcm-project/service-provider-manager/pkg/client/resource_manager` | Generated SP Resource Manager client |
| `github.com/onsi/ginkgo/v2` | Test framework (test dependency) |
| `github.com/onsi/gomega` | Test matchers (test dependency) |

### 10.7 tools.go

```go
//go:build tools

package tools

import (
	_ "github.com/onsi/ginkgo/v2/ginkgo"
)
```

---

## 11. Testing Strategy

### 11.1 Unit Tests

- **Framework**: Ginkgo + Gomega
- **Location**: `*_test.go` files alongside source
- **Mocking**: `net/http/httptest` for HTTP-level mocking of generated client calls
- **Coverage areas**:
  - Configuration loading and precedence
  - Output formatting (table, JSON, YAML)
  - Command flag parsing and validation
  - API response handling and error parsing
  - Input file parsing (YAML/JSON)

Example test pattern:

```go
var _ = Describe("Policy Commands", func() {
    var (
        server *httptest.Server
        // ...
    )

    BeforeEach(func() {
        server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
            // Mock API responses
        }))
    })

    AfterEach(func() {
        server.Close()
    })

    Describe("policy create", func() {
        It("should create a policy from a YAML file", func() {
            // ...
        })
    })
})
```

Running tests:

```bash
make test                          # All unit tests
go test -run TestName ./internal/commands  # Specific test
```

### 11.2 E2E Tests

- **Location**: `test/e2e/`
- **Build tag**: `//go:build e2e` (excluded from `make test`)
- **Requirements**: Live DCM stack (API Gateway + backends)
- **Framework**: Ginkgo + Gomega
- **Scope**: Full command execution against real services

Running E2E tests:

```bash
make test-e2e   # Requires DCM_API_GATEWAY_URL pointing to live stack
```

---

## 12. Scope Boundaries

### 12.1 In Scope (v1alpha1)

- Policy CRUD operations (create, list, get, update, delete)
- Service type read operations (list, get)
- Catalog item operations (create, list, get, delete)
- Catalog item instance operations (create, list, get, delete)
- SP resource read operations (list, get)
- Version display
- Output formatting (table, JSON, YAML)
- Configuration via file, environment variables, and flags
- Pagination support for list operations
- TLS support with custom CA certificates, client certificates (mTLS), and skip-verify
- Shell autocompletion generation (bash, zsh, fish, powershell)
- Container image for distribution

### 12.2 Out of Scope (v1alpha1)

- Authentication and authorization (no auth in v1alpha1 API Gateway)
- Interactive/wizard-style resource creation
- Watch/streaming operations
- Plugin/extension system
- Offline mode or local caching
- Bulk operations
- Resource diff/dry-run
- Health check command for API Gateway connectivity
