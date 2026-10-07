---
title: "Component Blueprint: Pull Request Metadata Collector"
entity_name: Pull Request Metadata Collector
description: "Not part of the MVP. Pull-request metadata fetched from the
  hosting platform for one pull request or a list — title, description,
  author, labels and branch."
slug: component-pr-metadata-collector
type: component-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/platform
test_paths:
  - internal/platform
---

# Pull Request Metadata Collector

## Overview

Fetches what the hosting platform knows about a pull request: title,
description, author, labels and head branch. It takes one pull-request number
or a list of them and returns one record each, in the shape
`schemas/collection.strata` defines.

That is the whole of it. It does not decide which pull requests matter, does
not merge its output into anything, and is not a layer over
[[component-commit-collector]] — the two are independent collectors that a
caller may use together or separately. **The list of pull requests comes from
[[component-commit-collector]]**, which extracts the pull-request number the
hosting platform appends to each squash commit subject, so identifying the
pull requests merged into a branch since its last release needs no platform
call at all.

**Deferred beyond the first milestone.** Every repository adopting this
tooling squash-merges with the pull-request description as the commit body, so
title, description and author already reach the release through git. This
component earns its place when a repository does not do that, or when labels
become something a project wants read.

- **Role:** pull-request metadata the hosting platform holds.
- **Scope:** in — fetching the declared fields for a given pull request or
  list; out — deciding which pull requests to fetch, merging records, and any
  write to the platform.
- **Composed into:** [[module-release-core]] — a sibling of
  [[component-commit-collector]], not a layer over it; see the module for
  relationships.
- **Source & tests:** `internal/platform`.

## Interface

- **Surface:** given one pull-request number or a list, return a
  `PullRequestRecord` for each, as defined by `schemas/collection.strata`.
- **Guarantees:**
  - The component SHALL populate only the fields
    `schemas/collection.strata` declares.
  - The component SHALL batch a list into as few requests as the platform
    permits.
  - IF the hosting platform is unreachable or unauthenticated, THEN the
    component SHALL report the failure without partially populating a record.
  - WHEN a requested pull request does not exist, the component SHALL report
    its absence distinctly from a failure.
  - The component SHALL NOT write to the hosting platform.
  - The component SHALL NOT determine which pull requests to fetch.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the metadata a release reads.

## Change Guidance

The component SHALL NOT become a prerequisite of version computation.

**Rationale:** A version must be derivable from a clone, so that it is
reproducible offline and verifiable by a reviewer who has only the
repository. Routing any part of the bump signal through the hosting platform
would forfeit both.

---

WHEN support for an additional hosting platform is added, the fetch surface
SHALL be expressed behind one interface so no caller learns which platform
answered.

---

WHEN a field is added to `PullRequestRecord`, something in the release process
SHALL consume it in the same change.

**Rationale:** The record is deliberately limited to what a release uses.
Fields collected speculatively become fields nobody can safely remove.

- **Verification gates:** the component's unit suite against recorded platform
  responses, including the unreachable, unauthenticated and absent-pull-request
  paths.

## Key Decisions

- [[adr-005]] — hosting-platform labels are not the release trigger, which is
  why this component reads descriptive metadata and not control signals.

## Open Questions

- [ ] Whether labels should be readable as an optional bump signal for
  adopters who want them, given that they are rejected as the trigger.
  Decided with the autorelease question in [[container-release-cli]].

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-commit-collector]] — the component that identifies which pull
  requests a release covers
- [[frd-changelog-authoring]] — the feature this component serves
