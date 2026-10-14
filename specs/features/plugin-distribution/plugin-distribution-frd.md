---
title: "FRD: Plugin Distribution"
description: Delivering the release CLI through the `release` agent plugin —
  the plugin provisions the CLI version it was published with, and its one
  skill teaches an agent to cut a release with the release-management
  standards built in.
slug: frd-plugin-distribution
type: frd
status: approved
sources:
  - adr-008
  - cli-distribution-policy
  - release-tooling-standards
  - repository-release-policy
---

# Plugin Distribution

## Overview

Enabling the `release` plugin in a repository is the whole of adoption.
Every agent session there can cut a release: the plugin brings the CLI at
the exact version its skill was written for, and the skill brings the
judgement, so the agent knows both how to drive each command and what a
well-run release process looks like. Nothing is installed by hand, no
version is chosen by hand, and nothing the person installed is replaced
or changed. `specs` is the first repository to adopt it
after this one.

The plugin and the CLI are released together and carry one version, and
the plugin provisions exactly that version ([[adr-008]]). A skill describes
one version's command surface, so the agent never meets a binary that
answers the same commands differently: whatever else the machine has
installed, the skill and the CLI it runs always agree.

Provisioning happens the first time a skill needs the CLI, and the copy is
reused from then on, so a session that never releases pays nothing. A
skill that cannot get the CLI stops and says why, with the remedy.

This feature SHALL comply with [[cli-distribution-policy]].

## Terminology

- **Pinned version** — the version in the plugin's manifest, which is the
  CLI release the plugin was published with.
- **Provisioning** — making the pinned CLI runnable as `release` in an
  agent session: finding, fetching, verifying and placing the binary.
- **Cloud session** — an agent session in a managed, ephemeral container
  (Claude Code on the web); anything else is a **local session**.
- **Plugin data directory** — the per-plugin directory the agent harness
  keeps across sessions and plugin updates (`${CLAUDE_PLUGIN_DATA}`, under
  the person's home directory in Claude Code). It is shared by every
  repository that enables the plugin on that machine.
- **Reported version** — what `release --version` prints after
  `release `: `v0.2.0` for a published build, `devel` for a build from a
  working tree. A CLI is at the pinned version when its reported version is
  `v` followed by the pinned version.

## Requirements

### FEAT-PLG-001: The Pinned CLI When a Skill Needs It

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-001
Feature: The Pinned CLI When a Skill Needs It

  As an agent working in a repository
  I want the release CLI provisioned when a release skill needs it
  So that I can cut a release without installing anything first

  The version provisioned is the plugin's own, never the latest release.
  Fetching is the slow path: a session that already holds the pinned
  version reaches it without the network.

  @AC-PLG-001.1
  Scenario Outline: The pinned build for the platform is provisioned
    Given the plugin's pinned version is "0.2.0"
    And the session runs on "<os>" "<arch>"
    And no release CLI is provisioned
    When a release skill runs
    Then the release command reports version "0.2.0"
    And the binary came from the "<archive>" release archive

    Examples:
      | os      | arch  | archive                             |
      | linux   | amd64 | release_0.2.0_linux_amd64.tar.gz    |
      | linux   | arm64 | release_0.2.0_linux_arm64.tar.gz    |
      | darwin  | amd64 | release_0.2.0_darwin_amd64.tar.gz   |
      | darwin  | arm64 | release_0.2.0_darwin_arm64.tar.gz   |
      | windows | amd64 | release_0.2.0_windows_amd64.zip     |
      | windows | arm64 | release_0.2.0_windows_arm64.zip     |

  @AC-PLG-001.2
  Scenario: A provisioned pinned version is reused without the network
    Given the pinned version is already in the plugin data directory
    And the network is unavailable
    When a release skill runs
    Then the release command reports the pinned version

  @AC-PLG-001.3
  Scenario: A plugin update provisions its own version
    Given the plugin data directory holds version "0.1.0"
    And the plugin is updated to pinned version "0.2.0"
    When a release skill runs
    Then the release command reports version "0.2.0"

  @AC-PLG-001.4
  Scenario: Every CLI step in the skill runs the provisioned binary
    Given the pinned CLI was provisioned by a release skill
    When the skill runs a release command
    Then the command runs the provisioned binary of the pinned version

  @AC-PLG-001.7 @negative
  Scenario: A session that runs no release skill provisions nothing
    Given a session in which no release skill runs
    Then nothing is fetched and nothing is placed for the CLI

  @AC-PLG-001.5 @negative
  Scenario: An archive failing its checksum is never executed
    Given the downloaded archive does not match its entry in SHA256SUMS
    When a release skill runs
    Then the archive is discarded without being extracted or run
    And the session reports a checksum mismatch for that archive

  @AC-PLG-001.6 @negative
  Scenario Outline: A provisioning failure stops only the skill
    Given provisioning fails because "<cause>"
    When a release skill runs
    Then the skill stops before running any release command
    And one message names the cause and its remedy

    Examples:
      | cause                                    |
      | the platform has no release archive      |
      | the release cannot be reached            |
      | no credential can read the repository    |
      | the archive fails its checksum           |
```

### FEAT-PLG-002: Where the CLI Is Installed

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-002
Feature: Where the CLI Is Installed

  As a person whose machine runs the session
  I want provisioning to install where it is appropriate for that machine
  So that a session never surprises me with changes to my system

  A cloud container is disposable and the session is its only user; a
  local machine is the person's own, with their package manager and their
  PATH. Provisioning treats them differently for that reason.

  @AC-PLG-002.1
  Scenario: A cloud session provisions into the plugin data directory
    Given a cloud session
    When a release skill runs
    Then the pinned CLI is placed under the plugin data directory
    And it is runnable as "release" from any process in the session

  @AC-PLG-002.2 @negative
  Scenario: A local session changes nothing outside the plugin data directory
    Given a local session
    When a release skill runs
    Then no file outside the plugin data directory is created or changed
    And no system package is installed or upgraded

  @AC-PLG-002.3
  Scenario: A release CLI already provisioned for the pinned version is reused
    Given the plugin's cached copy of the pinned version is provisioned
    When a release skill runs
    Then provisioning fetches nothing and places nothing

  @AC-PLG-002.5 @negative
  Scenario Outline: A release CLI on the path is never run
    Given "release" on the path reports version "<reported>"
    And the plugin's pinned version is "0.2.0"
    When a release skill runs a release command
    Then the command runs the pinned version "0.2.0"
    And the "release" on the path is never run
    And the "release" on the path is left unchanged

    Examples:
      | reported |
      | 0.1.0    |
      | 0.2.0    |

  @AC-PLG-002.4
  Scenario: A repository can direct the skills to its own build
    Given the repository's agent settings name its own release binary
    When a release skill runs
    Then provisioning fetches nothing and places nothing
    And the skill runs the named binary
```

A local session uses a session-only copy of the pinned version, even where
the machine also has the CLI from a package manager at another version.
A Homebrew formula, once published, serves the person's own terminal and
is not a provisioning route ([[adr-008]]).

### FEAT-PLG-003: Retrieval While the Repository Is Private

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-003
Feature: Retrieval From a Private Repository

  As a maintainer of a private release repository
  I want provisioning to use the credentials the session already has
  So that the CLI reaches sessions without a new secret per machine

  The release assets of a private repository are not anonymously
  readable. Provisioning uses an existing credential rather than asking
  for a new one, and never widens where a credential goes. Once the
  repository is public, no credential is needed.

  @AC-PLG-003.1
  Scenario Outline: The first available credential is used
    Given "<source>" holds a credential that can read the repository
    And no source earlier in the order holds one
    When a release skill runs
    Then the pinned CLI is provisioned using that credential

    Examples:
      | source                                    |
      | the GH_TOKEN environment variable         |
      | the GITHUB_TOKEN environment variable     |
      | the GitHub CLI's stored login             |
      | the session's own repository access       |

  @AC-PLG-003.4
  Scenario: A public repository's release needs no credential
    Given the release repository is public
    And no source holds a credential
    When a release skill runs
    Then the pinned CLI is provisioned

  @AC-PLG-003.2 @negative
  Scenario: A credential is never recorded
    Given provisioning used a credential
    Then no credential appears in the session's output
    And no credential is written to disk by provisioning

  @AC-PLG-003.3 @negative
  Scenario: A credential is sent only to the repository's host
    Given provisioning used a credential
    Then the credential was sent to no host other than the GitHub API
```

### FEAT-PLG-004: One Version for Plugin and CLI

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-004
Feature: One Version for Plugin and CLI

  As a maintainer
  I want the plugin and the CLI released together under one version
  So that a skill always describes the binary its plugin provisions

  The release commit is where the two are bound: the plugin manifest's
  version is written by the same cut that writes the changelog. The
  marketplace that vends the plugin serves the latest published release.

  @AC-PLG-004.1
  Scenario: Cutting a release writes the plugin's version
    Given the repository's release configuration
    When a release of version "0.2.0" is cut
    Then the release commit sets the plugin manifest's version to "0.2.0"

  @AC-PLG-004.2 @negative
  Scenario: A release whose plugin version disagrees is not published
    Given a stage pull request for version "0.2.0"
    And the plugin manifest on its merge commit names another version
    When the stage pull request merges
    Then no tag is created and no release is published
    And the publication run reports the disagreement

  @AC-PLG-004.3 @manual
  Scenario: Publishing a release makes it the one the marketplace serves
    Given the marketplace serves the plugin at version "0.2.0"
    When version "0.3.0" is published with every asset attached
    Then the marketplace serves the plugin as tagged "v0.3.0"
    And the marketplace's own file is unchanged
```

### FEAT-PLG-005: Cutting a Release From a Skill

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-005
Feature: Cutting a Release From a Skill

  As an agent with no prior knowledge of the release CLI
  I want a skill that teaches me to cut a release with it
  So that a person can ask me to cut a release and get a stage pull request

  The skill teaches the workflow and its judgement calls; the command
  surface itself is the CLI's own help, which is normative under
  [[contract-release-cli]] and so cannot drift from the binary.

  @AC-PLG-005.1 @manual
  Scenario: A release is cut end to end from the skill
    Given a repository configured for the release CLI
    And an agent whose only guidance is the cut-release skill
    When a person asks the agent to cut a release
    Then the agent previews the version and notes before writing
    And the agent opens a stage pull request for that version

  @AC-PLG-005.2 @manual
  Scenario: A provisioning failure is reported, not worked around
    Given provisioning the pinned version fails
    When the cut-release skill runs
    Then the agent reports the cause and its remedy
    And the agent does not build, download or substitute the CLI another way

  @AC-PLG-005.3 @manual @negative
  Scenario: The skill never tags or publishes
    When the cut-release skill runs
    Then the agent creates no tag and publishes no release
```

### FEAT-PLG-006: Release-Management Standards

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-006
Feature: Release-Management Standards

  As an agent setting up or reviewing a repository's release process
  I want the release-management standards built into the cut-release skill
  So that my advice follows the same rules the release CLI enforces

  The plugin has one skill for users, cut-release, and the standards are
  its references rather than a skill of their own. They are copies of
  this corpus's standards rather than a paraphrase, so the advice and the
  specification cannot disagree. The
  first version covers the command-line tool or library and the
  continuously deployed service archetypes; web and mobile apps follow.

  @AC-PLG-006.1
  Scenario Outline: Each standard ships unchanged
    Given the corpus standard "<standard>"
    Then the cut-release skill carries a byte-identical copy of it

    Examples:
      | standard                      |
      | repository-release-policy.md  |
      | release-tooling-standards.md  |
      | cli-distribution-policy.md    |

  @AC-PLG-006.2 @negative
  Scenario: A standard edited without its plugin copy fails the checks
    Given a corpus standard differs from its plugin copy
    When the repository's checks run
    Then the checks fail and name the standard
```

### FEAT-PLG-007: Adoption and Updates Through the Marketplace

**Acceptance Criteria:**

```gherkin
@FEAT-PLG-007
Feature: Adoption and Updates Through the Marketplace

  As a maintainer adopting the release process in a repository
  I want to enable the plugin once, in the repository's shared settings
  So that every session there has it, and new releases reach it unaided

  The plugin is enabled at project scope, so the repository rather than
  each person decides it is in use. Releases reach installed plugins
  through the marketplace; the plugin then provisions the CLI it pins.

  @AC-PLG-007.1
  Scenario: A repository enables the plugin for every session in it
    Given a repository whose shared agent settings add the brettcallaghan marketplace
    And enable the release plugin
    When a person starts an agent session in that repository
    Then the cut-release skill is available in the session

  @AC-PLG-007.2
  Scenario: A published release reaches the repository through the marketplace
    Given a repository with the release plugin enabled at version "0.2.0"
    And marketplace updates are turned on for the person
    When version "0.3.0" is published
    Then a later session has the plugin at version "0.3.0"
    And the next release skill provisions CLI version "0.3.0"

  @AC-PLG-007.3 @manual
  Scenario: A cloud session receives the plugin from the account
    Given the release plugin is added to the plugins synced to the person's account
    When the person starts a cloud session
    Then the cut-release skill is available in the session
```

The marketplace is `brettcallaghan`, in `brettcallaghan/agent-plugins`,
which vends all of Brett's agent plugins; it serves this plugin from the
`release/v<major>` branch of the current major, which names that major's
latest published release, and its `specs`
dependency alongside it.

A cloud session does not load plugins that a repository's settings enable;
it loads the plugins synced to the person's account, and that sync follows
the marketplace. Adding the plugin to the account is a one-time step per
account ([[adr-008]]).

## References

- [[adr-008]] — why the plugin provisions a pinned release, and where it
  installs it
- [[cli-distribution-policy]] — CDP-014, verifying a download before
  running it, which AC-PLG-001.5 applies to provisioning
- [[contract-release-cli]] — the help output the cut-release skill defers
  to
- [[frd-release-staging]] — the cut the skill drives
