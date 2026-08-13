# Specification: DCM CLI OIDC Authentication

## 1. Overview

The DCM CLI (`dcm`) authenticates to the control plane using OIDC. When
authentication is configured, API requests include a Bearer JWT in the
`Authorization` header. Authentication is optional: when neither an issuer URL
nor a static token is configured, requests are sent without credentials so the
CLI continues to work against control planes with `AUTH_DISABLED=true`.

The CLI uses the OAuth 2.0 Device Authorization Grant (RFC 8628) against a
Keycloak public client. Humans run `dcm login` to obtain tokens; CI and scripts
MAY bypass the interactive flow with `DCM_TOKEN` / `--token`.

**Scope:**

- OIDC Device Authorization Grant login (`dcm login`)
- Token storage (OS keyring primary, file fallback)
- Automatic access-token refresh before API calls
- Bearer token injection via authenticated HTTP transport
- Logout with refresh-token revocation (`dcm logout`)
- Static token bypass for CI/scripting
- Unauthenticated passthrough when auth is not configured

**Out of scope:**

- Authorization Code flow with PKCE (localhost callback server)
- Configurable OIDC client ID (hardcoded `dcm-cli`)
- Automatic issuer URL discovery from the control plane (FLPATH-4643)
- Token introspection or local JWT signature verification
- Multi-account / multi-profile credential management UI

**Reference documents:**

- [FLPATH-4477](https://redhat.atlassian.net/browse/FLPATH-4477) - Add OIDC authentication to DCM CLI
- [FLPATH-4432](https://redhat.atlassian.net/browse/FLPATH-4432) - Control-plane authentication (dependency)
- [FLPATH-4643](https://redhat.atlassian.net/browse/FLPATH-4643) - Auto-discover issuer URL (follow-up)
- [DCM CLI Specification](dcm-cli.spec.md) - base CLI architecture and non-auth topics
- [OIDC Auth Test Plan](https://github.com/dcm-project/utilities/blob/main/test-plans/FLPATH-4477-dcm-cli-oidc-auth-test-plan.md) - QE validation companion (lives in dcm-project/utilities)
- RFC 8628 - OAuth 2.0 Device Authorization Grant
- RFC 7009 - OAuth 2.0 Token Revocation
- Keycloak realm: [`deploy/keycloak/realm-export.json`](https://github.com/dcm-project/control-plane/blob/main/deploy/keycloak/realm-export.json) in dcm-project/control-plane (`dcm-cli` public client)

---

## 2. Architecture

```
                         +------------------+
                         |     Keycloak     |
                         |  (OIDC issuer)   |
                         +--------+---------+
                                  ^
                     discovery /  |  device auth /
                     token refresh|  revocation
                                  |
+------------------+              |              +------------------------+
|                  |  dcm login   |              |  DCM Control Plane     |
|  dcm CLI         +--------------+              |  (Bearer JWT required  |
|                  |                             |   when AUTH enabled)   |
|  +------------+  |   API calls + Bearer JWT    |                        |
|  | AuthTransport+----------------------------->|  /api/v1alpha1/*       |
|  +------+-----+  |                             +------------------------+
|         |        |
|  +------+-----+  |     +------------------+
|  | TokenStore |--+---->| OS keyring       |
|  +------------+  |     | or ~/.dcm/       |
|                  |     |   tokens.json    |
+------------------+     +------------------+
```

```
dcm-cli/
├── internal/
│   ├── auth/
│   │   ├── auth.go          ← DeviceLogin, RevokeToken, PreferredUsername
│   │   ├── token.go         ← TokenData, TokenStore (keyring + file)
│   │   └── transport.go     ← AuthTransport (Bearer inject + refresh)
│   ├── commands/
│   │   ├── login.go         ← dcm login
│   │   ├── logout.go        ← dcm logout
│   │   ├── helpers.go       ← buildHTTPClient wraps AuthTransport
│   │   └── root.go          ← --issuer-url, --token flags
│   └── config/
│       └── config.go        ← issuer-url, token (token never persisted)
```

---

## 3. Topic Dependency Graph

| # | Topic                      | Prefix | Depends On |
|---|----------------------------|--------|------------|
| 1 | Auth Configuration         | ACFG   | -          |
| 2 | Device Login               | LGN    | 1          |
| 3 | Token Storage              | TOK    | 1          |
| 4 | Authenticated Transport    | TRN    | 1, 3       |
| 5 | Logout & Revocation        | LGO    | 1, 3       |

```
Topic 1: Auth Configuration        (independent)
  |
  +---> Topic 2: Device Login             (depends on 1)
  +---> Topic 3: Token Storage            (depends on 1)
          |
          +---> Topic 4: Auth Transport   (depends on 1, 3)
          +---> Topic 5: Logout           (depends on 1, 3)
```

Topics 2 and 3 can be delivered in parallel after Topic 1.
Topics 4 and 5 depend on Topics 1 and 3.

---

## 4. Topic Specifications

### 4.1 Auth Configuration

#### Overview

Auth-related settings participate in the existing CLI configuration precedence
(flags > environment variables > config file > defaults). The issuer URL
identifies the OIDC provider and keys stored credentials. A static token
bypasses the interactive OIDC flow for CI and scripting.

Out of scope: Auto-discovery of the issuer URL from the control plane.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-ACFG-010 | The CLI MUST accept `--issuer-url` as a global flag and bind it to config key `issuer-url` | MUST | |
| REQ-ACFG-020 | The CLI MUST accept environment variable `DCM_ISSUER_URL` for the issuer URL | MUST | |
| REQ-ACFG-030 | The CLI MUST persist `issuer-url` in `~/.dcm/config.yaml` when set via config save (e.g. after login) | MUST | |
| REQ-ACFG-040 | The CLI MUST accept `--token` as a global flag for a static Bearer token | MUST | |
| REQ-ACFG-050 | The CLI MUST accept environment variable `DCM_TOKEN` for a static Bearer token | MUST | |
| REQ-ACFG-060 | The static token MUST NOT be written to the config file (`yaml:"-"`) | MUST | Never persisted |
| REQ-ACFG-070 | Auth configuration MUST follow the same precedence as other CLI settings: flags > env vars > config file > defaults | MUST | |
| REQ-ACFG-080 | The default values for `issuer-url` and `token` MUST be empty strings | MUST | Auth is opt-in |
| REQ-ACFG-090 | The OIDC client ID MUST be the hardcoded public client `dcm-cli` and MUST NOT be configurable via flag, env var, or config file | MUST | See DD-020 |

#### Configuration Introduced

| Config Key | Env Var | Flag | Default | Required | Description |
|------------|---------|------|---------|----------|-------------|
| issuer-url | DCM_ISSUER_URL | --issuer-url | `""` | For login/logout | OIDC issuer URL (Keycloak realm URL) |
| token | DCM_TOKEN | --token | `""` | No | Static Bearer token; never written to config file |

#### Acceptance Criteria

##### AC-ACFG-010: Issuer URL via flag

- **Validates:** REQ-ACFG-010, REQ-ACFG-070
- **Given** `--issuer-url https://keycloak.example.com/realms/dcm` is passed
- **When** configuration is loaded
- **Then** `IssuerURL` MUST equal that value
- **Aligns with QE:** TC-05, TC-01

##### AC-ACFG-020: Issuer URL via environment variable

- **Validates:** REQ-ACFG-020, REQ-ACFG-070
- **Given** `DCM_ISSUER_URL` is set and no `--issuer-url` flag is passed
- **When** configuration is loaded
- **Then** `IssuerURL` MUST equal the environment variable value
- **Aligns with QE:** TC-12

##### AC-ACFG-030: Issuer URL persisted to config

- **Validates:** REQ-ACFG-030
- **Given** a successful `dcm login` with an issuer URL
- **When** config is saved
- **Then** `~/.dcm/config.yaml` MUST contain `issuer-url` with the login issuer
- **Aligns with QE:** TC-11, TC-18

##### AC-ACFG-040: Static token via flag and env

- **Validates:** REQ-ACFG-040, REQ-ACFG-050
- **Given** `--token` or `DCM_TOKEN` is set
- **When** configuration is loaded
- **Then** `Token` MUST equal the provided value
- **Aligns with QE:** TC-06

##### AC-ACFG-050: Static token never persisted

- **Validates:** REQ-ACFG-060
- **Given** a static token is set via flag or environment
- **When** config is saved (e.g. after login)
- **Then** the config file MUST NOT contain the token value
- **Aligns with QE:** TC-06 (Step 4)

##### AC-ACFG-060: Auth defaults empty

- **Validates:** REQ-ACFG-080
- **Given** no issuer URL or token is provided via flag, env, or config
- **When** configuration is loaded
- **Then** both `IssuerURL` and `Token` MUST be empty
- **Aligns with QE:** TC-09

##### AC-ACFG-070: Client ID hardcoded

- **Validates:** REQ-ACFG-090
- **Given** any auth operation (login, refresh, revoke)
- **When** the OIDC/OAuth client is constructed
- **Then** the client ID MUST be `dcm-cli`

#### Dependencies

None - independently deliverable (extends existing Topic 2 configuration in the base CLI spec).

---

### 4.2 Device Login

#### Overview

`dcm login` performs the OAuth 2.0 Device Authorization Grant against the
configured OIDC issuer. The CLI discovers provider endpoints, initiates device
authorization, prints the verification URL and user code, attempts to open a
browser, polls for the token response, stores credentials, and persists
issuer/control-plane URL settings to the config file.

Out of scope: Authorization Code + PKCE, headless-only login without a printed
URL (browser open is best-effort; the printed URL is the reliable path).

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-LGN-010 | The CLI MUST provide a `dcm login` command | MUST | |
| REQ-LGN-020 | `dcm login` MUST require a non-empty issuer URL (`--issuer-url` or `DCM_ISSUER_URL` or config) and MUST fail with a clear error when absent | MUST | |
| REQ-LGN-030 | `dcm login` MUST perform OIDC discovery against `{issuer-url}/.well-known/openid-configuration` | MUST | via go-oidc |
| REQ-LGN-040 | `dcm login` MUST initiate a Device Authorization Grant using client ID `dcm-cli` and scopes `openid`, `profile`, `email`, `offline_access` | MUST | |
| REQ-LGN-050 | `dcm login` MUST print the verification URL to stderr (preferring `verification_uri_complete` when present) | MUST | |
| REQ-LGN-060 | When `verification_uri_complete` is absent, `dcm login` MUST print the user code and instruct the user to visit `verification_uri` | MUST | |
| REQ-LGN-070 | When `verification_uri_complete` is present, `dcm login` SHOULD also print the base `verification_uri` and user code as an alternate path | SHOULD | |
| REQ-LGN-080 | `dcm login` SHOULD attempt to open the verification URL in the system browser; failure to open MUST NOT fail the login | SHOULD | Best-effort |
| REQ-LGN-090 | `dcm login` MUST poll the token endpoint until authorization completes or the command context times out | MUST | |
| REQ-LGN-100 | The login command context MUST time out after 5 minutes | MUST | |
| REQ-LGN-110 | On success, `dcm login` MUST persist access token, refresh token, optional ID token, expiry, and token endpoint via the TokenStore for the issuer URL | MUST | |
| REQ-LGN-120 | On success, `dcm login` MUST save `issuer-url` (and `control-plane-url` when set) to the config file; config save failure MUST warn on stderr and MUST NOT fail the login | MUST | |
| REQ-LGN-130 | On success, `dcm login` MUST print a success message to stderr including access-token TTL and that auto-refresh is enabled; when `preferred_username` is present in the access token it MUST be included | MUST | |

#### Acceptance Criteria

##### AC-LGN-010: Login requires issuer URL

- **Validates:** REQ-LGN-020
- **Given** no issuer URL is configured
- **When** `dcm login` is invoked
- **Then** the command MUST fail with an error indicating `--issuer-url` / `DCM_ISSUER_URL` is required
- **Aligns with QE:** TC-05

##### AC-LGN-020: Interactive device flow succeeds

- **Validates:** REQ-LGN-010, REQ-LGN-030, REQ-LGN-040, REQ-LGN-050, REQ-LGN-090, REQ-LGN-110
- **Given** a reachable OIDC issuer with the `dcm-cli` public client configured for device auth
- **When** the user completes browser authentication within the timeout
- **Then** tokens MUST be stored for that issuer
- **And** stderr MUST include the verification URL during the flow
- **Aligns with QE:** TC-01

##### AC-LGN-030: Login timeout

- **Validates:** REQ-LGN-100
- **Given** the user does not complete browser authentication
- **When** 5 minutes elapse
- **Then** `dcm login` MUST fail due to context timeout
- **Aligns with QE:** TC-14

##### AC-LGN-040: Config persistence after login

- **Validates:** REQ-LGN-120
- **Given** a successful login with `--issuer-url` and `--control-plane-url`
- **When** login completes
- **Then** `~/.dcm/config.yaml` MUST contain both values
- **And** subsequent commands MUST be able to use the saved issuer without re-passing the flag
- **Aligns with QE:** TC-11, TC-18

##### AC-LGN-050: Success message with username and TTL

- **Validates:** REQ-LGN-130
- **Given** a successful login whose access token includes `preferred_username`
- **When** login completes
- **Then** stderr MUST report the username, remaining access-token TTL, and that auto-refresh is enabled
- **Aligns with QE:** TC-01

##### AC-LGN-060: Browser open is best-effort

- **Validates:** REQ-LGN-080
- **Given** the system cannot open a browser (e.g. headless environment)
- **When** `dcm login` runs
- **Then** the command MUST still print the verification URL
- **And** MUST continue polling for the token

#### Dependencies

Depends on Topic 1 (Auth Configuration). Uses Topic 3 (Token Storage) at completion.

---

### 4.3 Token Storage

#### Overview

Credentials from login (and subsequent refresh) are persisted keyed by
normalized issuer URL. The primary backend is the OS keyring; when the keyring
is unavailable, a file store under `~/.dcm/tokens.json` is used.

Out of scope: Encrypting the file store beyond filesystem permissions;
cross-machine credential sync.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-TOK-010 | The TokenStore MUST support Save, Load, and Delete operations keyed by issuer URL | MUST | |
| REQ-TOK-020 | Stored TokenData MUST include access token, refresh token, optional ID token, expiry, and token endpoint | MUST | |
| REQ-TOK-030 | NewTokenStore MUST prefer the OS keyring when a probe write/delete succeeds | MUST | Service name `dcm-cli` |
| REQ-TOK-040 | When the keyring probe fails, NewTokenStore MUST fall back to a file-based store | MUST | |
| REQ-TOK-050 | The file-based store MUST write `~/.dcm/tokens.json` with directory mode `0700` and file mode `0600` | MUST | Atomic write via temp + rename |
| REQ-TOK-060 | Issuer URLs used as store keys MUST be normalized by stripping trailing slashes | MUST | |
| REQ-TOK-070 | Tokens for different issuer URLs MUST be stored independently | MUST | |
| REQ-TOK-080 | Load of a missing issuer MUST return a nil TokenData without error | MUST | |
| REQ-TOK-090 | Delete of a missing issuer MUST succeed without error | MUST | |
| REQ-TOK-100 | TokenData string/log representations MUST redact secrets (MUST NOT emit raw token values) | MUST | |
| REQ-TOK-110 | Access-token expiry checks MUST use an unverified JWT `exp` claim decode with a clock-skew buffer (30s; see REQ-TRN-120) | MUST | |

#### Acceptance Criteria

##### AC-TOK-010: Save and load round-trip

- **Validates:** REQ-TOK-010, REQ-TOK-020
- **Given** TokenData for an issuer
- **When** Save then Load are called for that issuer
- **Then** the loaded data MUST match the saved access token, refresh token, and token endpoint

##### AC-TOK-020: Keyring preferred with file fallback

- **Validates:** REQ-TOK-030, REQ-TOK-040
- **Given** the OS keyring is unavailable
- **When** NewTokenStore is called
- **Then** a file-based store MUST be returned
- **Aligns with QE:** TC-13 (file path)

##### AC-TOK-030: File permissions

- **Validates:** REQ-TOK-050
- **Given** the file-based store is used
- **When** tokens are saved
- **Then** `~/.dcm` MUST be mode `0700` and `tokens.json` MUST be mode `0600`
- **Aligns with QE:** TC-13

##### AC-TOK-040: Issuer normalization

- **Validates:** REQ-TOK-060
- **Given** tokens saved under an issuer URL with a trailing slash
- **When** Load is called without the trailing slash (or vice versa)
- **Then** the stored credentials MUST be found

##### AC-TOK-050: Multi-issuer isolation

- **Validates:** REQ-TOK-070
- **Given** tokens saved for two different issuers
- **When** each is loaded
- **Then** each issuer MUST return its own credentials

##### AC-TOK-060: Missing credentials are soft-absent

- **Validates:** REQ-TOK-080, REQ-TOK-090
- **Given** no credentials for an issuer
- **When** Load or Delete is called
- **Then** Load MUST return nil without error and Delete MUST succeed

##### AC-TOK-070: Token redaction

- **Validates:** REQ-TOK-100
- **Given** a TokenData instance with real token values
- **When** string or log output is produced for that TokenData
- **Then** the result MUST be a redacted placeholder and MUST NOT contain the token values
- **Aligns with QE:** TC-16

##### AC-TOK-080: Expiry with clock skew

- **Validates:** REQ-TOK-110
- **Given** an access token whose `exp` is 20 seconds in the future and clock skew of 30 seconds
- **When** IsExpired is evaluated
- **Then** the token MUST be treated as expired
- **And** a token with `exp` 31 seconds in the future MUST NOT be treated as expired for the same skew

#### Dependencies

Depends on Topic 1 (Auth Configuration) for issuer URL as the storage key.

---

### 4.4 Authenticated Transport

#### Overview

`AuthTransport` wraps the HTTP RoundTripper used by generated API clients.
When a static token is configured it is injected directly. Otherwise, when an
issuer URL is set, stored credentials are loaded and refreshed as needed.
When neither is configured, requests pass through unauthenticated.

Out of scope: Retrying API calls after a mid-flight 401; proactive background
refresh timers.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-TRN-010 | When `issuer-url` or `token` is set, `buildHTTPClient` MUST wrap the transport with AuthTransport | MUST | |
| REQ-TRN-020 | When neither `issuer-url` nor `token` is set, API requests MUST be sent without an Authorization header | MUST | AUTH_DISABLED compatibility |
| REQ-TRN-030 | When a static token is set, AuthTransport MUST inject `Authorization: Bearer <token>` and MUST NOT consult the TokenStore | MUST | |
| REQ-TRN-040 | When a static token is set it MUST take precedence over any stored OIDC tokens | MUST | |
| REQ-TRN-050 | When using stored tokens and a valid (non-expired) access token exists, AuthTransport MUST inject it as a Bearer token | MUST | |
| REQ-TRN-060 | When the stored access token is expired (per clock skew), AuthTransport MUST refresh using the refresh token and token endpoint before the request | MUST | |
| REQ-TRN-070 | After a successful refresh, AuthTransport MUST persist the new TokenData via the TokenStore | MUST | |
| REQ-TRN-080 | When refresh fails (or no refresh token exists), AuthTransport MUST return an error directing the user to run `dcm login` | MUST | |
| REQ-TRN-090 | Concurrent refresh within a single process MUST be serialized with a mutex; a second waiter MUST re-load the store after acquiring the lock | MUST | |
| REQ-TRN-100 | When sending a Bearer token over an `http://` URL, AuthTransport MUST print a one-time warning to stderr | MUST | |
| REQ-TRN-110 | When an issuer URL is set but no stored credentials exist, AuthTransport MUST pass the request through without an Authorization header | MUST | Soft unauthenticated |
| REQ-TRN-120 | The refresh clock-skew buffer MUST be 30 seconds | MUST | |

#### Acceptance Criteria

##### AC-TRN-010: Unauthenticated passthrough

- **Validates:** REQ-TRN-020
- **Given** no issuer URL and no static token
- **When** an API command runs
- **Then** the request MUST NOT include an Authorization header
- **Aligns with QE:** TC-09

##### AC-TRN-020: Static token injection

- **Validates:** REQ-TRN-030, REQ-TRN-010
- **Given** `DCM_TOKEN` or `--token` is set
- **When** an API request is made
- **Then** the Authorization header MUST be `Bearer <token>`
- **Aligns with QE:** TC-06

##### AC-TRN-030: Static token precedence

- **Validates:** REQ-TRN-040
- **Given** both a static token and stored OIDC credentials exist
- **When** an API request is made
- **Then** the static token MUST be used
- **Aligns with QE:** TC-07

##### AC-TRN-040: Stored token used after login

- **Validates:** REQ-TRN-050
- **Given** a successful login and a configured issuer URL
- **When** an API command runs before access-token expiry
- **Then** the request MUST include `Authorization: Bearer <access_token>`
- **Aligns with QE:** TC-02

##### AC-TRN-050: Automatic refresh

- **Validates:** REQ-TRN-060, REQ-TRN-070, REQ-TRN-120
- **Given** stored credentials whose access token is expired within the 30s skew window, with a valid refresh token
- **When** an API request is made
- **Then** the transport MUST refresh the token, persist the result, and send the new access token
- **Aligns with QE:** TC-03

##### AC-TRN-060: Refresh failure is actionable

- **Validates:** REQ-TRN-080
- **Given** stored credentials that cannot be refreshed
- **When** an API request is made
- **Then** the error MUST instruct the user to run `dcm login`
- **Aligns with QE:** TC-19

##### AC-TRN-070: HTTP Bearer warning

- **Validates:** REQ-TRN-100
- **Given** a Bearer token will be sent to an `http://` control-plane URL
- **When** the first such request is made in the process
- **Then** stderr MUST contain a warning about sending a Bearer token over unencrypted HTTP
- **And** subsequent requests in the same process MUST NOT repeat the warning
- **Aligns with QE:** TC-08

##### AC-TRN-080: Issuer set but no credentials

- **Validates:** REQ-TRN-110
- **Given** an issuer URL is configured and the TokenStore has no entry for it
- **When** an API request is made
- **Then** the request MUST proceed without an Authorization header
- **Aligns with QE:** TC-04 (post-logout)

##### AC-TRN-090: Concurrent refresh serialized in-process

- **Validates:** REQ-TRN-090
- **Given** two concurrent requests in the same process that both need a refresh
- **When** AuthTransport refreshes the access token
- **Then** refresh MUST be serialized with a mutex
- **And** the second waiter MUST re-load the store after acquiring the lock and MUST NOT perform a redundant refresh when the first already succeeded

##### AC-TRN-100: Concurrent CLI processes with stored token

- **Validates:** REQ-TRN-050
- **Given** valid stored credentials
- **When** multiple CLI invocations use the stored token
- **Then** each MUST be able to authenticate with a non-expired access token
- **And** multi-process refresh races are a known limitation (no cross-process lock)
- **Aligns with QE:** TC-17

#### Dependencies

Depends on Topic 1 (Auth Configuration) and Topic 3 (Token Storage).

---

### 4.5 Logout & Revocation

#### Overview

`dcm logout` clears stored credentials for the configured issuer. When a
refresh token is present, the CLI attempts RFC 7009 revocation at the
provider's revocation endpoint before deleting local credentials. Revocation
failure is non-fatal: local credentials are still cleared.

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-LGO-010 | The CLI MUST provide a `dcm logout` command | MUST | |
| REQ-LGO-020 | `dcm logout` MUST require a non-empty issuer URL and MUST fail with a clear error when absent | MUST | |
| REQ-LGO-030 | When no stored credentials exist for the issuer, `dcm logout` MUST print that no credentials were found and exit successfully | MUST | |
| REQ-LGO-040 | When a refresh token is present, `dcm logout` MUST attempt revocation at the OIDC provider's `revocation_endpoint` discovered via OIDC metadata | MUST | |
| REQ-LGO-050 | The revocation request MUST include `token`, `token_type_hint=refresh_token`, and `client_id=dcm-cli` as form-encoded POST body | MUST | |
| REQ-LGO-060 | If the provider metadata has no revocation endpoint, revocation MUST be skipped without error | MUST | |
| REQ-LGO-070 | If revocation fails, `dcm logout` MUST warn on stderr and continue to clear local credentials | MUST | |
| REQ-LGO-080 | After optional revocation, `dcm logout` MUST delete stored credentials for the issuer and print a success message | MUST | |

#### Acceptance Criteria

##### AC-LGO-010: Logout requires issuer URL

- **Validates:** REQ-LGO-020
- **Given** no issuer URL is configured
- **When** `dcm logout` is invoked
- **Then** the command MUST fail indicating `--issuer-url` / `DCM_ISSUER_URL` is required

##### AC-LGO-020: Logout with stored credentials

- **Validates:** REQ-LGO-010, REQ-LGO-040, REQ-LGO-050, REQ-LGO-080
- **Given** stored credentials with a refresh token for the issuer
- **When** `dcm logout` runs
- **Then** the CLI MUST attempt token revocation
- **And** MUST delete local credentials
- **And** MUST print a logged-out success message
- **Aligns with QE:** TC-04

##### AC-LGO-030: Logout when not logged in

- **Validates:** REQ-LGO-030
- **Given** no stored credentials for the issuer
- **When** `dcm logout` runs
- **Then** the CLI MUST report that no credentials were found
- **And** MUST exit successfully
- **Aligns with QE:** TC-10

##### AC-LGO-040: Revocation failure still clears local state

- **Validates:** REQ-LGO-070, REQ-LGO-080
- **Given** stored credentials and a revocation endpoint that returns an error
- **When** `dcm logout` runs
- **Then** stderr MUST contain a revocation warning
- **And** local credentials MUST still be deleted

##### AC-LGO-050: Missing revocation endpoint is skipped

- **Validates:** REQ-LGO-060
- **Given** an OIDC provider whose discovery document omits `revocation_endpoint`
- **When** `dcm logout` runs with stored credentials
- **Then** local credentials MUST be deleted without treating missing revocation as failure

#### Dependencies

Depends on Topic 1 (Auth Configuration) and Topic 3 (Token Storage).

---

## 5. Cross-Cutting Concerns

### 5.1 Secret Handling

Normative requirements live in the topic sections. This cross-cut is an index only:

| Concern | Source requirement |
|---------|--------------------|
| Token redaction in string/log output | REQ-TOK-100 |
| Static token never persisted to config | REQ-ACFG-060 |
| Restrictive file-store permissions | REQ-TOK-050 |

#### Acceptance Criteria

##### AC-XC-SEC-010: No secret leakage

- **Validates:** REQ-TOK-100, REQ-ACFG-060, REQ-TOK-050
- **Given** tokens exist in memory, config save paths, and file storage
- **When** string/log output, config file contents, and file modes are inspected
- **Then** secrets MUST be redacted or absent from config, and file perms MUST be restrictive
- **Aligns with QE:** TC-13, TC-16

### 5.2 Error Messaging

#### Requirements

| ID | Requirement | Priority | Notes |
|----|-------------|----------|-------|
| REQ-XC-ERR-010 | Missing issuer URL on login/logout MUST produce a clear required-flag error | MUST | |
| REQ-XC-ERR-020 | Failed token refresh MUST produce an actionable error referencing `dcm login` | MUST | |
| REQ-XC-ERR-030 | Non-fatal auth side effects (config save failure on login, revocation failure on logout) MUST warn on stderr without failing the primary operation | MUST | |

#### Acceptance Criteria

##### AC-XC-ERR-010: Actionable auth errors

- **Validates:** REQ-XC-ERR-010, REQ-XC-ERR-020, REQ-XC-ERR-030
- **Given** missing issuer, failed refresh, or non-fatal side-effect failure
- **When** the corresponding command runs
- **Then** the user-facing message MUST match the requirement (hard fail vs warn)

---

## 6. Consolidated Configuration Reference

| Config Key | Env Var | Flag | Default | Required | Persisted | Topic |
|------------|---------|------|---------|----------|-----------|-------|
| issuer-url | DCM_ISSUER_URL | --issuer-url | `""` | login/logout | Yes | 1 |
| token | DCM_TOKEN | --token | `""` | No | No | 1 |

Related non-auth settings used by auth flows (defined in the base CLI spec):

| Config Key | Env Var | Flag | Role in auth |
|------------|---------|------|--------------|
| control-plane-url | DCM_CONTROL_PLANE_URL | --control-plane-url | Saved on login when set; target for Bearer API calls |
| tls-* | DCM_TLS_* | --tls-* | Applied to HTTP client used for OIDC and API calls |

Constants (not configurable):

| Name | Value |
|------|-------|
| OIDC client ID | `dcm-cli` |
| OIDC scopes | `openid`, `profile`, `email`, `offline_access` |
| Login timeout | 5 minutes |
| Refresh clock skew | 30 seconds |
| Keyring service name | `dcm-cli` |
| File token path | `~/.dcm/tokens.json` |

---

## 7. Design Decisions

See [Design Decisions](../decisions/dcm-cli-oidc-auth.decisions.md).

---

## 8. Assumptions

- Keycloak (or a compatible OIDC provider) exposes Device Authorization Grant
  endpoints for the `dcm-cli` public client
- The `dcm-cli` client is configured with audience mapping suitable for the
  control-plane API (e.g. `dcm-api`)
- Access tokens are JWTs containing an `exp` claim (and optionally
  `preferred_username`) readable without signature verification on the client
- The control plane validates Bearer JWTs when auth is enabled and accepts
  unauthenticated requests when `AUTH_DISABLED=true`
- Users targeting an auth-enabled control plane either run `dcm login` or
  supply `DCM_TOKEN` / `--token`
- OS keyring availability varies by environment; file fallback is acceptable
  for headless/CI hosts

---

## 9. Requirement ID Index

| Prefix | Topic | Count |
|--------|-------|-------|
| REQ-ACFG-NNN | 4.1: Auth Configuration | 9 |
| REQ-LGN-NNN | 4.2: Device Login | 13 |
| REQ-TOK-NNN | 4.3: Token Storage | 11 |
| REQ-TRN-NNN | 4.4: Authenticated Transport | 12 |
| REQ-LGO-NNN | 4.5: Logout & Revocation | 8 |
| REQ-XC-ERR-NNN | 5.2: Error Messaging | 3 |
| **Total** | | **56** |
