#!/usr/bin/env bash
#
# Regenerate every snapshot case's expected/ artifacts and the golden help
# under testdata/help/ from the current implementation. It runs the same test
# that replays them, with -update, so the two cannot disagree.
#
# It does not judge whether the diff is right. Review it before committing:
# a changed snapshot is a changed command surface.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

go test -count=1 ./internal/cli -run 'TestSnapshotCases|TestGoldenHelp' -update

echo
echo "review the diff before committing:"
echo "  git -C \"$REPO_ROOT\" diff -- testdata"
