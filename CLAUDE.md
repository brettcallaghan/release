# Project Instructions for AI Agents

@AGENTS.md

<!--
Claude Code reads CLAUDE.md and not AGENTS.md, so AGENTS.md is imported above
and the two cannot drift. Add only Claude-specific instructions below.
-->

## Session bootstrap — check this first (Claude Code)

**If `bd` is missing from PATH, or `bd ready` reports no issues, the
SessionStart hook did not run. Run it by hand before doing anything else:**

```bash
CLAUDE_PROJECT_DIR="$(git rev-parse --show-toplevel)" \
  bash "$(git rev-parse --show-toplevel)/.claude/hooks/session-start.sh"
```

It is idempotent and takes a few minutes cold, because it builds `bd` from
source. Claude Code loads hooks only from the project root, so a session that
attaches this repository alongside another never runs it; CLAUDE.md is still
read from a nested repository, which is why this instruction works where the
hook does not. `docs/session-bootstrap.md` has the durable fix.
