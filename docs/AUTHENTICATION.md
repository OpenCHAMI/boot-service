<!--
SPDX-FileCopyrightText: © 2026 OpenCHAMI a Series of LF Projects, LLC

SPDX-License-Identifier: MIT
-->

# Authentication with OpenCHAMI TokenSmith

This repository has two authentication-related surfaces:

1. a reusable `pkg/auth` package that supports JWT, JWKS, scopes, and service-token middleware
2. the `cmd/server` runtime, which uses Fabrica-generated JWT authentication instead of `pkg/auth`

This document covers both, and keeps them separate.

## Package-Level Auth Support

The `pkg/auth` package currently supports:

- static RSA public key validation
- JWKS-backed validation
- issuer and audience checks
- required claim and required scope checks
- non-enforcing and dev modes
- service-to-service middleware
- claim extraction from request context

### Quick Example

```go
import "github.com/openchami/boot-service/pkg/auth"

config := auth.DefaultConfig()
config.JWKSURL = "https://auth.example.com/.well-known/jwks.json"
config.JWTIssuer = "https://auth.example.com"
config.JWTAudience = "boot-service"
config.RequiredScopes = []string{"boot:read"}

authMiddleware := config.CreateMiddleware(logger)
```

Apply it to routes in the usual Chi pattern:

```go
r := chi.NewRouter()
r.Use(authMiddleware)
```

### Other Package Helpers

The package also exposes:

- `auth.DevConfig()`
- `auth.NonEnforcingConfig()`
- `auth.CreateScopeMiddleware(...)`
- `auth.CreateServiceTokenMiddleware(...)`
- `auth.GetClaimsFromRequest(r)`

## Current Server Runtime Behavior

Authentication is off unless `auth_enabled: true` is set. When it is on, the
server enforces JWTs with Fabrica's generated TokenSmith middleware, not
`pkg/auth`, and refuses to start without `tokensmith_url` and a JWKS URL.

| Routes | Token required |
| --- | --- |
| `/nodes`, `/bmcs`, `/bootconfigurations` and everything under them | Yes |
| `/bootparameters`, `/boot/v1/bootparameters` | Yes, including reads |
| `/bootscript`, `/boot/v1/bootscript` | No: nodes request these before they can hold a token |
| `/service/*`, `/boot/v1/service/*`, `/health` | No |
| `/openapi.json`, `/docs`, `/.well-known/api-catalog`, `/metrics` | No |

Requests the service makes to its own resource API run in process, so boot
scripts and HSM sync keep working without a token.

### What Is and Is Not Checked

The generated middleware verifies the signature against the JWKS, and checks
token lifetime and that TokenSmith's required claims are present.

**It does not check who a token was issued for or by.** Any token signed by a key
in the JWKS is accepted, including one minted for a different service. Treat
`auth_enabled` as proof that a token came from your TokenSmith, not as access
control between services.

The generated `jwt_issuer`, `jwt_audience`, `jwt_public_key`, and
`auth_non_enforcing` settings are not read, so the server refuses to start with
them while authentication is enabled rather than appear to apply them.

### Authorization

`TOKENSMITH_AUTHZ_MODE` defaults to `off`. **Leave it off:** the generated
authentication does not pass token roles to AuthZ, so `enforce` denies every
token whatever `authz/policy.csv` says. Boot scripts are unaffected, because they
bypass AuthZ. The policy already grants `/bootparameters` to the same roles as
`/bootconfigurations`, ready for when roles are passed through.

## Runtime Configuration Inputs

For the current server binary, the live top-level auth-related inputs are:

```yaml
auth_enabled: false
jwks_url: ""
tokensmith_url: ""
tokensmith_target_service: "hsm"
tokensmith_bootstrap_policy_scopes_hint: ""
tokensmith_refresh_skew_sec: 120
# tokensmith_bootstrap_token: "<bootstrap-jwt>"
```

Standardized environment variables:

- `TOKENSMITH_JWKS_URL`: JWKS URL used to validate JWTs; takes precedence over `jwks_url`
- `TOKENSMITH_URL`: base TokenSmith URL, required when `auth_enabled` is true, for example `http://localhost:8080`
- `TOKENSMITH_BOOTSTRAP_TOKEN`: bootstrap JWT exchanged for short-lived HSM service tokens
- `TOKENSMITH_TARGET_SERVICE`: target service name requested from TokenSmith, typically `hsm`
- `TOKENSMITH_BOOTSTRAP_POLICY_SCOPES_HINT`: optional scope hint used in diagnostics, for example `hsm:read`
- `TOKENSMITH_REFRESH_SKEW_SEC`: refresh skew in seconds before a cached service token is treated as stale
- `TOKENSMITH_AUTHZ_MODE`: `off`, `shadow`, or `enforce`; see [Authorization](#authorization)

`enable_auth` is a deprecated alias for `auth_enabled`. It used to affect only
startup validation and HSM service-token exchange, and now also enforces JWTs.

## HSM Service-Token Exchange

When all of the following are true:

- `auth_enabled: true`
- `hsm_url` is set
- `tokensmith_url` is set

the server exchanges a bootstrap token for short-lived HSM service tokens and
uses those for HSM API calls.

If `auth_enabled: false`, the server logs that the TokenSmith URL is ignored for
HSM integration.

## Testing and Examples

The auth package has integration tests for:

- non-enforcing mode
- static-key token validation
- scope middleware
- service-to-service token checks
- expired and invalid token handling

See `docs/AUTHENTICATION_TESTING.md` for the current test surface.

## Troubleshooting

### The server asks for `tokensmith_url` or `TOKENSMITH_JWKS_URL`

That is expected when `auth_enabled: true`, or the deprecated `enable_auth: true`.
Set both, or turn authentication off.

### Every token gets `401`

The middleware requires TokenSmith's full claim set, not only a valid signature.
The warning it logs names the missing claim, for example `nbf claim is required`.

### Every token gets `403`

`TOKENSMITH_AUTHZ_MODE` is set to `enforce`. See [Authorization](#authorization).

### A token is accepted in package tests but not by your app

The server binary uses Fabrica's generated middleware, not `pkg/auth`, so `pkg/auth`
settings do not apply to it.

## Additional Resources

- [AUTHENTICATION_TESTING.md](AUTHENTICATION_TESTING.md) for auth test coverage
- [CONFIGURATION.md](CONFIGURATION.md) for current server runtime keys
- [OpenCHAMI TokenSmith Documentation](https://github.com/OpenCHAMI/tokensmith)
