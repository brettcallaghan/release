---
title: "Component Blueprint: Delegated Command Runner"
entity_name: Delegated Command Runner
description: Runs a repository's configured command for a named release step
  and reports its outcome.
slug: component-delegated-command-runner
type: component-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/delegate
test_paths:
  - internal/delegate
---

# Delegated Command Runner

## Overview

The tooling delegates every step whose correct behaviour depends on the
repository's ecosystem: validating a release, writing a version into
manifests, and preflight checks ([[adr-005]]). This component is the one
place a configured command runs, so every delegating step stops, reports and
refuses the same way.

- **Role:** run a configured command for a named step and report the outcome.
- **Scope:** in — running a step's command or commands and capturing their
  outcome; out — deciding when a step runs, which is the
  [[component-stage-orchestrator]]'s, and reading the configuration, which is
  the module's configuration model.
- **Composed into:** [[module-release-core]] — consumed by
  [[component-stage-orchestrator]].
- **Source & tests:** `internal/delegate`.

## Interface

- **Surface:** run one step with a step name, its configured command (or its
  absence) and a working directory; run a step with an ordered list of
  commands. Each returns success, a failure, or an unconfigured result.
- **Guarantees:**
  - WHEN a step has a configured command, the component SHALL run it in the
    given working directory, the repository root, and return success only on
    exit status 0.
  - IF the command exits non-zero, THEN the component SHALL return a failure
    carrying the command, its exit status, and its standard output and
    standard error in full.
  - WHEN a step has no configured command, the component SHALL return an
    unconfigured result naming the step, never success.
  - WHEN a step has several commands, the component SHALL run them in order
    and stop at the first failure.
  - The component SHALL run a command as a string through `sh -c`, with the
    environment inherited and standard input empty.
  - IF a command cannot be started, THEN the component SHALL return an error,
    distinct from a command that ran and failed.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the configuration requirements.

## Change Guidance

The component SHALL NOT run the tooling's own `git` or `gh` invocations.

**Rationale:** those are argument lists run without a shell ([[adr-006]]); a
configured command is a string the repository wrote, and only it goes through
`sh -c`.

- **Verification gates:** the component's unit suite runs real `sh -c`
  commands in a temporary directory, covering success, a non-zero exit with
  output on both streams, the unconfigured step, and a preflight list that
  stops at its first failure.

## Key Decisions

- [[adr-005]] — repository-specific behaviour is delegated rather than
  implemented.
- [[adr-006]] — the tooling's own git calls stay argument lists.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-tool-configuration]] — the feature this component serves
