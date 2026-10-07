#!/usr/bin/env bash
# check-skills.sh — verify this repo's invariants about its own skills.
#
# Why this exists
#   An 80-column reflow can wrap a SKILL.md's YAML `description` without
#   continuation indentation. The frontmatter then stops parsing, the skill's
#   discovery description is gone, and it never triggers. This gate says so.
#
# Usage
#   scripts/check-skills.sh            # report problems
#   scripts/check-skills.sh --strict   # treat warnings as errors too
#
# Exit status
#   0  clean (warnings allowed unless --strict)
#   1  errors found
#   2  bad invocation

set -uo pipefail
REPO_ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"
cd "$REPO_ROOT"

case "${1-}" in
  --strict) STRICT=1 ;;
  "")       STRICT=0 ;;
  *)        echo "usage: $(basename "$0") [--strict]" >&2; exit 2 ;;
esac

command -v python3 >/dev/null 2>&1 || { echo "check-skills: python3 required" >&2; exit 2; }

STRICT=$STRICT python3 - <<'PY'
import glob, os, re, sys

# The skill re-injection cap: after a compaction Claude re-reads at most
# ~5,000 tokens of a loaded skill, and anything past that is simply gone.
# Expressed in bytes at ~4 bytes/token so the gate needs no tokenizer.
BODY_BYTE_MAX = 20000

errors, warnings, notes = [], [], []
def err(w, m):  errors.append(f"{w}: {m}")
def warn(w, m): warnings.append(f"{w}: {m}")

def is_generated(p):
    # .claude/skills/ holds committed symlinks to the plugin/skills/ sources.
    # A symlinked skill is already checked via the plugin/skills glob, so
    # checking it twice would double-report.
    return os.path.islink(os.path.dirname(p))

SKILLS = [p for p in sorted(glob.glob('plugin/skills/*/SKILL.md'))
                  + sorted(glob.glob('.claude/skills/*/SKILL.md'))
          if not is_generated(p)]

# ---- 1. frontmatter parses, and the two required fields are valid ----------
try:
    import yaml
    HAVE_YAML = True
except ImportError:
    HAVE_YAML = False
    notes.append("frontmatter: pyyaml not installed — using a reduced structural check (pip install pyyaml for the full parse)")

for p in SKILLS:
    text = open(p).read()
    if not text.startswith('---\n'):
        err(p, "does not begin with a YAML frontmatter block"); continue
    parts = re.split(r'^---[ \t]*\n', text, maxsplit=2, flags=re.M)
    if len(parts) < 3:
        err(p, "frontmatter block is not closed by a --- line"); continue
    fm, body = parts[1], parts[2]
    if HAVE_YAML:
        try:
            d = yaml.safe_load(fm)
        except Exception as e:
            # This is the failure mode that motivated the script: a wrapped
            # scalar whose continuation lines lost their indentation.
            err(p, f"frontmatter does not parse ({type(e).__name__}). A wrapped "
                   f"`description:` needs its continuation lines indented.")
            continue
        if not isinstance(d, dict):
            err(p, "frontmatter is not a mapping"); continue
    else:
        # One pass: group each key with the lines that follow it.
        raw, cur = {}, None            # key -> [inline value, continuation lines, items]
        for ln in fm.split('\n'):
            if not ln.strip():
                continue
            m = re.match(r'^([A-Za-z0-9_-]+):[ ]*(.*)$', ln)
            if m:
                cur = raw[m.group(1)] = [m.group(2).strip(), [], []]
            elif ln[0] in ' \t' and cur is not None:
                cur[1].append(ln.strip())
            elif (ln == '-' or ln.startswith('- ')) and cur is not None and cur[0] == '':
                cur[2].append(ln[1:].strip())
            else:
                err(p, f"frontmatter line is neither a key nor an indented "
                       f"continuation: {ln[:60]!r}")
        d = {}
        for k, (inline, cont, items) in raw.items():
            if items:
                d[k] = items
            elif re.fullmatch(r'[|>][-+0-9]*', inline):
                d[k] = ('\n' if inline[0] == '|' else ' ').join(cont)
            else:
                d[k] = ' '.join([inline] + cont).strip().strip('"')

    name, desc = d.get('name'), d.get('description')
    if name is None or name == '':
        err(p, "missing `name`")
    elif not isinstance(name, str):
        err(p, f"name must be a string, not {type(name).__name__}")
    else:
        if len(name) > 64:                     err(p, f"name is {len(name)} chars (max 64)")
        if not re.fullmatch(r'[a-z0-9-]+', name): err(p, f"name '{name}' must be lowercase letters, numbers, hyphens")
        for w in ('anthropic', 'claude'):
            if w in name.lower():              err(p, f"name contains the reserved word '{w}'")
        if os.path.basename(os.path.dirname(p)) != name:
            err(p, f"name '{name}' does not match its directory")
    if desc is None or desc == '':
        err(p, "missing `description` — without it the skill will not be discovered")
    elif not isinstance(desc, str):
        err(p, f"description must be a string, not {type(desc).__name__}")
    else:
        if len(desc) > 1024: err(p, f"description is {len(desc)} chars (max 1024)")
        if len(desc) < 40:   warn(p, f"description is only {len(desc)} chars — likely too thin to trigger on")
        for pov in (' I ', 'I can ', 'you can ', 'You can '):
            if pov in desc: warn(p, f"description may not be third person (found {pov!r})")

    # Size the body against the 5,000-token skill re-injection cap — what
    # Claude re-reads after a compaction. Lines are the wrong unit: the same
    # content rewrapped from 80 to 200 columns loses two thirds of its lines
    # and not one token, so a line gate is satisfied by reflowing. Bytes are
    # proportional to tokens, need no tokenizer (this must run in CI and on a
    # laptop with nothing installed), and at the ~4 bytes/token rule of thumb
    # for English prose plus markdown, 5,000 tokens is ~20,000 bytes.
    n = len(body.encode('utf-8'))
    if n > BODY_BYTE_MAX:
        warn(p, f"body is {n} bytes (~{n // 4} tokens); above ~{BODY_BYTE_MAX} "
                f"the tail is truncated from context after a compaction")

# ---- 2. .claude/skills symlinks resolve to a plugin skill -----------------
PLUGIN_SKILLS = os.path.realpath('plugin/skills')
for link in sorted(glob.glob('.claude/skills/*')):
    if os.path.islink(link):
        target = os.path.realpath(link)
        if not os.path.isfile(os.path.join(target, 'SKILL.md')):
            err(link, "symlink does not resolve to a directory holding SKILL.md")
        elif os.path.commonpath([target, PLUGIN_SKILLS]) != PLUGIN_SKILLS:
            err(link, f"symlink resolves outside plugin/skills/ ({target}); "
                      f"a .claude/skills link must point at a plugin skill")

# ---- 3. plugin and marketplace manifests are valid JSON --------------------
import json
for p in ('plugin/.claude-plugin/plugin.json', '.claude-plugin/marketplace.json'):
    try:
        with open(p) as f:
            json.load(f)
    except OSError:
        err(p, "missing")
    except ValueError as e:
        err(p, f"does not parse as JSON ({e})")

# ---- 4. vendored copies match their sources byte for byte ------------------
VENDORED = (
    ('plugin/skills/changelog/references/changelog.strata', 'schemas/changelog.strata'),
    ('plugin/skills/changelog/references/changelog-contract.md', 'specs/contracts/changelog-contract.md'),
)
for copy, source in VENDORED:
    try:
        with open(copy, 'rb') as f: a = f.read()
    except OSError:
        a = None
    try:
        with open(source, 'rb') as f: b = f.read()
    except OSError:
        err(source, "vendored source is missing")
        continue
    if a is None:
        err(copy, f"missing; vendored copy of {source} — refresh with: cp -f {source} {copy}")
    elif a != b:
        err(copy, f"differs from {source} — refresh with: cp -f {source} {copy}")

print("=== check-skills ===")
for e in errors:   print(f"  ERROR {e}")
for w in warnings: print(f"  WARN  {w}")
for n in notes: print(f"  NOTE  {n}")
print(f"--- {len(errors)} error(s), {len(warnings)} warning(s) ---")
strict = os.environ.get('STRICT') == '1'
if errors or (strict and warnings):
    print("RESULT: FAIL"); sys.exit(1)
print("RESULT: PASS" + (" (warnings present)" if warnings else ""))
PY
