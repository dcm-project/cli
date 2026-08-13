# Design Decisions: DCM CLI OIDC Authentication

### DD-010: Device Authorization Grant over Auth Code + PKCE

**Decision:** Use the OAuth 2.0 Device Authorization Grant (RFC 8628) for
interactive CLI login instead of Authorization Code flow with PKCE.

**Rationale:** A CLI cannot reliably bind a localhost redirect URI across
platforms, remote SSH sessions, containers, and headless environments. Device
flow prints a URL and user code, optionally opens a browser, and polls for
completion. This matches how other CLIs (AWS, Azure, GitHub) authenticate
humans without embedding a temporary HTTP server. The original Jira story
allowed either flow; the implementation chose device grant.

**Related requirements:** REQ-LGN-030, REQ-LGN-040, REQ-LGN-050, REQ-LGN-090

### DD-020: Hardcoded public client ID `dcm-cli`

**Decision:** The OIDC client ID is the constant `dcm-cli` and is not
configurable via flag, environment variable, or config file.

**Rationale:** The Keycloak realm already provisions a public `dcm-cli` client
with device authorization and audience mapping for the control-plane API.
Making the client ID configurable adds operator complexity without a current
multi-tenant or multi-realm requirement. If a future deployment needs a
different client, that can be revisited deliberately.

**Related requirements:** REQ-ACFG-090, REQ-LGN-040, REQ-LGO-050

### DD-030: Keyring-first token storage with file fallback

**Decision:** Persist tokens in the OS keyring when available (service name
`dcm-cli`, account key = normalized issuer URL). If a keyring probe fails,
fall back to `~/.dcm/tokens.json` with directory mode `0700` and file mode
`0600`.

**Rationale:** Keyring backends (macOS Keychain, Secret Service, Windows
Credential Manager) are the most secure default for interactive developer
workstations. Headless CI and minimal containers often lack a keyring agent;
file fallback keeps the CLI functional there without a separate "storage
backend" config flag. Restrictive file permissions mitigate casual leakage on
shared hosts.

**Related requirements:** REQ-TOK-030, REQ-TOK-040, REQ-TOK-050

### DD-040: Static `DCM_TOKEN` / `--token` bypass for CI

**Decision:** Allow a static Bearer token via `DCM_TOKEN` or `--token` that
bypasses device login and TokenStore refresh logic. The token is never written
to the YAML config file.

**Rationale:** Automated pipelines cannot complete an interactive device flow.
A pre-minted token (e.g. from a confidential client or service account) is the
standard CI pattern. Omitting the token from config.yaml prevents accidental
persistence of long-lived secrets in a commonly copied file. Static token takes
precedence over stored OIDC credentials when both are present.

**Related requirements:** REQ-ACFG-040, REQ-ACFG-050, REQ-ACFG-060, REQ-TRN-030,
REQ-TRN-040

### DD-050: Auth is opt-in (AUTH_DISABLED compatibility)

**Decision:** When neither `issuer-url` nor `token` is set, API requests are
sent without an Authorization header. AuthTransport is only wrapped into the
HTTP client when one of those values is present.

**Rationale:** Development stacks may run with `AUTH_DISABLED=true` on the
control plane. Requiring login in that mode would break existing unauthenticated
workflows. Opt-in auth preserves backward compatibility while supporting
auth-enabled deployments after `dcm login` or with `DCM_TOKEN`.

**Related requirements:** REQ-ACFG-080, REQ-TRN-010, REQ-TRN-020, REQ-TRN-110

### DD-060: Unverified JWT decode for client-side claims

**Decision:** Read `exp` and `preferred_username` from the access-token JWT
payload without signature verification.

**Rationale:** The CLI is not an authorization server and does not make
security decisions based on these claims. Expiry is used only to decide when to
refresh; username is display-only on successful login. Full JWT validation
belongs on the control plane. Avoiding a JWKS fetch keeps login/refresh paths
simpler and offline-capable for the expiry check.

**Related requirements:** REQ-TOK-110, REQ-LGN-130

### DD-070: 30-second clock-skew buffer before refresh

**Decision:** Treat an access token as expired when `now >= exp - 30s`, and use
that threshold in AuthTransport before API calls.

**Rationale:** Small clock differences between the workstation and Keycloak can
cause the control plane to reject a token that the CLI still considers valid.
Refreshing slightly early avoids cascading 401 failures. Thirty seconds matches
common OAuth client practice and is short relative to typical access-token
lifetimes (e.g. 5 minutes in the DCM Keycloak realm).

**Related requirements:** REQ-TOK-110, REQ-TRN-060, REQ-TRN-120
