---
title: Release Tooling Standards
description: What the release tooling guarantees and validates — stage
  branch and release commit shape, pre-publication verification, version
  computation, and its configuration surface.
slug: release-tooling-standards
type: standards
status: approved
updated_at: 2026-10-07T00:00:00Z
version: "0.2"
---

# Release Tooling Standards

These requirements govern the release tooling itself: what it computes,
what it refuses, and what it validates. They are enforceable by the tool
and testable without a repository in most cases.

They say nothing about how a project should organise its branches, how
long it supports a major, or how it distributes artifacts. Those are
choices a project makes, and they live in
[[repository-release-policy]] and [[cli-distribution-policy]].

**The tooling is branch-agnostic.** It accepts a target branch and does
not interpret its name. Any branch may be a release line, and which
branches a project uses for which majors is policy the tool neither knows
nor enforces.

Requirements carry stable identifiers so that tests, blueprints and reviews
can cite them individually. An identifier is never renumbered once assigned,
and a retired identifier is never reused: nothing validates a requirement
citation, so renumbering breaks references silently.

---

## Stage Branches

**RTS-001:** A stage branch SHALL carry the branch-name prefix configured
for the repository.

---

**RTS-002:** The release tooling SHALL use `cut/` as the default stage
prefix.

**Rationale:** "Cut a release" is what the operation is called, and the
command reads as `release cut minor`. The specifications keep the term *stage
branch* for the concept regardless of what a project names the prefix.

---

**RTS-003:** A stage branch name SHALL contain the version it releases,
without a tag prefix — for example `cut/1.2.3`.

**Rationale:** The name is cross-checked against the commit contents
before publication, and it makes a duplicate release attempt visible as a
branch collision rather than as a tag collision after the fact.

---

**RTS-004:** A stage branch SHALL be cut from the tip of the branch it
targets.

---

**RTS-005:** The first commit on a stage branch SHALL be the release commit.

---

**RTS-006:** A stage branch MAY carry further commits that modify only
declared release paths.

**Rationale:** A maintainer correcting the notes during review adds a commit,
and treating that as malformed would make the review surface unusable for the
one thing it exists for. What must hold is the path constraint, not the commit
count.

---

**RTS-007:** A release commit SHALL modify only version manifests and the
changelog.

---

**RTS-008:** A release commit SHALL NOT modify source code.

---

**RTS-009:** The release tooling SHALL reject a configured stage prefix
that the repository's release-branch pattern also matches.

**Rationale:** Branch protection rules and workflow filters are written as
globs, and a pattern matching both would apply release-branch protection to
stage branches or fire a release from a protected-branch push.

---

## Version Computation

**RTS-010:** The release tooling SHALL compute the base version from the
latest release tag reachable from the target branch.

**Rationale:** Reachability already separates release lines — a branch cut
before a later tag existed cannot reach it — so no branch-name convention
and no per-major query is needed.

---

**RTS-011:** The release tooling SHALL compute the version bump
deterministically from commit metadata.

---

**RTS-012:** The release tooling SHALL NOT require a bump signal that lives
outside the repository's git history.

**Rationale:** A version must be derivable from a clone, so that it is
reproducible offline and checkable by a reviewer who has nothing else.

---

**RTS-013:** The release tooling SHALL NOT derive a version number from a
language model's output.

---

**RTS-014:** WHEN an authored proposal disagrees with the computed bump,
the release tooling SHALL halt and require human confirmation.

**Rationale:** Disagreement usually means a change was mis-typed or
mis-trailered, which is the failure the deterministic classifier cannot
detect on its own ([[adr-003]]).

---

**RTS-015:** The release tooling SHALL increment exactly one version
component by one unless leaps are enabled by configuration.

---

**RTS-016:** WHERE leaps are enabled by configuration, the release tooling
SHALL accept a version that increments a component by more than one.

---

**RTS-017:** WHILE the base version is below 1.0.0, the release tooling
SHALL map a bump of major to a minor increment by default.

**Rationale:** Semantic versioning leaves pre-1.0 increments under-specified.
Remapping the major bump keeps breaking changes inside the 0.x series rather
than reaching 1.0.0 by accident. Only the major bump is remapped: a minor
bump increments the minor and a patch bump increments the patch, below 1.0.0
exactly as above it.

---

**RTS-018:** WHERE a project configures the pre-1.0 remapping off, the
release tooling SHALL map a bump of major to a major increment below
1.0.0.

---

**RTS-019:** The release tooling SHALL require an explicit graduation to
produce version 1.0.0.

---

**RTS-020:** WHERE no releasable change exists since the latest reachable
tag, the release tooling SHALL refuse to create a stage branch.

---

## Prereleases and Canaries

**RTS-021:** The release tooling SHALL derive a prerelease identifier for a
planned version on request.

---

**RTS-022:** A prerelease version SHALL order before the version it
previews.

---

**RTS-023:** A canary identifier SHALL NOT be a prerelease of a version the
project plans to publish.

**Rationale:** A canary is an artifact for an arbitrary commit, not a
preview of a planned release. Giving it a planned version's prerelease
identifier implies a graduation path it does not have.

---

**RTS-024:** The release tooling SHALL derive a canary identifier that is
unique to the commit it describes.

---

## Configuration

**RTS-025:** The release tooling SHALL express repository-specific
behaviour as configuration rather than as repository-specific code paths.

---

**RTS-026:** The release tooling SHALL accept a configured command for
validating a release.

---

**RTS-027:** The release tooling SHALL accept a configured command for
propagating a version across a repository's manifests.

---

**RTS-028:** The release tooling SHALL accept configured preflight
commands.

---

**RTS-029:** The release tooling SHALL accept a configured stage-branch
prefix.

---

**RTS-030:** The release tooling SHALL NOT interpret the name of a target
branch.

**Rationale:** Any branch may be a release line. A tool that reads meaning
into a branch name imposes a topology its adopters did not choose.

---

**RTS-031:** WHEN no release tag is reachable from the target branch, the
release tooling SHALL treat the base version as 0.0.0.

**Rationale:** A project's first release is otherwise a special case running
through a different code path than every subsequent one. Treating the absent
tag as 0.0.0 makes the first release ordinary: a patch bump produces 0.0.1, a
minor bump 0.1.0, and an explicit graduation 1.0.0.

---

**RTS-032:** The release tooling SHALL accept a configured tag prefix,
defaulting to `v`.

**Rationale:** `v1.2.3` is the prevailing convention and the one the adopting
repositories use, but it is a convention rather than part of the version, so
a project that tags bare versions is not excluded.

---

**RTS-033:** The release tooling SHALL accept a configured list of paths a
release commit may modify.

**Rationale:** Which files carry a version differs by ecosystem and by
repository, so the tooling cannot know them. Declaring them makes the release
commit's scope checkable without the tooling understanding any manifest
format.

---

## References

- [[adr-001]] — the default branch is the default release line, and
  branching topology is policy rather than tooling
- [[adr-002]] — stage branches as the release trigger, verified
  independently of the branch name
- [[adr-003]] — deterministic bump computation with authored prose
- [[repository-release-policy]] — the branching, support and readiness
  policy a project chooses
- [[cli-distribution-policy]] — artifact publication and distribution
  policy
- [[container-release-cli]] — the tooling these requirements govern
