#!/usr/bin/env sh
#
# propagate-version.sh — this repository's commands.propagate.
#
# `release cut` runs it in the stage checkout with RELEASE_VERSION set and
# commits what it changes, provided every path is in release_paths. It
# writes the version into the plugin manifest, so the plugin and the CLI it
# provisions carry one version, and pins the marketplace's release entry to
# the release's tag, v<version>, so the marketplace serves the plugin as
# released rather than as the default branch has it.
set -eu

: "${RELEASE_VERSION:?RELEASE_VERSION is not set; release cut sets it}"
manifest="plugin/.claude-plugin/plugin.json"
marketplace=".claude-plugin/marketplace.json"

python3 -I - "$manifest" "$marketplace" "$RELEASE_VERSION" <<'PY'
import json, re, sys
manifest, marketplace, version = sys.argv[1], sys.argv[2], sys.argv[3]

# Both files are checked before either is written, so a failure leaves
# neither changed.
text = open(manifest, encoding="utf-8").read()
new_manifest, n = re.subn(r'("version"\s*:\s*")[^"]*(")', r"\g<1>" + version + r"\g<2>", text, count=1)
if n != 1:
    sys.exit(f"{manifest}: no version field to update")
if json.loads(new_manifest)["version"] != version:
    sys.exit(f"{manifest}: version was not written")

market = json.load(open(marketplace, encoding="utf-8"))
entries = [p for p in market.get("plugins", []) if p.get("name") == "release"]
if len(entries) != 1:
    sys.exit(f"{marketplace}: no plugin entry named release to pin")
entries[0]["source"] = {
    "source": "git-subdir",
    "url": "brettcallaghan/release",
    "path": "plugin",
    "ref": "v" + version,
}

open(manifest, "w", encoding="utf-8").write(new_manifest)
open(marketplace, "w", encoding="utf-8").write(json.dumps(market, indent=2, ensure_ascii=False) + "\n")
PY
