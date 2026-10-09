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
import json, os, re, shutil, sys, tempfile
manifest, marketplace, version = sys.argv[1], sys.argv[2], sys.argv[3]

# Both files are validated, and both new contents written to temp files beside
# their targets, before either target is replaced. A failure up to that point
# leaves both unchanged; the only window left is between the two os.replace
# calls.
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

staged = []
try:
    for target, content in (
        (manifest, new_manifest),
        (marketplace, json.dumps(market, indent=2, ensure_ascii=False) + "\n"),
    ):
        fd, tmp = tempfile.mkstemp(prefix=".propagate-", dir=os.path.dirname(target) or ".")
        staged.append((tmp, target))
        with os.fdopen(fd, "w", encoding="utf-8") as f:
            f.write(content)
        shutil.copymode(target, tmp)
    for tmp, target in staged:
        os.replace(tmp, target)
        staged = [(t, g) for t, g in staged if t != tmp]
finally:
    for tmp, _ in staged:
        try:
            os.unlink(tmp)
        except FileNotFoundError:
            pass
PY
