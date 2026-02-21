# Sightjack — task runner
# https://just.systems

set shell := ["bash", "-eu", "-o", "pipefail", "-c"]

# Default: show help
default: help

# Help: list available recipes
help:
    @just --list --unsorted

# Define specific commands
MARKDOWNLINT := "bunx markdownlint-cli2"

# Install prek hooks (pre-commit + pre-push) with quiet mode
prek-install:
    prek install -t pre-commit -t pre-push --overwrite
    @sed 's/-- "\$@"/--quiet -- "\$@"/' .git/hooks/pre-commit > .git/hooks/pre-commit.tmp && mv .git/hooks/pre-commit.tmp .git/hooks/pre-commit && chmod +x .git/hooks/pre-commit
    @sed 's/-- "\$@"/--quiet -- "\$@"/' .git/hooks/pre-push > .git/hooks/pre-push.tmp && mv .git/hooks/pre-push.tmp .git/hooks/pre-push && chmod +x .git/hooks/pre-push
    @echo "prek hooks installed (quiet mode)"

# Run all prek hooks on all files
prek-run:
    prek run --all-files

# Lint markdown files
lint-md:
    @{{MARKDOWNLINT}} --fix "*.md" "docs/**/*.md"

# Version from git tags
VERSION := `git describe --tags --always --dirty 2>/dev/null || echo "dev"`
COMMIT := `git rev-parse --short HEAD 2>/dev/null || echo "dev"`
DATE := `date -u +%Y-%m-%dT%H:%M:%SZ`
LDFLAGS := "-X github.com/hironow/sightjack/internal/cmd.version=" + VERSION + " -X github.com/hironow/sightjack/internal/cmd.commit=" + COMMIT + " -X github.com/hironow/sightjack/internal/cmd.date=" + DATE

# Build the binary with version info
build:
    go build -ldflags "{{LDFLAGS}}" -o sightjack ./cmd/sightjack

# Build and install to /usr/local/bin
install: build
    mv sightjack /usr/local/bin/

# Run all tests
test:
    go test ./... -count=1 -timeout=300s

# Run tests with verbose output
test-v:
    go test ./... -count=1 -timeout=300s -v

# Run tests with race detector
test-race:
    go test ./... -race -count=1 -timeout=300s

# Run tests with coverage report
cover:
    go test ./... -coverprofile=coverage.out -count=1 -timeout=300s
    go tool cover -func=coverage.out

# Open coverage in browser
cover-html: cover
    go tool cover -html=coverage.out

# Format code
fmt:
    gofmt -w .

# Run go vet
vet:
    go vet ./...

# Lint semgrep rules (cobra best practices)
lint-semgrep:
    semgrep --config .semgrep/ ./internal/cmd/ ./cmd/sightjack/

# Lint (fmt check + vet + markdown lint + semgrep)
lint: vet lint-md lint-semgrep
    @gofmt -l . | grep . && echo "gofmt: files need formatting" && exit 1 || true

# Format, vet, test — full check before commit
check: fmt vet test

# Run sightjack doctor (quick smoke test after build)
doctor: build
    ./sightjack doctor

# Start Jaeger v2 (OTel trace viewer + MCP) on http://localhost:16686
jaeger:
    docker compose -f docker/compose.yaml up -d
    @echo "Jaeger UI:      http://localhost:16686"
    @echo "OTLP endpoint:  http://localhost:4318"
    @echo "MCP endpoint:   http://localhost:16687/mcp"
    @echo ""
    @echo "Run sightjack with tracing:"
    @echo "  OTEL_EXPORTER_OTLP_ENDPOINT=http://localhost:4318 sightjack run"

# Stop Jaeger
jaeger-down:
    docker compose -f docker/compose.yaml down

# Generate CLI Markdown docs from cobra commands
docs:
    go run ./internal/tools/docgen

# Clean build artifacts
clean:
    rm -f sightjack coverage.out
    go clean
