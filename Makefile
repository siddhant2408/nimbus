.PHONY: build dev test clean frontend

# Build the binary with embedded frontend assets.
build: frontend
	go build -o bin/symphony ./cmd/symphony

# Build frontend assets.
frontend:
	cd web && npm ci && npm run build

# Run the binary in dev mode.
dev:
	go run ./cmd/symphony $(ARGS)

# Run all Go tests.
test:
	go test ./...

# Run Go linter.
vet:
	go vet ./...

# Clean build artifacts.
clean:
	rm -rf bin/ web/dist/ web/node_modules/
