#!/usr/bin/env bash
set -euo pipefail

repo_root="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$repo_root"
export GOWORK=off

echo "==> generic scope; frame submission is admitted in package periodic (item 11)"
go test -count=1 -run '^(TestListenerPublicSurfaceIsReceiveOnly|TestProductCodeHasNoOutboundOrMutationSurface|TestTestsDoNotOpenAPlatformEndpoint|TestRepositoryHasNoDomainSpecificAssumptions)$' .

# Item 11: «un API pe CAN pentru publicare cu periodicitate deterministica de registri».
# The socket write of a frame is admitted only in package periodic.
if rg --hidden --glob '!.git/**' --glob '*.go' --glob '!periodic/**' --glob '!*_test.go' -n 'unix\.Write\(' .; then
  echo "Frame submission outside package periodic."
  exit 1
fi

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
