---
title: "Component Blueprint: Bump Classifier"
entity_name: Bump Classifier
description: The signal floor computed deterministically from the explicit
  records commits carry in git, with what drove it and the commits that carry
  no signal reported alongside it.
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

Resolves one of major, minor, patch or none from the explicit records
commits carry in git — the **signal floor** — reports what drove it, and
reports which commits carried no signal at all (the *unclassified*
commits). It is the deterministic half of [[adr-003]]: its answer is
reproducible from a clone and needs no model. It is not the last word. A
commit with no signal is invisible to it, so [[component-bump-suggester]]
raises the changelog's suggestion to this floor rather than taking the
floor as the bump.

Sources:

1. **A bump named by the maintainer** — `release cut minor`. Explicit beats
   inferred, always.
2. **A release-type trailer** — `Release-Type: major|minor|patch|none` in the
   commit's trailers.
3. **Conventional commits** — a type in the commit subject (`feat`, `fix` and
   the rest), with `!` before the colon, or a `BREAKING CHANGE:` footer,
   marking a breaking change.

Sources 2 and 3 read material a person wrote into git with the commit, which
is what keeps the computation reproducible from a clone. None is required:
the adopting repositories do not use conventional commits consistently
([[adr-004]]), and a commit with no signal is reported rather than refused.

`release version` uses only this component, with a named bump where one is
given, and reports the unclassified commits. `release cut` uses it as the
floor beneath the suggestion.

Reporting what drove the answer is not a convenience. A version a maintainer
disagrees with is otherwise undiagnosable, and the usual cause — one
trailer marked major, or one mis-typed commit — is invisible in the result
alone.

- **Role:** the signal floor, its justification, and the commits it could
  not see into.
- **Scope:** in — signal detection, precedence, aggregation, the
  unclassified list; out — reading git or a hosting platform, authoring,
  suggesting or confirming a bump, and computing a version.
- **Composed into:** [[module-release-core]] — consumes
  [[component-commit-collector]] records, feeds
  [[component-bump-suggester]] and [[component-evidence-builder]]; see it
  for relationships.
- **Source & tests:** `internal/bump`.

## Interface

- **Surface:** given an optional named bump, commit records, and an enabled
  signal set, return the signal floor, the commits that determined it with
  the signal each indicated, the signal each commit carries, and the commits
  that carry no enabled signal.
- **Guarantees:**
  - WHEN the maintainer names a bump, the component SHALL return it.
  - WHEN the maintainer names a bump, the component SHALL report that the
    impact was named rather than inferred, and SHALL still report the
    signal floor the commits indicate.
  - The component SHALL return the highest impact indicated by any enabled
    signal across all commits.
  - The component SHALL report every commit that contributed to the result.
  - The component SHALL report every commit that carries no enabled signal.
  - WHEN no commit carries any enabled signal, the component SHALL return a
    floor of none.
  - The component SHALL be a pure function of its inputs.
  - The component SHALL NOT require any signal that lives outside git.
  - The component SHALL NOT read an authored changelog, a suggestion, or
    any other output of a producer.
- **Signal mappings:** the enabled signal set names any of `release-type`,
  `conventional-type` and `breaking-marker`.
  - `release-type`: a `Release-Type` trailer of `major`, `minor`, `patch`
    or `none`, key and value case-insensitive, indicates that impact.
  - `conventional-type`: a subject type of `feat` indicates minor; `fix`,
    `perf`, `refactor` and `revert` indicate patch; any other type is no signal.
  - `breaking-marker`: a `!` immediately before the subject's colon, or a
    line in the body's final paragraph beginning `BREAKING CHANGE:` or
    `BREAKING-CHANGE:`, indicates major. The footer is read from the body
    because git's trailer rules reject a key containing a space.
  - A `none` from `release-type` is a signal: the commit is classified, as
    having no impact. It never lowers what another signal indicates, on the
    same commit or another.
  - WHERE one commit indicates the winning impact through several signals,
    it is reported once, preferring `breaking-marker`, then
    `release-type`, then `conventional-type`.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the signal floor and the unclassified
  commits.

## Change Guidance

WHEN a signal is added, its precedence against every existing signal SHALL
be specified in the same change.

**Rationale:** The floor is an aggregate over signals. A signal added without
stated precedence makes the result depend on commit order, which is not
reproducible in any useful sense.

---

The component SHALL NOT read the authored changelog, or any other output of
a producer.

**Rationale:** [[adr-003]] keeps the floor independent of the model so that
an explicit record in git cannot be outvoted by it. A classifier that read
the changelog would make the floor depend on a model's run.

---

The component SHALL NOT treat a commit with no signal as an error.

**Rationale:** The adopting repositories do not use conventional commits
consistently. Such a commit is reported, the producer reads its diff, and a
person confirms the suggestion; refusing here would stop the release before
the producer had read it.

- **Verification gates:** the component's unit suite, which is table-driven
  over signal combinations and precedence cases, including a named bump that
  contradicts every inferred signal and a range in which some commits carry
  no signal.

## Key Decisions

- [[adr-003]] — the signal floor is computed from explicit records in git,
  and the suggestion cannot go below it.
- [[adr-004]] — no per-commit signal is required.
- [[adr-005]] — hosting-platform labels are not the trigger, so every signal
  is one carried in git or named at the command line.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-bump-suggester]] — the component that raises the suggestion to
  the floor
- [[frd-version-computation]] — the feature this component serves
