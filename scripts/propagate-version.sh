#!/usr/bin/env sh
#
# propagate-version.sh — this repository's commands.propagate.
#
# `release cut` runs it in the stage checkout with RELEASE_VERSION set and
# commits what it changes, provided every path is in release_paths. It
# writes the version into the plugin manifest, so the plugin and the CLI it
# provisions carry one version.
set -eu

: "${RELEASE_VERSION:?RELEASE_VERSION is not set; release cut sets it}"
manifest="plugin/.claude-plugin/plugin.json"

python3 -I - "$manifest" "$RELEASE_VERSION" <<'PY'
import json, re, sys
path, version = sys.argv[1], sys.argv[2]
text = open(path, encoding="utf-8").read()
new, n = re.subn(r'("version"\s*:\s*")[^"]*(")', r"\g<1>" + version + r"\g<2>", text, count=1)
if n != 1:
    sys.exit(f"{path}: no version field to update")
if json.loads(new)["version"] != version:
    sys.exit(f"{path}: version was not written")
open(path, "w", encoding="utf-8").write(new)
PY
