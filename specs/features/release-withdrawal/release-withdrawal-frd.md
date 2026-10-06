---
title: "FRD: Release Withdrawal"
description: Withdrawing a published version from every channel that
  supports it, reporting the channels that do not, and leaving the tag in
  place.
slug: frd-release-withdrawal
type: frd
status: draft
updated_at: 2026-10-06T00:00:00Z
sources:
  - cli-distribution-policy
  - repository-release-policy
---

# Release Withdrawal

## Overview

Correcting a defect by publishing the next release governs how the code is
fixed. It says nothing about the harmful version still being installable
while that fix is built, and for a tool people install by piping a script
into a shell, the gap between the two matters.

Withdrawal is one operation across channels that behave differently: some
support it directly, some only support marking a version deprecated, and
some have no mechanism at all. The capability is therefore as much about
reporting what could not be withdrawn as about withdrawing.

This feature is deferred beyond the first milestone. It is specified now
because the obligation exists from the first published release, and a
project needs to know whether it can discharge it before it needs to.

This feature SHALL comply with [[cli-distribution-policy]].

## Terminology

- **Channel** — one distribution destination: a package registry, a release
  archive, a package-manager formula, a container registry.
- **Withdrawal** — making a published version unavailable for new
  installations through a channel that supports doing so.
- **Deprecation** — marking a published version as discouraged where a
  channel supports no stronger action.

## Requirements

### FEAT-WD-001: Withdrawing a Published Version

**Acceptance Criteria:**

```gherkin
@FEAT-WD-001
Feature: Withdrawing a Published Version

  As a maintainer
  I want one operation that withdraws a bad release everywhere it can be withdrawn
  So that a harmful version stops being installable while the correction is built

  Channels differ in what they permit, so the operation is partial by nature.
  What it must never be is silently partial: a channel that still carries the
  version after a withdrawal is reported, because the maintainer's next action
  depends on knowing.

  @AC-WD-001.1
  Scenario: Withdrawal applies to every channel supporting it
    When the maintainer withdraws a published version
    Then the tooling withdraws it from every declared channel that supports withdrawal

  @AC-WD-001.2
  Scenario: A channel supporting only deprecation is deprecated
    Given a declared channel that supports deprecation but not withdrawal
    When the maintainer withdraws a published version
    Then the tooling marks the version deprecated on that channel

  @AC-WD-001.3
  Scenario: Channels that still carry the version are reported
    Given a declared channel that supports neither withdrawal nor deprecation
    When the maintainer withdraws a published version
    Then the tooling reports that channel as still carrying the version

  @AC-WD-001.4 @negative
  Scenario: Withdrawal leaves the tag in place
    When the maintainer withdraws a published version
    Then the tooling does not delete or move that version's tag

  @AC-WD-001.5
  Scenario: A partial withdrawal is re-runnable
    Given a withdrawal in which one channel failed
    When the maintainer re-runs the withdrawal
    Then the tooling withdraws from the failed channel without erring on the channels already withdrawn
```

## References

- [[cli-distribution-policy]] — the withdrawal obligations these criteria
  verify
- [[repository-release-policy]] — correcting a release forward, which
  withdrawal accompanies rather than replaces
