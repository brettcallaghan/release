---
title: "Component Blueprint: Changelog Authoring Bridge"
description: The seam across which an external author receives a changelog
  corpus and returns an authored one, with the returned corpus validated
  and its proposed impact checked.
slug: component-changelog-authoring-bridge
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed:
  - contract-changelog
source_paths:
  - cli/release/internal/authoring
test_paths:
  - cli/release/internal/authoring
---

# Changelog Authoring Bridge

## Overview

The boundary between the container and whoever writes the prose. It writes
a corpus out, waits for an authored corpus back, validates it, and compares
its `proposedBump` against the computed impact.

The author is deliberately outside the container. A language model is
available on the workstation where a maintainer stages a release, and
putting the call inside the tool would oblige every adopter to hold an API
credential and a per-call budget for something they already have. So the
bridge exchanges files and knows nothing about who filled them in — a
skill driving a model, a maintainer in an editor, or a script.

That ignorance is the component's value. It makes authoring replaceable,
and it is what keeps the tool free of model configuration.

- **Role:** the exchange with the author, and the validation of what comes
  back.
- **Scope:** in — corpus exchange, schema validation, impact comparison;
  out — calling a model, and composing prose.
- **Composed into:** [[container-release-cli]] — between
  [[component-changelog-corpus-builder]] and
  [[component-changelog-renderer]]; see it for relationships.
- **Source & tests:** `cli/release/internal/authoring`.

## Interface

- **Surface:** write a corpus to a path, read an authored corpus from a
  path, validate it, and report agreement or disagreement with a computed
  impact.
- **Guarantees:**
  - The component SHALL validate a returned corpus against the schema
    [[contract-changelog]] names before any consumer reads it.
  - IF a returned corpus declares a `schemaVersion` the component does not
    implement, THEN the component SHALL reject it.
  - WHEN the returned `proposedBump` differs from the computed impact, the
    component SHALL report a disagreement.
  - The component SHALL NOT resolve a disagreement between the returned
    `proposedBump` and the computed impact.
  - The component SHALL preserve returned prose byte-for-byte.
  - The component SHALL NOT call a language model.
  - The component SHALL NOT require network access.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the halt-on-disagreement rule.

## Change Guidance

The component SHALL NOT acquire a language-model client, an API
credential, or a network dependency.

**Rationale:** Authoring runs where a model already exists. Moving the call
inside the container would impose a credential and a budget on every
adopter, which is the cost this design exists to avoid.

---

The component SHALL NOT edit returned prose, including normalising
whitespace or rewrapping lines.

**Rationale:** Prose that the tool rewrites is prose nobody reviewed. The
stage pull request is the review surface, and it must show exactly what
will be published.

- **Verification gates:** the component's unit suite, including rejection
  of an unknown schema version, the disagreement path, and a byte-identical
  round trip of prose containing markdown and non-ASCII text.

## Key Decisions

- [[adr-003]] — a proposal is compared, never trusted.
- [[adr-005]] — changelog authoring is a replaceable step rather than a
  render template.

## Open Questions

- [ ] Whether the bridge should invoke the author as a subprocess, or only
  exchange files and let the caller orchestrate. Affects whether staging
  can run unattended. Decided with the autorelease question in
  [[container-release-cli]].

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[contract-changelog]] — the corpus exchanged across this seam
- [[frd-changelog-authoring]] — the feature this component serves
