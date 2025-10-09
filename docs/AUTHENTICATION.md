# Authentication with OpenCHAMI TokenSmith

This document describes how the OpenCHAMI boot service integrates with the [OpenCHAMI TokenSmith](https://github.com/OpenCHAMI/tokensmith) middleware for JWT-based authentication and authorization.

## Overview

The boot service uses the OpenCHAMI TokenSmith middleware to provide:

- **JWT Token Validation**: Support for RSA and ECDSA signed tokens
- **JWKS Integration**: Automatic key rotation via JSON Web Key Sets  
- **Scope-based Authorization**: Fine-grained access control
- **Service-to-Service Authentication**: Internal service communication
- **Flexible Configuration**: Multiple deployment scenarios

## Architecture

```
┌─────────────────┐    ┌─────────────────┐    ┌─────────────────┐
│   Client/User   │    │  TokenSmith     │    │   Boot Service  │
│                 │    │  Auth Service   │    │                 │
└─────────────────┘    └─────────────────┘    └─────────────────┘
         │                       │                       │
         │ 1. Authenticate       │                       │
         ├──────────────────────→│                       │
         │                       │                       │
         │ 2. JWT Token          │                       │
         │←──────────────────────┤                       │
         │                       │                       │
         │ 3. API Request + JWT  │                       │
         ├─────────────────────────────────────────────→│
         │                       │                       │
         │                       │ 4. Validate Token    │
         │                       │ (JWKS or Static Key)  │
         │                       │←──────────────────────┤
         │                       │                       │
         │                       │ 5. Token Valid        │
         │                       ├──────────────────────→│
         │                       │                       │
         │ 6. API Response       │                       │
         │←─────────────────────────────────────────────┤
```

## Quick Start

### 1. Basic Configuration

Create an authentication configuration:

```go
import "github.com/openchami/boot-service/pkg/auth"

// For development (authentication disabled)
config := auth.DevConfig()

// For production
config := auth.DefaultConfig()
config.JWKSURL = "https://your-auth-server/.well-known/jwks.json"
config.JWTIssuer = "your-issuer"
config.JWTAudience = "boot-service"
```

### 2. Create Middleware

```go
authMiddleware := config.CreateMiddleware(logger)
```

### 3. Apply to Routes

```go
import "github.com/go-chi/chi/v5"

r := chi.NewRouter()

// Public routes
r.Get("/health", healthHandler)

// Protected routes
r.Group(func(r chi.Router) {
    r.Use(authMiddleware)
    r.Get("/boot/{nodeId}", bootHandler)
})
```

## Configuration Options

### Core Settings

- **`enabled`**: Enable/disable authentication entirely
- **`jwksUrl`**: URL to fetch JSON Web Key Set (recommended)
- **`jwtPublicKey`**: Static RSA public key in PEM format
- **`jwksRefreshInterval`**: How often to refresh JWKS cache

### Validation Options

- **`validateExpiration`**: Check token expiration (recommended: true)
- **`validateIssuer`**: Validate token issuer (recommended: true in production)
- **`validateAudience`**: Validate token audience (recommended: true in production)
- **`requiredClaims`**: List of required claims (e.g., ["sub", "iss", "aud"])
- **`requiredScopes`**: List of required scopes (e.g., ["boot:read"])

### Development Options

- **`allowEmptyToken`**: Allow requests without tokens (dev only)
- **`nonEnforcing`**: Log errors but don't block requests (testing only)

## Configuration Examples

> **📝 Note**: For comprehensive configuration examples with detailed documentation, see `config.example.yaml` in the project root. Copy this file to `config.yaml` and customize for your environment.

### Development Environment

```yaml
auth:
  enabled: false  # Authentication disabled
```

### Production with JWKS

```yaml
auth:
  enabled: true
  jwks_url: "https://auth.openchami.org/.well-known/jwks.json"
  jwks_refresh_interval: "1h"
  jwt_issuer: "https://auth.openchami.org"
  jwt_audience: "boot-service"
  validate_expiration: true
  validate_issuer: true
  validate_audience: true
  required_claims: ["sub", "iss", "aud"]
  required_scopes: ["boot:read"]
```

### OpenCHAMI Cluster

```yaml
auth:
  enabled: true
  jwks_url: "http://tokensmith:8080/.well-known/jwks.json"
  jwt_issuer: "openchami-tokensmith"
  jwt_audience: "openchami-cluster"
  required_claims: ["sub", "cluster_id", "openchami_id"]
  required_scopes: ["boot:read"]
```

## Usage Patterns

### Basic Authentication

```go
// All routes require valid JWT
r.Use(authMiddleware)
```

### Scope-based Authorization

```go
import "github.com/openchami/boot-service/pkg/auth"

// Require specific scope
readScope := auth.CreateScopeMiddleware("boot:read")
writeScope := auth.CreateScopeMiddleware("boot:write")

r.Group(func(r chi.Router) {
    r.Use(authMiddleware)
    r.Use(readScope)
    r.Get("/boot/list", listBootConfigsHandler)
})

r.Group(func(r chi.Router) {
    r.Use(authMiddleware)
    r.Use(writeScope)
    r.Post("/boot", createBootConfigHandler)
})
```

### Service-to-Service Authentication

```go
// Require service token for internal APIs
serviceAuth := auth.CreateServiceTokenMiddleware("boot-service")

r.Group(func(r chi.Router) {
    r.Use(authMiddleware)
    r.Use(serviceAuth)
    r.Get("/internal/stats", internalStatsHandler)
})
```

### Accessing Claims in Handlers

```go
func bootHandler(w http.ResponseWriter, r *http.Request) {
    // Get claims from request context
    claims, err := auth.GetClaimsFromRequest(r)
    if err != nil {
        http.Error(w, "Failed to get claims", http.StatusInternalServerError)
        return
    }

    // Access user information
    userID := claims.Subject
    email := claims.Email
    scopes := claims.Scope
    clusterID := claims.ClusterID

    // Use claims for business logic
    log.Printf("Boot request from user %s (%s) with scopes %v", 
               userID, email, scopes)
}
```

## Token Structure

The TokenSmith middleware expects tokens with the following claims structure:

### Standard JWT Claims (RFC 7519)

- **`iss`** (issuer): Token issuer
- **`sub`** (subject): User or service identifier  
- **`aud`** (audience): Intended recipients
- **`exp`** (expiration): Token expiration time
- **`iat`** (issued at): Token issuance time
- **`nbf`** (not before): Token validity start time

### OpenCHAMI Claims

- **`cluster_id`**: OpenCHAMI cluster identifier
- **`openchami_id`**: OpenCHAMI entity identifier
- **`scope`**: Array of granted scopes
- **`groups`**: User group memberships

### NIST SP 800-63B Claims (for high-assurance environments)

- **`auth_level`**: Authentication assurance level (IAL1, IAL2, IAL3)
- **`auth_factors`**: Number of authentication factors used
- **`auth_methods`**: Authentication methods (e.g., ["password", "mfa"])
- **`session_id`**: Session identifier
- **`session_exp`**: Session expiration time

## Security Considerations

### Production Deployment

1. **Always use HTTPS** in production environments
2. **Use JWKS** for automatic key rotation
3. **Validate issuer and audience** claims
4. **Require appropriate scopes** for sensitive operations
5. **Set reasonable token lifetimes** (e.g., 1 hour for user tokens, 5 minutes for service tokens)

### Token Scopes

Common scopes for boot service operations:

- **`boot:read`**: Read boot configurations and node information
- **`boot:write`**: Create/update boot configurations
- **`boot:admin`**: Administrative operations
- **`node:read`**: Read node information
- **`node:write`**: Update node state

### Error Handling

The middleware returns appropriate HTTP status codes:

- **`401 Unauthorized`**: Missing or invalid token
- **`403 Forbidden`**: Insufficient scopes or permissions
- **`500 Internal Server Error`**: Server-side errors (e.g., JWKS fetch failures)

## Integration with OpenCHAMI Ecosystem

### TokenSmith Service

The boot service integrates with the OpenCHAMI TokenSmith service for:

- **Token Validation**: Verify JWT signatures and claims
- **JWKS Endpoints**: Automatic key rotation
- **Service Tokens**: Internal service communication

### Fabrica Policy Engine

For advanced authorization scenarios, the boot service can integrate with Fabrica for:

- **Policy-based Access Control**: Complex authorization rules
- **Role-based Access**: User role management
- **Resource-specific Permissions**: Fine-grained access control

### BSS Integration

When deployed with BSS (Boot Script Service):

- **Shared Authentication**: Common JWT validation
- **Consistent Authorization**: Uniform scope requirements
- **Service Discovery**: Automatic service token generation

## Monitoring and Observability

### Logging

Authentication events are logged at appropriate levels:

```go
2025-01-08T10:30:15Z INFO Authentication successful user=user123 scopes=[boot:read,boot:write]
2025-01-08T10:30:16Z WARN Token validation failed error="token expired"
2025-01-08T10:30:17Z ERROR JWKS refresh failed error="connection timeout"
```

### Metrics

Key metrics to monitor:

- **Authentication success/failure rates**
- **Token validation latency**
- **JWKS refresh frequency and errors**
- **Scope authorization failures**

### Health Checks

The authentication system provides health indicators:

- **JWKS connectivity** (when using JWKS URLs)
- **Token validation performance**
- **Configuration validation**

## Troubleshooting

### Common Issues

1. **"missing authorization header"**
   - Ensure clients send `Authorization: Bearer <token>` header

2. **"invalid token"**
   - Check token expiration and signature
   - Verify JWKS URL is accessible
   - Confirm issuer/audience claims

3. **"insufficient scope"**
   - Verify required scopes are granted to the user/service
   - Check scope configuration in auth settings

4. **"JWKS fetch failed"**
   - Verify JWKS URL is accessible from boot service
   - Check network connectivity and DNS resolution
   - Confirm JWKS endpoint returns valid JSON

### Debug Configuration

For debugging authentication issues:

```yaml
auth:
  enabled: true
  nonEnforcing: true  # Log errors but don't block
  # ... other settings
```

This logs authentication failures without blocking requests, useful for troubleshooting in staging environments.

## Migration Guide

### From Custom Auth to TokenSmith

1. **Update Dependencies**:
   ```bash
   go get github.com/openchami/tokensmith/middleware
   ```

2. **Replace Custom Middleware**:
   ```go
   // Old
   customAuth := myapp.CreateAuthMiddleware(config)
   
   // New  
   authConfig := auth.DefaultConfig()
   tokensmithAuth := authConfig.CreateMiddleware(logger)
   ```

3. **Update Configuration**:
   - Convert custom auth settings to TokenSmith format
   - Add JWKS URL for automatic key rotation
   - Define required scopes and claims

4. **Test Integration**:
   - Verify token validation works correctly
   - Test scope-based authorization
   - Confirm error handling behavior

### From No Auth to TokenSmith

1. **Start with Development Config**:
   ```go
   config := auth.DevConfig() // Authentication disabled
   ```

2. **Gradually Enable Features**:
   - Enable authentication with permissive settings
   - Add required claims validation
   - Implement scope-based authorization

3. **Production Hardening**:
   - Enable all validation options
   - Define strict scope requirements
   - Configure proper JWKS integration

## Additional Resources

- [OpenCHAMI TokenSmith Documentation](https://github.com/OpenCHAMI/tokensmith)
- [JWT Best Practices (RFC 8725)](https://datatracker.ietf.org/doc/html/rfc8725)
- [OpenID Connect Core 1.0](https://openid.net/specs/openid-connect-core-1_0.html)
- [OAuth 2.0 JWT Bearer Token Profiles](https://datatracker.ietf.org/doc/html/rfc7523)