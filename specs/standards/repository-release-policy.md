---
title: Repository Release Policy
description: The release choices a project makes for itself — which
  branches are release lines, how long majors are supported, how fixes
  propagate, what readiness means, how release notes are reviewed, and how
  versions relate across artifacts.
slug: repository-release-policy
type: standards
status: approved
updated_at: 2026-10-09T00:00:00Z
version: "0.2"
---

# Repository Release Policy

These are the choices a project makes about how it releases. The tooling
does not enforce them and in most cases cannot: it is branch-agnostic by
design ([[adr-001]]), so the topology below is a practice a project adopts
rather than a rule a tool applies.

Adopt them as written, vary them, or replace them. What matters is that a
project decides, writes its decision down, and configures its branch
protection and continuous delivery to match — because the tooling will
happily release from any branch it is pointed at.

The Archetypes section collects defaults for recognisable kinds of project.
A project that fits one can adopt its defaults rather than deciding each
requirement from scratch.

Requirements carry stable identifiers so that tests, blueprints and reviews
can cite them individually. An identifier is never renumbered once assigned,
and a retired identifier is never reused: nothing validates a requirement
citation, so renumbering breaks references silently.

---

## Release Lines

**RRP-001:** A project SHALL release its current major version from its
default branch.

**Rationale:** The current major is where nearly all releases happen, and
the default branch is where nearly all work lands. Separating them buys a
stabilization window that a project keeping its default branch releasable
does not need, and costs a promotion step on every release.

---

**RRP-002:** A project MAY release any major version from any branch.

**Rationale:** The tooling does not interpret branch names
([[release-tooling-standards]], RTS-030), so this is a genuine choice. A
project releasing its current major from a dedicated branch takes on
maintaining that branch itself.

---

**RRP-003:** A project SHALL release each major version from exactly one
branch.

**Rationale:** Two branches claiming one major leave no answer to which one
a patch belongs on.

---

**RRP-004:** A project SHOULD release a superseded major version from a
branch dedicated to that major.

---

**RRP-005:** A project SHOULD name a branch dedicated to a major version
`release/vN`, where N is the major it maintains.

---

**RRP-006:** A project SHALL cut a branch dedicated to a major version from
a commit carrying a release tag of that major.

**Rationale:** A maintenance line beginning at an untagged commit ships code
that was never published, and its first release contains changes no
changelog describes.

---

## Support Window

**RRP-007:** A project SHALL declare how many superseded major versions it
supports.

---

**RRP-008:** A project SHOULD support exactly one superseded major version.

---

**RRP-009:** A project SHALL NOT release from a branch whose major has
passed out of its support window.

---

**RRP-010:** A project SHALL retain the branch and the tags of a major that
has reached end of life.

**Rationale:** Deleting them breaks provenance for artifacts that remain
installed and installable long after support ends.

---

## Change Propagation

**RRP-011:** WHEN a defect affects both the current major and a supported
superseded major, the maintainer SHALL land the fix on the current major's
release line first.

**Rationale:** The current line carries the current code and the complete
test matrix. Not every fix is back-ported, so authoring against the
superseded line would route the majority of fixes through the weaker of the
two.

---

**RRP-012:** The maintainer SHALL propagate a fix to a superseded major's
branch by cherry-pick.

---

**RRP-013:** The maintainer SHALL NOT merge a superseded major's branch
into the current major's release line.

**Rationale:** A merge makes that branch's tags reachable from the current
line, so the base version resolves to a maintenance tag and the next
release is computed from the wrong ancestor.

---

**RRP-014:** A project SHOULD audit, for each supported superseded major,
the fixes marked for back-port that have not reached it.

---

**RRP-015:** A project SHOULD audit, for each supported superseded major,
the commits present on its branch and absent from the current line.

**Rationale:** Such a commit is either a fix for code the current line no
longer has, which is expected, or a forgotten forward-port, which is not.
The two are indistinguishable without a human looking.

---

## Release Readiness

**RRP-016:** A project SHALL define release readiness as the set of checks
required to pass on a stage pull request.

**Rationale:** "Releasable" is otherwise undefined and untestable. Making it
the required-check set gives it one meaning, puts it under the project's
control, and makes it enforceable by branch protection rather than by
convention.

---

**RRP-017:** A project SHALL keep every branch it releases from ready at
every commit.

---

**RRP-018:** IF work merged to a release line cannot be released, THEN the
project SHALL revert it rather than defer the release.

**Rationale:** A release line with unreleasable work on it blocks every
release from that line, including urgent ones. Reverting restores the
ability to ship; waiting does not.

---

**RRP-019:** A project SHOULD include its complete artifact build in the
checks required on a stage pull request.

---

**RRP-020:** A project SHOULD include a publication dry run for every
channel it publishes to in the checks required on a stage pull request.

**Rationale:** There is no release-candidate phase, so anything that would
fail at publication has to fail in the stage pull request instead.

---

**RRP-021:** A project SHALL require a stage pull request to be up to date
with its target branch before it can be merged.

**Rationale:** A stage branch computed against an earlier tip carries an
incomplete changelog, and a version that is wrong outright if a breaking
change landed in the interim.

---

## Release Notes in Pull Requests

Release notes are authored per pull request and reviewed with the code, so
that the release changelog and the version are built from reviewed records in
git ([[adr-003]]).

**RRP-036:** A project SHALL configure squash merges into a release line to
use the pull request title and description as the commit message.

**Rationale:** The squash commit is the only record of a pull request that
git keeps. A commit body made of the branch's commit messages drops the
reviewed `## Release notes` block, and the release tooling then sees a commit
with no reviewed signal ([[release-tooling-standards]], RTS-035).

---

**RRP-037:** A pull request into a release line SHALL carry a
`## Release notes` block in its description, and the project's required
checks SHALL verify it with `release notes --check`.

**Rationale:** A block that is missing, malformed, or stale because the pull
request moved on after it was written is caught before merge rather than at
release time, when the change's author and reviewer have moved on.

---

**RRP-038:** The reviewer of a pull request into a release line SHALL review
its `## Release notes` block with the code it describes, including the
impact it proposes.

**Rationale:** The block's impact counts toward the computed version without
further confirmation, so the review is the only point at which a person checks
it. A block approved without being read makes a model's proposal a version.

---

## Version Relationships

**RRP-022:** A project SHALL declare whether the artifacts it publishes
share one version.

---

**RRP-023:** A project publishing artifacts that are extensions of one
versioned thing SHOULD give them all one version.

**Rationale:** Where every artifact is an expression of the same underlying
release — a compiler and the runtime libraries its output depends on — one
version means a consumer never has to work out which combinations are
compatible. The cost is a version bump on artifacts that did not change.

---

**RRP-024:** A project SHALL NOT originate a new major version on a branch
dedicated to a superseded major.

**Rationale:** A breaking change reaching a superseded line would either be
released as a minor, silently breaking consumers who trusted the major, or
force a major from a branch that cannot host one.

---

## Correcting a Release

**RRP-025:** A project SHOULD correct a defect in a published release by
publishing a subsequent release rather than by replacing the published one.

---

**RRP-026:** A project SHALL NOT move or delete a published release tag.

**Rationale:** Consumers, provenance attestations and third-party package
formulae resolve against tags. A moved tag invalidates every artifact
verified against it.

---

**RRP-027:** WHEN a published release is harmful, the project SHALL
withdraw it from every channel that supports withdrawal, in addition to
publishing a correction.

**Rationale:** Correcting forward governs how the defect is fixed in code.
It does not make the harmful version safe to leave installable while the
fix is built.

---

## Verifying a Release Commit

The release tooling does not verify a release commit. These checks are the
project's, discharged by the checks required on a stage pull request, because
what a release commit may legitimately touch differs by ecosystem and only the
project knows it.

**RRP-029:** A project SHALL declare the paths a release commit may modify.

**Rationale:** Version numbers live in different files in every ecosystem, so
the set cannot be inferred. Declaring it is what makes the next requirement
checkable without anything understanding a manifest format.

---

**RRP-030:** A project's required checks SHALL verify that a release commit
modifies only the paths it declared.

---

**RRP-031:** A project's required checks SHALL verify that the version
recorded in a release commit matches the version named by its stage branch.

---

**RRP-032:** A project's required checks SHALL verify that a recorded version
legally succeeds the latest release tag reachable from the target branch.

---

**RRP-033:** A project's required checks SHALL report every failed check
rather than only the first.

---

**RRP-034:** A project SHALL NOT treat a stage branch prefix as evidence that
a commit is a release commit.

**Rationale:** Any contributor can create a branch matching the prefix. The
prefix records that a release was intended; it is not an authorization to
publish.

---

**RRP-035:** A project SHALL prevent a stage pull request from merging WHILE
any required check fails.

---

## Coordinated Disclosure

**RRP-028:** WHEN a security fix applies to more than one supported release
line, the project SHALL prepare a stage pull request on every affected line
before merging any of them.

**Rationale:** Merging sequentially publishes the fix, and its description,
while the remaining lines are still exploitable.

---

## Archetypes

Defaults for recognisable kinds of project. An archetype is a starting set
of answers, not an additional constraint.

### Command-Line Tool or Library

Consumers install published artifacts and pin versions, so the support
window and distribution guarantees matter more than release frequency.

- Current major from the default branch (RRP-001), one superseded major
  supported (RRP-008), `release/vN` for it (RRP-005).
- Artifacts share one version where they are extensions of one release
  (RRP-023).
- Distribution obligations as in [[cli-distribution-policy]].

### Continuously Deployed Service

Nothing is installed and nothing is pinned, so most of this document does
not apply.

- One release line, no superseded majors, so RRP-004 through RRP-015 are out
  of scope.
- Readiness (RRP-016) carries the whole weight, because there is no support
  window to fall back on.
- Correcting forward (RRP-025) is the only correction mechanism *for the
  repository*: there is no published artifact to withdraw. A deployed
  environment is a separate matter and may be corrected by rolling back to a
  previously deployed revision, which changes what is running without
  changing what the repository has released.

A project of this kind may not need release tooling at all. Where it does,
the staging mechanism is still useful for the changelog rather than for the
version.

---

## References

- [[adr-001]] — the tooling is branch-agnostic; the default branch is the
  default release line
- [[adr-002]] — stage branches as the release trigger
- [[adr-003]] — release notes and impact reviewed in pull requests
- [[release-tooling-standards]] — what the tooling enforces, as distinct
  from what a project chooses
- [[cli-distribution-policy]] — the distribution half of a project's
  release policy
