---
title: "Component Blueprint: Tag Reader"
entity_name: Tag Reader
description: Release tags reachable from a ref, parsed to versions and
  ordered, so the base version of a release line is a git query.
slug: component-tag-reader
type: component-blueprint
status: approved
kind: standard
contracts_consumed: []
source_paths:
  - internal/gitread
test_paths:
  - internal/gitread
---

# Tag Reader

## Overview

Answers one question: what is the latest release already published on this
branch's line? It lists tags **reachable** from a ref, discards those that
do not parse as release tags, and returns the highest.

Reachability is what lets the tooling stay branch-agnostic. A branch cut
before a later tag existed cannot reach it, so a query scoped to a branch is
already scoped to that branch's line — mechanically, without the tool
knowing anything about which branches carry which majors ([[adr-001]]).

- **Role:** the base version for a release line.
- **Scope:** in — reachable tag enumeration, parsing, ordering; out —
  creating tags, and deciding the next version.
- **Composed into:** [[module-release-core]] — consumed by
  [[component-version-calculator]]; see it for relationships.
- **Source & tests:** `internal/gitread`.

## Interface

- **Surface:** latest reachable release version for a ref; the full ordered
  list of reachable release versions.
- **Guarantees:**
  - The component SHALL consider only tags reachable from the named ref.
  - The component SHALL consider a tag a release tag WHEN its name is the
    configured tag prefix followed by a version, or is a bare version, and
    that version carries no prerelease and no build metadata.
  - WHEN two reachable release tags carry the same version, the component
    SHALL return the one whose name carries the configured prefix.
  - The component SHALL return, with the highest version, the name of the
    tag it was read from.
  - The component SHALL ignore a tag that does not parse as a release tag
    rather than failing.
  - WHEN no release tag is reachable, the component SHALL report the absence
    distinctly from an error.
  - The component SHALL NOT infer a major version from a branch name.
  - The component SHALL NOT require network access.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — version computation from reachable tags.

## Change Guidance

The component SHALL NOT filter tags by major version.

**Rationale:** Reachability already scopes the query. A major filter would
make the answer depend on a branch-name convention, which is exactly the
coupling [[adr-001]] removes.

---

WHEN back-porting behaviour is specified, this component's reachability
assumption SHALL be re-examined: merging one release line into another makes
its tags reachable from the target and silently changes every answer this
component gives. This is why [[repository-release-policy]] requires
cherry-picks rather than merges between lines.

- **Verification gates:** the component's unit suite against fixture
  repositories, including the no-tags case and a branched topology where a
  later major's tags are unreachable.

## Key Decisions

- [[adr-001]] — the tooling is branch-agnostic; reachability distinguishes
  release lines without it.
- [[adr-006]] — git is reached by running the `git` binary, so
  reachability is exactly what `git for-each-ref --merged` reports.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-version-computation]] — the feature this component serves
