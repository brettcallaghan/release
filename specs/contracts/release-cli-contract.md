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
regenerated from the implementation that realizes it, and until then hold the
earlier wording of the commands it changes.

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
| `impact` | The impact the version uses: the named bump, or the signal floor |
| `named` | Whether a bump was named |
| `determined_by` | The commits that set the signal floor, each `hash`, `subject` and `signal` |
| `unclassified` | The commits that carry no enabled signal, each `hash` and `subject`; `[]` when every commit carries one |
| `next` | The next version |

The **signal floor** is the highest impact the commits' explicit records in
git indicate: `Release-Type:` trailers, conventional types and breaking
markers ([[frd-version-computation]]). Read-only, and safe to run against
any repository at any time. It invokes no producer, so `impact` is never a
suggestion: `release cut` may suggest a higher bump once a producer has
classified the unclassified commits.

A non-empty `unclassified` changes neither the answer's other keys nor the
exit status. It tells a caller that the suggested bump `release cut` reaches
over the same range may be higher than `impact`.

WHEN a bump is named and the commits since the base version indicate no
impact, it answers the named version and prints a warning on standard error
that nothing is releasable. IF a named bump is below the signal floor, THEN
it refuses, naming the floor and the commits that set it.

| Exit | When |
|---|---|
| 0 | A version was resolved, including a named bump over a range indicating no impact |
| 1 | No release is warranted, a named bump is below the signal floor, or the policy refuses the version; the answer is printed first except for the named bump below the floor |
| 2 | An invalid bump, a repository it cannot read, or no target it can resolve |

### `release changelog [bump]`

Builds the release evidence bundle, authors the structured changelog from
it, suggests the bump, renders the section, and answers with it.

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to collect from |
| `--skip-notes` | Author with the mechanical producer instead of the configured one, and read no cache |
| `--redo-notes` | Discard any cached corpus and author again |
| `--corpus <path>` | Read the corpus at this path WHEN it exists, and write the authored corpus there otherwise, instead of the cache |
| `--force` | Accept a named bump below the signal floor. Requires `bump` |
| `--write` | Write the rendered section into the changelog file. Requires `bump` |
| `--json` | Render the answer as JSON instead of YAML |

Read-only unless `--write` is given. It never prompts and confirms nothing:
without a named bump it renders the section under the suggested bump, as a
preview of what `release cut` would offer.

The evidence bundle holds every commit in the range, read from git, with its
changed files and its diff within `changelog.diff_budget.release`
([[contract-changelog#evidence-bundle]]). It answers one mapping with these
keys, in this order:

| Key | Value |
|---|---|
| `version` | The version the section is rendered under |
| `bump` | The bump that version uses: the named bump, or the suggested bump |
| `named` | Whether a bump was named |
| `suggested` | The suggested bump |
| `floor` | The signal floor |
| `reasons` | The entries that drove the suggestion, each `kind`, `breaking`, `scope`, `summary` and `sources` |
| `raised_by_floor` | Whether the signal floor raised the suggestion above the entries' kinds |
| `unclassified` | The commits that carry no enabled signal, each `hash` and `subject` |
| `corpus_path` | Where the authored corpus is kept, relative to the repository root when inside it; null under `--skip-notes` |
| `section` | The rendered changelog section |

The suggestion follows [[component-bump-suggester]]: a breaking entry
suggests major, else a feature entry minor, else a fix, performance,
refactor or revert entry patch, else none, raised to the signal floor WHEN the floor is
higher. A named bump below the signal floor is a refusal unless `--force` is
given. A named bump below the suggestion proceeds, and standard error warns
that it is below the suggestion and names the entries that drove it.

WHERE `--skip-notes` is given, the entries' kinds come from conventional
subjects only, so the suggestion is the signal floor, and standard error
says that no producer read the diffs.

WHEN the configured producer is unavailable or fails, the provider SHALL
report the failure on standard error and stop, naming `--skip-notes` as the
way to author with the mechanical producer. It SHALL NOT fall back to
another producer on its own. The same holds for `release cut`.

| Exit | When |
|---|---|
| 0 | The section was rendered, and written under `--write` |
| 1 | No release is warranted; the policy refuses the version; a named bump below the signal floor without `--force`; `--force` without a bump; the producer fails or its corpus is rejected |
| 2 | `--skip-notes` with `--redo-notes` or `--corpus`; `--write` without a bump; an invalid bump; a missing producer prerequisite |

### `release cut [bump]`

Cuts a release: builds the evidence, authors and renders the changelog,
confirms the bump, writes the release commit onto a stage branch, and opens
its pull request.

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to cut from |
| `--skip-notes` | Author with the mechanical producer in place of an authoring exchange |
| `--force` | Stage the named bump even when it is below the signal floor or the range holds nothing releasable. Requires `bump` |
| `--redo-notes` | Discard any cached corpus and author again |
| `--no-pull-request` | Create the stage branch and stop; open no pull request |
| `--dry-run` | Report every intended action and the changelog section, and change nothing |
| `--yes` | Do not prompt: accept the suggested bump when none is named, and proceed to the writes |
| `--json` | Render the answer as JSON instead of YAML |

The only command that writes a release to the repository or the hosting
platform. The release commit carries the confirmed bump as a
`Release-Type:` trailer, so the version is recomputable from git without a
producer. The stage pull request shows the changelog section, the suggested
bump and its reasons, and the confirmed bump ([[frd-release-staging]]);
reviewing it is where a person reviews the authored changelog before
anything is published.

It answers one mapping with keys `version`, `branch`, `commit`,
`pull_request` (the URL, or null) and `writes` (every write made, in order,
each a line such as `push cut/1.9.0 to origin`), in that order. Under
`--dry-run` the keys are `version`, `branch`, `writes` (the writes it would
make) and `section` (the changelog section the release commit would write).

**Confirming the bump.** A named bump is a person's decision and needs no
further confirmation: at or above the suggestion it proceeds; below the
suggestion it proceeds with a warning on standard error naming the entries
that drove the suggestion; below the signal floor it is a refusal unless
`--force` is given. WHEN no bump is named, the suggested bump needs a
person's confirmation:

- WHERE `--yes` is given, the provider SHALL take the suggestion as
  confirmed.
- WHILE standard input is a terminal and `--yes` is not given, the provider
  SHALL show on standard error the suggested bump, the entries that drove
  it, whether the signal floor raised it, and the unclassified commits, and
  SHALL prompt for the bump, offering the suggestion. An empty answer or
  `y` accepts the suggestion; `major`, `minor` or `patch` names a bump,
  which is then treated as a named bump; anything else declines, which is a
  refusal.
- Otherwise the provider SHALL halt: write nothing, show the same on
  standard error naming `release cut --yes` and `release cut <bump>`, either
  of which confirms, and answer one mapping with the keys `suggested`,
  `floor`, `reasons`, `raised_by_floor`, `unclassified` and `corpus_path`,
  as `release changelog` defines them, in that order.

WHEN no bump is named, the suggestion is none, and the authored changelog
holds an entry with no kind, the provider SHALL halt the same way, with or
without `--yes`, naming `release cut <bump>`: no producer classified those
commits, so there is no suggestion to accept. WHEN no bump is named and the
range holds nothing releasable, the provider SHALL refuse with nothing to
release.

**Releasability and `--force`.** A range holds something releasable WHEN
the suggested bump, the signal floor included, is patch or above, or the
authored changelog holds an entry with no kind. A named bump over a range
with nothing releasable is a refusal suggesting `--force`, which stages it
anyway; a named bump answering the halt for an entry with no kind needs no
`--force`. `--force` has two jobs on `cut`: it stages a named bump over a
range with nothing releasable, and it accepts a named bump below the signal
floor. `--force` without a bump is a refusal.

| Exit | When |
|---|---|
| 0 | The release was staged, or under `--dry-run` reported |
| 1 | A refusal, having written nothing: a halt for an unconfirmed bump, which answers as above; and, answering nothing, nothing to release, a named bump below the signal floor without `--force`, `--force` without a bump, a declined prompt, an existing stage branch, an unconfigured `commands.propagate`, a platform CLI missing or not authenticated when a pull request is to be opened, a failing configured command, a path outside `release_paths`, a failing or rejected producer |
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
`producer_command`, `diff_budget` with `release`, and `diff_exclude`) and
`adapters` (holding `pull_requests` and `diffs`), in that order. An unset command is null and an empty list `[]`. The keys, their
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

**Rationale:** Nothing to release, a bump awaiting confirmation, and a
policy violation are outcomes rather than faults. A script distinguishing "no release
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
command other than `release cut` or `release changelog --write`.

---

The provider SHALL NOT invoke a producer in `release version` or
`release config`.

**Rationale:** These are the commands continuous integration and scripts run,
where no model or model credential is available.

---

The provider SHALL NOT stage a version derived from a suggested bump that no
named bump, prompt answer or `--yes` confirms.

**Rationale:** A suggestion rests on a producer's output, which varies
between runs. The confirmation is what makes
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
prompt for the bump WHEN no bump is named.

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
  network access. `release changelog` and `release cut` may wait on a
  producer, and `release cut` on the hosting platform.
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

- [[adr-003]] — a producer authors the structured changelog from evidence
  read from git, the bump is suggested from its entries' kinds and floored
  by the signals in git, and a person confirms it, which is why
  `release cut` prompts or halts and records the confirmed bump on the
  release commit.
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
- [[contract-changelog]] — the evidence bundle and the corpus
- [[component-bump-suggester]] — how the suggested bump is derived
- [[frd-version-computation]], [[frd-changelog-authoring]],
  [[frd-changelog-rendering]], [[frd-release-staging]] — the behaviour behind
  each command
