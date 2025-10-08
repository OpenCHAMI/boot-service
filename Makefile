.PHONY: build run test generate clean dev codegen-init

build:
	go build -o bin/server cmd/server/main.go

run: build
	./bin/server

test:
	go test ./...

# Initialize code generation (run after adding resources)
codegen-init:
	fabrica codegen init

# Generate handlers, storage, and OpenAPI specs
generate:
	fabrica generate --handlers --storage --openapi

# Development workflow: regenerate and build
dev: clean codegen-init generate build
	@echo "✅ Development build complete"

clean:
	rm -rf bin/
	rm -f cmd/server/*_generated.go
	rm -f internal/storage/storage_generated.go
	rm -f pkg/client/*_generated.go
	rm -f pkg/resources/register_generated.go
