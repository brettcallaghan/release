---
title: "Component Blueprint: Commit Collector"
entity_name: Commit Collector
description: Commits between two refs, with trailers, conventional-commit
  parts, the pull request number, title and description git carries, and
  changed-file lists, parsed out of the commits themselves.
slug: component-commit-collector
type: component-blueprint
status: approved
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
without touching a hosting platform. For repositories that merge through pull
requests that is more than it sounds: a squash commit's subject carries the
pull request title and its number, and where the repository uses the pull
request description as the squash body, the description is in git too. A
merge commit carries the number in its subject and the title in its body.

So this component — not [[component-pr-metadata-collector]] — is the source
of pull request titles and descriptions, which reach the producer as
evidence for the changelog. The hosting platform collector is an optional
enrichment for commits whose body lacks the description, not a dependency.

Each record also lists the files its commit changed, with line counts. The
list is cheap and always collected; it tells a producer where a change
landed even when its diff was cut to fit the budget.

- **Role:** the record set every downstream component reads, and the
  component that identifies which pull requests a release covers.
- **Scope:** in — commit enumeration, subject and body parsing,
  conventional-commit parsing, pull request number, title and description
  extraction, trailer parsing, the changed-file list; out — reading diffs,
  deciding what a trailer or type means, and fetching anything from a
  hosting platform.
- **Composed into:** [[module-release-core]] — feeds
  [[component-bump-classifier]], [[component-diff-collector]] and
  [[component-evidence-builder]]; see it for relationships.
- **Source & tests:** `internal/gitread`.

## Interface

- **Surface:** an ordered `CommitRecord` per commit, as defined by
  `schemas/collection.strata`, for a base and a target ref: hash, subject,
  body, trailers, author, author date, the conventional type, scope and
  breaking marker where the subject has them, the changed files, and the
  pull request number, title and description where git carries them.
- **Range:** every commit reachable from the target ref and not from the
  base tag, merge commits included, newest first as `git log` lists them.
  WHEN there is no base tag, every commit reachable from the target ref.
- **Conventional parts:** a subject of the form
  `<type>[(<scope>)][!]: <description>` yields its type, its scope, and
  `!` as the breaking marker; a subject of any other form yields none of
  them. The parts are recorded, not interpreted.
- **Pull request information:**
  - A squash commit, whose subject ends ` (#<n>)`, yields number `n`, the
    subject without that suffix as the title, and the body, less its final
    trailer paragraph, as the description. The subject is left unchanged.
  - A merge commit whose subject reads `Merge pull request #<n> from
    <branch>` yields number `n`, the body's first paragraph as the title,
    and the rest of the body, less its final trailer paragraph, as the
    description.
  - Any other commit carries no pull request information.
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
  - The component SHALL extract the pull request number, title and
    description from a squash commit and from a merge commit, as above.
  - The component SHALL parse trailers from the commit body as key-value
    pairs.
  - The component SHALL record a subject's conventional type, scope and
    breaking marker where it has them.
  - The component SHALL preserve the commit body verbatim in addition to
    anything parsed from it.
  - The component SHALL list every file a non-merge commit changed,
    including paths the diff budget excludes.
  - WHEN a commit carries no pull request number, the component SHALL emit
    the record with the pull request information absent rather than
    skipping the commit.
  - The component SHALL NOT require network access or call a hosting
    platform.
  - The component SHALL NOT interpret a trailer or a conventional type.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the commit metadata a release reads.

## Change Guidance

WHEN the record shape changes, `schemas/collection.strata` SHALL be updated
in the same change.

---

WHEN a trailer key or a conventional type acquires meaning, that meaning
SHALL be implemented in the component that consumes it, not here.

**Rationale:** Keeping extraction free of interpretation is what lets the
bump classifier, the evidence builder, the mechanical producer and future
consumers read the same records without each needing its own collector.

---

The component SHALL remain usable against a repository hosted on any
hosting platform, or none.

- **Verification gates:** the component's unit suite against fixture
  repositories covering squash commits with and without descriptions,
  merge commits, direct commits, conventional and non-conventional
  subjects, and renamed, deleted and binary files.

## Key Decisions

- [[adr-003]] — the changelog is authored from commit evidence in git,
  which is what makes this component's output release-critical rather than
  cosmetic.

## Open Questions

None. The tooling does not inspect the repository's squash-message setting.
Squash merges carrying the pull request description are a policy of the
adopting project ([[repository-release-policy]]); a commit whose body lacks
the description still reaches the producer with its subject, files and diff.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-version-computation]], [[frd-changelog-authoring]] — the features
  this component serves
