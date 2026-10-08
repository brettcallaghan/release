---
title: "Component Blueprint: Version Calculator"
entity_name: Version Calculator
description: The next version for a release line, carrying the version
  rules the tooling enforces — the leap guard, pre-1.0 semantics, and
  prerelease derivation.
slug: component-version-calculator
type: component-blueprint
status: approved
updated_at: 2026-10-08T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/version
test_paths:
  - internal/version
---

# Version Calculator

## Overview

Takes a base version, a computed impact and a policy, and returns the next
version. Every version rule the project has decided lives here and nowhere
else.

Three of them are easy to get wrong and expensive to get wrong. A version
may increment a component by exactly one unless leaps are explicitly
enabled. Below 1.0.0 one mapping shifts and only one: a major bump
increments the minor component, so breaking changes stay inside the 0.x
series. Minor and patch bumps behave below 1.0.0 exactly as above it, and
reaching 1.0.0 requires an explicit graduation rather than falling out of a
bump.

It carries no rule about which branch it is computing for. Whether a
branch may originate a major is the adopting project's policy, not
something the tooling knows ([[adr-001]]).

This is the most correctness-critical component in the container, and it
is a pure function, so it can be tested exhaustively without a repository.

- **Role:** the next version, under policy.
- **Scope:** in — applying impact to a base version under policy
  constraints; out — reading tags, classifying impact, and writing
  anything.
- **Composed into:** [[module-release-core]] — consumes
  [[component-tag-reader]] and [[component-bump-classifier]]; see it for
  relationships.
- **Source & tests:** `internal/version`.

## Interface

- **Surface:** given a base version, a computed impact and a policy,
  return the next version or a policy violation.
- **Guarantees:**
  - From a base version, the component SHALL increment exactly one version
    component by one unless leaps are enabled by policy.
  - WHILE the base version is below 1.0.0, the component SHALL map an impact
    of major to a minor increment by default.
  - WHILE the base version is below 1.0.0, the component SHALL map an impact
    of minor to a minor increment and an impact of patch to a patch
    increment.
  - WHEN no base version is supplied, the impact is not none and graduation
    is not requested, the component SHALL return 0.0.0 (RTS-031), and SHALL
    accept only 0.0.0 as a proposed version.
  - The component SHALL require an explicit graduation to produce 1.0.0.
  - WHEN graduation is requested and the base is below 1.0.0, the component
    SHALL return 1.0.0 whatever the impact.
  - IF graduation is requested and the base is 1.0.0 or above, THEN the
    component SHALL return a policy violation.
  - IF the base is below 1.0.0, the impact is major, no graduation is
    requested and the pre-1.0 remapping is off, THEN the component SHALL
    return a policy violation rather than 1.0.0.
  - From a base version, the computed next version SHALL always increment
    exactly one component by one; the leap policy affects only whether a proposed version is
    accepted.
  - The component SHALL check a proposed version against a base under the
    policy, and SHALL return a policy violation WHEN the proposal increments
    any component by more than one while leaps are off, does not follow the
    base, leaves a component after the incremented one non-zero, or reaches
    1.0.0 or above from a base below it without graduation.
  - The component SHALL derive a prerelease identifier for a planned
    version on request.
  - A prerelease version the component returns SHALL order before the
    version it previews.
  - WHEN the impact is none, the component SHALL report that no release is
    warranted rather than returning a version. This is the path taken when a
    maintainer names no bump and the producer concludes nothing releasable
    landed.
  - The component SHALL be a pure function of its inputs.
  - The component SHALL NOT derive any constraint from the name of the
    branch it is computing for.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — version computation, the leap guard,
  pre-1.0 semantics, and prerelease derivation.

## Change Guidance

WHEN a version-policy rule changes, this component and the acceptance
criteria in [[frd-version-computation]] SHALL be updated in the same
change.

---

The component SHALL express every policy rule as a function of its
declared inputs, with no rule conditioned on the repository it is running
in.

**Rationale:** A rule that reads its environment cannot be tested
exhaustively, and this is the component where exhaustive testing is the
whole defence.

---

The component SHALL NOT widen a policy violation into a warning.

**Rationale:** A forbidden version that is emitted with a warning gets
published. The violation is the result.

- **Verification gates:** the component's unit suite, which is exhaustive
  over the cross product of impact, base-version regime (pre-1.0 and
  post-1.0), and leap policy.

## Key Decisions

- [[adr-001]] — the tooling is branch-agnostic, which is why no rule here
  is conditioned on a branch.
- [[adr-003]] — the impact this component applies is computed, never
  proposed.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-version-computation]] — the feature this component serves
