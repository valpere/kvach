#!/usr/bin/env bash
# go-check.sh — Common verification functions for kvach skills.
#
# Source this file in skill scripts that need build/test/vet checks:
#   source "$(dirname "$0")/../lib/go-check.sh"

set -euo pipefail

# go_build runs `go build ./...` and returns 0 on success.
go_build() {
    echo "==> go build ./..."
    go build ./...
}

# go_test runs `go test ./...` and returns 0 on success.
go_test() {
    echo "==> go test ./..."
    go test ./...
}

# go_vet runs `go vet ./...` and returns 0 on success.
go_vet() {
    echo "==> go vet ./..."
    go vet ./...
}

# go_fmt runs gofmt and returns 0 if all files are formatted.
go_fmt_check() {
    echo "==> gofmt -l ."
    local unformatted
    unformatted=$(gofmt -l . 2>&1) || true
    if [ -n "$unformatted" ]; then
        echo "Unformatted files:"
        echo "$unformatted"
        return 1
    fi
    return 0
}

# go_fmt_fix formats all Go files in place.
go_fmt_fix() {
    echo "==> gofmt -w ."
    gofmt -w .
}

# full_check runs build + vet + test. Returns 0 only if all pass.
full_check() {
    go_build && go_vet && go_test
}
