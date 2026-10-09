---
title: "FRD: Release Staging"
description: Cutting a release as a stage branch and its pull request, with
  the tooling confining its writes to the paths a project declares.
slug: frd-release-staging
type: frd
status: draft
updated_at: 2026-10-09T00:00:00Z
sources:
  - adr-001
  - adr-002
  - adr-003
  - frd-version-computation
  - release-tooling-standards
---

# Release Staging

## Overview

Nothing reaches a release branch except through a reviewed pull request.
That is the principle this feature exists to implement: a release is
prepared as a branch holding one commit that touches only version manifests
and the changelog, and a human reads the version and the notes in a diff
small enough to actually read before anything is published.

The release commit also records how its impact was decided. It carries the
confirmed impact as a `Release-Type:` trailer, so an impact a person
confirmed, or a maintainer named, is in git and the version can be recomputed
from a clone without re-running a model ([[adr-003]]). The stage pull request
shows the reviewer the authored changelog, the suggested bump and its
reasons, and the commits no signal classified. Reviewing the stage pull
request is where a person reviews the changelog before anything is
published.

Verifying that the commit is well formed is the project's job rather than the
tooling's, because which paths a release commit may touch differs by ecosystem
and only the project knows them. The tooling confines its writes to the paths
the project declared; the project's required checks confirm it did
([[repository-release-policy]], RRP-029 through RRP-035).

Forcing a release does two things, and only with a named bump: it stages a
named bump over a range with nothing releasable, and it accepts a named
bump below the signal floor. A range whose suggested bump is none but which
holds an entry with no kind is releasable: the cut halts for a named bump,
which needs no force, and accepting the suggestion does not answer it.

This milestone stops at the pull request. Tagging and publication are
specified separately, which makes everything here non-destructive: a staging
run that goes wrong produces a branch and a pull request to close.

This feature SHALL comply with [[release-tooling-standards]].

## Terminology

- **Stage branch** — a branch carrying the configured prefix, `cut/` by
  default, whose first commit is the release commit. It normally holds only
  that commit, and may hold more where the notes are corrected during review.
- **Release commit** — a commit modifying only version manifests and the
  changelog, carrying the release's confirmed impact as a `Release-Type:`
  trailer.
- **Confirmed impact**, **suggested bump**, **signal floor**, **named
  bump**, **unclassified commit** and **releasable** — as
  [[frd-version-computation]] defines them.
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
  state behind for someone to clean up. Confirming the impact is one of those
  checks.

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

  @AC-STG-001.15
  Scenario: The release commit carries the confirmed impact as a trailer
    Given the confirmed impact of the release is minor
    When the tooling creates the release commit
    Then the commit carries the trailer Release-Type: minor

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

  @AC-STG-001.16
  Scenario: The pull request body shows the suggested bump and its reasons
    Given the suggested bump is minor, driven by one feature entry
    When the tooling opens the stage pull request
    Then the pull request body shows the confirmed impact, the suggested bump, and its reasons naming that entry

  @AC-STG-001.17
  Scenario: The pull request body names the unclassified commits
    Given two commits in the range carry no signal
    When the tooling opens the stage pull request
    Then the pull request body names both commits as unclassified

  @AC-STG-001.19
  Scenario: Cutting interactively prompts for the suggested bump
    Given the maintainer names no bump
    And standard input is a terminal
    When the maintainer stages a release
    Then the tooling shows the suggested bump and its reasons and prompts the maintainer to confirm it or choose another before any write

  @AC-STG-001.20
  Scenario: An explicit acceptance stages the suggested bump without a prompt
    Given the maintainer names no bump and passes --yes
    And the suggested bump is minor
    When the maintainer stages a release
    Then the tooling stages the release with a confirmed impact of minor and shows no prompt

  @AC-STG-001.21 @negative
  Scenario: Non-interactively, cutting without acceptance halts with the suggestion
    Given the maintainer names no bump and does not pass --yes
    And standard input is not a terminal
    When the maintainer stages a release
    Then the tooling halts with the exit status the CLI contract assigns to an unconfirmed impact, creating no branch
    And it shows the suggested bump and its reasons and how to confirm it

  @AC-STG-001.4
  Scenario: Nothing releasable means no branch
    Given no releasable change exists since the latest tag reachable from the target branch
    And the maintainer names no bump
    When the maintainer stages a release
    Then the tooling refuses and creates no branch

  @AC-STG-001.13
  Scenario: A named bump over nothing releasable is refused unless forced
    Given no releasable change exists since the latest tag reachable from the target branch
    And the suggested bump is none and every entry of the structured changelog has a kind
    And the maintainer names a bump without forcing the release
    When the maintainer stages a release
    Then the tooling refuses, says nothing is releasable, names how to force the release, and creates no branch

  @AC-STG-001.14
  Scenario: A forced named bump stages over nothing releasable
    Given no releasable change exists since the latest tag reachable from the target branch
    And the maintainer names a bump and forces the release
    When the maintainer stages a release
    Then the tooling stages the named bump

  @AC-STG-001.18
  Scenario Outline: User-visible changes without signals stage without forcing
    Given no commit since the latest reachable tag carries an impact signal
    And the structured changelog carries <entry>
    And the maintainer names a bump of minor without forcing the release
    When the maintainer stages a release
    Then the tooling stages the release

    Examples:
      | entry                        |
      | an entry of the kind feature |
      | an entry with no kind        |

  @AC-STG-001.5
  Scenario: An unconfirmed impact halts before any write
    Given the maintainer names no bump and the suggested bump awaits confirmation
    And no person has confirmed it
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
- [[adr-003]] — an unreviewed impact is confirmed by a person and recorded
  on the release commit
- [[frd-version-computation]] — the suggested bump, its confirmation, and
  what is releasable
- [[repository-release-policy]] — the verification the project performs
- [[component-stage-orchestrator]] — the component realizing this feature
