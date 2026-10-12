# Session bootstrap

How this repository's tools get installed in a Claude Code cloud session.
This is an operations note, not a specification.

## What the hook does

`.claude/settings.json` registers two hooks:

- **SessionStart** runs `.claude/hooks/session-start.sh`, which installs `bd`,
  hydrates the tracker from the `beads/data` branch with
  `scripts/beads-setup.sh`, builds the `release` CLI onto PATH, and builds the
  `specs` CLI when a sibling `specs` checkout is attached. It then
  runs `bd prime --hook-json` for the tracker's workflow context.
- **Stop** runs `.claude/hooks/dolt-push.sh`, which pushes the tracker to
  `beads/data` at the end of every turn.

The tracker needs no token, proxy bypass or extra network host: a cloud
session's GitHub credential can push `refs/heads/*`, and the data lives on a
branch. See `AGENTS.md`, "How beads persist in THIS project".

## When the hook does not run

Claude Code loads hooks from the **project root** only. A session that
attaches this repository alongside another (for example `specs`,
whose `specs` CLI gates this corpus) clones both as siblings under a shared
parent with no `.claude/`, so neither repository's hook runs. `CLAUDE.md`
tells the agent to run the hook by hand.

## The durable fix: an environment setup script

A [setup script](https://code.claude.com/docs/en/cloud-environments#setup-scripts)
runs before Claude Code launches, independent of the project root, and is
cached with the environment. Configure it at claude.ai/code: open the
environment selector, hover the environment, use the settings icon, and paste
into **Setup script**. It installs only `bd`; the hook still does the
per-repository work that needs a clone.

```bash
#!/bin/bash
# Provision bd for sessions in this environment. Best-effort throughout:
# a non-zero exit fails the whole session.
export HOME="${HOME:-/root}"
export DEBIAN_FRONTEND=noninteractive

# bd embeds Dolt, whose regex support needs the ICU headers to build.
apt-get update -qq >/dev/null 2>&1 || true
apt-get install -y -qq libicu-dev >/dev/null 2>&1 || true

go install github.com/steveyegge/beads/cmd/bd@latest >/dev/null 2>&1 || true
[ -x "$(go env GOPATH)/bin/bd" ] \
  && install -m 0755 "$(go env GOPATH)/bin/bd" /usr/local/bin/bd 2>/dev/null || true

# Without worktree.baseRef=head a swarm worktree branches from the default
# branch instead of session HEAD, and a bead loses its sprint's commits.
mkdir -p "$HOME/.claude"
if [ ! -s "$HOME/.claude/settings.json" ]; then
  printf '{"worktree":{"baseRef":"head"}}\n' > "$HOME/.claude/settings.json"
elif command -v python3 >/dev/null 2>&1; then
  python3 -c 'import json,os; p=os.path.expanduser("~/.claude/settings.json"); d=json.load(open(p)); d.setdefault("worktree",{})["baseRef"]="head"; json.dump(d,open(p,"w"),indent=2)' 2>/dev/null || true
fi
exit 0
```

It duplicates the hook's `bd` install, so the two can drift; keep the script
to `bd` and the hook to work that needs the clone.
