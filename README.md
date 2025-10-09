# boot-service

Resource-based REST API with Fabrica created with Fabrica.

## Quick Start

### Configuration

Set up your configuration file:

```bash
# Copy the example configuration
cp config.example.yaml config.yaml

# Edit the configuration for your environment
# See config.example.yaml for detailed documentation of all options
```

### Generate Handlers

```bash
fabrica generate
```

### Run the Server

```bash
# Using configuration file
go run cmd/server/main.go

# Or with command-line flags
go run cmd/server/main.go --port 8082 --enable-auth --hsm-url http://localhost:27779
```

## Configuration

The boot service supports configuration via:

1. **Configuration file** (`config.yaml`) - Recommended for most deployments
2. **Environment variables** - Useful for containerized deployments
3. **Command-line flags** - Quick testing and overrides

### Example Configurations

See `config.example.yaml` for comprehensive configuration examples including:

- Development environment (authentication disabled)
- Production environment (full authentication and validation)
- Kubernetes/container deployments
- External service integration (HSM, TokenSmith, BSS)

For detailed configuration documentation, see [Configuration Guide](docs/CONFIGURATION.md).

### Key Configuration Options

- **Authentication**: Enable/disable JWT authentication with TokenSmith
- **Storage**: Configure data storage backend (file system, future database support)
- **External Services**: HSM (Hardware State Manager), BSS (Boot Script Service)
- **Performance**: Timeouts, rate limiting, connection pooling
- **Monitoring**: Metrics, logging, health checks

## Documentation

See [docs/](docs/) for detailed documentation:
