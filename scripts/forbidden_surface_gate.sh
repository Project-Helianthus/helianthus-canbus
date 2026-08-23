#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
export GOWORK=off

echo "==> generic scope and receive-only surface"
go test -count=1 -run '^(TestListenerPublicSurfaceIsReceiveOnly|TestProductCodeHasNoOutboundOrMutationSurface|TestTestsDoNotOpenAPlatformEndpoint|TestRepositoryHasNoDomainSpecificAssumptions)$' .

blocked_words=("Gr""ee" "Grow""att" "V""RF" "H""VAC" "M""94" "M""115")
for term in "${blocked_words[@]}"; do
  if rg --hidden --glob '!.git/**' -n -i -w -- "$term" .; then
    echo "Found domain-specific term."
    exit 1
  fi
done

blocked_fragments=("20"" kbit/s" "20""kbit/s" "CAN""+")
for term in "${blocked_fragments[@]}"; do
  if rg --hidden --glob '!.git/**' -n -i -F -- "$term" .; then
    echo "Found out-of-scope transport assumption."
    exit 1
  fi
done
