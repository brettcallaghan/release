---
title: "Component Blueprint: Bump Classifier"
entity_name: Bump Classifier
description: Release impact computed deterministically from a named bump or
  the reviewed signals commits carry, with what drove the result and the
  commits that carry no signal reported alongside it.
slug: component-bump-classifier
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/bump
test_paths:
  - internal/bump
---

# Bump Classifier

## Overview

Resolves one of major, minor, patch or none from reviewed records in git,
reports what drove the answer, and reports which commits carried no signal
at all. It is the deterministic half of [[adr-003]]: its answer is
reproducible from a clone. It is not the last word. A commit with no signal
is invisible to it, so [[component-impact-reconciler]] weighs its answer
against the producer's proposal and decides.

Sources:

1. **A bump named by the maintainer** — `release cut minor`. Explicit beats
   inferred, always.
2. **A notes block** — the `Release-Type:` line of a commit body's
   `## Release notes` block, reviewed with the code in its pull request and
   parsed by [[component-notes-codec]].
3. **A release-type trailer** — `Release-Type: major|minor|patch|none` in the
   commit's trailers.
4. **Conventional commits** — a type in the commit subject (`feat`, `fix` and
   the rest), with `!` before the colon marking a breaking change.

Sources 2 to 4 all read material that lives in git and was reviewed before it
got there, which is what keeps the computation reproducible from a clone.
None is required: the adopting repositories do not use conventional commits
consistently ([[adr-004]]), and a commit with no signal is reported rather
than refused.

Reporting what drove the answer is not a convenience. A version a maintainer
disagrees with is otherwise undiagnosable, and the usual cause — one block
marked major, or one mis-typed commit — is invisible in the result alone.

- **Role:** the computed release impact, its justification, and the
  commits it could not see into.
- **Scope:** in — signal detection, precedence, aggregation, the
  unclassified list; out — reading git or a hosting platform, parsing a
  notes block, authoring, reconciling with a proposal, and computing a
  version.
- **Composed into:** [[module-release-core]] — consumes
  [[component-commit-collector]] records and [[component-notes-codec]]
  output, feeds [[component-impact-reconciler]] and
  [[component-evidence-builder]]; see it for relationships.
- **Source & tests:** `internal/bump`.

## Interface

- **Surface:** given an optional named bump, commit records with each
  commit's parsed notes block (or none), and an enabled signal set, return
  the computed impact, the commits that determined it with the signal each
  indicated, the signal each commit carries, and the commits that carry no
  enabled signal.
- **Guarantees:**
  - WHEN the maintainer names a bump, the component SHALL return it.
  - WHEN the maintainer names a bump, the component SHALL report that the
    impact was named rather than inferred, and SHALL still report the
    impact the commits indicate.
  - The component SHALL return the highest impact indicated by any enabled
    signal across all commits.
  - The component SHALL report every commit that contributed to the result.
  - The component SHALL report every commit that carries no enabled signal.
  - WHEN no commit carries any enabled signal, the component SHALL return an
    impact of none.
  - The component SHALL be a pure function of its inputs.
  - The component SHALL NOT require any signal that lives outside git.
  - The component SHALL NOT read an authored corpus or a proposal.
- **Signal mappings:** the enabled signal set names any of `notes-block`,
  `release-type`, `conventional-type` and `breaking-marker`.
  - `notes-block`: a commit body's parsed `## Release notes` block indicates
    its `Release-Type:` impact. A block that does not parse is no signal.
  - `release-type`: a `Release-Type` trailer of `major`, `minor`, `patch`
    or `none`, key and value case-insensitive, indicates that impact.
  - `conventional-type`: a subject type of `feat` indicates minor; `fix`,
    `perf` and `refactor` indicate patch; any other type is no signal.
  - `breaking-marker`: a `!` immediately before the subject's colon, or a
    line in the body's final paragraph beginning `BREAKING CHANGE:` or
    `BREAKING-CHANGE:`, indicates major. The footer is read from the body
    because git's trailer rules reject a key containing a space.
  - A `none` from `notes-block` or `release-type` is a signal: the commit is
    classified, as having no impact. It never lowers what another signal
    indicates, on the same commit or another.
  - WHERE one commit indicates the winning impact through several signals,
    it is reported once, preferring `breaking-marker`, then `notes-block`,
    then `release-type`, then `conventional-type`.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — deterministic computation from reviewed
  signals.

## Change Guidance

WHEN a signal is added, its precedence against every existing signal SHALL
be specified in the same change.

**Rationale:** Impact is an aggregate over signals. A signal added without
stated precedence makes the result depend on commit order, which is not
reproducible in any useful sense.

---

The component SHALL NOT read the `proposed_bump` field of an authored
changelog, or any other output of a producer.

**Rationale:** [[adr-003]] keeps the computation and the proposal
independent so that [[component-impact-reconciler]] can see when they
differ. A classifier that read the proposal would agree with it by
construction.

---

The component SHALL NOT treat a commit with no signal as an error.

**Rationale:** The adopting repositories do not use conventional commits
consistently, and direct pushes carry no notes block. Such a commit is
reported, and the reconciler asks a person about it; refusing here would
stop the release before the producer had read its diff.

- **Verification gates:** the component's unit suite, which is table-driven
  over signal combinations and precedence cases, including a named bump that
  contradicts every inferred signal and a range in which some commits carry
  no signal.

## Key Decisions

- [[adr-003]] — the impact is computed from reviewed records in git, and a
  person confirms anything unreviewed.
- [[adr-004]] — no per-commit signal is required.
- [[adr-005]] — hosting-platform labels are not the trigger, so every signal
  is one carried in git or named at the command line.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-impact-reconciler]] — the component that decides the impact
- [[frd-version-computation]] — the feature this component serves
