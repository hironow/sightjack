# sightjack task runner

# Run all tests
test:
    go test ./...

# Run tests with verbose output
test-v:
    go test -v ./...

# Run tests with coverage
test-cov:
    go test -coverprofile=coverage.out ./...
    go tool cover -func=coverage.out

# Build binary
build:
    go build -o sightjack ./cmd/sightjack

# Run linter
lint:
    go vet ./...

# Format code
fmt:
    gofmt -w .

# Clean build artifacts
clean:
    rm -f sightjack coverage.out
