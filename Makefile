.PHONY: build test clean docker

# Build the binary
build:
	go build -o openpact ./cmd/openpact

# Run tests
test:
	go test -v ./...

# Run tests with coverage (excludes chat providers which require external services)
coverage:
	go test -coverprofile=coverage.out $(shell go list ./... | grep -v /internal/providers/)
	go tool cover -func coverage.out

# Clean build artifacts
clean:
	rm -f openpact coverage.out coverage.html

# Build Docker image
docker:
	docker build -t openpact:latest .

# Run locally
run: build
	./openpact start

# Format code
fmt:
	go fmt ./...

# Lint (requires golangci-lint)
lint:
	golangci-lint run
