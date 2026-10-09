---
title: "Component Blueprint: Impact Reconciler"
entity_name: Impact Reconciler
description: The confirmed release impact, decided from the computed impact,
  the commits with no reviewed signal, the authored proposal and a named
  bump — or a halt naming what a person must confirm, and why.
slug: component-impact-reconciler
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/reconcile
test_paths:
  - internal/reconcile
---

# Impact Reconciler

## Overview

Two readings of a release's impact meet here. The Bump Classifier's is
computed from reviewed records in git and is reproducible from a clone, but
it is blind to any commit that carries no signal. The producer's proposal is
read from the evidence, diffs included, and comes with reasons, but nobody
reviewed it. This component decides which governs, and when neither may
govern alone, it says so and stops ([[adr-003]]).

| Situation | Outcome | Decided by |
|---|---|---|
| A bump is named | the named bump | `named` |
| Every commit has a reviewed signal, and the proposal is absent or no higher than the computed impact | the computed impact | `computed` |
| Every commit has a reviewed signal, and the proposal is higher | halt: confirm one or the other | — |
| Some commits have no reviewed signal, and the computed impact is below major | halt: confirm the proposal or name another bump | — |
| Some commits have no reviewed signal, and the computed impact is major | the computed impact | `computed` |

A confirmation is a named bump. A front end that prompts passes the answer
back as one, and a non-interactive caller names it after showing the halt to
the person it acts for, so both paths reach the same row of the table.

- **Role:** the confirmed impact, or a halt with reasons.
- **Scope:** in — the decision above, releasability, and the reasons; out —
  computing the impact, authoring the proposal, prompting, and recording
  the decision in git.
- **Composed into:** [[module-release-core]] — consumes
  [[component-bump-classifier]] and
  [[component-changelog-authoring-bridge]] outputs, feeds
  [[component-version-calculator]] and [[component-stage-orchestrator]];
  see the module for relationships.
- **Source & tests:** `internal/reconcile`.

## Interface

- **Surface:** `reconcile.Reconcile` takes the computed impact with the
  commits that drove it, the commits carrying no enabled signal, the
  authored proposal (`proposed_bump`, `proposal_rationale` and
  `proposal_drivers`) or none, whether the authored corpus holds a
  user-visible change, and an optional named bump. It returns either a
  decision — the confirmed impact, who decided it (`named` or
  `computed`), whether the range is releasable, and any warning — or a
  halt carrying the computed impact, the proposal and its reasons, and the
  unclassified commits.
- **Guarantees:**
  - The component SHALL be a pure function of its inputs.
  - WHEN a bump is named, the component SHALL decide it, whatever the
    computed impact and the proposal.
  - WHEN a named bump is lower than the computed impact or the proposal,
    the component SHALL say so as a warning.
  - WHEN no bump is named, every commit carries a reviewed signal, and the
    proposal is absent or no higher than the computed impact, the component
    SHALL decide the computed impact.
  - WHEN no bump is named and the proposal is higher than the computed
    impact, the component SHALL halt.
  - WHEN no bump is named, a commit carries no reviewed signal, and the
    computed impact is below major, the component SHALL halt.
  - A halt SHALL carry the proposal, its rationale and its drivers, the
    computed impact with its drivers, and the unclassified commits, so a
    person can confirm from it alone.
  - WHEN a halt has no proposal to offer, or the proposal is the mechanical
    producer's restatement of the computed impact, it SHALL say that no
    producer read the unclassified commits' diffs.
  - The component SHALL report the range releasable WHEN the computed
    impact is patch or above, or the authored corpus holds a user-visible
    change.
  - The component SHALL NOT decide an impact from a proposal alone: a
    proposal becomes an impact only through a named bump.
  - The component SHALL NOT prompt, read the repository, or invoke a
    producer.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — deterministic computation, confirmation of
  an unreviewed impact, and releasability.

## Change Guidance

WHEN a row is added to the decision table, the guarantees and the
reconciler's table-driven tests SHALL change in the same change.

---

The component SHALL NOT lower a computed impact without a named bump.

**Rationale:** The computed impact comes from reviewed records. A proposal
lower than it is a reason for a person to look, not a reason to publish a
smaller version than the reviewers agreed to.

---

- **Verification gates:** the component's unit suite, table-driven over
  every row of the decision table, with and without a proposal, including a
  named bump below the computed impact.

## Key Decisions

- [[adr-003]] — reviewed records decide; a person confirms anything
  unreviewed, and the confirmation is committed.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-bump-classifier]] — the computed impact and the unclassified
  commits
- [[component-stage-orchestrator]] — records the decision as a
  `Release-Type:` trailer
- [[frd-version-computation]] — the feature this component serves
