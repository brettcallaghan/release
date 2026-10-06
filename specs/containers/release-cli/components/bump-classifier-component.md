---
title: "Component Blueprint: Bump Classifier"
description: Release impact computed deterministically from commit
  metadata, with the commits that drove the result reported alongside it.
slug: component-bump-classifier
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - cli/release/internal/bump
test_paths:
  - cli/release/internal/bump
---

# Bump Classifier

## Overview

Turns commit records into one of major, minor, patch or none — and reports
which commits drove the answer. It is the deterministic half of
[[adr-003]]: the authority on release impact, against which an authored
proposal is checked.

It reads three signals, each independently enablable: a release-type
trailer in the commit body, a conventional-commit type in the subject, and
a breaking-change marker in either. All three live in git, which is what
keeps the computation reproducible from a clone.

Reporting the driving commits is not a convenience. A version a maintainer
disagrees with is otherwise undiagnosable, and the most common cause — one
mis-typed commit — is invisible in the result alone.

- **Role:** the computed release impact, and its justification.
- **Scope:** in — signal detection, precedence, aggregation; out — reading
  git or a forge, and computing a version.
- **Composed into:** [[container-release-cli]] — consumes
  [[component-commit-collector]] records, feeds
  [[component-version-calculator]]; see it for relationships.
- **Source & tests:** `cli/release/internal/bump`.

## Interface

- **Surface:** given commit records and an enabled signal set, return the
  computed impact together with the records that determined it.
- **Guarantees:**
  - The component SHALL return the highest impact indicated by any enabled
    signal across all records.
  - The component SHALL report every record that contributed to the result.
  - WHEN no record carries any enabled signal, the component SHALL return
    an impact of none.
  - The component SHALL be a pure function of its inputs.
  - The component SHALL NOT read a language model's output.
  - The component SHALL NOT require any signal that lives outside git.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — deterministic bump computation.

## Change Guidance

WHEN a signal is added, its precedence against every existing signal SHALL
be specified in the same change.

**Rationale:** Impact is an aggregate over signals. A signal added without
stated precedence makes the result depend on commit order, which is not
reproducible in any useful sense.

---

The component SHALL NOT accept a language model's proposal as an input.

**Rationale:** [[adr-003]] requires the computation and the proposal to be
independent so that disagreement is detectable. A classifier that read the
proposal could only ever agree with it.

- **Verification gates:** the component's unit suite, which is table-driven
  over signal combinations and precedence cases.

## Key Decisions

- [[adr-003]] — the bump is computed deterministically and a model's
  proposal is checked against it.
- [[adr-005]] — forge labels are not the trigger, so the default signals
  are the ones carried in git.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[frd-version-computation]] — the feature this component serves
