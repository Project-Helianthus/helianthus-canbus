#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"

first="m""aster"
second="sl""ave"
for term in "$first" "$second"; do
  if rg --hidden --glob '!.git/**' -n -i -w -- "$term" .; then
    echo "Found legacy terminology."
    exit 1
  fi
done
