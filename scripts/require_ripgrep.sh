#!/usr/bin/env bash
set -euo pipefail

if ! command -v rg >/dev/null 2>&1; then
  echo 'ripgrep (rg) is required for repository scan gates' >&2
  exit 1
fi
