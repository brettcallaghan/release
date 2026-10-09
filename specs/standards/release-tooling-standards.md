---
title: Release Tooling Standards
description: What the release tooling guarantees and validates — stage
  branch and release commit shape, pre-publication verification, version
  computation, bump suggestion and confirmation, and its configuration
  surface.
slug: release-tooling-standards
type: standards
status: approved
updated_at: 2026-10-09T00:00:00Z
version: "0.5"
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

**RTS-034:** A release commit SHALL carry the confirmed bump of the release it
records as a `Release-Type:` trailer.

**Rationale:** A bump confirmed by a person, or named by a maintainer, exists
nowhere else in git. Recording it on the release commit keeps the published
version recomputable and checkable from a clone without re-running a model
([[adr-003]]).

---

## Version Computation

**RTS-010:** The release tooling SHALL compute the base version from the
latest release tag reachable from the target branch.

**Rationale:** Reachability already separates release lines — a branch cut
before a later tag existed cannot reach it — so no branch-name convention
and no per-major query is needed.

---

**RTS-011:** The release tooling SHALL compute the signal floor of a range
deterministically from the explicit records its commits carry in git:
`Release-Type:` trailers, conventional-commit types and breaking-change
markers, and SHALL report the commits that carry none of them as
unclassified.

**Rationale:** Each of these records is written by a person and reaches git
with the commit, so the floor needs no model and no network to reproduce. A
commit may carry none of them; the suggested bump of RTS-014 covers such a
commit, and reporting it shows what the floor did not see.

---

**RTS-012:** The release tooling SHALL NOT read a bump signal that lives
outside the repository's git history.

**Rationale:** A version must be derivable from a clone, so that it is
reproducible offline and checkable by a reviewer who has nothing else. This
rules out pull request labels and other hosting-platform state as signals.

---

**RTS-013:** The release tooling SHALL NOT derive a version number from a
language model's output that no person has confirmed.

**Rationale:** A model's output varies between runs. A suggested bump becomes
a version only through a person's confirmation, recorded as the
`Release-Type:` trailer of RTS-034 on the release commit ([[adr-003]]).

---

**RTS-014:** The release tooling SHALL derive a suggested bump from the
entry kinds of the authored changelog — major WHEN any entry is breaking, else
minor WHEN any entry is a feature, else patch WHEN any entry is a fix, a
performance change, a refactoring or a revert, else none — raised to the signal floor
of RTS-011 WHERE the floor is higher, and SHALL show the suggestion and its
reasons to a person before it pushes a stage branch.

**Rationale:** The changelog is the model's reading of the evidence, so the
suggestion follows from what it says changed. The floor keeps an explicit
record in git from being outvoted by the model. The reasons — the entries that
drove the suggestion, and whether the floor raised it — are what let a person
confirm it, or notice that it is wrong, before the version is fixed.

---

**RTS-015:** From a base version, the release tooling SHALL increment exactly
one version component by one unless leaps are enabled by configuration. The
first release, which has no base, is 0.0.0 (RTS-031).

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

**RTS-020:** WHERE no releasable change (RTS-037) exists since the latest
reachable tag, the release tooling SHALL refuse to create a stage branch
unless the maintainer names a bump and explicitly forces the release. WHERE
the range is releasable only because an entry has no kind and the suggested
bump is none, the release tooling SHALL halt for a named bump, SHALL NOT
accept the suggestion in place of one, and SHALL NOT require a force for
it.

**Rationale:** A named bump states the impact but not that anything
changed. Staging it over a range with nothing releasable publishes a release
with nothing in it, which is usually a mistake, and occasionally intended (a
rebuild).

---

**RTS-036:** WHEN no bump is named, the release tooling SHALL halt before it
pushes a stage branch until a person confirms the suggested bump, by
answering an interactive prompt or by accepting the suggestion explicitly.
WHEN a named bump is below the signal floor, the release tooling SHALL refuse
it unless the release is forced. WHEN a named bump is below the suggested bump
but not below the signal floor, the release tooling SHALL warn and proceed.

**Rationale:** Without a confirmation the version would depend on a model's
judgement that nobody checked. Non-interactively the tooling stops with the
suggestion and its reasons, so an agent confirms after showing them to the
person it acts for. A named bump is a person's decision, so it overrides the
suggestion with a warning; overriding an explicit record in git takes a
deliberate force.

---

**RTS-037:** The release tooling SHALL treat a range as holding a releasable
change WHEN its suggested bump (RTS-014), the signal floor included, is patch
or above, or WHEN any entry of the authored changelog has no kind. A range
with no commits holds none.

**Rationale:** Many commits carry no signal at all, so the signals alone
under-report what changed; the entries' kinds say what the producer read.
An entry with no kind is a change the producer could not classify, which
only the mechanical producer writes, for a commit whose subject is not a
conventional commit: it may matter to a user, so a person names its bump.
A range of documentation and chore entries alone, or of omitted commits
alone, holds nothing to release.

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

**RTS-031:** WHEN no release tag is reachable from the target branch and a
release is warranted, the release tooling SHALL release 0.0.0, whatever the
impact, unless graduation is requested, which produces 1.0.0.

**Rationale:** A project's first tag marks where its versioned history starts,
so it carries the zero version rather than a first increment. Every later
release increments from that tag exactly as RTS-015 describes, so only the
first release differs. A project that wants to start elsewhere tags that
version by hand before its first cut.

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
- [[adr-003]] — AI-authored changelog with a suggested, confirmed bump
- [[repository-release-policy]] — the branching, support and readiness
  policy a project chooses
- [[cli-distribution-policy]] — artifact publication and distribution
  policy
- [[container-release-cli]] — the tooling these requirements govern
