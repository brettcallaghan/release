#!/usr/bin/env bash
#
# Measure statement coverage of the release CLI across every layer that
# exercises it, and render the result into README.md.
#
# Three legs, run separately so each layer's contribution is visible:
#
#   unit      every package's own tests, minus the snapshot replays
#   snapshot  the data-driven cases under testdata/cases/ and the golden help
#   e2e       cmd/release's tests, which build the binary and run it
#
# The e2e leg's subject is a separate process, so `go test -coverprofile`
# sees nothing it does: the binary is built with `go build -cover`, each run
# writes counters to GOCOVERDIR, and `go tool covdata textfmt` turns them into
# a profile the other legs can be merged with.
#
# Usage:
#   scripts/coverage.sh              measure, write README, write reports
#   scripts/coverage.sh --check      measure, fail if the README is stale
#   scripts/coverage.sh --min 70     also fail below a total percentage
#
# Reports land in build/coverage/ (gitignored). Regenerate with the Go
# version in go.mod: statement counts come from the toolchain, so another
# version can move the numbers and make --check fail on an unrelated change.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
OUT_DIR="$REPO_ROOT/build/coverage"
MODULE="github.com/brettcallaghan/release"

# Instrument every package, not just the one under test: a snapshot case run
# from internal/cli spends most of its statements elsewhere.
COVERPKG="$MODULE/..."

# Split out of the unit leg by name. A replay test added without being listed
# here is reported under "unit", which moves a number between columns and
# leaves the total correct.
SNAPSHOT_TESTS='TestSnapshotCases|TestGoldenHelp'

REPORT_ARGS=()
while [ $# -gt 0 ]; do
  case "$1" in
    --check) REPORT_ARGS+=(--check) ;;
    --min)
      [ $# -ge 2 ] || { echo "--min requires a percentage" >&2; exit 2; }
      REPORT_ARGS+=(--min "$2"); shift ;;
    -h|--help) echo "usage: scripts/coverage.sh [--check] [--min PCT]"; exit 0 ;;
    *) echo "unknown argument: $1" >&2; exit 2 ;;
  esac
  shift
done

# A stale directory would let a deleted package keep reporting coverage.
rm -rf "$OUT_DIR"
mkdir -p "$OUT_DIR/e2e-data"
cd "$REPO_ROOT"

# -count=1 everywhere: a cached result replays without running, so the e2e
# leg would write no counters and the others would report the cache.
unit_packages=()
while IFS= read -r pkg; do
  [ "$pkg" = "$MODULE/cmd/release" ] && continue
  unit_packages+=("$pkg")
done < <(go list ./...)

echo "==> unit tests"
go test -count=1 -coverpkg="$COVERPKG" -coverprofile="$OUT_DIR/unit.cov" \
  -skip "$SNAPSHOT_TESTS" "${unit_packages[@]}"

echo
echo "==> snapshot cases and golden help"
go test -count=1 -coverpkg="$COVERPKG" -coverprofile="$OUT_DIR/snapshot.cov" \
  -run "$SNAPSHOT_TESTS" ./internal/cli

echo
echo "==> end-to-end binary tests"
RELEASE_E2E_COVERDIR="$OUT_DIR/e2e-data" go test -count=1 ./cmd/release
go tool covdata textfmt -i="$OUT_DIR/e2e-data" -o="$OUT_DIR/e2e.cov"

echo
# Held rather than propagated, so a failing --check or --min still leaves the
# annotated source behind to diagnose it from.
report_status=0
python3 "$REPO_ROOT/scripts/coverage-report.py" \
  --module "$MODULE" \
  --source-root "$REPO_ROOT" \
  --exclude-generated \
  --layer "Unit tests=$OUT_DIR/unit.cov" \
  --layer "Snapshot cases=$OUT_DIR/snapshot.cov" \
  --layer "End-to-end (built binary)=$OUT_DIR/e2e.cov" \
  --merged-out "$OUT_DIR/coverage.out" \
  --markdown-out "$OUT_DIR/summary.md" \
  --readme "$REPO_ROOT/README.md" \
  "${REPORT_ARGS[@]+"${REPORT_ARGS[@]}"}" || report_status=$?

if [ -f "$OUT_DIR/coverage.out" ]; then
  go tool cover -func="$OUT_DIR/coverage.out" -o "$OUT_DIR/coverage-func.txt"
  go tool cover -html="$OUT_DIR/coverage.out" -o "$OUT_DIR/coverage.html"
  echo "tables:           build/coverage/summary.md"
  echo "per function:     build/coverage/coverage-func.txt"
  echo "annotated source: build/coverage/coverage.html"
fi

exit "$report_status"
