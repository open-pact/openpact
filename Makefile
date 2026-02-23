.PHONY: build test clean docker

# Build the binary
build:
	go build -o openpact ./cmd/openpact

# Run tests
test:
	go test -v ./...

# Run tests with coverage
coverage:
	go test -coverprofile=coverage.out ./...
	go tool cover -html=coverage.out -o coverage.html

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
