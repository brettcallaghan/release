---
title: "Component Blueprint: Evidence Builder"
entity_name: Evidence Builder
description: Collected commit records, parsed notes blocks and budgeted
  diffs joined into the evidence bundle a producer authors from.
slug: component-evidence-builder
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/evidence
test_paths:
  - internal/evidence
---

# Evidence Builder

## Overview

A producer can write only as well as what it is shown. This component
decides what that is: per commit, the record git holds, the reviewed notes
its body carries, and its diff where the notes are missing. The result is an
evidence bundle, the single document a producer reads, at pull request time
and at release time alike.

It assembles and never interprets. It writes no entry, derives no summary,
and decides no impact; the signal each commit carries is copied from the
classifier so the producer can see what was already decided, not
recomputed here. Keeping it this narrow is what lets the same bundle feed a
language model, a command, and the mechanical producer.

- **Role:** the evidence bundle.
- **Scope:** in — joining records, notes, diffs and signals per commit,
  ordering, the budget report, and the adapters used; out — collecting
  anything, authoring, and classifying.
- **Composed into:** [[module-release-core]] — consumes
  [[component-commit-collector]], [[component-notes-codec]],
  [[component-diff-collector]] and [[component-bump-classifier]] outputs,
  feeds [[component-changelog-authoring-bridge]]; see the module for
  relationships.
- **Source & tests:** `internal/evidence`.

## Interface

- **Surface:** `evidence.Build` takes the purpose (`PULL_REQUEST` or
  `RELEASE`), the range (base and head commit hashes; no base for a line
  with no release), the base version at release time, the commit records,
  each commit's parsed notes or parse errors, the diffs collected, each
  commit's classifier signal, the diff budget report, and the adapters
  that ran. It returns an `Evidence` instance as `schemas/evidence.strata`
  defines it.
- **Guarantees:**
  - The component SHALL be a pure function of its inputs.
  - The component SHALL emit a bundle that validates against
    `schemas/evidence.strata`.
  - The component SHALL list commits oldest first.
  - The component SHALL include every commit in the range, whether or not it
    carries notes or a diff.
  - The component SHALL carry each commit's parsed notes, and its parse
    errors WHEN its block does not parse, unchanged.
  - The component SHALL carry the budget report unchanged, so the producer
    knows what it was not shown.
  - The component SHALL record every adapter that contributed, and the
    range it read.
  - The component SHALL NOT author, summarise or reword anything.
  - The component SHALL NOT classify a commit; a `signal` is the
    classifier's, copied.
- **Pinned rules:**
  - At release time a commit carries `diff` only WHEN it has no notes block
    or its block does not parse. At pull request time every commit carries
    `diff`.
  - `base_version` is set at release time and absent at pull request time.
  - A diff is attached to the commit it names; a diff naming no commit in
    the range is an error.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the changelog sources a release reads.
- [[contract-changelog]] — the evidence bundle.

## Change Guidance

WHEN an adapter is added, it SHALL record itself in the bundle's adapters.

---

The component SHALL NOT drop a commit to save budget.

**Rationale:** Budget is spent on diffs, and the Diff Collector reports what
it cut. A commit missing from the bundle is a change the producer cannot
know it never saw, which is the silent omission the budget report exists to
prevent.

---

- **Verification gates:** the component's unit suite over fixture inputs at
  both purposes, including commits with a block, without one, and with one
  that does not parse, plus schema validation of every bundle.

## Key Decisions

- [[adr-003]] — notes are authored from evidence, and reviewed records come
  first.
- [[adr-004]] — authoring is a replaceable step, which is why this
  component stops at a bundle.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[contract-changelog]] — the evidence bundle's semantics
- [[frd-changelog-authoring]] — the feature this component serves
