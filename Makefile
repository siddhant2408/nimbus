.PHONY: build dev start test clean frontend

# Build the binary with embedded frontend assets.
build: frontend
	go build -o bin/nimbus ./cmd/nimbus

# Build frontend assets.
frontend:
	cd web && npm ci && npm run build

# Run the binary in dev mode.
dev:
	go run ./cmd/nimbus $(ARGS)

# Start the server at port 9090.
start: build
	./bin/nimbus --port 9090 WORKFLOW.md $(ARGS)

# Run all Go tests.
test:
	go test ./...

# Run Go linter.
vet:
	go vet ./...

# Clean build artifacts.
clean:
	rm -rf bin/ web/dist/ web/node_modules/
