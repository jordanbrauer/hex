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

# Run golangci-lint, plus the format + generated-docs checks CI also gates on.
lint: fmt-check man-check
    golangci-lint run ./...

# Format all Go source in place.
fmt:
    gofmt -s -w .

# Verify formatting is clean; fail if anything would change. Helper for lint.
[private]
fmt-check:
    @diff=$(gofmt -s -l .); \
    if [ -n "$diff" ]; then \
        echo "unformatted files:"; echo "$diff"; exit 1; \
    fi

# Verify generated manpage markdown matches the command tree. Helper for lint.
[private]
man-check:
    #!/usr/bin/env bash
    set -euo pipefail
    go run ./cmd/hex gen-man
    if ! git diff --quiet -- docs/man; then
        echo "Generated manpage markdown is stale."
        echo "Run 'just man' (or 'go run ./cmd/hex gen-man') and commit docs/man/*.md."
        git diff -- docs/man
        exit 1
    fi

# Tidy the module graph.
tidy:
    go mod tidy

# Regenerate the manpage markdown sources and render them to roff with
# pandoc. Requires pandoc on PATH. Generated: docs/man/hex.{1,3}.md;
# hand-authored: docs/man/hex.5.md, docs/man/hex.7.md.
man:
    go run ./cmd/hex gen-man
    @mkdir -p man
    @for f in docs/man/*.md; do \
        base=$(basename "$f" .md); \
        pandoc -s -t man "$f" -o "man/$base"; \
        echo "→ man/$base"; \
    done

# Full pre-commit gate: lint (fmt-check + man-check + golangci-lint), vet, race tests.
check: lint vet race

# Remove build/test artifacts.
clean:
    rm -f coverage.out coverage.html
    go clean -testcache
