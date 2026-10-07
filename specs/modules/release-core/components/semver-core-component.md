---
title: "Component Blueprint: Semver Core"
description: Version parsing and comparison, and every branch, tag and
  prerelease name derived from a version.
slug: component-semver-core
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - internal/semver
test_paths:
  - internal/semver
---

# Semver Core

## Overview

The vocabulary type every other component in the container speaks. It
parses version strings, orders them, and derives every name a release
needs from one — the tag, the stage branch, a canary identifier, a
prerelease identifier.

Naming lives here rather than being spread across the components that need
names, because a tag and the stage branch that produces it must agree by
construction. A project's required checks cross-check a branch name against
the version recorded in its commit, and that check is only meaningful if one
component generated both forms.

- **Role:** the version type, its ordering, and its derived names.
- **Scope:** in — parsing, comparison, formatting, name derivation; out —
  deciding *which* version comes next, which is
  [[component-version-calculator]].
- **Composed into:** [[module-release-core]] — imported by nearly every
  other component; see it for this component's relationships.
- **Source & tests:** `internal/semver`.

## Interface

- **Surface:** a `Version` type with parse, compare and format operations,
  plus name derivation for tags, stage branches, canary and prerelease
  identifiers.
- **Guarantees:**
  - The component SHALL parse a version string and format it back to the
    identical string.
  - The component SHALL order versions by precedence as defined by semantic
    versioning, including prerelease precedence.
  - The component SHALL derive a tag name and a stage branch name from one
    version such that each can be parsed back to that version.
  - The component SHALL NOT accept a version string carrying a tag prefix;
    callers strip the prefix before parsing.
  - The component SHALL NOT read git, the filesystem, or the network.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the naming forms a release uses.

## Change Guidance

WHEN a name form changes, both its derivation and its inverse parse SHALL
be updated in the same change.

**Rationale:** The verifier's cross-check depends on the two being
inverses. A one-sided change passes its own tests and breaks verification.

---

This component SHALL NOT depend on any other component in the container.

- **Verification gates:** the component's own unit suite, including the
  round-trip property tests for every derived name form.

## Key Decisions

- [[adr-002]] — the stage branch name is cross-checked against a recorded
  version, which is why derivation and its inverse live together.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-version-computation]] — the feature this component serves
