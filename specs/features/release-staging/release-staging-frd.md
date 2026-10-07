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

Verifying that the commit is well formed is the project's job rather than the
tooling's, because which paths a release commit may touch differs by ecosystem
and only the project knows them. The tooling confines its writes to the paths
the project declared; the project's required checks confirm it did
([[repository-release-policy]], RRP-029 through RRP-035).

This milestone stops at the pull request. Tagging and publication are
specified separately, which makes everything here non-destructive: a staging
run that goes wrong produces a branch and a pull request to close.

This feature SHALL comply with [[release-tooling-standards]].

## Terminology

- **Stage branch** — a branch carrying the configured prefix, `cut/` by
  default, whose first commit is the release commit. It normally holds only
  that commit, and may hold more where the notes are corrected during review.
- **Release commit** — a commit modifying only version manifests and the
  changelog.
- **Target branch** — the release branch a stage branch is merged into.
- **Declared release paths** — the paths a project states a release commit
  may modify.

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
  Scenario: Cutting produces a branch from the target tip whose first commit is the release commit
    When the maintainer cuts a release against a release branch
    Then the tooling creates a branch from that branch's tip whose first commit is the release commit

  @AC-STG-001.10
  Scenario: A correction committed during review does not invalidate the branch
    Given a stage pull request to which a maintainer has added a commit correcting the notes
    When the project's required checks run
    Then the additional commit does not by itself make the stage branch invalid

  @AC-STG-001.11
  Scenario: A named bump settles the impact
    Given the maintainer names a bump of minor
    When the tooling cuts a release
    Then the staged version increments the minor component and no impact is inferred

  @AC-STG-001.12
  Scenario: Cutting without notes skips the authoring exchange
    Given the maintainer requests a release without notes and names a bump
    When the tooling cuts a release
    Then the tooling stages the release without invoking a changelog producer

  @AC-STG-001.2
  Scenario: The release commit touches only declared release paths
    When the tooling creates a release commit
    Then the commit modifies only paths the project declared as release paths

  @AC-STG-001.3
  Scenario: The pull request is titled for the version it releases
    When the tooling opens the stage pull request
    Then the pull request title names the version being released

  @AC-STG-001.9
  Scenario: The pull request body carries the rendered release notes
    When the tooling opens the stage pull request
    Then the pull request body contains the rendered notes for the staged version under a "Release Notes" heading

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

## References

- [[adr-001]] — the release lines a stage branch may target
- [[adr-002]] — stage branches as the release trigger, verified
  independently of the branch name
- [[adr-003]] — disagreement over impact halts the release
- [[repository-release-policy]] — the verification the project performs
- [[component-stage-orchestrator]] — the component realizing this feature
