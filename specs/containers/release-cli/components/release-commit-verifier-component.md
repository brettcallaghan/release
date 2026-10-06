---
title: "Component Blueprint: Release Commit Verifier"
description: A candidate release commit checked against the rules that make
  it publishable — its file set, its recorded version, its branch name, and
  its succession.
slug: component-release-commit-verifier
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - cli/release/internal/verify
test_paths:
  - cli/release/internal/verify
---

# Release Commit Verifier

## Overview

Decides whether a commit is a release commit. It checks four things: that
it touches only version manifests and the changelog, that the version it
records matches the version named by its stage branch, that this version
legally succeeds the latest tag reachable from the target branch, and that
the target branch may receive it.

It runs twice in a release's life, which is the point. Before merge it is a
check on the stage pull request, so a maintainer never merges a malformed
release. After merge it is the gate delivery runs before publishing,
because a branch name any contributor can create is a signal that a release
was intended and not evidence that one is well formed ([[adr-002]]).

- **Role:** the publishability decision.
- **Scope:** in — file-set, version-match, succession and target checks;
  out — tagging, publishing, and creating anything.
- **Composed into:** [[container-release-cli]] — consumes
  [[component-semver-core]], [[component-tag-reader]] and
  [[component-version-calculator]]; see it for relationships.
- **Source & tests:** `cli/release/internal/verify`.

## Interface

- **Surface:** given a commit, its stage branch name and its target branch,
  return a verdict with every failed check named.
- **Guarantees:**
  - The component SHALL reject a commit that modifies any path other than a
    declared version manifest or the changelog.
  - The component SHALL reject a commit whose recorded version differs from
    the version named by its stage branch.
  - The component SHALL reject a version that does not legally succeed the
    latest tag reachable from the target branch.
  - The component SHALL report every failed check, not only the first.
  - The component SHALL reach the same verdict before and after the merge.
  - The component SHALL NOT trust the stage branch prefix as evidence that
    a commit is a release commit.
  - The component SHALL NOT create a tag or publish anything.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the release-commit shape and the
  pre-publication verification rules.

## Change Guidance

WHEN a check is added, it SHALL be added to the single verdict function so
that the pre-merge check and the publication gate cannot diverge.

**Rationale:** Two implementations of "is this publishable" will disagree
eventually, and the disagreement surfaces as something that passed review
and then failed to publish, or worse, the reverse.

---

The component SHALL NOT accept a configuration that disables a check.

**Rationale:** Every check guards something that cannot be undone after
publication. A disabled check is indistinguishable from an absent one at
the moment it would have mattered.

- **Verification gates:** the component's unit suite, with one case per
  rejection reason plus the accepting case, and a test asserting that the
  pre-merge and post-merge invocations agree on a shared fixture set.

## Key Decisions

- [[adr-002]] — the branch prefix is a signal, not an authorization, so
  delivery verifies the commit independently.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[frd-release-staging]] — the feature this component serves
