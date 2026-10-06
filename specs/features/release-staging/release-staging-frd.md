---
title: "FRD: Release Staging"
description: Producing a release as a single-commit stage branch and its
  pull request, and verifying that a candidate release commit is
  publishable before and after it merges.
slug: frd-release-staging
type: frd
status: draft
updated_at: 2026-10-06T00:00:00Z
sources:
  - adr-001
  - adr-002
  - adr-003
  - release-tooling-standards
---

# Release Staging

## Overview

Nothing reaches a release branch except through a reviewed pull request.
That is the principle this feature exists to implement: a release is
prepared as a branch holding one commit that touches only version manifests
and the changelog, and a human reads the version and the notes in a diff
small enough to actually read before anything is published.

Verification is the other half. A branch name matching the stage prefix is
something any contributor can create, so it records that a release was
intended and proves nothing about whether the commit is one. The same
verdict runs twice — as a check on the stage pull request, and as the gate
delivery runs before publishing — from one implementation, so the two
cannot drift apart.

This milestone stops at the pull request. Tagging and publication are
specified separately, which makes everything here non-destructive: a
staging run that goes wrong produces a branch and a pull request to close.

This feature SHALL comply with [[release-tooling-standards]].

## Terminology

- **Stage branch** — a branch carrying the configured prefix and holding
  exactly one commit.
- **Release commit** — a commit modifying only version manifests and the
  changelog.
- **Target branch** — the release branch a stage branch is merged into.
- **Verdict** — the result of checking a candidate release commit, naming
  every failed check.

## Requirements

### FEAT-STG-001: Release Staging

**Acceptance Criteria:**

```gherkin
@FEAT-STG-001
Feature: Release Staging

  As a maintainer
  I want a release prepared as a reviewable pull request
  So that nothing modifies a release branch without review

  Every check that can fail is performed before the first write, so that a
  refusal leaves the repository untouched rather than leaving a partial
  state behind for someone to clean up.

  @AC-STG-001.1
  Scenario: Staging produces a single-commit branch from the target tip
    When the maintainer stages a release against a release branch
    Then the tooling creates a branch from that branch's tip holding exactly one commit

  @AC-STG-001.2
  Scenario: The release commit touches only manifests and the changelog
    When the tooling creates a release commit
    Then the commit modifies only version manifests and the changelog

  @AC-STG-001.3
  Scenario: The pull request carries the rendered release notes
    When the tooling opens the stage pull request
    Then the pull request body contains the rendered notes for the staged version

  @AC-STG-001.4
  Scenario: Nothing releasable means no branch
    Given no releasable change exists since the latest tag reachable from the target branch
    When the maintainer stages a release
    Then the tooling refuses and creates no branch

  @AC-STG-001.5
  Scenario: A disagreement over impact halts before any write
    Given the authored proposal and the computed impact disagree
    When the maintainer stages a release
    Then the tooling halts and makes no write to the repository

  @AC-STG-001.6 @negative
  Scenario: Staging never modifies the target branch
    When the maintainer stages a release
    Then the target branch is unchanged

  @AC-STG-001.7 @negative
  Scenario: Staging never tags or publishes
    When the maintainer stages a release
    Then the tooling creates no tag and publishes nothing

  @AC-STG-001.8
  Scenario: A partial write is reported in full
    Given a write fails after the tooling has already made another
    When the tooling reports the failure
    Then the report names every change the tooling made
```

### FEAT-STG-002: Release Commit Verification

**Acceptance Criteria:**

```gherkin
@FEAT-STG-002
Feature: Release Commit Verification

  As a release engineer
  I want a candidate release commit checked rather than trusted
  So that a branch anyone can create cannot cause a publication

  One implementation produces the verdict, and it runs both before the merge
  as a pull request check and after it as the publication gate. Two
  implementations would eventually disagree, and the disagreement would
  surface as something that passed review and then failed to publish.

  @AC-STG-002.1
  Scenario: A well-formed release commit is accepted
    Given a commit modifying only version manifests and the changelog, recording a version that matches its stage branch name and legally succeeds the latest tag reachable from the target branch
    When the tooling verifies it
    Then the verdict is that the commit is publishable

  @AC-STG-002.2 @negative
  Scenario: A commit touching source code is rejected
    Given a commit on a stage branch that also modifies source files
    When the tooling verifies it
    Then the verdict rejects the commit

  @AC-STG-002.3 @negative
  Scenario: A version disagreeing with the branch name is rejected
    Given a commit recording version 1.9.0 on a stage branch named for version 1.8.0
    When the tooling verifies it
    Then the verdict rejects the commit

  @AC-STG-002.4 @negative
  Scenario: A version that does not legally succeed the base is rejected
    Given a commit recording version 1.7.0 on a branch whose latest reachable tag is 1.8.0
    When the tooling verifies it
    Then the verdict rejects the commit

  @AC-STG-002.5
  Scenario: Every failed check is reported
    Given a commit failing more than one check
    When the tooling verifies it
    Then the verdict names every check that failed

  @AC-STG-002.6
  Scenario: The verdict is the same before and after the merge
    Given a candidate release commit
    When the tooling verifies it as a pull request check and again after it is merged
    Then the two verdicts agree

  @AC-STG-002.7 @negative
  Scenario: The branch prefix alone does not establish publishability
    Given a commit on a branch carrying the stage prefix that modifies source files
    When the tooling verifies it
    Then the verdict rejects the commit
```

## References

- [[adr-001]] — the release lines a stage branch may target
- [[adr-002]] — stage branches as the release trigger, verified
  independently of the branch name
- [[adr-003]] — disagreement over impact halts the release
- [[component-stage-orchestrator]],
  [[component-release-commit-verifier]] — the components realizing this
  feature
