---
title: "Component Blueprint: Bump Suggester"
entity_name: Bump Suggester
description: The suggested bump derived from the structured changelog's
  entry kinds and raised to the signal floor, with its reasons — and the
  confirmed bump, from a person's confirmation or a named bump, or a halt
  naming what a person must confirm.
slug: component-bump-suggester
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/suggest
test_paths:
  - internal/suggest
---

# Bump Suggester

## Overview

Two readings of a release's impact meet here. The Bump Classifier's
*signal floor* is computed from explicit records in git and is reproducible
from a clone, but it is blind to any commit that carries no signal. The
structured changelog is the producer's reading of the evidence, diffs
included, and says what changed, but nobody has reviewed it yet. This
component derives a **suggested bump** from the changelog's entry kinds,
never lets it fall below the floor, and gives the reasons for it. A person
then confirms the suggestion or names a bump instead ([[adr-003]]).

The suggestion:

1. **From the entry kinds:** major WHEN any entry is breaking; else minor
   WHEN any entry is `FEATURE`; else patch WHEN any entry is `FIX`,
   `PERFORMANCE`, `REFACTOR` or `REVERT`; else none.
2. **Raised to the floor:** WHEN the signal floor is higher, the floor.

The decision, given the suggestion:

| Input | Outcome | Decided by |
|---|---|---|
| A named bump at or above the suggestion | the named bump | `named` |
| A named bump below the suggestion, at or above the floor | the named bump, with a warning | `named` |
| A named bump below the floor, not forced | refusal naming the floor and the commits that set it | — |
| A named bump below the floor, forced | the named bump, with a warning | `named` |
| No named bump, the suggestion accepted | the suggestion | `confirmed` |
| No named bump, the suggestion not accepted | halt: confirm the suggestion or name a bump | — |
| No named bump, the suggestion none, an entry with no kind | halt: name a bump; acceptance does not apply | — |
| No named bump, the suggestion none, every entry with a kind | not releasable: no decision; the caller refuses | — |

The component never prompts. A halt is a result carrying the suggestion
and its reasons; the confirmation is an input to the next call. A front end
that prompts on a terminal passes the answer back as an acceptance or as a
named bump; `--yes` passes an acceptance without prompting; a
non-interactive caller without `--yes` stops at the halt and shows it.
Because the authored changelog is cached for the range, the second call
derives the same suggestion the person saw.

- **Role:** the suggested bump with its reasons, and the confirmed bump or a
  halt.
- **Scope:** in — the suggestion, the decision above, releasability, the
  reasons and warnings; out — computing the signal floor, authoring the
  changelog, prompting, and recording the decision in git.
- **Composed into:** [[module-release-core]] — consumes
  [[component-bump-classifier]] and
  [[component-changelog-authoring-bridge]] outputs, feeds
  [[component-version-calculator]] and [[component-stage-orchestrator]];
  see the module for relationships.
- **Source & tests:** `internal/suggest`.

## Interface

- **Surface:** `suggest.Suggest` takes the structured changelog's entries
  (each entry's `kind`, `breaking` and `sources`) and its `omitted` list,
  the signal floor with the commits that drove it, the unclassified commits, the
  number of commits in the range, an optional named bump, whether the
  release is forced, and whether the suggestion is accepted. It returns
  either a **decision** — the confirmed bump, who decided it (`named` or
  `confirmed`), the suggestion with its reasons, whether the range is
  releasable, and any warning — or a **halt** carrying the suggestion, its
  reasons, the signal floor with its drivers, the unclassified commits, and
  whether only a named bump can answer it — or a **refusal** of a named bump
  below the floor — or, with no named bump over a range that is not
  releasable, the suggestion reported as not releasable.
- **Reasons:** the entries that drove the suggestion from the kinds, each
  with its summary and sources; and, WHEN the floor raised it, the floor
  and the commits with the signals that set it.
- **Guarantees:**
  - The component SHALL be a pure function of its inputs.
  - The component SHALL derive the suggestion from the entry kinds as
    above, and SHALL raise it to the signal floor WHERE the floor is
    higher.
  - The component SHALL report the reasons for every suggestion, including
    whether the floor raised it.
  - WHEN a bump is named at or above the signal floor, the component SHALL
    decide it, and SHALL warn WHEN it is below the suggestion.
  - IF a named bump is below the signal floor and the release is not
    forced, THEN the component SHALL refuse it, naming the floor and the
    commits that set it.
  - WHEN a named bump below the signal floor is forced, the component SHALL
    decide it with a warning.
  - WHEN no bump is named and the suggestion is accepted, the component
    SHALL decide the suggestion as `confirmed`.
  - WHEN no bump is named and the suggestion is not accepted, the component
    SHALL halt.
  - WHEN no bump is named, the suggestion is none, and an entry has no
    kind, the component SHALL halt for a named bump whether or not the
    suggestion is accepted, and SHALL name the entries with no kind as
    changes no producer classified.
  - WHEN no bump is named, the suggestion is none, and every entry has a
    kind, the component SHALL report the range not releasable and SHALL
    NOT halt.
  - WHEN a bump is named over a range whose suggestion is none and where an
    entry has no kind, the component SHALL decide it without requiring a
    force.
  - A halt SHALL carry the suggestion, its reasons, the signal floor with its
    drivers, and the unclassified commits, so a person can confirm from it
    alone.
  - The component SHALL report the range releasable WHEN the suggestion,
    the floor included, is patch or above, or WHEN any entry has no kind
    ([[release-tooling-standards]]).
  - The component SHALL NOT decide a bump from the suggestion alone: the
    suggestion becomes the confirmed bump only through an acceptance.
  - The component SHALL NOT prompt, read the repository, or invoke a
    producer.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the suggested bump, its confirmation, the
  signal floor, and releasability.

## Change Guidance

WHEN a row is added to the decision table, or a kind's place in the
suggestion changes, the guarantees and the component's table-driven tests
SHALL change in the same change.

---

The component SHALL NOT suggest a bump below the signal floor.

**Rationale:** The floor comes from explicit records a person wrote into
git. A changelog that reads lower is a reason for a person to look, not a
reason to publish a smaller version than those records require; going below
the floor takes a named bump and a force.

---

- **Verification gates:** the component's unit suite, table-driven over
  every row of the decision table, the suggestion from each kind with and
  without breaking entries, a floor above and below the kinds, a
  suggestion of none with an entry that has no kind, and a suggestion of
  none with every entry kinded.

## Key Decisions

- [[adr-003]] — the bump is suggested from the changelog's entry kinds,
  floored by explicit git signals, and confirmed by a person; the
  confirmation is committed.

## Open Questions

- [ ] What a named `none` over a releasable range yields. The range
  holds a change with no kind, but the bump increments nothing; the Version
  Calculator returns no release for it. Decided with
  [[frd-version-computation]].

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-bump-classifier]] — the signal floor and the unclassified
  commits
- [[component-stage-orchestrator]] — records the confirmed bump as a
  `Release-Type:` trailer
- [[frd-version-computation]] — the feature this component serves
