---
title: "Contract: Release CLI Command Surface"
description: The commands, arguments, flags, output and exit codes the release
  command-line tool vends to maintainers and to the scripts that drive it.
slug: contract-release-cli
type: contract
status: draft
updated_at: 2026-10-09T00:00:00Z
provider: container-release-cli
visibility: public
specs:
  - testdata/help/
version: "2.0"
---

# Release CLI Command Surface

The command surface of the release tool: what a maintainer types and what a
script can depend on. Provided by [[container-release-cli]]. Visibility:
public.

Public because the surface is consumed outside the repository that defines it.
A maintainer types these commands; a continuous-delivery workflow invokes them;
a wrapper script parses their output. Renaming a flag or changing an exit code
breaks those consumers as surely as changing a library signature breaks a
caller, so the surface is versioned and this document is what it is versioned
against.

## Specification

| Artifact | Location | Normative for |
|----------|----------|---------------|
| Golden help output | `testdata/help/` | Shape — one file per command, asserted by test |
| This document | — | Semantics, exit codes, change policy |

The golden files are generated from the implementation and committed, so a
change to the surface shows up as a diff in review. They are normative for
*shape* in the same way a schema is: where they and this document disagree,
one of them is wrong and the disagreement is a defect to resolve rather than a
precedence rule to apply.

While this document is a draft, it is ahead of the golden files: they are
regenerated from the implementation that realizes it, and until then hold no
file for `release notes` and the earlier wording of the commands it changes.

## Commands

### `release version [bump]`

Resolves and prints the version a release would carry, without writing
anything.

| Argument | Values | Default | Meaning |
|---|---|---|---|
| `bump` | `major`, `minor`, `patch` | inferred | Names the impact instead of inferring it |

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to compute for. Defaults to the configured target, then the repository's default branch |
| `--json` | Render the answer as JSON instead of YAML |

Answers one mapping with these keys, in this order:

| Key | Value |
|---|---|
| `base` | The base version, or null when no release tag is reachable |
| `impact` | The impact the version uses: the named bump, or the impact computed from the commits' reviewed signals |
| `named` | Whether a bump was named |
| `determined_by` | The commits that determined the impact, each `hash`, `subject` and `signal` |
| `unclassified` | The commits that carry no enabled signal, each `hash` and `subject`; `[]` when every commit carries one |
| `next` | The next version |

Read-only, and safe to run against any repository at any time. It invokes no
producer and reconciles nothing, so `impact` is what the reviewed signals
indicate and never a proposal.

A non-empty `unclassified` changes neither the answer's other keys nor the
exit status. It tells a caller that `release changelog` and `release cut`
will ask for the impact to be confirmed over the same range, unless a bump
is named or the computed impact is major.

WHEN a bump is named and the commits since the base version indicate no
impact, it answers the named version and prints a warning on standard error
that nothing is releasable.

| Exit | When |
|---|---|
| 0 | A version was resolved, including a named bump over a range indicating no impact |
| 1 | No release is warranted, or the policy refuses the version; the answer is printed first |
| 2 | An invalid bump, a repository it cannot read, or no target it can resolve |

### `release notes`

Authors the `## Release notes` block for the current branch's pull request,
or checks the block the pull request already has. The block's grammar is
[[contract-changelog#release-notes-block]].

| Flag | Meaning |
|---|---|
| `--base <branch>` | The branch the pull request merges into. Defaults to the configured target, then the repository's default branch |
| `--check` | Check the pull request's block instead of authoring one. Invokes no producer |
| `--write-pr` | Also write the block into the pull request's description |
| `--redo-notes` | Discard any cached notes and author again |
| `--json` | Render the answer as JSON instead of YAML |

The range runs from the merge base of `origin/<base>` and the current
branch's head to that head. Authoring collects the range's commits and their
diffs within `changelog.diff_budget.pull_request`, exchanges the evidence
bundle with the configured producer, and answers one mapping with these
keys, in this order:

| Key | Value |
|---|---|
| `head` | The commit hash the block was authored from |
| `base` | The base branch |
| `notes_path` | Where the authored notes are cached |
| `block` | The rendered `## Release notes` block, ready to paste |
| `pull_request` | The pull request's URL. Present only under `--write-pr` |

WHEN the current branch's pull request already holds a current block and
`--redo-notes` is not given, the provider SHALL answer that block and invoke
no producer. WHEN the branch has no pull request, or the hosting platform's
CLI cannot read it, authoring proceeds without one.

WHERE `--write-pr` is given, the provider SHALL write the block into the
pull request's description, replacing any block it holds and changing no
other part of it. IF the platform's CLI is missing or not authenticated, the
branch has no pull request, or the pull request's head is not the head the
block was authored from, THEN the provider SHALL refuse and write nothing.

WHERE `--check` is given, the provider SHALL read the pull request's
description and answer one mapping with these keys, in this order:

| Key | Value |
|---|---|
| `pull_request` | The pull request's URL |
| `head` | The pull request's current head |
| `status` | `current`, `missing`, `malformed` or `stale` |
| `problem` | Why the block is not current, naming the line of a malformed block; null when current |

A block is `stale` WHEN the pull request's diff has changed since the head
its marker records. A project's required checks run `release notes --check`
([[repository-release-policy]]); it needs no producer and no model
credential.

| Exit | When |
|---|---|
| 0 | A block was authored, answered or written; under `--check`, the block is `current` |
| 1 | Under `--check`, the block is `missing`, `malformed` or `stale`, or the pull request cannot be read; under `--write-pr`, a refusal above; the producer fails or its notes are rejected |
| 2 | `--check` with `--write-pr` or `--redo-notes`; the `mechanical` producer, which serves releases only; a missing producer prerequisite; no base it can resolve |

### `release changelog [bump]`

Builds the release evidence bundle, authors the changelog from it,
reconciles the impact, renders the section, and answers with it.

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to collect from |
| `--skip-notes` | Author with the mechanical producer instead of the configured one, and read no cache |
| `--redo-notes` | Discard any cached corpus and author again |
| `--corpus <path>` | Read the corpus at this path WHEN it exists, and write the authored corpus there otherwise, instead of the cache |
| `--write` | Write the rendered section into the changelog file |
| `--json` | Render the answer as JSON instead of YAML |

Read-only unless `--write` is given. It never prompts.

The evidence bundle holds the range's commits, their notes blocks, and the
diffs of the commits without one, within `changelog.diff_budget.release`.
WHEN the impact is confirmed, it answers one mapping with these keys, in
this order:

| Key | Value |
|---|---|
| `version` | The version the release carries |
| `corpus_path` | Where the authored corpus is kept, relative to the repository root when inside it; null under `--skip-notes` |
| `section` | The rendered changelog section |

A named bump governs, and confirms an impact the reconciliation would
otherwise halt on. WHEN no bump is named and the impact needs a person's
confirmation ([[frd-version-computation]]), the provider SHALL write
nothing, explain on standard error which bump to name to confirm the
impact, and answer one mapping with these keys, in this order:

| Key | Value |
|---|---|
| `computed` | The impact computed from the reviewed signals |
| `proposed` | The producer's proposed impact, or null |
| `rationale` | The producer's reasons for its proposal, or null |
| `drivers` | The sources the proposal rests on, each a change source as [[contract-changelog]] defines it |
| `unclassified` | The commits that carry no enabled signal, each `hash` and `subject` |
| `corpus_path` | As above |

WHERE `--skip-notes` is given, the proposal restates the computed impact, so
a halt there comes only from unclassified commits, and standard error says
that no producer read their diffs.

| Exit | When |
|---|---|
| 0 | The section was rendered, and written under `--write` |
| 1 | No release is warranted; the policy refuses the version; the impact needs confirmation; the producer fails or its corpus is rejected |
| 2 | `--skip-notes` with `--redo-notes` or `--corpus`; an invalid bump; a missing producer prerequisite |

### `release cut [bump]`

Cuts a release: resolves the version, authors and renders the changelog,
confirms the impact, writes the release commit onto a stage branch, and
opens its pull request.

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to cut from |
| `--skip-notes` | Author with the mechanical producer in place of an authoring exchange |
| `--force` | Stage the named bump even when the range holds nothing releasable. Requires `bump` |
| `--redo-notes` | Discard any cached corpus and author again |
| `--no-pull-request` | Create the stage branch and stop; open no pull request |
| `--dry-run` | Report every intended action and the changelog section, and change nothing |
| `--yes` | Do not prompt. Only meaningful interactively |
| `--json` | Render the answer as JSON instead of YAML |

The only command that writes a release to the repository or the hosting
platform. The release commit carries the confirmed impact as a
`Release-Type:` trailer, and the stage pull request shows the confirmed
impact, the proposal, its rationale and drivers, and the unclassified
commits ([[frd-release-staging]]).

It answers one mapping with keys `version`, `branch`, `commit`,
`pull_request` (the URL, or null) and `writes` (every write made, in order,
each a line such as `push cut/1.9.0 to origin`), in that order. Under
`--dry-run` the keys are `version`, `branch`, `writes` (the writes it would
make) and `section` (the changelog section the release commit would write).

**Confirming the impact.** WHEN no bump is named and the impact needs a
person's confirmation:

- WHILE standard input is a terminal and `--yes` is not given, the provider
  SHALL show on standard error the computed impact with the commits that
  drove it, the proposal with its rationale and drivers, and the
  unclassified commits, and SHALL prompt for the impact, offering the
  proposal. The answer is a named bump. An empty or unrecognised answer
  declines, which is a refusal.
- Otherwise the provider SHALL refuse, write nothing, answer nothing, and
  show the same on standard error, naming `release cut <bump>`, which
  confirms it.

WHERE `--skip-notes` is given, the proposal restates the computed impact, so
the provider asks for confirmation only over unclassified commits, and says
that no producer read their diffs.

**Releasability and `--force`.** A range holds something releasable WHEN a
reviewed signal in it indicates patch or above, or the authored corpus holds
a user-visible change. A named bump over a range with nothing releasable is
a refusal suggesting `--force`, which stages it anyway. `--force` without a
bump is a refusal.

| Exit | When |
|---|---|
| 0 | The release was staged, or under `--dry-run` reported |
| 1 | A refusal, having written nothing and answering nothing: nothing to release, an unconfirmed impact, a declined prompt, `--force` without a bump, an existing stage branch, an unconfigured `commands.propagate`, a platform CLI missing or not authenticated when a pull request is to be opened, a failing configured command, a path outside `release_paths`, a failing or rejected producer |
| 2 | `--skip-notes` with `--redo-notes`; a bump other than `major`, `minor` or `patch`; a missing producer prerequisite |
| 3 | A write failed after another succeeded; the answer lists every write made, and standard error names the commands that finish or discard the release |

### `release config`

Answers with the effective configuration — declared values over the
defaults — and validates it.

| Flag | Meaning |
|---|---|
| `--json` | Render the answer as JSON instead of YAML |

Answers one mapping with keys `target`, `tag_prefix`, `stage_prefix`,
`release_branches`, `release_paths`, `leaps`, `pre1_remap`, `signals`,
`commands` (holding `validate`, `propagate` and `preflight`), `changelog`
(holding `groups`, `other_heading`, `breaking_heading`, `producer`,
`producer_command`, `diff_budget` with `release` and `pull_request`, and
`diff_exclude`) and `adapters` (holding `pull_requests` and `diffs`), in
that order. An unset command is null and an empty list `[]`. The keys, their
defaults and what is refused are [[frd-tool-configuration]]'s.

### Global flags

| Flag | Meaning |
|---|---|
| `-C, --repo <path>` | Operate on the repository at this path instead of the working directory |
| `--config <path>` | Use this configuration file instead of discovering one |
| `-q, --quiet` | Suppress informational output on standard error; errors still print |
| `-v, --verbose` | Print additional diagnostics to standard error |
| `--no-color` | Disable styled diagnostics on standard error |
| `-h, --help` | Print usage for the program or a command |
| `-V, --version` | Print the tool's own version |

## Guarantees

WHEN a command succeeds, the provider SHALL exit 0.

---

WHEN a command refuses a stated, expected condition, the provider SHALL exit
1.

**Rationale:** Nothing to release, an impact awaiting confirmation, a stale
notes block, and a policy violation are outcomes rather than faults. A script distinguishing "no release
was warranted" from "the tool broke" needs them separated from both success
and failure.

---

IF a command is invoked with invalid arguments or an unacceptable
configuration, THEN the provider SHALL exit 2.

---

IF a command fails after making a change to the repository, THEN the provider
SHALL exit 3 and report every change it made.

---

The provider SHALL print a command's answer only to standard output.

---

The provider SHALL render every answer as [[cli-output-standards]] specifies:
YAML by default, and JSON carrying the same values in the same order WHERE
`--json` is given.

**Rationale:** One rendering a person and an agent can both read without a
key, and one a consumer already parsing JSON can take unchanged. A caller
never has to choose between readable output and parseable output.

---

The provider SHALL print diagnostics only to standard error.

**Rationale:** `release version --json | jq` must work without the caller
filtering progress messages out of the stream.

---

WHERE `--json` is given, the provider SHALL emit exactly one JSON object and
no other content on standard output.

---

The provider SHALL print usage for every command and every flag in response to
`--help`.

---

The provider SHALL NOT write to the repository or the hosting platform in any
command other than `release cut`, `release changelog --write`, or
`release notes --write-pr`.

---

The provider SHALL NOT invoke a producer in `release version`,
`release config`, or `release notes --check`.

**Rationale:** These are the commands continuous integration and scripts run,
where no model or model credential is available.

---

The provider SHALL NOT derive a version from a producer's proposal that no
named bump, prompt answer, or reviewed signal in git confirms.

**Rationale:** A proposal varies between runs. The confirmation is what makes
the version reproducible, and `release cut` records it on the release commit
as a `Release-Type:` trailer ([[adr-003]]).

---

WHERE `--dry-run` is given, the provider SHALL make no change of any kind,
except that `release cut` refreshes the remote-tracking ref
`origin/<target>` (and the tags that follow it) by fetching the target.

**Rationale:** A cut, dry or not, must describe origin's tip rather than a
stale local copy. A remote-tracking ref only mirrors origin, and any fetch
moves it; fetching without it would also lose the tags version resolution
reads.

---

WHILE standard input is a terminal and `--yes` is not given, the provider
SHALL prompt before the first write performed by `release cut`, and SHALL
prompt for the impact WHEN it needs a person's confirmation.

---

The provider SHALL NOT prompt WHILE standard input is not a terminal.

**Rationale:** A command that blocks on a prompt in continuous delivery hangs
until the job times out, which is the least diagnosable failure available to
it.

---

The provider SHALL report, for every version it computes, what determined the
impact.

**Rationale:** A version a maintainer disagrees with is otherwise
undiagnosable, and the usual cause is one mis-marked change.

## Operational Expectations

- **Availability / SLO:** not applicable — the tool is invoked and exits.
- **Latency:** `release version` and `release config` SHALL complete without
  network access. `release notes --check` waits on the hosting platform only.
  `release notes`, `release changelog` and `release cut` may wait on a
  producer and on the hosting platform.
- **AuthN/AuthZ:** commands touching the hosting platform SHALL use the
  credentials of the platform's own CLI rather than requiring a token of their
  own.
- **Consumer obligations:** consumers SHALL branch on exit codes rather than
  on message text, and SHALL parse an answer rather than diagnostics. Either
  rendering may be parsed; both carry the same values.
- **Provider obligations:** the provider SHALL treat message text as
  unversioned and exit codes as versioned.

## Versioning and Change Policy

- **Versioning scheme:** semantic versioning on the tool's own version.
- **Compatible changes:** the provider MAY add a command, add a flag, add a
  property to an answer, and change message text, without a major version
  change.
- **Breaking changes:** WHEN a command or flag is removed or renamed, a
  default changes, an exit code's meaning changes, or a property of an answer
  is removed, renamed or re-typed in either rendering, the provider SHALL increment its major version.
- **Deprecation:** WHEN a command or flag is deprecated, it SHALL continue to
  work for one major version, and invoking it SHALL print a notice to standard
  error naming its replacement.

## Key Decisions

- [[adr-003]] — the version comes from reviewed records in git, and a person
  confirms any impact a producer proposed that nobody reviewed, which is why
  `release changelog` and `release cut` halt and `release cut` prompts.
- [[adr-005]] — the command surface is a thin front end over
  [[module-release-core]], which is why no command carries release logic of
  its own.

## Open Questions

None. `release cut` prompts before its first write when standard input is a
terminal, and `--yes` suppresses the prompt. Non-interactive invocations never
prompt, so no automation needs the flag.

## References

- `shared/c4-standards.md` — C4 mapping rules and contract visibility tiers
- [[container-release-cli]] — provider's blueprint
- [[cli-output-standards]] — the shape of every answer on standard output
- [[module-release-core]] — the library every command calls
- [[frd-tool-configuration]] — the configuration `release config` reports
- [[contract-changelog]] — the evidence bundle, the corpus, and the notes
  block grammar
- [[repository-release-policy]] — the required check that runs
  `release notes --check`
- [[frd-version-computation]], [[frd-changelog-authoring]],
  [[frd-changelog-rendering]], [[frd-release-staging]] — the behaviour behind
  each command
