---
title: "ADR-004: Existing Release Tooling Is Not Adopted"
description: The field survey behind building rather than adopting — what
  Auto, release-please, semantic-release, changesets and cargo-release each
  provide, and the reason each is rejected.
slug: adr-004
type: adr
status: approved
---

# ADR-004: Existing Release Tooling Is Not Adopted

**Status:** approved
**Date:** 2026-10-06

## Context

Several mature tools automate most of a release process, and two of them
implement a large part of this model already. Building anything without
accounting for them would be negligent, and "we looked and they did not fit"
is a claim that has to be supported per tool rather than asserted.

The governing requirement is the one in [[adr-002]]: nothing modifies a
release branch without a pull request, which means a release must exist as a
reviewable artifact before it is published. Two further constraints come from
the adopting projects. Conventional commits are not used consistently and
branches are squash-merged, so a bump mechanism keyed solely on commit
message grammar fits poorly. And release notes are expected to be authored
from diffs rather than assembled from commit subjects, which makes changelog
generation a step that must be replaceable.

This decision records what the survey found. What is built instead is
[[adr-005]].

## Decision

No existing release tool is adopted. Each surveyed tool is rejected for the
reason stated below, and the reasons share a root cause: none of them
produces a release as a reviewable artifact before publishing it.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| Auto | Maintenance branches are first-class (`versionBranches`)<br>The current release line is configurable (`baseBranch`)<br>Labels drive bumps, so conventional commits are optional and squash-merging is normal<br>Pre-1.0 semantics solved by its `magic-zero` plugin<br>Canary and prerelease commands already exist<br>30 publication plugins including `crates`, `npm`, `vscode`, `brew`, `docker`<br>`exec` plugin binds shell commands to any lifecycle hook, so artifact work needs no JavaScript | Commits the version and changelog directly to the release branch, so there is no reviewable release artifact and no commit for delivery to verify<br>That architecture is the root cause of its personal-access-token requirement, which the `protected-branch` plugin reduces to a write-scoped PAT but does not remove<br>Changelog is a template engine with per-line hooks: `beforeCommitChangelog` is a plain hook firing *after* the file is written, so it cannot replace the rendered body<br>No support-window enforcement, propagation audit, withdrawal, or provenance<br>Release cadence has decayed — v11.3.6 in November 2025, last commit June 2026 | Direct commits to a release branch are disqualifying under [[adr-002]], and no plugin can change when or where the version commit is created |
| release-please | Release PR maps onto the stage-branch model<br>No push to a protected branch, so no PAT<br>`--target-branch` covers maintenance branches | Conventional commits are mandatory, which the adopting projects do not use consistently<br>Changelog is commit-subject-derived with no authoring hook<br>Provides less of the model than Auto: no topology support beyond the flag | Mandatory conventional commits is a worse fit than Auto's labels, and it offers no path to diff-authored release notes |
| semantic-release | Mature, large plugin ecosystem | Publishes on merge with no reviewable artifact<br>Conventional commits mandatory | Fails the review-surface requirement, without Auto's compensating topology support |
| changesets | Contributor-authored notes<br>Excellent multi-package support | Built around independently versioned packages<br>Requires a changeset file per change | Versioning is lockstep, and the per-change file is friction the commit metadata already carries |
| cargo-release | Native to Cargo workspaces | Rust only | The adopting repositories publish Cargo, npm, editor-extension and binary artifacts from one version |
| Build instead | — | — | N/A — selected; see [[adr-005]] |

## Consequences

**Positive:**
- A reviewable release artifact becomes possible, which is the requirement
  no surveyed tool satisfies.
- No third-party tool's release cadence becomes a dependency of this
  project's ability to release.
- Contributors are not obliged to adopt a commit convention they do not use,
  because the accepted impact signals can include a trailer the tooling
  defines.
- Changelog authoring can be a replaceable step rather than a render
  template, which is what diff-authored prose requires.

**Negative:**
- Bump computation and changelog assembly are rebuilt, and those are the
  parts the surveyed tools do best.
- Auto's publication plugin ecosystem is forgone. Adopting repositories
  write their own artifact workflows, which is more work for each of them
  than installing a plugin would have been.
- Maintaining release tooling is a standing cost, and this ADR rejects two
  tools partly on cadence grounds — a standard this project's own tooling
  will be held to.

## References

- [[adr-002]] — the review-surface requirement that disqualifies every
  surveyed tool
- [[adr-003]] — the bump and prose split, which constrains what a changelog
  engine must allow
- [[adr-005]] — what is built instead, and its shape
- [Auto](https://intuit.github.io/auto/) — the surveyed tool closest to this
  model
- [release-please](https://github.com/googleapis/release-please)
