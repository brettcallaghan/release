---
title: "Component Blueprint: Bump Classifier"
description: Release impact computed deterministically from a named bump, an
  authored changelog, or commit metadata, with whatever drove the result
  reported alongside it.
slug: component-bump-classifier
type: component-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - internal/bump
test_paths:
  - internal/bump
---

# Bump Classifier

## Overview

Resolves one of major, minor, patch or none, and reports what drove the
answer. It is the deterministic half of [[adr-003]]: the authority on release
impact, against which a producer's proposal is checked.

Sources are tried in precedence order:

1. **A bump named by the maintainer** — `release cut minor`. Explicit beats
   inferred, always.
2. **An authored changelog** — the baseline path. Each entry carries a `kind`
   and a `breaking` flag, and the mapping from those to an impact is a pure
   function: any breaking entry means major, otherwise any `FEATURE` means
   minor, otherwise patch. The entries may have been written by a language
   model; the mapping over them is arithmetic, so the result is reproducible
   from the corpus.
3. **Conventional commits** — a type in the commit subject (`feat`, `fix` and
   the rest), with `!` before the colon marking a breaking change.
4. **A release-type trailer** — `Release-Type: major|minor|patch|none` in the
   commit body, for projects wanting an explicit per-change signal without
   adopting conventional commits.

Sources 2 through 4 all read material that lives in git, which is what keeps
the computation reproducible from a clone.

Reporting what drove the answer is not a convenience. A version a maintainer
disagrees with is otherwise undiagnosable, and the usual cause — one entry
marked breaking, or one mis-typed commit — is invisible in the result alone.

- **Role:** the resolved release impact, and its justification.
- **Scope:** in — signal detection, precedence, aggregation; out — reading
  git or a hosting platform, authoring a changelog, and computing a version.
- **Composed into:** [[module-release-core]] — consumes
  [[component-commit-collector]] records, feeds
  [[component-version-calculator]]; see it for relationships.
- **Source & tests:** `internal/bump`.

## Interface

- **Surface:** given an optional named bump, an optional authored changelog,
  commit records, and an enabled signal set, return the resolved impact
  together with what determined it.
- **Guarantees:**
  - WHEN the maintainer names a bump, the component SHALL return it.
  - WHEN the maintainer names a bump, the component SHALL report that the
    impact was named rather than inferred.
  - WHEN no bump is named and an authored changelog is supplied, the component
    SHALL resolve the impact from its entries.
  - WHEN an authored changelog contains an entry marked breaking, the
    component SHALL resolve an impact of major.
  - The component SHALL return the highest impact indicated by any enabled
    signal across all inputs.
  - The component SHALL report every entry and every record that contributed
    to the result.
  - WHEN no input carries any enabled signal, the component SHALL return an
    impact of none.
  - The component SHALL be a pure function of its inputs.
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

The component SHALL NOT read the `proposed_bump` field of an authored
changelog.

**Rationale:** [[adr-003]] requires the computation and the proposal to be
independent so that disagreement is detectable. Resolving from an authored
changelog's *entries* preserves that independence — the entries are evidence
and the proposal is a conclusion — but reading the proposal itself would make
the classifier agree with it by construction.

- **Verification gates:** the component's unit suite, which is table-driven
  over signal combinations and precedence cases, including a named bump that
  contradicts every inferred signal.

## Key Decisions

- [[adr-003]] — the bump is computed deterministically and a model's
  proposal is checked against it.
- [[adr-005]] — hosting-platform labels are not the trigger, so every signal
  is one carried in git or named at the command line.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-version-computation]] — the feature this component serves
