---
title: "Contract: Release CLI Command Surface"
description: The commands, arguments, flags, output and exit codes the release
  command-line tool vends to maintainers and to the scripts that drive it.
slug: contract-release-cli
type: contract
status: approved
updated_at: 2026-10-07T00:00:00Z
provider: container-release-cli
visibility: public
consumers: []
specs:
  - testdata/help/
version: "1.0"
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

## Commands

### `release version [bump]`

Resolves and prints the version a release would carry, without writing
anything.

| Argument | Values | Default | Meaning |
|---|---|---|---|
| `bump` | `major`, `minor`, `patch` | inferred | Names the impact instead of inferring it |

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to compute for. Defaults to the configured target, then the current branch |
| `--json` | Emit the result as one JSON object |

Prints the base version, the resolved impact, what drove that impact, and the
next version. Read-only, and safe to run against any repository at any time.

### `release changelog [bump]`

Assembles a changelog corpus, authors it, renders it, and prints the result.

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to collect from |
| `--skip-notes` | Assemble mechanically and invoke no producer |
| `--redo-notes` | Discard any cached corpus and author again |
| `--corpus <path>` | Read or write the corpus at an explicit path instead of the cache |
| `--write` | Write the rendered section into the changelog file |
| `--json` | Emit the corpus and the rendered output as one JSON object |

Read-only unless `--write` is given.

### `release cut [bump]`

Cuts a release: resolves the version, authors and renders the changelog,
writes the release commit onto a stage branch, and opens its pull request.

| Flag | Meaning |
|---|---|
| `--target <branch>` | The release line to cut from |
| `--skip-notes` | Stage without an authoring exchange. Requires `bump` |
| `--redo-notes` | Discard any cached corpus and author again |
| `--no-pull-request` | Create the stage branch and stop; open no pull request |
| `--dry-run` | Report every intended action and change nothing |
| `--yes` | Do not prompt before writing. Only meaningful interactively |
| `--json` | Emit the outcome as one JSON object |

The only command that writes to the repository or the hosting platform.

### `release config`

Prints the effective configuration — declared values and defaults — and
validates it.

| Flag | Meaning |
|---|---|
| `--json` | Emit the effective configuration as one JSON object |

### Global flags

| Flag | Meaning |
|---|---|
| `-C, --repo <path>` | Operate on the repository at this path instead of the working directory |
| `--config <path>` | Use this configuration file instead of discovering one |
| `-q, --quiet` | Suppress informational output; errors still print |
| `-v, --verbose` | Print additional diagnostics to standard error |
| `--no-color` | Disable styled output |
| `-h, --help` | Print usage for the program or a command |
| `-V, --version` | Print the tool's own version |

## Guarantees

WHEN a command succeeds, the provider SHALL exit 0.

---

WHEN a command refuses a stated, expected condition, the provider SHALL exit
1.

**Rationale:** Nothing to release, an impact disagreement, and a policy
violation are outcomes rather than faults. A script distinguishing "no release
was warranted" from "the tool broke" needs them separated from both success
and failure.

---

IF a command is invoked with invalid arguments or an unacceptable
configuration, THEN the provider SHALL exit 2.

---

IF a command fails after making a change to the repository, THEN the provider
SHALL exit 3 and report every change it made.

---

The provider SHALL print machine-readable output only to standard output.

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
command other than `release cut`, or `release changelog --write`.

---

WHERE `--dry-run` is given, the provider SHALL make no change of any kind.

---

WHILE standard input is a terminal, the provider SHALL prompt before the first
write performed by `release cut`.

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
  network access. `release changelog` and `release cut` may wait on a producer
  and on the hosting platform.
- **AuthN/AuthZ:** commands touching the hosting platform SHALL use the
  credentials of the platform's own CLI rather than requiring a token of their
  own.
- **Consumer obligations:** consumers SHALL branch on exit codes rather than
  on message text, and SHALL pass `--json` where they parse output.
- **Provider obligations:** the provider SHALL treat message text as
  unversioned and exit codes as versioned.

## Versioning and Change Policy

- **Versioning scheme:** semantic versioning on the tool's own version.
- **Compatible changes:** the provider MAY add a command, add a flag, add a
  field to `--json` output, and change message text, without a major version
  change.
- **Breaking changes:** WHEN a command or flag is removed or renamed, a
  default changes, an exit code's meaning changes, or a `--json` field is
  removed or re-typed, the provider SHALL increment its major version.
- **Deprecation:** WHEN a command or flag is deprecated, it SHALL continue to
  work for one major version, and invoking it SHALL print a notice to standard
  error naming its replacement.

## Key Decisions

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
- [[module-release-core]] — the library every command calls
- [[frd-tool-configuration]] — the configuration `release config` reports
- [[frd-version-computation]], [[frd-changelog-authoring]],
  [[frd-changelog-rendering]], [[frd-release-staging]] — the behaviour behind
  each command
