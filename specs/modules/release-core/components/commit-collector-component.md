---
title: "Component Blueprint: Commit Collector"
entity_name: Commit Collector
description: Commits between two refs, with pull-request identity, trailers,
  descriptions and changed-file lists parsed out of the commits themselves.
slug: component-commit-collector
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/gitread
test_paths:
  - internal/gitread
---

# Commit Collector

## Overview

Reads the commits in a range and extracts everything a release needs from them
without touching a hosting platform. For squash-merged repositories that is
more than it sounds: a squash commit subject carries the pull-request title and
its number, and where the repository is configured to use the pull-request
description as the squash body, the description is in git too.

So this component — not [[component-pr-metadata-collector]] — is the
primary source of pull-request titles and descriptions, and with them of the
reviewed `## Release notes` blocks those descriptions carry. The hosting
platform collector exists for what is missing, not for what is normal.

Each record also lists the files its commit changed, with line counts. The
list is cheap and always collected; it tells a producer where a change
landed even when its diff was cut to fit the budget or never read.

- **Role:** the record set every downstream component reads, and the
  component that identifies which pull requests a release covers.
- **Scope:** in — commit enumeration, subject and body parsing,
  pull-request number extraction, trailer parsing, the changed-file list;
  out — reading diffs, parsing a notes block, and fetching anything from a
  hosting platform.
- **Composed into:** [[module-release-core]] — feeds
  [[component-notes-codec]], [[component-bump-classifier]],
  [[component-diff-collector]] and [[component-evidence-builder]]; see it
  for relationships.
- **Source & tests:** `internal/gitread`.

## Interface

- **Surface:** an ordered `CommitRecord` per commit, as defined by
  `schemas/collection.strata`, for a base and a target ref.
- **Range:** every commit reachable from the target ref and not from the
  base tag, merge commits included, newest first as `git log` lists them.
  WHEN there is no base tag, every commit reachable from the target ref.
  At pull-request time the base is the merge base of the pull request's
  base branch and its head.
- **Pull-request number:** a subject ending ` (#<n>)` yields `n`; the
  subject is left unchanged.
- **Trailers:** git's own parse of the message's final paragraph
  (`%(trailers:only,unfold)`, the `git interpret-trailers` rules), so a key
  containing a space is never a trailer. WHEN a key repeats, its last value
  is kept.
- **Changed files:** `files` lists a `FileChange` per path the commit
  changed against its first parent — `path`, `previous_path` for a rename
  or copy, `status`, and `additions` and `deletions` — from git's
  `--name-status` and `--numstat` with rename detection. A binary file,
  for which git counts no lines, has both counts 0. A merge commit's list is
  empty, as `git log` reports it.
- **Guarantees:**
  - The component SHALL extract a pull-request number from a squash commit
    subject where the hosting platform appended one.
  - The component SHALL parse trailers from the commit body as key-value
    pairs.
  - The component SHALL preserve the commit body verbatim in addition to
    anything parsed from it.
  - The component SHALL list every file a non-merge commit changed,
    including paths the diff budget excludes.
  - WHEN a commit carries no pull-request number, the component SHALL emit
    the record with the number absent rather than skipping the commit.
  - The component SHALL NOT require network access.
  - The component SHALL NOT interpret a trailer's meaning.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the commit metadata a release reads.

## Change Guidance

WHEN the record shape changes, `schemas/collection.strata` SHALL be updated
in the same change.

---

WHEN a trailer key acquires meaning, that meaning SHALL be implemented in
the component that consumes it, not here.

**Rationale:** Keeping extraction free of interpretation is what lets the
bump classifier, the notes codec, the evidence builder and future consumers
read the same records without each needing its own collector.

---

The component SHALL remain usable against a repository hosted on any
hosting platform, or none.

- **Verification gates:** the component's unit suite against fixture
  repositories covering squash commits with and without descriptions,
  merge commits, direct commits, and renamed, deleted and binary files.

## Key Decisions

- [[adr-003]] — the bump is computed from reviewed records in commits,
  which is what makes this component's output release-critical rather than
  cosmetic.

## Open Questions

None. The tooling does not inspect the repository's squash-message setting.
Squash merges carrying the pull request description are a policy of the
adopting project ([[repository-release-policy]]); a commit whose body lacks
its block is a commit with no reviewed signal, which
[[component-impact-reconciler]] handles.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-version-computation]], [[frd-changelog-authoring]] — the features
  this component serves
