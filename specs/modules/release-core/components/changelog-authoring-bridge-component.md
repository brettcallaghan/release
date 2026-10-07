---
title: "Component Blueprint: Changelog Authoring Bridge"
description: The seam across which a selected producer receives a changelog
  corpus and returns an authored one, with the returned corpus validated
  against the schema and its proposed impact checked.
slug: component-changelog-authoring-bridge
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/authoring
test_paths:
  - internal/authoring
---

# Changelog Authoring Bridge

## Overview

The boundary between the module and whoever writes the prose. It selects a
producer, hands it the assembled corpus, takes an authored corpus back,
**validates that structured representation against the schema**, and compares
its `proposed_bump` against the computed impact.

Validation is the bridge's main job. Everything downstream — the renderer, the
changelog writer, the staging sequence — assumes a conforming corpus, so this
is the one place a malformed one is caught before reaching something that
would render it partially.

Producers are plugins rather than a fixed set: a language model, a configured
command, a conventional-commit parser, or a person editing a file.
[[component-changelog-producer]] governs what every producer must do, the same
way [[component-changelog-corpus-builder]] governs its adapters. The bridge
tells them apart only by name.

The default producer runs outside the process. A language model is available
on the workstation where a maintainer stages a release, and putting the call
inside the tooling would oblige every adopter to hold an API credential and a
per-call budget for something they already have. That ignorance is the
component's value: it makes authoring replaceable and keeps the tooling free
of model configuration.

Authoring is slow, and for a model producer it is paid for, so the bridge
caches what comes back. Re-staging after a correction reuses the authored
corpus rather than paying for it twice.

- **Role:** producer selection, the exchange, validation, and caching.
- **Scope:** in — selecting a producer, exchanging a corpus, schema
  validation, impact comparison, caching the result; out — calling a model
  directly, and composing prose.
- **Composed into:** [[module-release-core]] — between
  [[component-changelog-corpus-builder]] and
  [[component-changelog-renderer]]; see the module for relationships.
- **Source & tests:** `internal/authoring`.

## Interface

- **Surface:** select a producer, exchange a corpus with it, validate the
  result, cache it, and report agreement or disagreement with a computed
  impact.
- **Guarantees:**
  - The component SHALL validate a returned corpus against the schema
    [[contract-changelog]] names before any consumer reads it.
  - IF a returned corpus declares a `schema_version` the component does not
    implement, THEN the component SHALL reject it.
  - WHEN the returned `proposed_bump` differs from the computed impact, the
    component SHALL report a disagreement.
  - The component SHALL NOT resolve a disagreement between the returned
    `proposed_bump` and the computed impact.
  - The component SHALL preserve returned prose byte-for-byte.
  - The component SHALL cache an authored corpus keyed by the commit range it
    was authored from.
  - WHEN a cached corpus exists for the range being staged, the component
    SHALL reuse it rather than invoking a producer.
  - The component SHALL allow the maintainer to discard a cached corpus and
    author again.
  - WHEN the maintainer requests staging without notes, the component SHALL
    be skipped entirely.
  - The component SHALL NOT call a language model itself.
  - The component SHALL NOT require network access of its own.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the halt-on-disagreement rule.
- [[contract-changelog]] — the corpus semantics it validates against.

## Change Guidance

The component SHALL NOT acquire a language-model client, an API credential,
or a network dependency.

**Rationale:** Authoring runs where a model already exists. Moving the call
inside the module would impose a credential and a budget on every adopter,
which is the cost this design exists to avoid.

---

The component SHALL NOT edit returned prose, including normalising whitespace
or rewrapping lines.

**Rationale:** Prose the tooling rewrites is prose nobody reviewed. The stage
pull request is the review surface, and it must show exactly what will be
published.

---

WHEN a producer is added, it SHALL conform to
[[component-changelog-producer]] rather than being special-cased here.

**Rationale:** A producer the bridge knows by more than its name is a producer
nothing else can replace.

---

The component SHALL NOT invalidate a cached corpus on anything but a change to
the commit range or an explicit request.

**Rationale:** A cache that clears on an unrelated change makes every re-run
of a failed staging cost again, which is the expense it exists to prevent.

---

- **Verification gates:** the component's unit suite, including rejection of
  an unknown schema version, the disagreement path, a byte-identical round
  trip of prose containing markdown and non-ASCII text, and a cache hit that
  does not invoke a producer.

## Key Decisions

- [[adr-003]] — a proposal is compared, never trusted.
- [[adr-005]] — changelog authoring is a replaceable step rather than a render
  template.

## Open Questions

- [ ] Whether the cache lives inside the repository or outside it. Inside, it
  survives a clean checkout and risks being committed; outside, it is
  invisible to a reviewer wondering why re-staging was instant.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-producer]] — what every producer must do
- [[contract-changelog]] — the corpus exchanged across this seam
- [[frd-changelog-authoring]] — the feature this component serves
