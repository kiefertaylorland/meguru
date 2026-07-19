#!/usr/bin/env bash
# CI gate: every core package (./internal/...) must have >= 80% statement
# coverage (docs/product/TECH_STACK.md §7; constitution CI gates).
# cmd/meguru is excluded as a wiring-only main covered end-to-end by
# tests/e2e; tests/{e2e,integration} are test-only packages.
set -euo pipefail
cd "$(dirname "$0")/.."

threshold=80.0
out=$(go test -count=1 -cover ./internal/...)
echo "$out"

# Guard: every internal package must actually report coverage, so a new
# package without tests can't silently dodge the gate.
want=$(go list ./internal/... | wc -l | tr -d ' ')
got=$(echo "$out" | grep -c 'coverage: [0-9.]*% of statements' || true)
if [ "$got" -ne "$want" ]; then
  echo "FAIL: $got of $want internal packages reported coverage (package without tests?)"
  exit 1
fi

fail=0
while IFS= read -r line; do
  pkg=$(echo "$line" | awk '{print $2}')
  pct=$(echo "$line" | grep -o 'coverage: [0-9.]*' | awk '{print $2}')
  if awk -v p="$pct" -v t="$threshold" 'BEGIN { exit !(p < t) }'; then
    echo "FAIL: $pkg ${pct}% < ${threshold}%"
    fail=1
  fi
done < <(echo "$out" | grep 'coverage: [0-9.]*% of statements')

if [ "$fail" -eq 0 ]; then
  echo "OK: all internal packages >= ${threshold}%"
fi
exit "$fail"
