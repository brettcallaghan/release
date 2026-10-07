---
title: "FRD: Tool Configuration"
description: Expressing every repository-specific behaviour as configuration
  — the commands the tooling delegates to, the values it defaults, and the
  configurations it refuses.
slug: frd-tool-configuration
type: frd
status: approved
updated_at: 2026-10-07T00:00:00Z
sources:
  - adr-001
  - adr-005
  - release-tooling-standards
---

# Tool Configuration

## Overview

One tool serves repositories that share almost nothing: a Go module, a
Cargo and npm monorepo, an editor extension. What they differ on —
how a version is written into their manifests, what counts as a passing
validation, what their branches are called — is configuration, and a
repository needing something the configuration cannot express is a gap in
the configuration surface rather than a reason to fork.

Configuration is also where the tooling's branch-agnosticism becomes
visible. It accepts a target branch and a stage prefix and interprets
neither beyond what it was told, because which branches carry which majors
is the adopting project's policy and not the tool's business
([[adr-001]]).

This feature SHALL comply with [[release-tooling-standards]].

## Terminology

- **Delegated command** — a shell command the repository configures, which
  the tooling runs in place of built-in behaviour.
- **Stage prefix** — the branch-name prefix that marks a branch as carrying
  a release.
- **Release-branch pattern** — the glob a repository uses to identify its
  release branches in branch protection and workflow filters.

## Requirements

### FEAT-CFG-001: Delegated Commands

**Acceptance Criteria:**

```gherkin
@FEAT-CFG-001
Feature: Delegated Commands

  As an adopting project
  I want the tooling to run my commands for the steps that are specific to my repository
  So that one tool serves repositories with different toolchains and layouts

  The tooling delegates rather than implements wherever the correct behaviour
  depends on the repository's ecosystem. It knows that a version must be
  written into manifests; it does not know what this repository's manifests
  are.

  @AC-CFG-001.1
  Scenario Outline: A configured command replaces built-in behaviour
    Given the repository configures a command for "<step>"
    When the tooling reaches that step
    Then the tooling runs the configured command

    Examples:
      | step                |
      | validation          |
      | version propagation |
      | preflight           |

  @AC-CFG-001.2
  Scenario: A failing delegated command stops the operation
    Given a configured validation command that exits non-zero
    When the tooling reaches validation
    Then the tooling stops and reports the command's output

  @AC-CFG-001.3 @negative
  Scenario: A step with no configured command is not silently skipped
    Given the repository configures no version propagation command
    When the tooling reaches version propagation
    Then the tooling reports that the step is unconfigured rather than continuing
```

### FEAT-CFG-002: Defaults and Refusals

**Acceptance Criteria:**

```gherkin
@FEAT-CFG-002
Feature: Defaults and Refusals

  As an adopting project
  I want sensible defaults and a refusal when my configuration cannot work
  So that adoption needs little configuration and a misconfiguration fails early

  A configuration that cannot work is rejected when it is read, not when the
  step that depends on it is reached, because by then a release may be
  half-staged.

  @AC-CFG-002.1
  Scenario Outline: An unconfigured value takes its documented default
    Given the repository configures no value for "<setting>"
    When the tooling reads its configuration
    Then the effective value is "<default>"

    Examples:
      | setting      | default |
      | stage prefix | stage/  |
      | target branch | the repository's default branch |

  @AC-CFG-002.2 @negative
  Scenario: A stage prefix colliding with the release-branch pattern is refused
    Given the repository configures a stage prefix that its release-branch pattern also matches
    When the tooling reads its configuration
    Then the tooling refuses the configuration

  @AC-CFG-002.3
  Scenario: An invalid configuration is refused before any write
    Given a configuration the tooling cannot accept
    When the maintainer runs a command that would write to the repository
    Then the tooling refuses before making any write

  @AC-CFG-002.4 @negative
  Scenario: A verification check cannot be disabled by configuration
    Given a configuration attempting to disable a verification check
    When the tooling reads its configuration
    Then the tooling refuses the configuration

  @AC-CFG-002.5 @negative
  Scenario: The tooling does not interpret a target branch name
    Given a target branch whose name suggests it maintains a particular major version
    When the tooling reads its configuration
    Then the tooling derives no constraint from that name
```

## References

- [[adr-001]] — the tooling is branch-agnostic, so branch names carry no
  configuration meaning
- [[adr-005]] — repository-specific behaviour is delegated rather than
  implemented
- [[release-tooling-standards]] — the configuration requirements these
  criteria verify
