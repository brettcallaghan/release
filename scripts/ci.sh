#!/usr/bin/env bash
#
# Everything a contributor should see green before pushing. swarm.toml's
# [[checks]] is the per-bead bar; this is the pre-push bar, and adds the
# slower checks a bead does not pay for.
set -uo pipefail
cd "$(git rev-parse --show-toplevel)"

rc=0
# No `set -e` and no `&&` chaining: one failure must not hide the checks
# behind it. Every status is recorded and the script fails at the end.
check() { # check <label> <sh -c command>
  printf '\n=== %s\n' "$1"
  if sh -c "$2"; then :; else rc=1; printf '!!! FAILED: %s\n' "$1"; fi
}

check "format"    'scripts/format-check'
check "vet"       'go vet ./...'
check "build"     'go build ./...'
check "test-go"   'go test ./...'
check "coverage"  'scripts/coverage.sh --check'
check "corpus"    'command -v specs >/dev/null 2>&1 || { echo "corpus: SKIPPED (specs not on PATH)"; exit 0; }; specs check --strict'
check "schemas"   'command -v strata >/dev/null 2>&1 || { echo "schemas: SKIPPED (strata not on PATH)"; exit 0; }; scripts/generate-schemas.sh --check'
check "skills"    'scripts/check-skills.sh --strict'
check "plugin"    'command -v claude >/dev/null 2>&1 || { echo "plugin: SKIPPED (claude not on PATH)"; exit 0; }; claude plugin validate plugin --strict; a=$?; claude plugin validate . --strict; b=$?; exit $(( a | b ))'

printf '\n=== RESULT: %s\n' "$([ $rc -eq 0 ] && echo PASS || echo FAIL)"
exit $rc
