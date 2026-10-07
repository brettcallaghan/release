# Agent Instructions

This repository holds the release and branching specification corpus under
`specs/` and the Go `release` CLI that implements it. Work is tracked with
**bd** (beads); run `bd prime` for full workflow context.

```bash
bd ready                # find available work
bd show <id>            # view issue details
bd update <id> --claim  # claim work atomically
bd close <id>           # complete work
```

## Layout

| Path | Holds |
|---|---|
| `specs/` | The corpus: FRDs, container and module blueprints, contracts, ADRs, standards. Checked by `specs check --strict` |
| `schemas/` | Strata schemas for the changelog corpus and collection |
| `cmd/release/` | The entry point, and the end-to-end tests that run the built binary |
| `internal/cli/` | The command surface: command tree, exit statuses, output |
| `internal/` (other) | The release core module's components, one package each, as they land |
| `testdata/cases/` | Snapshot cases: `case.toml` + optional `input/` + `expected/` |
| `testdata/help/` | Golden `--help` output, one file per command; normative per the contract |

The container blueprint puts all release logic in the core module and none in
`internal/cli`. ADR-005 keeps the core's components pure functions of their
inputs, with git and hosting-platform I/O in separate components.

## Non-Interactive Shell Commands

**ALWAYS use non-interactive flags** with file operations to avoid hanging on confirmation prompts.

Shell commands like `cp`, `mv`, and `rm` may be aliased to include `-i` (interactive) mode on some systems, causing the agent to hang indefinitely waiting for y/n input.

**Use these forms instead:**
```bash
# Force overwrite without prompting
cp -f source dest           # NOT: cp source dest
mv -f source dest           # NOT: mv source dest
rm -f file                  # NOT: rm file

# For recursive operations
rm -rf directory            # NOT: rm -r directory
cp -rf source dest          # NOT: cp -r source dest
```

**Other commands that may prompt:**
- `scp` - use `-o BatchMode=yes` for non-interactive
- `ssh` - use `-o BatchMode=yes` to fail instead of prompting
- `apt-get` - use `-y` flag
- `brew` - use `HOMEBREW_NO_AUTO_UPDATE=1` env var

## Pipes hide exit statuses

**The symptom is a command that succeeded reporting failure, or one that failed
reporting success.** Both come from putting a pipe between a command and the
status you read.

**Never read `$?` through a pipe.** `cmd | tail` gives you `tail`'s status, so a
failing command reads as a pass. Redirect to a file and read the status
directly:

```bash
cmd >out 2>&1; echo $?          # right
cmd 2>&1 | tail -5; echo $?     # WRONG — that is tail's status
```

**Under `set -o pipefail`, a reader that closes the pipe early inverts it.**
`grep -q`, `head` and `tail -n` exit as soon as they have what they need; the
writer then dies on SIGPIPE, and `pipefail` reports the whole pipeline as
failed. Measured: `bd show <id> | grep -qi closed` reported a CLOSED bead
as not closed.

```bash
if bd show "$id" | grep -qi closed; then      # WRONG under pipefail
out="$(bd show "$id" --json)"                 # right — capture, then test
```

The fix is the same in both directions: **put the value in a variable or a
file, then test it.** A pipeline is for moving data, not for carrying a verdict.

## Tool claims are measured at the point of use

A claim about what a command does binds only where somebody ran it. Before you
act on one, or restate it in a bead, a commit or a document, run the command
and carry its invocation and output with the claim. Where you cannot run it,
mark it **unverified** and say what would settle it. This binds hardest when
correcting someone else's claim: the replacement is a new claim.

## Historical prose: don't

Every skill file, reference, config comment and script header is loaded into an
agent's context window. Tokens spent narrating how a rule came to exist are
tokens not spent on the rule. Write the current state; leave the story out.

**The test:** would a reader who has never seen this project's history behave
differently for having read this sentence? If no, delete it.

**State the fact and its failure mode. Drop the account of how it was learned.**

- Yes: "`worktree.baseRef` must be `head`. Unset, a worktree branches from the
  default branch instead of session HEAD, and the bead loses every commit the
  sprint branch carries."
- No: "This was misdiagnosed for a full session as X. The proof that settled it
  was Y. It was then repaired at the source and pushed, and the hook re-checks
  it each session as a regression guard."

Both convey the rule. The second spends 4x the tokens on a story that changes
nothing a reader does.

**Where the history goes instead.** It is not lost, it moves to where it is
retrievable but not re-read every session:

| Content | Home |
|---|---|
| Why a rule exists; what was tried and rejected | the bead, and the commit message |
| A measurement that justified a decision | the bead |
| What a change retired, and why | the commit message |
| The rule itself, and what breaks without it | the skill file |

A commit message is the right place for a retirement narrative: it is attached
to the change, `git log` finds it, and nothing loads it into context.

**Three narrow exceptions** — keep it short even then:

1. **A rule that looks removable but is not.** One sentence saying why the
   obvious simplification is wrong, so it is not "fixed" every six months.
2. **A trap with a plausible wrong answer.** Where an agent would otherwise
   guess confidently and wrongly, name the wrong answer.
3. **A non-obvious measured constraint.** State it as a current fact, not as a
   discovery narrative — "`bd` resolves the shared database from any worktree
   through git's common directory", not "we measured that…".

**Do not cross-reference issue ids in skill files.** Skills are exported to
other projects, where `<this-project>-abc.12` resolves to nothing. A reader
there sees a dangling reference and cannot look it up. Cross-reference the bead
in the COMMIT that makes the change; keep the skill file self-contained.
This applies to `plugin/skills/**` and anything vendored from it. Files that
never leave this repo — `CLAUDE.md`, `AGENTS.md`, `docs/`, `.beads/config.yaml`
— may cite ids, and should still keep the narrative short.

## Checks

Two lists, and they are not the same list:

| | |
|---|---|
| `scripts/ci.sh` | everything a contributor should see green before pushing |
| `swarm.toml`'s `[[checks]]` | the quality bar for ONE COMMIT — what a bead runs before its own close |

Anything too slow to pay per bead belongs in `ci.sh` and not in
`swarm.toml`. Neither is the release bar: the three-platform matrix, the
cross-compile and the busybox smoke test live in `.github/workflows/cli.yml`.

- **`format` is a file list, not an exit status.** `gofmt -l` exits 0 whether
  or not it found anything, which is why the check is `scripts/format-check`.
- **Snapshots are regenerated, never hand-edited.**
  `scripts/regenerate-snapshots.sh` rewrites `testdata/cases/*/expected/` and
  `testdata/help/` from the implementation. Review the diff: a changed
  snapshot is a changed command surface, and `testdata/help/` is normative
  under the contract.
- **Coverage is committed.** `scripts/coverage.sh` writes the figures into
  `README.md`, and CI's `coverage` job fails when they are stale. A change
  to Go code is not finished until the script has run and the README diff is
  committed with it. Use the Go version in `go.mod`; another toolchain moves
  the statement counts.
- **Every snapshot case cites what it verifies** (an AC id, or a
  `slug#anchor` for a contract section) in
  `cites`, so a criterion with no case is findable.

## How beads persist in THIS project

**The `beads/data` branch is the source of truth.** Issues live in a Dolt
database synced to the ordinary branch `beads/data` on this repository's git
remote, with the `release-` prefix. Dolt also keeps its bookkeeping branch
`__dolt_remote_info__` there. `.beads/*.jsonl` is **not tracked** and is not
how issues travel.

### What this requires of you

- **Outside a cloud session, run `./scripts/beads-setup.sh` once after
  cloning.** It needs `bd`, `git` and `python3`, and no token. In a cloud
  session the SessionStart hook runs it for you.
- **Otherwise nothing.** A `Stop` hook (`.claude/hooks/dolt-push.sh`) pushes
  at the end of every turn. Beads you create or close are durable without
  ceremony. Pushing by hand (`bd dolt push`) is never wrong, just redundant.
- **Never `bd bootstrap` or `bd init` by hand.** Both read Dolt's default ref,
  `refs/dolt/data`, which does not exist here, and `bd` has no config key for
  any other ref. The failure is silent: measured on bd 1.3.1 against a remote
  holding only `beads/data`, `bd bootstrap --yes` exits 0, prints "Created
  fresh database", and `bd stats` reads 0 issues. `scripts/beads-setup.sh` is the only supported way to
  hydrate a clone. Refresh a broken one with
  `rm -rf .beads/embeddeddolt && ./scripts/beads-setup.sh`, after
  `bd dolt push` if it holds beads not yet pushed.
- **Never `bd export` as a persistence step.** It writes an untracked file and
  persists nothing. Export only for a viewer or for interchange.
- **`bd remember` memories are excluded from `bd export`.** Durable context
  belongs in an issue description, not a memory.

### Write access to the tracker is write access to the repository

A Claude Code cloud session's GitHub credential is accepted on `refs/heads/*`
and refused on `refs/dolt/*`, which is why the data lives on a branch. Anyone
who can push a branch here can push the tracker; anyone who can clone can read
it. No PAT, proxy bypass, Dolt credential or extra network host is involved.

- **Never put a force-push block on `beads/**` or `__dolt_remote_info__`.**
  Dolt syncs with `--force-with-lease`; a rule forbidding force pushes breaks
  every `bd dolt push`. Restricting who may push, and forbidding deletion, is
  fine.
- **Never merge, rebase or open a pull request from `beads/data`.** Its
  commits are Dolt's storage, not source files.
- **`refs/dolt/data` must stay absent.** A clone that reached it would get a
  tracker that is populated, plausible and wrong. `dolt-push.sh` refuses to
  push unless the Dolt remote targets `refs/heads/beads/data`.
- **swarm's pre-flight probe pushes to `refs/dolt/*` and will report the
  tracker as not durable.** That is a false positive here. Verify the real
  path with `bd dolt push` (exit 0, "Push complete.") and proceed.

The hook exports `BD_NO_REMOTE_ADOPT=1` so `bd` never adds a Dolt remote
without the branch ref, and `.beads/config.yaml` keeps `dolt.auto-push`,
`import.auto` and `backup.enabled` off.

A skill that needs a durable tracker never names a credential — it requires
only "write access to the tracker's Dolt remote", and the mechanism is this
repository's business.

### Two hazards that remain

- **The id prefix travels with the database.** A bootstrapped clone mints ids
  from the database's configured `issue_prefix`, not from `.beads/config.yaml`.
  The hook warns when it is not `release`. Do not repair it with
  `bd rename-prefix --repair`, which rewrites ids to random hashes when it
  misdetects the prefix.
- **The hook is not always registered.** Claude Code loads hooks from the
  project root only, so a session attaching this repo alongside another never
  runs it — and then nothing hydrates the database. Run
  `./scripts/beads-setup.sh` by hand. The manual fallback is at the top of `CLAUDE.md`; the durable fix
  is the per-environment setup script in `docs/session-bootstrap.md`.
