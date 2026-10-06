---
title: "Component Blueprint: Pull Request Metadata Collector"
description: Pull-request titles, descriptions and authorship fetched from
  the forge for commits whose bodies do not already carry them.
slug: component-pr-metadata-collector
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - cli/release/internal/forge
test_paths:
  - cli/release/internal/forge
---

# Pull Request Metadata Collector

## Overview

Fills the gaps git cannot. For each commit carrying a pull-request number,
it fetches the title, the description and the authoring account from the
forge — and it fetches only what the commit record does not already hold.

Pull-request titles and descriptions are the cheapest high-quality input
to changelog authoring: they are written by the author, reviewed by a
reviewer, and far richer than a commit subject. This component exists so
that a repository which does not carry descriptions in its squash commits
still gets them.

It is an enricher, never a prerequisite. The release path must complete
without it.

- **Role:** pull-request metadata the commits did not carry.
- **Scope:** in — fetching titles, descriptions and accounts by
  pull-request number; out — labels as a release trigger, and any write to
  the forge.
- **Composed into:** [[container-release-cli]] — enriches the records from
  [[component-commit-collector]]; see it for relationships.
- **Source & tests:** `cli/release/internal/forge`.

## Interface

- **Surface:** given commit records, return the same records with
  pull-request title, description and account populated where available.
- **Guarantees:**
  - The component SHALL fetch only fields absent from the commit record.
  - The component SHALL NOT overwrite a field the commit record already
    carries.
  - IF the forge is unreachable or unauthenticated, THEN the component
    SHALL return the records unchanged and report the degradation.
  - The component SHALL batch requests so that cost grows with the number
    of pull requests rather than the number of commits.
  - The component SHALL NOT write to the forge.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the commit and pull-request metadata a
  release reads.

## Change Guidance

The component SHALL NOT become a prerequisite of version computation.

**Rationale:** A version must be derivable from git alone, so that it is
reproducible offline and verifiable by a reviewer who has only a clone.
Routing the bump signal through the forge would forfeit both.

---

WHEN support for an additional forge is added, the fetch surface SHALL be
expressed behind one interface so no downstream component learns which
forge answered.

- **Verification gates:** the component's unit suite against recorded forge
  responses, including the unreachable-forge and unauthenticated paths.

## Key Decisions

- [[adr-005]] — forge labels are not the release trigger, which is why this
  component reads descriptive metadata and not control signals.

## Open Questions

- [ ] Whether to read pull-request labels as an *optional* bump signal for
  adopters who want them, given that labels are rejected as the *trigger*.
  Decided with the autorelease question in [[container-release-cli]].

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[frd-changelog-authoring]] — the feature this component serves
