<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Authentication with OpenCHAMI TokenSmith

This repository has two authentication-related surfaces:

1. a reusable `pkg/auth` package that supports JWT, JWKS, scopes, and service-token middleware
2. the `cmd/server` runtime configuration, which currently uses only a narrower subset of auth-related settings

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

The standalone server binary does **not** currently attach `pkg/auth`
middleware inside `cmd/server/main.go`.

As of the current branch, the auth-related runtime behavior is:

- `enable_auth: true` requires top-level `tokensmith_url`
- `enable_auth` is used for TokenSmith-dependent startup behavior and HSM service-token exchange
- if `enable_auth: true`, `hsm_url` is set, and `tokensmith_url` is set, then a bootstrap token is required
- the generated AuthZ classifier exists in `cmd/server/authz_classifier.go`, but the server entrypoint does not currently wire request auth middleware to the route tree

That means you should not assume the HTTP resource APIs are JWT-protected just
because `enable_auth` is true.

## Runtime Configuration Inputs

For the current server binary, the live top-level auth-related inputs are:

```yaml
enable_auth: false
tokensmith_url: ""
tokensmith_target_service: "hsm"
tokensmith_bootstrap_policy_scopes_hint: ""
tokensmith_refresh_skew_sec: 120
# tokensmith_bootstrap_token: "<bootstrap-jwt>"
```

Standardized environment variables:

- `TOKENSMITH_URL`: base TokenSmith URL used when `enable_auth` triggers startup validation or HSM service-token exchange, for example `http://localhost:8080`
- `TOKENSMITH_BOOTSTRAP_TOKEN`: bootstrap JWT exchanged for short-lived HSM service tokens
- `TOKENSMITH_TARGET_SERVICE`: target service name requested from TokenSmith, typically `hsm`
- `TOKENSMITH_BOOTSTRAP_POLICY_SCOPES_HINT`: optional scope hint used in diagnostics, for example `hsm:read`
- `TOKENSMITH_REFRESH_SKEW_SEC`: refresh skew in seconds before a cached service token is treated as stale

Deprecated compatibility environment variable:

- `TOKENSMITH_SCOPES`

## JWKS and Static-Key Notes

JWKS and static RSA key support are implemented in `pkg/auth`, but they are not
currently part of the server's documented top-level runtime config path.

If you are embedding the auth package in another service or extending this one,
use the package config directly rather than copying old nested `auth:` YAML
examples into the current server config.

## HSM Service-Token Exchange

When all of the following are true:

- `enable_auth: true`
- `hsm_url` is set
- `tokensmith_url` is set

the server exchanges a bootstrap token for short-lived HSM service tokens and
uses those for HSM API calls.

If `enable_auth: false`, the server logs that the TokenSmith URL is ignored for
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

### The server asks for `tokensmith_url`

That is expected when `enable_auth: true`. It is a startup validation rule in
the current server config path.

### A token is accepted in package tests but not by your app

Check whether you are using the standalone server binary or wiring `pkg/auth`
middleware in your own router. They are not the same thing today.

### You expected JWKS config in `config.yaml` to protect routes

The current server entrypoint does not wire `pkg/auth.CreateMiddleware` into the
route tree. Package-level support exists, but runtime enforcement in the server
binary is not documented as active.

## Additional Resources

- [AUTHENTICATION_TESTING.md](AUTHENTICATION_TESTING.md) for auth test coverage
- [CONFIGURATION.md](CONFIGURATION.md) for current server runtime keys
- [OpenCHAMI TokenSmith Documentation](https://github.com/OpenCHAMI/tokensmith)
