---
title: "FRD: Tool Configuration"
description: Expressing every repository-specific behaviour as configuration
  — the commands the tooling delegates to, the evidence it collects, the
  producer and signals it uses, the values it defaults, and the
  configurations it refuses.
slug: frd-tool-configuration
type: frd
status: approved
sources:
  - adr-001
  - adr-003
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

Configuration also bounds the evidence a producer reads. How much patch
text a release may carry, which paths are noise, which producer authors,
and which signals set the impact's floor differ by repository, and each has a default that serves a repository declaring
nothing. How the evidence is collected and used is [[frd-changelog-authoring]]
and [[frd-version-computation]]; this document is what can be configured, and
what is refused.

This feature SHALL comply with [[release-tooling-standards]].

## Terminology

- **Delegated command** — a shell command the repository configures, which
  the tooling runs in place of built-in behaviour.
- **Stage prefix** — the branch-name prefix that marks a branch as carrying
  a release.
- **Release-branch pattern** — the glob a repository uses to identify its
  release branches in branch protection and workflow filters.
- **Diff budget** — the most patch text, in bytes, a release's evidence
  bundle may carry.
- **Diff exclusion** — a path glob whose patches no evidence bundle carries.
  The path still appears in its commit's file list.
- **Adapter** — a collector of evidence that can be turned on or off: the
  diff adapter and the optional pull request adapter, which enriches commits
  from the hosting platform. The commit adapter is always on.
- **Producer** — what authors from the evidence: `claude`, a configured
  `command`, or `mechanical`, the built-in producer that needs no model,
  which `--skip-notes` also selects.
- **Signal** — an explicit record in a commit that indicates its impact,
  named in configuration as `release-type`, `conventional-type` or
  `breaking-marker` ([[frd-version-computation]]).

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
      | stage prefix | cut/    |
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

### FEAT-CFG-003: Evidence Collection Settings

**Acceptance Criteria:**

```gherkin
@FEAT-CFG-003
Feature: Evidence Collection Settings

  As an adopting project
  I want to bound and filter the diffs a producer reads
  So that the evidence fits a model's context and leaves out what no reader cares about

  Diffs are collected by default, because they are the only record of a
  change nobody described. The budget and the exclusions are settings
  because a repository's size and its generated paths are its own. The pull
  request adapter is an optional enrichment, off by default; nothing
  depends on it.

  @AC-CFG-003.1
  Scenario Outline: An unconfigured evidence setting takes its documented default
    Given the repository configures no value for "<setting>"
    When the tooling reads its configuration
    Then the effective value is "<default>"

    Examples:
      | setting                               | default |
      | changelog.diff_budget.release         | 204800  |
      | adapters.diffs                        | true    |
      | adapters.pull_requests                | false   |

  @AC-CFG-003.2
  Scenario Outline: Lockfiles, vendored code and snapshot outputs are excluded by default
    Given the repository configures no diff exclusions
    When the tooling reads its configuration
    Then "<path>" is a diff exclusion

    Examples:
      | path                  |
      | **/go.sum             |
      | **/package-lock.json  |
      | **/yarn.lock          |
      | **/pnpm-lock.yaml     |
      | **/Cargo.lock         |
      | **/poetry.lock        |
      | vendor/               |
      | testdata/**/expected/ |

  @AC-CFG-003.3
  Scenario: A declared exclusion list replaces the default whole
    Given the repository configures "gen/**" as its only diff exclusion
    When the tooling reads its configuration
    Then "gen/**" is the only diff exclusion
    And "**/go.sum" is not a diff exclusion

  @AC-CFG-003.4
  Scenario: The diff adapter is accepted when turned on
    Given the repository configures the diff adapter on
    When the tooling reads its configuration
    Then the tooling accepts the configuration

  @AC-CFG-003.5
  Scenario: Turning the diff adapter off leaves diffs out of the evidence
    Given the repository configures the diff adapter off
    When the tooling collects evidence for a release
    Then the evidence carries no diff
    And the evidence does not list the diff adapter among the adapters used

  @AC-CFG-003.6 @negative
  Scenario Outline: A diff budget that is not a positive integer is refused
    Given the repository configures "<value>" as a diff budget
    When the tooling reads its configuration
    Then the tooling refuses the configuration, naming the setting

    Examples:
      | value |
      | 0     |
      | -1    |
      | 100KB |

  @AC-CFG-003.7 @negative
  Scenario: A malformed diff exclusion is refused
    Given the repository configures a diff exclusion that is not a valid glob
    When the tooling reads its configuration
    Then the tooling refuses the configuration, naming the glob

  @AC-CFG-003.8 @negative
  Scenario: The pull request adapter is refused until it is available
    Given the repository configures the pull request adapter on
    When the tooling reads its configuration
    Then the tooling refuses the configuration, naming the setting
```

### FEAT-CFG-004: Producer and Signal Settings

**Acceptance Criteria:**

```gherkin
@FEAT-CFG-004
Feature: Producer and Signal Settings

  As an adopting project
  I want to choose what authors my notes and which records in git set the floor of my impact
  So that a repository with no model, or with its own conventions, can still release

  The mechanical producer exercises no judgement: it composes entries from
  conventional commit subjects, so the bump suggested from its changelog is
  the signal floor alone. Signals are what set that floor.

  @AC-CFG-004.1
  Scenario Outline: A producer is configured by name
    Given the repository configures "<producer>" as its changelog producer
    When the tooling reads its configuration
    Then the tooling accepts the configuration

    Examples:
      | producer   |
      | claude     |
      | command    |
      | mechanical |

  @AC-CFG-004.2
  Scenario: The default producer is claude
    Given the repository configures no changelog producer
    When the tooling reads its configuration
    Then the effective producer is "claude"

  @AC-CFG-004.3
  Scenario: The mechanical producer authors a release without a model
    Given the repository configures "mechanical" as its changelog producer
    When the maintainer prepares a release changelog
    Then the tooling composes the corpus with the mechanical producer and starts no producer process

  @AC-CFG-004.5 @negative
  Scenario Outline: An unusable producer setting is refused
    Given the repository configures <setting>
    When the tooling reads its configuration
    Then the tooling refuses the configuration

    Examples:
      | setting                                            |
      | a changelog producer naming no producer            |
      | the command producer with no producer command      |

  @AC-CFG-004.6
  Scenario Outline: Every signal is enabled by default
    Given the repository configures no signals
    When the tooling reads its configuration
    Then "<signal>" is an enabled signal

    Examples:
      | signal            |
      | release-type      |
      | conventional-type |
      | breaking-marker   |

  @AC-CFG-004.7
  Scenario: A conventional type is no signal when its signal is not enabled
    Given the repository configures the signals "release-type" and "breaking-marker"
    And a commit whose only signal is the conventional-commit type feat
    When the tooling computes the signal floor
    Then the commit is reported as unclassified

  @AC-CFG-004.8 @negative
  Scenario: An unknown signal name is refused
    Given the repository configures a signal named "issue-label"
    When the tooling reads its configuration
    Then the tooling refuses the configuration, naming the signal
```

## References

- [[adr-001]] — the tooling is branch-agnostic, so branch names carry no
  configuration meaning
- [[adr-003]] — explicit signals in git set the floor of the suggested
  bump, which is why the mechanical producer's suggestion is that floor
- [[adr-005]] — repository-specific behaviour is delegated rather than
  implemented
- [[frd-changelog-authoring]] — how evidence is collected within the budget
  and the exclusions, and how producers author from it
- [[frd-version-computation]] — how enabled signals compute the impact
- [[contract-release-cli]] — `release config`, which reports these settings
- [[release-tooling-standards]] — the configuration requirements these
  criteria verify
