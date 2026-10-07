---
title: "Feature Blueprint: Tool Configuration"
description: How release core gains a validated configuration model and a
  delegated-command runner, and how the CLI discovers, loads and reports
  the configuration.
slug: fb-tool-configuration
type: feature-blueprint
status: draft
updated_at: 2026-10-07T00:00:00Z
delivery: single-pr
frd: frd-tool-configuration
---

# Tool Configuration

## Overview

**FRD:** [[frd-tool-configuration]]

The configuration is modelled, defaulted and validated in
[[module-release-core]], so a second front end inherits the same refusals;
[[container-release-cli]] only finds the file, decodes it into that model,
and refuses before any command runs. Delegated commands run through one new
module component so every step that delegates fails the same way. The
feature then replaces the default policy `release version` ships with
([[fb-version-computation]]) by the loaded configuration. Delta blocks in
dependency order: CFGM, RUN, LOAD, CMDCFG, WIRE.

## Element Deltas

### [[module-release-core]] (modified — configuration model)

**Intent:** One type holding every repository-specific value, its
documented defaults, and the validation that refuses a configuration that
cannot work. It extends the default policy the version computation feature
added to the module rather than defining a second one.

**Behavior:**
- The module SHALL define the configuration as these settings, each with
  the default shown:

  | Setting | Type | Default |
  |---|---|---|
  | `target` | branch name | see Open Questions |
  | `tag_prefix` | string | `v` |
  | `stage_prefix` | string | `cut/` |
  | `release_branches` | list of globs | empty |
  | `release_paths` | list of paths | empty |
  | `leaps` | bool | `false` |
  | `pre1_remap` | bool | `true` |
  | `signals` | list of signal names | every signal |
  | `commands.validate` | command | unset |
  | `commands.propagate` | command | unset |
  | `commands.preflight` | list of commands | empty |

- The module SHALL refuse a configuration WHEN any glob in
  `release_branches` matches the stage branch name of version `1.2.3` or
  `1.2.3-rc.1` under the configured `stage_prefix` (RTS-009).
- The module SHALL interpret a glob with Go `path.Match` semantics, where
  `*` does not cross `/`.
- The module SHALL refuse an empty `stage_prefix` and a `signals` entry that
  names no known signal.
- The module SHALL return every refusal reason found, not only the first.
- The module SHALL derive no constraint from the name in `target`.
- The configuration SHALL have no setting that disables a check the
  tooling performs.

**Anchors:** the release-core package holding the version computation
feature's default policy (its RES delta); the Interface section of
`specs/modules/release-core/release-core-module.md`.

### `Delegated Command Runner` in [[module-release-core]] (added)

**Intent:** One place that runs a configured command for a named step, so
validation, propagation and preflight stop, report and refuse identically.
Running each step at the right point of a cut is the release staging
feature's; this delta provides the runner and its failure semantics.

**New components:**
- **Delegated Command Runner** (Go package) — runs a repository's
  configured command for a named step. Takes a step name, the configured
  command (or its absence) and a working directory; returns success, the
  command's failure with its output, or an unconfigured result.

  ```component
  title: Delegated Command Runner
  slug: component-delegated-command-runner
  description: Runs a repository's configured command for a named release step and reports its outcome.
  ```

**Behavior:**
- WHEN a step has a configured command, the runner SHALL run it in the
  repository root and return success only on exit status 0.
- IF the command exits non-zero, THEN the runner SHALL return a failure
  carrying the command's exit status, standard output and standard error
  in full.
- WHEN a step has no configured command, the runner SHALL return an
  unconfigured result naming the step, never success.
- WHEN a step has several commands (`preflight`), the runner SHALL run
  them in order and stop at the first failure.
- The runner SHALL NOT read from a terminal: the command's standard input
  is empty.

**Anchors:** new package under `internal/` (the implementer's choice of
name); see Open Questions for how a command string is executed.

### `Configuration Loader` in [[container-release-cli]] (added)

**Intent:** Implement the Configuration Loader the container already
declares ([[container-release-cli#Configuration Loader]]): find the file,
decode it into the module's configuration, validate, and refuse before any
command runs.

**Behavior:**
- The loader SHALL read the file named by `--config` WHEN it is given, and
  otherwise the repository-root configuration file named in Open Questions,
  where the repository root is that of `-C/--repo` or the working
  directory.
- WHEN no configuration file exists and `--config` is not given, the loader
  SHALL use the module defaults.
- IF `--config` names a file that does not exist, THEN the loader SHALL
  refuse.
- The loader SHALL refuse a file containing a key the configuration does
  not define, naming the key.
- The loader SHALL run before every command's own work, and IF the
  configuration is refused, THEN the CLI SHALL exit 2, print every reason
  to standard error, and run nothing else.

**Anchors:** `internal/cli/root.go` (the root command, where a pre-run
hook loads before any subcommand); `github.com/BurntSushi/toml` is already
a module dependency if the file format is TOML.

### [[container-release-cli]] (modified — `release config`)

**Intent:** `release config [--json]` per [[contract-release-cli]]: answer
with the effective configuration, declared values merged over defaults.

**Behavior:**
- The command SHALL answer with one mapping holding every setting in the
  order of the CFGM table, using the setting names as keys and the
  `commands` settings nested under a `commands` key, rendered per
  [[cli-output-standards]].
- An unset command SHALL appear as null and an empty list as `[]`.
- WHEN the configuration is refused, the command SHALL exit 2 through the
  loader, like every other command.

**Anchors:** `internal/cli/root.go`; `testdata/help/` and `testdata/cases/`
(regenerated by `scripts/regenerate-snapshots.sh`).

### [[container-release-cli]] (modified — `release version` reads configuration)

**Intent:** The version computation feature shipped `release version` on
the module's default policy; this delta passes it the loaded
configuration instead.

**Behavior:**
- `release version` SHALL compute with the loaded configuration's
  `tag_prefix`, `stage_prefix`, `leaps`, `pre1_remap` and `signals`.
- WHEN `--target` is absent, `release version` SHALL use the configured
  `target`, and otherwise the default that Open Questions settles.

**Anchors:** the `release version` command in `internal/cli`.

### Relied-upon elements (unchanged)

- [[contract-release-cli]] — `release config`, `--config`, and exit 2 for
  an unacceptable configuration.
- [[cli-output-standards]] — the shape of `release config`'s answer.
- [[release-tooling-standards]] — RTS-009 and RTS-025 to RTS-033.

## Contracts

None. [[contract-release-cli]] already specifies `release config` and
`--config`.

## Tracker Reconciliation

- release-cb2.8 — prerequisite: the WIRE delta changes the `release
  version` command that work order creates.
- release-cb2.7 — prerequisite: CFGM extends the default policy that work
  order adds to release core.

## Sequencing Constraints

- The CFGM delta SHALL land before the LOAD, CMDCFG and WIRE deltas.
- The LOAD delta SHALL land before the CMDCFG and WIRE deltas.
- No lockstep-update rule in the owning blueprints' Change Guidance spans
  more than one delta block.

## AC Coverage

Legend: CFGM = [[module-release-core]] delta (— configuration model); RUN
= `Delegated Command Runner` delta; LOAD = `Configuration Loader` delta;
CMDCFG = [[container-release-cli]] delta (— `release config`); WIRE =
[[container-release-cli]] delta (— `release version` reads configuration).

| AC | Satisfied by | Verified by |
|----|--------------|-------------|
| AC-CFG-001.1 | **RUN (proves)** | unit |
| AC-CFG-001.2 | **RUN (proves)** | unit |
| AC-CFG-001.3 | **RUN (proves)** | unit |
| AC-CFG-002.1 | **CFGM (proves)**, CMDCFG | unit |
| AC-CFG-002.2 | **CFGM (proves)**, LOAD | unit |
| AC-CFG-002.3 | CFGM, **LOAD (proves)** | integration |
| AC-CFG-002.4 | CFGM, **LOAD (proves)** | unit |
| AC-CFG-002.5 | **CFGM (proves)**, WIRE | unit |

## Key Decisions

- [[adr-001]] — the target branch's name carries no meaning.
- [[adr-005]] — repository-specific behaviour is delegated, and the
  module, not the CLI, owns defaults and refusals.

## Definition of Done

- [ ] Every delta above is folded into its owning blueprint:
  - [ ] [[module-release-core]] — Interface records the configuration
    model, and Components lists the Delegated Command Runner
  - [ ] [[container-release-cli]] — Configuration Loader and Command
    Surface record discovery, refusal and `release config`
- [ ] Every AC in the coverage table is verified at its declared level
  and cited from its tests per the specs plugin's
  `shared/ac-traceability-standards.md`
- [ ] This blueprint's status → retired

## Open Questions

- [ ] **Configuration file name and format.** Proposed: `release.toml` at
  the repository root, in TOML like `swarm.toml`, recorded in an ADR. The
  alternative is YAML, matching the output format. Blocks LOAD. Decided by
  the project owner.
- [ ] **Default target branch.** [[frd-tool-configuration]]
  (AC-CFG-002.1) says the repository's default branch, but
  [[contract-release-cli]] says `--target` falls back to the configured
  target and then the current branch. Proposed: follow the FRD, take the
  default branch from `origin/HEAD`, and correct the contract. Blocks CFGM
  and WIRE. Decided by the project owner.
- [ ] **How a configured command runs.** Proposed: a string run through
  `sh -c` from the repository root, as `swarm.toml` checks are. The
  alternative is an argument list with no shell, which avoids quoting
  surprises but cannot use pipes or `&&`. swarm's ADR-025 takes the
  argument-list side for every external command, and adopting it here is
  still undecided, so this answer should agree with that one. Blocks RUN.
  Decided by the project owner.

## References

- `shared/c4-standards.md` — the module and container definitions these
  deltas are written against
- [[frd-tool-configuration]] — the requirements this realizes
- [[fb-version-computation]] — the default policy this feature replaces
- [[release-tooling-standards]] — the configuration requirements
