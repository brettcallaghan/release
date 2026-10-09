---
title: "Component Blueprint: Evidence Builder"
entity_name: Evidence Builder
description: Collected commit records, the pull request information git
  carries, budgeted diffs and classifier signals joined into the evidence
  bundle a producer authors the structured changelog from.
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
decides what that is: per commit, the record git holds, the pull request
number, title and description the commit carries, the files it changed, its
diff, and the signal the classifier read. The result is an evidence bundle,
the single document a producer reads when it authors a release's
structured changelog.

It assembles and never interprets. It writes no entry, derives no summary,
and decides no impact; the signal each commit carries is copied from the
classifier so the producer can see what git already records, not
recomputed here. Keeping it this narrow is what lets the same bundle feed a
language model, a command, and the mechanical producer.

- **Role:** the evidence bundle.
- **Scope:** in — joining records, pull request information, diffs and
  signals per commit, ordering, the budget report, and the adapters used;
  out — collecting anything, authoring, and classifying.
- **Composed into:** [[module-release-core]] — consumes
  [[component-commit-collector]], [[component-diff-collector]] and
  [[component-bump-classifier]] outputs, feeds
  [[component-changelog-authoring-bridge]]; see the module for
  relationships.
- **Source & tests:** `internal/evidence`.

## Interface

- **Surface:** `evidence.Build` takes the range (base and head commit
  hashes; no base for a line with no release), the base version, the commit
  records, the diffs collected, each commit's classifier signal, the diff
  budget report, and the adapters that ran. It returns an `Evidence`
  instance as [[contract-changelog]] defines it: one `CommitEvidence` per
  commit, carrying its record (with the pull request information git
  carries and its changed files), its diff, and its signal.
- **Guarantees:**
  - The component SHALL be a pure function of its inputs.
  - The component SHALL emit a bundle that validates against
    `schemas/evidence.strata`.
  - The component SHALL list commits oldest first.
  - The component SHALL include every commit in the range, whether or not it
    carries pull request information, a diff, or a signal.
  - The component SHALL carry each commit's record, including its pull
    request number, title and description where the record holds them,
    unchanged.
  - The component SHALL carry the budget report unchanged, so the producer
    knows what it was not shown.
  - The component SHALL record every adapter that contributed, and the
    range it read.
  - The component SHALL NOT author, summarise or reword anything.
  - The component SHALL NOT classify a commit; a `signal` is the
    classifier's, copied, and absent for an unclassified commit.
- **Pinned rules:**
  - Every commit carries `diff` WHEN the Diff Collector ran; a commit whose
    patch the budget cut still carries its `DiffRecord`, marked
    `truncated`, and its file list.
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
know it never saw, and the authoring bridge requires every commit to be
cited or omitted, which it cannot check against a commit the bundle lacks.

---

- **Verification gates:** the component's unit suite over fixture inputs,
  including squash commits carrying a pull request title and description,
  merge commits, direct commits, unclassified commits and truncated diffs,
  plus schema validation of every bundle.

## Key Decisions

- [[adr-003]] — the structured changelog is authored from commit evidence
  in git.
- [[adr-004]] — authoring is a replaceable step, which is why this
  component stops at a bundle.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[contract-changelog]] — the evidence bundle's semantics
- [[frd-changelog-authoring]] — the feature this component serves
