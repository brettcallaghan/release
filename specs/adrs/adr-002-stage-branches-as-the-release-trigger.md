---
title: "ADR-002: Stage Branches as the Release Trigger"
description: A release is triggered by merging a branch carrying the
  configured stage prefix into a release branch, with the project's required
  checks verifying the commit independently of the branch name.
slug: adr-002
type: adr
status: draft
updated_at: 2026-09-25T00:00:00Z
---

# ADR-002: Stage Branches as the Release Trigger

**Status:** draft
**Date:** 2026-09-25

## Context

Nothing should modify a release branch without a pull request. That is the
governing principle, and it is not a preference about convenience: a release
branch is what consumers' published artifacts are built from, so a change
arriving on one unreviewed is a change nobody agreed to ship.

A release needs two things that follow from it. It needs a trigger the
release process can key on — a workflow that builds artifacts, publishes
them, and creates the release. And it needs somewhere to capture the changes
the release itself makes: a changelog entry where a project keeps one, and
version numbers updated across binaries, package manifests and whatever else
records them.

Under [[adr-001]] the tooling knows nothing about which branches are release
lines, and a release line is typically also where ordinary work lands. So
"a merge into a release branch is a release" cannot be the trigger — it would
fire on every merged feature. Whatever distinguishes a release from a code
change must also give a human somewhere to review the computed version and
the changelog before anything is published.

## Decision

A release is triggered by merging a branch whose name carries the configured
stage prefix — `cut/` by default, as in `cut/1.8.0` — into a release branch.
A stage branch is cut from its target's tip and carries the release commit:
version manifests and the changelog, no source code. It normally holds exactly
one commit, and may hold more where the notes are corrected during review.

The project's required checks verify that the merged commit is a well-formed
release commit; the branch name is a signal, never an authorization. The
tooling does not perform that verification, because what a release commit may
legitimately touch differs by ecosystem and only the project knows it.

Throughout these specifications the concept is a *stage branch*, whatever
prefix a project configures for it.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| Any merge into a release branch releases | No extra branch concept | Trunk receives topic PRs, so every merged feature would publish | Incompatible with [[adr-001]] |
| A maintainer pushes a tag | Almost no machinery | No review surface for the version or the changelog<br>Publication depends on a local action that leaves no reviewable artifact | The changelog review is a required step, and an unreviewable local push cannot carry it |
| A bot-owned branch plus a label, as release-please does | Robust to branch renames<br>Proven at scale | The label is invisible in git history<br>Requires the bot to own the branch name | Branch prefix is legible in `git log` and in branch-protection globs; the label's real contribution — that CD must verify rather than trust — is adopted instead |
| Stage branch with a configurable prefix | Uniform across trunk and maintenance branches<br>The PR diff is exactly the version and the changelog<br>One commit makes squash, merge and rebase equivalent | Anyone may create a matching branch, so the prefix cannot authorize publication<br>A stage PR goes stale when its target advances | N/A — selected |

## Consequences

**Positive:**
- Trunk and maintenance branches use one protocol; the only difference
  between them is policy, not process.
- The stage PR's diff is the release note, so reviewing the release is
  reviewing one small diff.
- The branch touches only declared release paths whatever its commit count,
  so the merge method cannot silently change what a release contains. That
  removes the squash-merge hazard a promotion-based model carries.
- Hotfix branches need no special handling: a fix is a topic PR like
  any other, and the stage PR that follows is identical to any release.

**Negative:**
- The project must independently verify the merged commit's shape, target
  and version in its required checks. Trusting the prefix would let any
  contributor publish, and the tooling does not do this for them.
- A stage PR is invalidated whenever its target branch advances, so
  either branch protection must require the branch to be up to date or
  the tooling must regenerate the stage branch on target push.
- The stage prefix must not collide with the release-branch glob;
  protection rules and workflow filters have to be written to
  distinguish them.

## References

- [[adr-001]] — the branching model that makes a source-keyed trigger
  necessary
- [[adr-004]] — the engine that maintains the stage branch's contents
- [[release-tooling-standards]] — the normative shape of a release commit
