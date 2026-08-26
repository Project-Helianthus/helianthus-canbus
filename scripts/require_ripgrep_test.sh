#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
empty_path="$(mktemp -d)"
trap 'rm -rf "$empty_path"' EXIT

output="$(PATH="$empty_path" /bin/bash "$repo_root/scripts/require_ripgrep.sh" 2>&1)" || status=$?
if [[ "${status:-0}" -eq 0 ]]; then
  echo 'ripgrep gate accepted a missing executable' >&2
  exit 1
fi
if [[ "$output" != *'ripgrep (rg) is required'* ]]; then
  printf 'unexpected missing-ripgrep failure: %s\n' "$output" >&2
  exit 1
fi
