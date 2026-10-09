---
title: "FRD: Version Computation"
description: Resolving a release line's base version, classifying release
  impact from reviewed signals in git, reconciling it with an authored
  proposal and a person's confirmation, computing the next version under
  policy, and deriving the names a release uses.
slug: frd-version-computation
type: frd
status: draft
updated_at: 2026-10-09T00:00:00Z
sources:
  - adr-001
  - adr-003
  - frd-changelog-authoring
  - release-tooling-standards
---

# Version Computation

## Overview

Deciding a release's version number is the one part of releasing that
cannot be corrected after the fact. Prose can be edited and artifacts can
be rebuilt, but a published version is consumed by resolvers, installers
and package manifests, and withdrawing one is the only remedy. So this
capability is specified on its own, computes from git alone, and is a pure
function of its inputs wherever it can be.

It answers three questions in sequence — what was last released on this
line, what impact have the changes since had, and what version does that
make next — and derives every name a release needs from the answer.

The impact is computed deterministically from reviewed signals in git: a
notes-block impact reviewed in a pull request, a `Release-Type:` trailer, a
conventional-commit type, or a breaking-change marker. A producer that has
read the evidence also proposes an impact, with its reasons. The two are
reconciled: where every commit carries a reviewed signal and the proposal is
no higher, the computed impact stands. Where a commit carries none, or the
proposal is higher, a person confirms the impact before anything is written,
and the confirmation is committed as a `Release-Type:` trailer on the release
commit, so the version stays reproducible from a clone ([[adr-003]]). It
answers them for whatever branch it is pointed at, without interpreting the
branch's name ([[adr-001]]). A maintainer can run it without staging
anything, which makes it both the first useful command and the safest way to
check the tooling's judgement against their own.

This feature SHALL comply with [[release-tooling-standards]].

## Terminology

- **Release line** — any branch a project releases from. The tooling does
  not interpret which major a branch carries.
- **Base version** — the latest release already published on a line,
  resolved from the tags reachable from its branch.
- **Impact** — the release significance of a set of changes: major, minor,
  patch, or none.
- **Signal** — a reviewed record in git that indicates a commit's impact:
  the `Release-Type:` line of a notes block in the commit body, a
  release-type trailer, a conventional-commit type, or a breaking-change
  marker.
- **Classified commit** — a commit carrying at least one enabled signal,
  including one that indicates an impact of none. An **unclassified commit**
  carries no signal.
- **Computed impact** — the highest impact the signals in a range indicate.
- **Proposal** — the impact an authored corpus proposes, with its rationale
  and the sources that drove it ([[frd-changelog-authoring]]).
- **Confirmed impact** — the impact the release uses, after reconciliation.
- **Releasable** — a range is releasable WHEN its signals indicate an impact
  of patch or above, or its authored corpus carries at least one
  user-visible entry: one whose kind the project publishes, or one marked
  breaking.
- **Leap** — an increment of a version component by more than one.
- **Graduation** — the deliberate move from a version below 1.0.0 to 1.0.0.
- **Tag prefix** — the string a project prepends to a version when tagging,
  `v` by default. It is not part of the version.
- **Prerelease** — a preview of a version the project plans to publish, which
  orders before it and graduates to it: `1.9.0-rc.1`.
- **Canary** — an artifact for an arbitrary commit, which previews no planned
  version and never graduates: `1.8.0-canary.412.a3f4b2c`.

## Requirements

### FEAT-VER-001: Base Version Resolution

**Acceptance Criteria:**

```gherkin
@FEAT-VER-001
Feature: Base Version Resolution

  As a maintainer
  I want the base version read from the tags my branch can reach
  So that a release line's history needs no configuration to describe it

  Reachability is what separates release lines. A maintenance branch cut
  before the next major's first tag cannot reach that tag, so a query scoped
  to a branch is already scoped to its major and no per-major filter is
  needed.

  @AC-VER-001.1
  Scenario: The base version is the highest reachable release tag
    Given the tags 1.6.0, 1.7.0 and 1.8.0 are reachable from the target branch
    When the tooling resolves the base version
    Then the base version is 1.8.0

  @AC-VER-001.2
  Scenario: A tag that is not reachable is not considered
    Given the tag 2.0.0 exists in the repository and is not reachable from the target branch
    And the highest reachable release tag is 1.8.0
    When the tooling resolves the base version
    Then the base version is 1.8.0

  @AC-VER-001.3
  Scenario: A tag that is not a release tag is ignored
    Given a tag named "nightly-2026-10-01" is reachable from the target branch
    And the highest reachable release tag is 1.8.0
    When the tooling resolves the base version
    Then the base version is 1.8.0

  @AC-VER-001.4
  Scenario: A line with no release yet resolves to 0.0.0
    Given no release tag is reachable from the target branch
    When the tooling resolves the base version
    Then the base version is 0.0.0

  @AC-VER-001.6
  Scenario: The first release is 0.0.0
    Given no release tag is reachable from the target branch
    When the tooling computes the next version for an impact of minor
    Then the next version is 0.0.0

  @AC-VER-001.8
  Scenario: A release after 0.0.0 increments as usual
    Given the highest reachable release tag is 0.0.0
    When the tooling computes the next version for an impact of patch
    Then the next version is 0.0.1

  @AC-VER-001.7
  Scenario: A tag is recognised with or without the configured prefix
    Given the repository configures the tag prefix "v"
    And the tag v1.8.0 is reachable from the target branch
    When the tooling resolves the base version
    Then the base version is 1.8.0

  @AC-VER-001.5 @negative
  Scenario: The branch name does not determine the major
    Given the target branch is named "release/v1" and its highest reachable release tag is 2.3.0
    When the tooling resolves the base version
    Then the tooling does not constrain the base version by the major named in the branch
```

### FEAT-VER-002: Release Impact Classification

**Acceptance Criteria:**

```gherkin
@FEAT-VER-002
Feature: Release Impact Classification

  As a maintainer
  I want impact computed from reviewed records that live in git
  So that the version is reproducible from a clone and checkable by a reviewer

  Every enabled signal is read across every commit in the range and the
  highest impact found wins. A notes block's impact was reviewed with the
  pull request it describes, so it is a signal like a trailer. The commits
  that produced the answer are reported with it, because a version a
  maintainer disagrees with is otherwise undiagnosable and the usual cause is
  one mis-typed commit. The commits that carry no signal are reported too,
  because nothing reviewed vouches for their impact.

  @AC-VER-002.1
  Scenario Outline: An enabled signal maps to an impact
    Given a commit carrying "<signal>"
    When the tooling classifies impact
    Then the computed impact is "<impact>"

    Examples:
      | signal                            | impact |
      | a notes block reading Release-Type: minor | minor  |
      | a trailer reading Release-Type: minor | minor  |
      | a conventional-commit type of feat | minor  |
      | a conventional-commit type of fix  | patch  |
      | a breaking-change marker          | major  |

  @AC-VER-002.2
  Scenario: The highest impact across the range wins
    Given one commit in the range indicates patch and another indicates minor
    When the tooling classifies impact
    Then the computed impact is minor

  @AC-VER-002.3
  Scenario: The commits that drove the result are reported
    Given a range in which exactly one commit carries a breaking-change marker
    When the tooling classifies impact
    Then the tooling reports that commit as having determined the result

  @AC-VER-002.4
  Scenario: A range with no signal yields no impact
    Given no commit in the range carries an enabled signal
    When the tooling classifies impact
    Then the computed impact is none

  @AC-VER-002.6
  Scenario: Commits without a signal are reported as unclassified
    Given a range of three commits of which one carries no enabled signal
    When the tooling classifies impact
    Then the tooling reports that commit as unclassified

  @AC-VER-002.7
  Scenario: A signal of none classifies a commit
    Given a commit whose notes block reads Release-Type: none
    When the tooling classifies impact
    Then the commit is classified with an impact of none and is not reported as unclassified

  @AC-VER-002.5 @negative
  Scenario: Classification ignores any authored proposal
    Given an authored changelog proposing an impact of major
    And no commit in the range carries a signal above patch
    When the tooling classifies impact
    Then the computed impact is patch
```

### FEAT-VER-005: Impact Reconciliation

**Acceptance Criteria:**

```gherkin
@FEAT-VER-005
Feature: Impact Reconciliation

  As a maintainer
  I want the computed impact, the authored proposal and my own judgement reconciled before anything is written
  So that a model's reading of the evidence informs the version without ever deciding it unreviewed

  Reconciliation takes the computed impact, the commits that carry no signal,
  the proposal with its rationale and drivers, and any bump the maintainer
  named. Where every commit is classified and the proposal is no higher than
  the computed impact, the computed impact stands without a question: reviewed
  records decide. Where any commit is unclassified, or the proposal is higher,
  a person confirms the impact. Interactively the tooling asks. Otherwise it
  stops before writing anything and shows what it would have asked, so an
  agent confirms by naming the bump after showing the proposal to the person
  it acts for. A named bump is itself a confirmation.

  @AC-VER-005.1
  Scenario: Classified commits and an agreeing proposal need no confirmation
    Given every commit in the range is classified
    And the computed impact is minor
    And the proposal is minor
    When the tooling reconciles the impact
    Then the confirmed impact is minor and no confirmation is requested

  @AC-VER-005.2
  Scenario: A lower proposal does not lower the computed impact
    Given every commit in the range is classified
    And the computed impact is minor
    And the proposal is patch
    When the tooling reconciles the impact
    Then the confirmed impact is minor, no confirmation is requested, and the difference is reported

  @AC-VER-005.3
  Scenario: A higher proposal requires confirmation
    Given every commit in the range is classified
    And the computed impact is patch
    And the proposal is minor
    And the maintainer names no bump
    When the tooling reconciles the impact
    Then the tooling requires a person to confirm the impact

  @AC-VER-005.4
  Scenario: An unclassified commit requires confirmation
    Given one commit in the range is unclassified
    And the proposal is no higher than the computed impact
    And the maintainer names no bump
    When the tooling reconciles the impact
    Then the tooling requires a person to confirm the impact

  @AC-VER-005.5
  Scenario: The person confirming sees the proposal and its reasons
    Given confirmation is required
    And the tooling is running interactively
    When the tooling asks for confirmation
    Then it shows the computed impact, the proposal, the proposal's rationale and drivers, and the unclassified commits
    And the impact the person chooses is the confirmed impact

  @AC-VER-005.6
  Scenario: A named bump confirms the impact
    Given one commit in the range is unclassified
    And the proposal is major
    And the maintainer names a bump of minor
    When the tooling reconciles the impact
    Then the confirmed impact is minor and no confirmation is requested

  @AC-VER-005.7 @negative
  Scenario: Non-interactively, needed confirmation stops without writing
    Given confirmation is required
    And the tooling is not running interactively
    And the maintainer names no bump
    When the tooling reconciles the impact
    Then the tooling stops without writing to the repository
    And it shows the computed impact, the proposal, the proposal's rationale and drivers, the unclassified commits, and how to confirm by naming a bump

  @AC-VER-005.8
  Scenario: The authored corpus can make a range releasable
    Given no commit in the range carries a signal above none
    And the authored corpus carries an entry of the kind feature
    When the tooling decides whether the range is releasable
    Then the range is releasable

  @AC-VER-005.9
  Scenario: A range with neither a signal nor a user-visible entry is not releasable
    Given no commit in the range carries a signal above none
    And the authored corpus carries only entries the project does not publish
    When the tooling decides whether the range is releasable
    Then the range is not releasable

  @AC-VER-005.10
  Scenario: An empty range is not releasable
    Given no commit exists since the base version
    When the tooling decides whether the range is releasable
    Then the range is not releasable
```

### FEAT-VER-003: Next Version Computation

**Acceptance Criteria:**

```gherkin
@FEAT-VER-003
Feature: Next Version Computation

  As a maintainer
  I want version policy applied in one place
  So that every rule the project has decided is enforced identically everywhere

  Two rules govern the result and both refuse rather than warn. One component
  moves by one unless leaps are enabled, and below 1.0.0 exactly one mapping
  shifts: a major bump moves the minor, so breaking changes stay inside the 0.x
  series and reaching 1.0.0 needs an explicit graduation. Minor and patch bumps
  behave below 1.0.0 as they do above it. No rule is conditioned on which branch
  is being released from.

  @AC-VER-003.1
  Scenario Outline: An impact increments exactly one component by one
    Given a base version of 1.8.3
    When the tooling computes the next version for an impact of "<impact>"
    Then the next version is "<version>"

    Examples:
      | impact | version |
      | major  | 2.0.0   |
      | minor  | 1.9.0   |
      | patch  | 1.8.4   |

  @AC-VER-003.2 @negative
  Scenario: A leap is refused unless enabled by policy
    Given a base version of 1.8.3 and a policy that does not permit leaps
    When a version of 1.10.0 is proposed
    Then the tooling refuses the version

  @AC-VER-003.3
  Scenario: A leap is permitted when policy enables it
    Given a base version of 1.8.3 and a policy that permits leaps
    When a version of 1.10.0 is proposed
    Then the tooling accepts the version

  @AC-VER-003.4
  Scenario Outline: Below 1.0.0 the impact mapping shifts
    Given a base version of 0.6.4
    When the tooling computes the next version for an impact of "<impact>"
    Then the next version is "<version>"

    Examples:
      | impact | version |
      | major  | 0.7.0   |
      | minor  | 0.7.0   |
      | patch  | 0.6.5   |

  @AC-VER-003.5
  Scenario: Reaching 1.0.0 requires an explicit graduation
    Given a base version of 0.6.4
    When the maintainer graduates the version
    Then the next version is 1.0.0

  @AC-VER-003.6 @negative
  Scenario: An impact alone never produces 1.0.0
    Given a base version of 0.6.4 and no graduation
    When the tooling computes the next version for an impact of major
    Then the next version does not have a major component of 1

  @AC-VER-003.7
  Scenario: No impact means no release
    Given a confirmed impact of none
    When the tooling computes the next version
    Then the tooling reports that no release is warranted rather than returning a version
```

### FEAT-VER-004: Release Name Derivation

**Acceptance Criteria:**

```gherkin
@FEAT-VER-004
Feature: Release Name Derivation

  As a maintainer
  I want every name a release uses derived from one version
  So that a tag and the branch that produced it agree by construction

  Verification cross-checks a stage branch name against the version recorded
  in its commit. That check only means something if one component generated
  both forms, so derivation and its inverse parse live together and are
  tested as a round trip.

  @AC-VER-004.1
  Scenario Outline: A version derives each of its names
    Given the version 1.9.0
    When the tooling derives the "<form>"
    Then the result is "<name>"

    Examples:
      | form         | name       |
      | tag name     | v1.9.0     |
      | stage branch | cut/1.9.0  |

  @AC-VER-004.2
  Scenario: A derived name parses back to its version
    Given a stage branch name derived from the version 1.9.0
    When the tooling parses the version out of that branch name
    Then the parsed version is 1.9.0

  @AC-VER-004.3
  Scenario: The configured stage prefix is used
    Given the repository configures the stage prefix "release-prep/"
    When the tooling derives the stage branch name for version 1.9.0
    Then the result is "release-prep/1.9.0"

  @AC-VER-004.9
  Scenario: The configured tag prefix is used
    Given the repository configures the tag prefix "" with no prefix string
    When the tooling derives the tag name for version 1.9.0
    Then the result is "1.9.0"

  @AC-VER-004.4
  Scenario: A canary identifier carries no planned version
    When the tooling derives a canary identifier for a commit
    Then the identifier is not a prerelease of any version the project plans to publish

  @AC-VER-004.5 @negative
  Scenario: A tag prefix is not part of a version
    When the string "v1.9.0" is parsed as a version rather than as a tag name
    Then the tooling does not accept it

  @AC-VER-004.6
  Scenario: A prerelease names the version it previews and its ordinal
    Given a planned version of 1.9.0 and a prerelease ordinal of 1
    When the tooling derives a prerelease version
    Then the result is "1.9.0-rc.1"

  @AC-VER-004.7
  Scenario: A prerelease orders before the version it previews
    Given a prerelease identifier derived for the planned version 1.9.0
    When the tooling compares it against 1.9.0
    Then the prerelease orders before 1.9.0

  @AC-VER-004.8
  Scenario Outline: A canary names the current version, its pull request and its commit
    Given the latest released version is 1.8.0
    When the tooling derives a canary version for pull request "<pr>" at commit "<sha>"
    Then the result is "<version>"

    Examples:
      | pr  | sha     | version                    |
      | 412 | a3f4b2c | 1.8.0-canary.412.a3f4b2c   |
      | 87  | 9de01ff | 1.8.0-canary.87.9de01ff    |

  @AC-VER-004.10 @negative
  Scenario: A canary does not preview a planned version
    Given the latest released version is 1.8.0
    When the tooling derives a canary version
    Then the result is not a prerelease of any version the project plans to publish
```

## References

- [[adr-001]] — the tooling is branch-agnostic; reachability distinguishes
  release lines
- [[adr-003]] — reviewed records in git decide the impact; a person
  confirms anything unreviewed, and the confirmation is committed
- [[frd-changelog-authoring]] — the proposal and its reasons
- [[release-tooling-standards]] — the policy these criteria enforce
- [[component-semver-core]], [[component-tag-reader]],
  [[component-bump-classifier]], [[component-version-calculator]] — the
  components realizing this feature
