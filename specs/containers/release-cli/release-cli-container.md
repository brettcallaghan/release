---
title: "Container Blueprint: Release CLI"
entity_name: Release CLI
description: The command-line front end to the release core library — its
  command surface, configuration loading, and output.
slug: container-release-cli
type: container-blueprint
status: approved
updated_at: 2026-10-08T00:00:00Z
contracts_consumed: []
modules:
  - module-release-core
---

# Release CLI

## Overview

A single-binary command-line tool that exposes the release process to a
maintainer. It parses arguments, loads the repository's release
configuration, calls [[module-release-core]], and renders the result.

It holds no release logic. Version computation, changelog assembly, rendering
and staging all live in the module, so that a second front end — an HTTP
service, a tool-calling server — is a new wrapper rather than a second
implementation of the same rules.

- **Responsibility:** the command surface, configuration loading, output and
  exit codes.
- **Out of scope:** everything the module owns, plus building artifacts,
  publishing them, and verifying a merged release commit — the last of which
  is the adopting project's required checks ([[repository-release-policy]]).

## Container Context

```mermaid
flowchart LR
  maintainer["Maintainer or agent<br/>(workstation)"]
  cli["Release CLI<br/>[container]"]
  core["Release Core<br/>[module]"]
  producer["Changelog producer<br/>(model, command, or person)"]
  git["Git repository<br/>[local]"]
  platform["Hosting platform API<br/>(pull requests, metadata)"]

  maintainer -->|"release cut minor"| cli
  cli -->|"calls"| core
  core -->|"reads tags, commits"| git
  core -->|"writes branch and commit"| git
  core -->|"opens the stage pull request"| platform
  core -->|"exchanges a changelog corpus"| producer
```

Continuous delivery does not invoke this container. Verifying a merged release
commit is done by the project's required checks, and publishing is done by the
project's own workflows.

## Components

### Release Core

[[module-release-core]] — the library this container wraps; every release
capability lives there.

### Command Surface

Subcommands, flags and argument validation. `release version` prints the
computed version and its justification; `release changelog` assembles,
authors and renders without staging; `release cut [major|minor|patch]` stages
a release, with `--skip-notes` to stage without an authoring exchange.

`release version [major|minor|patch] [--target <branch>] [--json]` calls the
release core's version resolution operation with the loaded configuration's
policy: its `tag_prefix`, `stage_prefix`, `leaps`, `pre1_remap` and
`signals`. Without `--target` it resolves the configured `target`, and
without one the ref `origin/HEAD` points to, read locally
(`refs/remotes/origin/<branch>`); IF neither is available, THEN it exits 2
naming `git remote set-head origin --auto`. Its answer is one mapping with
keys `base`, `impact`, `named`, `determined_by` (each `hash`, `subject`,
`signal`) and `next`, in that order; it exits 1 after printing the answer
WHEN no release is warranted or the policy refuses the version, and 2 for an
invalid bump or a repository it cannot read. WHEN a bump is named and the
resolution's inferred impact is none, it prints a warning on standard error
and still exits 0.

`release changelog [major|minor|patch] [--target <branch>] [--skip-notes]
[--redo-notes] [--corpus <path>] [--write] [--json]` calls the release
core's changelog operation (`internal/changelog`), which resolves the
version as `release version` does, assembles the corpus from the range's
commits, exchanges it with the configured producer through the authoring
bridge, sets the corpus `version`, and renders it. Its answer is one
mapping with keys `version`, `corpus_path` (where the authored corpus is
kept, relative to the repository root when inside it; null under
`--skip-notes`) and `section` (the rendered changelog section, trimmed as
every multi-line value is), in that order. It exits 1 WHEN no release is
warranted or the policy refuses the version; WHEN the corpus's
`proposed_bump` disagrees with the computed impact and no bump is named,
it answers `computed`, `proposed` and `corpus_path` instead, explains on
standard error, writes nothing, and exits 1; and it exits 1 WHEN the
producer fails or its corpus is rejected. A named bump governs, and the
proposal is then advisory. `--skip-notes` invokes no producer and renders
the assembled corpus; `--redo-notes` discards the cached corpus and
authors again; `--corpus <path>` reads that path instead of the cache WHEN
it exists and writes the authored corpus there otherwise; `--write` places
the section in `CHANGELOG.md` through the changelog writer and changes
nothing else. `--skip-notes` with `--redo-notes` or `--corpus` is invalid,
and a missing producer prerequisite exits 2. The repository web URL the
renderer links sources to is derived from a GitHub `origin` remote, and
sources render as plain text otherwise.

`release cut [major|minor|patch] [--target <branch>] [--skip-notes]
[--force] [--redo-notes] [--no-pull-request] [--dry-run] [--yes] [--json]` passes its
flags to the release core's Stage Orchestrator (`internal/cut`). Without
`--target` it cuts the configured `target`, or else the branch `origin/HEAD`
points to, as a branch name on `origin`. WHILE standard input is a terminal
and neither `--yes` nor `--dry-run` is given, it shows the version and the
writes it will make on standard error and prompts once before the first
write; anything but `y` or `yes` declines, which is a refusal. Its answer is
one mapping with keys `version`, `branch`, `commit`, `pull_request` (the URL,
or null) and `writes` (every write made, in order, each a line such as
`push cut/1.9.0 to origin`); under `--dry-run` it is `version`, `branch`,
`writes` listing the writes it would make, and `section`, the changelog
section the release commit would write into `CHANGELOG.md`. It exits 1 on a refusal (nothing
to release, an impact disagreement, an existing stage branch, an
unconfigured `commands.propagate`, a `gh` that is missing or not
authenticated when a pull request is to be opened, a failing configured command, a path
outside `release_paths`, a declined prompt, a failing or rejected producer),
having written nothing and answering nothing; 3 WHEN a write failed after
another succeeded, answering with every write made and naming on standard
error the commands that finish or discard the release; and 0 otherwise.
`--skip-notes` or `--force` without a bump is a refusal. A named bump over
commits that indicate no impact is a refusal suggesting `--force`, which
stages it anyway. `--skip-notes` with `--redo-notes`, or a
bump other than `major`, `minor` or `patch`, is invalid (exit 2).

`release config [--json]` answers with the effective configuration, declared
values over the release core's defaults, as one mapping: `target`,
`tag_prefix`, `stage_prefix`, `release_branches`, `release_paths`, `leaps`,
`pre1_remap`, `signals`, `commands` holding `validate`, `propagate` and
`preflight`, `changelog` holding `groups`, `other_heading`,
`breaking_heading`, `producer` and `producer_command`, and `adapters`
holding `pull_requests` and `diffs`, in that order. An unset command is null and an empty list `[]`.
`target` is the configured one, or else the branch `origin/HEAD` points to;
IF neither is available, THEN it exits 2 naming
`git remote set-head origin --auto`.

The global flags `-C, --repo <path>` and `--config <path>` name the
repository and the configuration file for every command.

```component
title: Command Surface
slug: component-command-surface
description: Subcommand and flag definitions, argument validation, and the mapping from invocation to a core library call.
```

### Configuration Loader

Discovers the repository's release configuration, parses it, and refuses a
configuration the tooling cannot accept before any command runs.

- It runs as the root command's persistent pre-run, so it precedes every
  subcommand's own work; the root itself, `help` and `--help` load nothing.
- The repository root is the working tree containing `-C/--repo`, or the
  working directory; outside a working tree it is that directory.
- WHEN `--config` is given, it reads that file, and refuses one that does
  not exist. Otherwise it looks for `release.toml` and
  `config/release.toml` at the repository root: both present is refused,
  naming both; neither present means the release core's defaults.
- It decodes TOML over the release core's defaults, so a declared value,
  lists included, replaces its default. A key the configuration does not
  define is refused by name, which is also why no file can disable a check.
- It then applies the release core's validation, and reports every reason
  — unknown keys and validation refusals together — on standard error, and
  the CLI exits 2 with nothing else run.

```component
title: Configuration Loader
slug: component-configuration-loader
description: Locates and parses the repository's release configuration, and validates it before any command executes.
```

### Output Renderer

YAML by default and JSON under `--json`, plus exit codes. The module returns
structured results; this renders them as [[cli-output-standards]] specifies,
so one answer reads the same at a terminal, in a script and to an agent.

```component
title: Output Renderer
slug: component-output-renderer
description: Renders structured results from the core library as YAML by default or JSON on request, and sets exit codes.
```

## Provided Contracts

- [[contract-release-cli]] — the command surface: commands, arguments, flags,
  output and exit codes.

The changelog corpus contract is provided by [[module-release-core]] rather
than here.

## Consumed Contracts

None. The hosting platform's API is a third-party interface governed by its
vendor's published contract.

## Data

The container owns no persistent store and holds no state between
invocations. It reads a configuration file and writes nothing of its own.

## Infrastructure and Deployment

A single statically linked binary, distributed as a release artifact and
installable from source. It is invoked on a maintainer's workstation and
exits.

No daemon, no server, no scheduled component. Commands that write require a
workstation, because the authoring exchange needs a changelog producer that
continuous delivery does not have.

## Cross-Cutting Standards

This container SHALL comply with:

- [[release-tooling-standards]] — the configuration surface in particular.

## Change Guidance

The container SHALL NOT implement a release capability that belongs to
[[module-release-core]].

**Rationale:** Logic that migrates into the front end stops being reachable by
any other front end, which is the whole reason the module exists.

---

WHEN a command is added, it SHALL map onto capabilities the module already
exposes, or the module SHALL be extended in the same change.

---

The container SHALL NOT make a release decision the module could make.

**Rationale:** A default applied in argument parsing is a default no other
consumer inherits.

---

- **Verification gates:** the command-surface tests, including argument
  validation and exit codes, plus the configuration-loader tests.

## Key Decisions

- [[adr-005]] — the release process is a composable library behind a thin
  front end; this container is that front end.

## Open Questions

- [ ] How an autorelease policy can work without a language model in
  continuous delivery. Hosting-platform labels were rejected as the trigger
  because the autorelease path would need a model in a workflow, and
  therefore an API credential and a per-call budget. Until this is answered,
  staging is a workstation command and autorelease is out of scope. Decided
  by the project owner.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the library this container wraps
- [[contract-release-cli]] — the command surface this container vends
- [[cli-output-standards]] — the shape the Output Renderer gives every answer
- [[frd-tool-configuration]] — the configuration this container loads
