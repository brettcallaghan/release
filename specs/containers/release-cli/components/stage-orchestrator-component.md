---
title: "Component Blueprint: Stage Orchestrator"
description: The sequence that produces a stage branch and its pull
  request, and the only component that writes to a repository or a forge.
slug: component-stage-orchestrator
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed:
  - contract-changelog
source_paths:
  - cli/release/internal/stage
test_paths:
  - cli/release/internal/stage
---

# Stage Orchestrator

## Overview

Runs the sequence: read the base version, classify impact, compute the next
version, build the corpus, exchange it with the author, render, write the
changelog, write the version manifests, commit, branch, and open the pull
request. It holds no logic of its own beyond ordering and failure handling.

It is the only component that writes. Everything upstream reads or computes,
which means a failure part-way through leaves the repository in a state this
component is responsible for describing — and, where it can, not leaving at
all. It performs every check that can fail before the first write.

- **Role:** the staging sequence, and the only write surface.
- **Scope:** in — ordering, pre-flight checks, the commit, the branch, the
  pull request; out — every computation, which belongs to the components it
  calls.
- **Composed into:** [[container-release-cli]] — depends on every other
  component; see it for relationships.
- **Source & tests:** `cli/release/internal/stage`.

## Interface

- **Surface:** given a target release branch and a policy, produce a stage
  branch and open its pull request, or report why it did not.
- **Guarantees:**
  - The component SHALL perform every check that can fail before making any
    write.
  - The component SHALL create a stage branch holding exactly one commit.
  - The component SHALL include the rendered release notes in the pull
    request body.
  - WHEN no releasable change exists since the latest reachable tag, the
    component SHALL refuse without writing.
  - WHEN the author's proposed impact disagrees with the computed impact,
    the component SHALL halt without writing.
  - IF a write fails part-way, THEN the component SHALL report every
    change it made.
  - The component SHALL NOT tag, publish, or merge.
  - The component SHALL NOT modify the target branch.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — stage-branch shape, release-commit shape,
  and the refusal conditions.

## Change Guidance

WHEN a new failure condition is introduced, it SHALL be checked before the
first write if it can be.

**Rationale:** A condition checked after the commit exists turns a clean
refusal into a partial state a maintainer has to clean up by hand.

---

The component SHALL NOT modify the target release branch.

**Rationale:** Not modifying a release branch outside a reviewed pull
request is the principle the whole design rests on ([[adr-002]]). This is
the one component with the access to violate it.

---

The component SHALL NOT implement a computation that belongs to a
component it calls.

**Rationale:** Logic that migrates into the orchestrator stops being
testable without a repository, which is how the correctness-critical parts
lose their exhaustive tests.

- **Verification gates:** the end-to-end staging test against a fixture
  repository, covering the accepting path, the nothing-to-release refusal,
  the disagreement halt, and a partial-write report.

## Key Decisions

- [[adr-002]] — a release is staged as a single-commit branch and merged by
  pull request; nothing modifies a release branch directly.
- [[adr-003]] — disagreement between the computed and proposed impact halts
  the release.

## Open Questions

- [ ] Whether staging should offer an unattended mode that skips the
  authoring exchange and renders a mechanically derived corpus, for
  adopters with no model available. Decided with the autorelease question
  in [[container-release-cli]].

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[frd-release-staging]] — the feature this component serves
