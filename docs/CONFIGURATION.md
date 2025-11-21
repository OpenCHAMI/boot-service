<!--
SPDX-FileCopyrightText: 2025 OpenCHAMI Contributors

SPDX-License-Identifier: MIT
-->

# Configuration Guide

This document explains how to configure the OpenCHAMI boot service.

## Quick Start

1. **Copy the example configuration**:
   ```bash
   cp config.example.yaml config.yaml
   ```

2. **Edit for your environment**:
   ```bash
   # Edit the configuration file
   nano config.yaml  # or your preferred editor
   ```

3. **Start the service**:
   ```bash
   ./bin/boot-service serve
   ```

## Configuration Methods

The boot service supports multiple configuration methods in order of precedence:

1. **Command-line flags** (highest priority)
2. **Environment variables** (e.g., `BOOT_SERVICE_PORT=8082`)
3. **Configuration file** (`config.yaml`)
4. **Default values** (lowest priority)

### Configuration File

The recommended approach for most deployments is to use a configuration file:

- **Location**: `config.yaml` in the working directory
- **Format**: YAML
- **Example**: See `config.example.yaml` for comprehensive documentation
- **Git**: The actual `config.yaml` is gitignored to prevent accidental commits of sensitive data

### Environment Variables

Useful for containerized deployments:

```bash
export BOOT_SERVICE_PORT=8082
export BOOT_SERVICE_ENABLE_AUTH=true
export BOOT_SERVICE_HSM_URL=http://smd:27779
./bin/boot-service serve
```

### Command-line Flags

Useful for quick testing and overrides:

```bash
./bin/boot-service serve --port 8082 --enable-auth --hsm-url http://localhost:27779
```

## Configuration Sections

### Core Server Settings

```yaml
port: 8082                    # HTTP server port
host: "0.0.0.0"              # Bind interface
read_timeout: 30             # Request read timeout (seconds)
write_timeout: 30            # Response write timeout (seconds)
idle_timeout: 120            # Connection idle timeout (seconds)
```

### Storage Configuration

```yaml
data_dir: "./data"           # Directory for file storage
storage_type: "file"         # Storage backend (file, database)
```

### Feature Toggles

```yaml
enable_auth: false           # Enable JWT authentication
enable_metrics: true         # Enable Prometheus metrics
enable_legacy_api: true      # Enable BSS-compatible API
metrics_port: 9092          # Metrics endpoint port
```

### Authentication (when enable_auth: true)

```yaml
auth:
  enabled: true
  jwks_url: "https://auth.example.com/.well-known/jwks.json"
  jwt_issuer: "https://auth.example.com"
  jwt_audience: "boot-service"
  required_scopes: ["boot:read"]
```

### External Services

```yaml
hsm_url: "http://localhost:27779"    # Hardware State Manager

tokensmith:
  url: "http://localhost:8080"       # TokenSmith auth service
  timeout: 30
```

## Environment-Specific Examples

### Development

```yaml
# Minimal development configuration
enable_auth: false
enable_metrics: true
logging:
  level: "debug"
development:
  enabled: true
```

### Production

```yaml
# Production configuration with full security
enable_auth: true
auth:
  enabled: true
  jwks_url: "https://auth.openchami.org/.well-known/jwks.json"
  jwt_issuer: "https://auth.openchami.org"
  jwt_audience: "boot-service"
  required_scopes: ["boot:read"]
logging:
  level: "info"
  format: "json"
```

### Kubernetes/Container

```yaml
# Container-friendly configuration
port: 8080
host: "0.0.0.0"
data_dir: "/data"
auth:
  jwks_url: "http://tokensmith:8080/.well-known/jwks.json"
  jwt_issuer: "openchami-tokensmith"
hsm_url: "http://smd:27779"
logging:
  format: "json"
  output: "stdout"
```

## Validation

The service validates configuration at startup and will exit with an error if:

- Required authentication settings are missing when auth is enabled
- Invalid URLs are provided for external services
- Conflicting settings are detected

Check the startup logs for configuration validation results:

```
2025/10/09 11:00:27 Starting boot service with configuration:
2025/10/09 11:00:27   Server: 0.0.0.0:8082
2025/10/09 11:00:27   Storage: file (./data)
2025/10/09 11:00:27   Features: auth=false, hsm=true, metrics=true, legacy-api=true
```

## Security Considerations

### Configuration File Security

- **Never commit** `config.yaml` to version control (it's gitignored)
- **Use restrictive permissions**: `chmod 600 config.yaml`
- **Store secrets securely**: Consider using environment variables for sensitive data

### Production Checklist

- ✅ Enable authentication (`enable_auth: true`)
- ✅ Use JWKS for key rotation (`jwks_url`)
- ✅ Validate issuer and audience (`validate_issuer: true`, `validate_audience: true`)
- ✅ Require appropriate scopes (`required_scopes`)
- ✅ Use HTTPS for external service URLs
- ✅ Set reasonable timeouts
- ✅ Enable structured logging (`format: "json"`)

## Troubleshooting

### Common Issues

1. **Service won't start**:
   - Check configuration file syntax: `yamllint config.yaml`
   - Verify file permissions and existence
   - Check logs for specific validation errors

2. **Authentication not working**:
   - Verify `enable_auth` matches `auth.enabled`
   - Check JWKS URL accessibility
   - Validate issuer/audience claims in tokens

3. **External services unreachable**:
   - Verify URLs are accessible from the service
   - Check network connectivity and DNS resolution
   - Review timeout settings

### Debug Configuration

For troubleshooting configuration issues:

```yaml
logging:
  level: "debug"              # Verbose logging
auth:
  non_enforcing: true         # Log auth errors but don't block
development:
  enabled: true              # Additional debug features
```

## Migration

### From Command-line Flags

1. Create `config.yaml` based on your current flags
2. Test that the service starts correctly
3. Remove flags from your startup scripts

### Adding Authentication

1. Start with `enable_auth: false`
2. Configure auth section with permissive settings
3. Test with valid tokens
4. Gradually tighten validation requirements
5. Enable enforcement: `enable_auth: true`

## See Also

- [Authentication Documentation](AUTHENTICATION.md) - Detailed auth configuration
- [API Documentation](API.md) - REST API reference
- `config.example.yaml` - Comprehensive configuration example with comments
