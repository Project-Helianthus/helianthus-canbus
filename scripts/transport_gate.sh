#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
export GOWORK=off

go test -count=1 -run '^TestReceiveOnlyTransportMatrixT01ToT88$' .
