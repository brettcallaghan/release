#!/usr/bin/env bash
#
# Regenerate every snapshot case's expected/ artifacts, the golden help
# under testdata/help/ and the changelog renderer's golden files under
# internal/render/testdata/ from the current implementation. It runs the same test
# that replays them, with -update, so the two cannot disagree.
#
# It does not judge whether the diff is right. Review it before committing:
# a changed snapshot is a changed command surface.
set -euo pipefail

REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

go test -count=1 ./internal/cli -run 'TestSnapshotCases|TestGoldenHelp' -update
go test -count=1 ./internal/render -run 'TestGoldenFiles' -update

echo
echo "review the diff before committing:"
echo "  git -C \"$REPO_ROOT\" diff -- testdata internal/render/testdata"
