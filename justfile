# hex — task runner
#
# Run `just` with no arguments to list all recipes.

set shell := ["bash", "-euo", "pipefail", "-c"]

# List available recipes.
default:
    @just --list

# Build all packages.
build:
    go build ./...

# Run tests.
test:
    go test ./...

# Run tests with the race detector.
race:
    go test -race ./...

# Run tests verbosely, race-enabled, with coverage.
test-all:
    go test -race -v -cover ./...

# Emit an HTML coverage report to coverage.html.
cover:
    go test -race -coverprofile=coverage.out ./...
    go tool cover -html=coverage.out -o coverage.html
    @echo "→ coverage.html"

# Run go vet.
vet:
    go vet ./...

# Format all Go source in place.
fmt:
    gofmt -s -w .

# Verify formatting is clean; fail if anything would change.
fmt-check:
    @diff=$(gofmt -s -l .); \
    if [ -n "$diff" ]; then \
        echo "unformatted files:"; echo "$diff"; exit 1; \
    fi

# Tidy the module graph.
tidy:
    go mod tidy

# Full pre-commit gate: format check, vet, race tests.
check: fmt-check vet race

# Remove build/test artifacts.
clean:
    rm -f coverage.out coverage.html
    go clean -testcache
