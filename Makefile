.PHONY: build run test generate clean dev codegen-init

build:
	go build -o bin/server ./cmd/server/
	go build -o bin/client ./cmd/client/

run: build
	./bin/server

test:
	go test ./...

# Generate handlers, storage, and OpenAPI specs
generate:
	fabrica generate --handlers --storage --openapi --client

# Development workflow: regenerate and build
dev: clean generate build
	@echo "✅ Development build complete"

clean:
	rm -rf bin/
	rm -f cmd/server/*_generated.go
	rm -f internal/storage/storage_generated.go
	rm -f pkg/client/*_generated.go
	rm -f pkg/resources/register_generated.go
