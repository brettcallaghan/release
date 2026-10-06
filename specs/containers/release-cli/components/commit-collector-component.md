---
title: "Component Blueprint: Commit Collector"
description: Commits between two refs, with pull-request identity, trailers
  and descriptions parsed out of the commits themselves.
slug: component-commit-collector
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - cli/release/internal/gitread
test_paths:
  - cli/release/internal/gitread
---

# Commit Collector

## Overview

Reads the commits in a range and extracts everything a release needs from
them without touching a forge. For squash-merged repositories that is more
than it sounds: a squash commit subject carries the pull-request title and
its number, and where the repository is configured to use the pull-request
description as the squash body, the description is in git too.

So this component — not [[component-pr-metadata-collector]] — is the
primary source of pull-request titles and descriptions. The forge
collector exists for what is missing, not for what is normal.

- **Role:** the record set every downstream component reads.
- **Scope:** in — commit enumeration, subject and body parsing,
  pull-request number extraction, trailer parsing; out — fetching anything
  from a forge.
- **Composed into:** [[container-release-cli]] — feeds
  [[component-bump-classifier]] and
  [[component-changelog-corpus-builder]]; see it for relationships.
- **Source & tests:** `cli/release/internal/gitread`.

## Interface

- **Surface:** an ordered record per commit carrying hash, subject, body,
  pull-request number, parsed trailers, and author.
- **Guarantees:**
  - The component SHALL extract a pull-request number from a squash commit
    subject where the forge appended one.
  - The component SHALL parse trailers from the commit body as key-value
    pairs.
  - The component SHALL preserve the commit body verbatim in addition to
    anything parsed from it.
  - WHEN a commit carries no pull-request number, the component SHALL emit
    the record with the number absent rather than skipping the commit.
  - The component SHALL NOT require network access.
  - The component SHALL NOT interpret a trailer's meaning.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the commit metadata a release reads.

## Change Guidance

WHEN a trailer key acquires meaning, that meaning SHALL be implemented in
the component that consumes it, not here.

**Rationale:** Keeping extraction free of interpretation is what lets the
bump classifier, the corpus builder and future consumers read the same
records without each needing its own collector.

---

The component SHALL remain usable against a repository hosted on any
forge, or none.

- **Verification gates:** the component's unit suite against fixture
  repositories covering squash commits with and without descriptions,
  merge commits, and direct commits.

## Key Decisions

- [[adr-003]] — the bump is computed from commit metadata, which is what
  makes this component's output release-critical rather than cosmetic.

## Open Questions

- [ ] Whether the tool should assert the repository's squash-message
  setting, or only document it, given that the setting silently decides
  whether descriptions reach git. Decided alongside the repository settings
  guide.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[frd-version-computation]], [[frd-changelog-authoring]] — the features
  this component serves
