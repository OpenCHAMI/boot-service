<!--
SPDX-FileCopyrightText: 2026 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Authentication Testing Framework

This document describes the auth test utilities and integration tests in
`pkg/auth`.

It covers the package-level auth helpers, not route protection in the current
server entrypoint.

## What Is Covered Today

The current integration tests cover:

- non-enforcing mode
- static-key JWT validation
- scope middleware behavior
- service-to-service token checks
- expired token handling
- invalid token handling

The current tests use locally generated RSA keys and static-key validation.

## Key Test Helpers

From `pkg/auth/testing.go`:

- `TestingConfig(publicKeyPEM string)`
- `NonEnforcingConfig()`
- `GenerateTestKeyPair()`
- `CreateTestToken(...)`
- `CreateTestTokenWithScopes(...)`
- `CreateServiceToken(...)`
- `CreateStaticKeyConfig(...)`

## Example Test Flows

### Non-Enforcing Mode

```go
config := NonEnforcingConfig()
middleware := config.CreateMiddleware(nil)
```

### Static-Key Validation

```go
keyPair, _ := GenerateTestKeyPair()
config := CreateStaticKeyConfig(keyPair.PublicKeyPEM)
middleware := config.CreateMiddleware(nil)
```

### Scope Checks

```go
tokenWithReadScope, _ := CreateTestTokenWithScopes(keyPair, []string{"read"})
scopeMiddleware := CreateScopeMiddleware("read", "write")
```

### Service-to-Service Checks

```go
serviceToken, _ := CreateServiceToken(keyPair, "test-service", "boot-service", []string{"service:boot"})
serviceMiddleware := CreateServiceTokenMiddleware("boot-service")
```

## What Is Not Covered Here

- attaching auth middleware to the standalone server's generated routes
- policy engine wiring in `cmd/server`
- HSM bootstrap-token exchange in `cmd/server/main.go`

Those are separate concerns from the package tests.

## JWKS Status

JWKS support already exists in `pkg/auth/config.go`.

Current auth integration tests focus on static-key validation because it is a
cheap and deterministic test surface. A lack of JWKS-specific tests does not
mean JWKS support is absent.

## Example Server

The example server at `examples/auth-testing/main.go` remains useful for manual
exploration of the auth package behavior.

Run it with:

```bash
go run examples/auth-testing/main.go
```

## Summary

The auth package is tested and reusable. The most important distinction is that
these package tests do not imply that the standalone server binary is currently
wiring request auth middleware onto its route tree.
