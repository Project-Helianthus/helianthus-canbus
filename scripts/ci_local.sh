#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
export GOWORK=off

temporary_directory="$(mktemp -d)"
trap 'rm -rf "$temporary_directory"' EXIT

echo "==> terminology gate"
echo "==> ripgrep preflight test"
./scripts/require_ripgrep_test.sh
./scripts/require_ripgrep.sh
./scripts/terminology_gate.sh

echo "==> forbidden surface gate"
./scripts/forbidden_surface_gate.sh

echo "==> gofmt"
unformatted=""
while IFS= read -r file; do
  result="$(gofmt -l "$file")"
  if [[ -n "$result" ]]; then
    unformatted+="${result}"$'\n'
  fi
done < <(rg --files -g '*.go')
if [[ -n "$unformatted" ]]; then
  printf 'gofmt required for:\n%s' "$unformatted"
  exit 1
fi

echo "==> go vet"
go vet ./...

echo "==> go build"
go build ./...

echo "==> go test (race)"
go test -race -count=1 ./...

echo "==> Linux build and test"
if [[ "$(go env GOOS)" == "linux" ]]; then
  go test -count=1 ./...
else
  GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go test -c -o "$temporary_directory/canbus-linux-amd64.test" ./
  GOOS=linux GOARCH=arm64 CGO_ENABLED=0 go test -c -o "$temporary_directory/canbus-linux-arm64.test" ./
fi

if command -v golangci-lint >/dev/null 2>&1; then
  echo "==> golangci-lint"
  golangci-lint run ./...
else
  echo "==> golangci-lint not found; dedicated CI workflow remains authoritative"
fi

echo "==> transport matrix"
./scripts/transport_gate.sh

echo "All local CI checks passed."
