---
title: "FRD: Changelog Authoring"
description: Collecting the evidence of what changed — commit records, file
  lists and budgeted diffs — and exchanging it with an external producer that
  composes release notes and pull-request notes and proposes an impact with
  its reasons, which the tooling validates and reconciles rather than trusts.
slug: frd-changelog-authoring
type: frd
status: draft
updated_at: 2026-10-09T00:00:00Z
sources:
  - adr-003
  - adr-004
  - adr-005
  - contract-changelog
  - release-tooling-standards
  - repository-release-policy
---

# Changelog Authoring

## Overview

The code is the honest record of what changed. Release notes assembled from
commit subjects read like a commit log, because that is what they are. Good
notes say what changed from the reader's point of view, group related work,
leave out what no reader can observe, and explain what to do about anything
that breaks. That is synthesis, and it needs the evidence: commit bodies,
the files each commit touched, and the diffs themselves.

This feature collects that evidence and hands it to a producer that can do
the synthesis, then checks what comes back. It does so at two points:

- **At pull request time**, `release notes` gives the producer the pull
  request's commits and diff. The producer writes a `## Release notes` block
  for the pull request description: one entry per user-visible change and a
  proposed impact. The reviewer reviews the block with the code, and the
  squash merge carries it into the commit body.
- **At release time**, the tooling collects every commit since the base
  version with its reviewed notes block, or its diff where it has none. The
  producer composes the release changelog from that evidence and proposes an
  impact with its reasons.

The producer is outside the tooling: a maintainer with a language model on
their workstation, a skill driving one, a configured command, or a person in
an editor. The tooling never calls a model, so no adopter needs an API
credential to release. A proposal never becomes a version on its own; it is
reconciled against the reviewed signals in git as [[frd-version-computation]]
specifies ([[adr-003]]).

This feature SHALL comply with [[release-tooling-standards]],
[[repository-release-policy]] and [[contract-changelog]].

## Terminology

- **Evidence bundle** — what a producer reads: the commits in a range, each
  with its record, its parsed notes block where it has one, and its diff
  where one was collected, plus a report of what the diff budget left out.
  Defined by [[contract-changelog]].
- **Notes block** — the `## Release notes` section of a pull request
  description, and therefore of the squash commit body, in the grammar
  [[contract-changelog]] defines. It carries a proposed impact and one entry
  per user-visible change, grouped under the repository's changelog headings
  as the changelog section will show them.
- **Pull request notes** — the content of a notes block as structured data:
  the impact and the entries.
- **Diff budget** — the most patch text, in bytes, one evidence bundle may
  carry. Diffs beyond it are truncated or omitted, and the omission is
  recorded.
- **Corpus** — the structured release changelog defined by
  [[contract-changelog]].
- **Authored corpus** — a corpus returned by a producer, carrying entries, a
  proposed impact and the reasons for it.
- **Producer** — whatever authors a corpus or pull request notes: a language
  model driven by a skill, a configured command, the mechanical producer, or
  a person editing a file.
- **Mechanical producer** — the producer of last resort, built into the
  tooling. It composes entries from notes blocks and conventional commit
  subjects without judgement, and serves releases without notes and
  model-free automation.
- **Composition** — deciding what the entries are: splitting one commit into
  several changes, merging several commits into one change, omitting
  internal-only work, and setting each entry's kind.
- **Adapter** — one source of evidence: commits, diffs, or pull-request
  metadata from the hosting platform.

## Requirements

### FEAT-CLOG-001: Evidence Collection

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-001
Feature: Evidence Collection

  As a maintainer
  I want what each commit says and what it changed collected for the producer
  So that notes are authored from the change itself rather than from commit subjects

  Commit records are always collected, with their full bodies, trailers and
  the list of files each changed. Diffs are part of the default path: they are
  the only honest record of a change nobody described. They are collected
  within a configurable byte budget, with lockfiles, vendored and generated
  paths excluded, and whatever the budget leaves out is reported rather than
  dropped silently. Pull-request metadata from the hosting platform is an
  optional fallback for commits that carry no notes block, because squash
  merging already puts the title and description in git.

  @AC-CLOG-001.1
  Scenario: A pull request number is read from a squash commit subject
    Given a squash commit whose subject ends with a pull request reference
    When the tooling collects commit records
    Then the record carries that pull request number

  @AC-CLOG-001.2
  Scenario: A commit's full body is collected
    Given a squash commit whose body is the pull request description
    When the tooling collects commit records
    Then the record carries that body in full without consulting the hosting platform

  @AC-CLOG-001.8
  Scenario: Every commit's changed files are collected
    Given a commit that modifies one file, adds another and renames a third
    When the tooling collects commit records
    Then the record lists each file with its status, its previous path where renamed, and its added and deleted line counts

  @AC-CLOG-001.6
  Scenario: A commit with no pull request is still collected
    Given a commit pushed directly to the release line, carrying no pull request number
    When the tooling collects commit records
    Then the record is present with its pull request number absent

  @AC-CLOG-001.9
  Scenario: Diffs are collected by default
    Given the repository configures nothing about diffs
    And a commit in the range carries no notes block
    When the tooling collects evidence for a release
    Then the evidence carries that commit's diff

  @AC-CLOG-001.10
  Scenario: Excluded paths are left out of collected diffs
    Given a commit that modifies a lockfile and a source file
    When the tooling collects that commit's diff
    Then the diff carries the source file's patch and not the lockfile's
    And the commit's file list still names the lockfile

  @AC-CLOG-001.11
  Scenario: The excluded paths are configurable
    Given the repository configures "gen/**" as an excluded path
    And a commit that modifies "gen/bindings.go" and "cmd/main.go"
    When the tooling collects that commit's diff
    Then the diff carries the patch for "cmd/main.go" only

  @AC-CLOG-001.12
  Scenario: Diffs beyond the budget are truncated and the truncation is recorded
    Given the repository configures a diff budget smaller than the range's diffs
    When the tooling collects evidence
    Then the collected patch text does not exceed the budget
    And every diff or file the budget cut short is marked as truncated

  @AC-CLOG-001.3
  Scenario: Missing descriptions are fetched from the hosting platform when enabled
    Given the pull-request adapter is enabled
    And a commit carrying a pull request number and no notes block in its body
    When the tooling collects evidence
    Then the tooling fetches the title and description for that pull request from the hosting platform

  @AC-CLOG-001.4 @negative
  Scenario: Fetched metadata never overwrites what the commit carried
    Given the pull-request adapter is enabled
    And a commit whose body already carries a pull request description
    When the tooling collects evidence with the hosting platform reachable
    Then the tooling does not replace the description carried in the commit

  @AC-CLOG-001.5
  Scenario: An unreachable hosting platform degrades rather than failing
    Given the pull-request adapter is enabled
    And the hosting platform is unreachable
    When the tooling collects evidence
    Then the tooling returns the commit records and diffs unchanged and reports the degradation

  @AC-CLOG-001.13 @negative
  Scenario: The hosting platform is not consulted by default
    Given the repository has not enabled the pull-request adapter
    When the tooling collects evidence
    Then the tooling makes no request to the hosting platform
```

### FEAT-CLOG-002: Evidence Bundle

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-002
Feature: Evidence Bundle

  As a maintainer
  I want the producer to receive everything collected, uninterpreted
  So that the producer judges the change from the evidence and not from a draft the tooling already cut up

  The evidence bundle replaces a pre-assembled draft as the producer's input.
  It carries every commit in the range, oldest first, with what git said about
  it, the parsed notes block where one exists, and the diff where one was
  collected. It never classifies a change and never authors one: an entry, a
  kind or an impact in the bundle would be a guess presented as data. It is
  kept with the authored result, so a reviewer can see what the producer was
  shown.

  @AC-CLOG-002.1
  Scenario: The evidence bundle validates against the schema
    When the tooling builds an evidence bundle
    Then the bundle validates against the evidence schema the changelog contract names

  @AC-CLOG-002.6
  Scenario: Every commit in the range is carried, oldest first
    Given a range of three commits
    When the tooling builds an evidence bundle for a release
    Then the bundle carries all three commits, oldest first, each with its subject, body, trailers and changed files

  @AC-CLOG-002.7
  Scenario: The bundle states its purpose and range
    Given a release from base version 1.8.0
    When the tooling builds an evidence bundle for the release
    Then the bundle names its purpose as a release, its range, and the base version 1.8.0

  @AC-CLOG-002.4
  Scenario: Every contributing adapter is recorded
    Given the commit and diff adapters both contributed evidence
    When the tooling builds an evidence bundle
    Then the bundle records both adapters and the range each read

  @AC-CLOG-002.8
  Scenario: The budget report records what was left out
    Given the range's diffs exceed the diff budget
    When the tooling builds an evidence bundle
    Then the bundle's budget report states the bytes carried, the files omitted or truncated, and why each was

  @AC-CLOG-002.5 @negative
  Scenario: The bundle proposes no impact and authors no entry
    When the tooling builds an evidence bundle
    Then the bundle carries no proposed impact and no changelog entry

  @AC-CLOG-002.9 @negative
  Scenario: The bundle assigns no kind to a commit
    Given a commit whose subject carries no conventional-commit type
    When the tooling builds an evidence bundle
    Then the bundle assigns that commit no change kind

  @AC-CLOG-002.10
  Scenario: The evidence bundle is kept with the authored result
    Given an authoring exchange has completed for a range
    When a reviewer inspects the cached result for that range
    Then the evidence bundle the producer was given is available alongside it
```

### FEAT-CLOG-003: Authoring Exchange

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-003
Feature: Authoring Exchange

  As a maintainer
  I want the tooling to exchange evidence with a producer and check what returns
  So that generated prose is reviewed and a generated impact never becomes a version unconfirmed

  The exchange is files, not calls. Producers are plugins and the default one
  runs outside the tooling, which is why no adopter needs an API credential to
  cut a release. The producer reads the evidence bundle and returns an
  authored corpus. What returns is validated against the schema and checked
  against the evidence it cites, and its proposed impact is reconciled against
  the impact computed from reviewed signals, as the version computation
  feature specifies. Authoring is slow and may be paid for, so the result is
  cached against the commit range it describes.

  @AC-CLOG-003.12
  Scenario: The producer receives the evidence bundle
    Given a commit whose body describes four changes
    When the tooling performs an authoring exchange
    Then the producer's input is an evidence bundle carrying that commit's body in full

  @AC-CLOG-003.1
  Scenario: A returned corpus is validated before anything reads it
    Given a producer has returned a corpus that does not validate against the schema
    When the tooling reads it
    Then the tooling rejects it and no consumer receives it

  @AC-CLOG-003.2
  Scenario: An unimplemented schema version is rejected
    Given a returned corpus declaring a schema version the tooling does not implement
    When the tooling reads it
    Then the tooling rejects it

  @AC-CLOG-003.13
  Scenario: A proposal without its reasons is rejected
    Given a returned corpus carrying a proposed impact and no rationale
    When the tooling reads it
    Then the tooling rejects it

  @AC-CLOG-003.14 @negative
  Scenario: A source outside the evidence is rejected
    Given a returned corpus with an entry citing a commit that is not in the evidence bundle
    When the tooling reads it
    Then the tooling rejects it and no consumer receives it

  @AC-CLOG-003.3
  Scenario: Agreement between the proposal and reviewed signals proceeds
    Given every commit in the range carries a reviewed signal
    And the computed impact is minor
    And a returned corpus proposing an impact of minor
    When the tooling reconciles them
    Then the tooling proceeds with an impact of minor

  @AC-CLOG-003.4
  Scenario: A proposal above the computed impact halts for confirmation
    Given a returned corpus proposing an impact of minor
    And the computed impact is patch
    And the maintainer names no bump
    When the tooling reconciles them
    Then the tooling halts and requires a person to confirm the impact

  @AC-CLOG-003.5 @negative
  Scenario: A proposal never determines the version unconfirmed
    Given a returned corpus proposing an impact of major
    And every commit in the range carries a reviewed signal no higher than minor
    When the tooling resolves the version after a person confirms the computed impact
    Then the version is computed from the confirmed impact and not from the proposal

  @AC-CLOG-003.6
  Scenario: Returned prose is preserved exactly
    Given a returned corpus whose prose contains markdown formatting and non-ASCII characters
    When the tooling reads and later renders it
    Then the prose is carried through byte-for-byte

  @AC-CLOG-003.7 @negative
  Scenario: The tooling does not call a language model
    When the tooling performs an authoring exchange
    Then the tooling does not issue any request to a language model service

  @AC-CLOG-003.8
  Scenario: The configured producer is the one invoked
    Given the repository configures a producer by name
    When the tooling performs an authoring exchange
    Then the tooling invokes that producer

  @AC-CLOG-003.9
  Scenario: An authored corpus is cached against its commit range
    Given an authoring exchange has completed for a commit range
    When the tooling performs an authoring exchange for the same range
    Then the tooling reuses the cached corpus without invoking a producer

  @AC-CLOG-003.10
  Scenario: A cached corpus can be discarded
    Given a cached corpus exists for the commit range being released
    When the maintainer requests that authoring be redone
    Then the tooling invokes the producer and replaces the cached corpus

  @AC-CLOG-003.11
  Scenario: Authoring can be skipped entirely
    Given the maintainer requests a release without notes
    When the tooling reaches the authoring exchange
    Then the tooling invokes no configured producer and proceeds with the corpus the mechanical producer composes
```

### FEAT-CLOG-004: Release Composition

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-004
Feature: Release Composition

  As a reader of the release notes
  I want one entry per change I can observe, described from my point of view
  So that the notes tell me what changed rather than which pull requests merged

  An entry is a change, not a commit. The producer decides what the changes
  are: it may split one commit into several entries, merge several commits
  into one entry, and omit work no reader can observe. It sets each entry's
  kind from the evidence, writes a one-sentence summary and, where the change
  needs it, a sentence or two of detail, and proposes the release's impact
  with its rationale and the sources that drove it. The tooling accepts any
  composition whose sources are in the evidence, and reports what no entry
  cites so a reviewer can check that nothing visible was dropped.

  @AC-CLOG-004.1
  Scenario: One commit may yield several entries
    Given a squash commit whose evidence shows four user-visible changes
    When the tooling reads a returned corpus with four entries each citing that commit
    Then the tooling accepts it and renders four entries

  @AC-CLOG-004.2
  Scenario: Several commits may yield one entry
    Given two commits that together deliver one user-visible change
    When the tooling reads a returned corpus with one entry citing both commits
    Then the tooling accepts it and renders one entry citing both

  @AC-CLOG-004.3
  Scenario: Internal-only work may be omitted
    Given a commit that changes only tests and continuous integration configuration
    When the tooling reads a returned corpus with no entry citing that commit
    Then the tooling accepts it

  @AC-CLOG-004.4
  Scenario: Commits no entry cites are reported
    Given a returned corpus that cites no entry for one commit in the range
    When the tooling presents the authored notes for review
    Then the tooling names that commit as cited by no entry

  @AC-CLOG-004.5
  Scenario: The producer sets each entry's kind from the evidence
    Given a commit whose subject carries no conventional-commit type
    And whose diff adds a command-line flag
    When the vended changelog producer composes the release
    Then the entry describing that flag has the kind feature

  @AC-CLOG-004.6
  Scenario: An entry states its change in one sentence, with optional detail
    When the vended changelog producer composes the release
    Then each entry's summary is one sentence stating the change from the reader's point of view
    And any further explanation is carried in the entry's detail

  @AC-CLOG-004.7
  Scenario: The proposal carries its rationale and drivers
    When the vended changelog producer composes the release
    Then the corpus carries a proposed impact, a rationale for it, and the sources of the changes that drove it

  @AC-CLOG-004.8 @negative
  Scenario: A driver outside the evidence is rejected
    Given a returned corpus whose proposal drivers cite a pull request that is not in the evidence bundle
    When the tooling reads it
    Then the tooling rejects it

  @AC-CLOG-004.9
  Scenario: The mechanical producer carries a notes block's entries verbatim
    Given a commit whose body carries a notes block with two entries
    When the mechanical producer composes the release
    Then the corpus carries those two entries with their prose unchanged and not marked as derived

  @AC-CLOG-004.10
  Scenario: The mechanical producer derives an entry from a subject where there is no block
    Given a commit whose body carries no notes block
    When the mechanical producer composes the release
    Then the corpus carries one entry for that commit, derived from its subject and marked as derived
```

### FEAT-CLOG-005: Pull Request Notes

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-005
Feature: Pull Request Notes

  As a reviewer
  I want the release notes for a pull request written into its description before I review it
  So that I review the notes and their impact together with the code they describe

  The best time to describe a change is while someone who understands it is
  looking at it. `release notes` collects the pull request's commits and diff,
  has the producer author pull request notes from them, renders the notes as
  a notes block, and writes the block into the pull request description or
  prints it for someone to paste. The block records the head commit it was
  written for, so `release notes --check`, which needs no producer and no
  model credential, can fail a pull request whose block is missing, malformed
  or stale. The block's entries sit under the repository's changelog
  headings, rendered as the changelog section renders them, so a heading
  rather than a type prefix gives each entry its kind.

  @AC-CLOG-005.1
  Scenario: The pull request's commits and diff are collected
    Given a pull request whose branch carries three commits since its merge base
    When the maintainer runs release notes for that pull request
    Then the producer's input is an evidence bundle for the pull request carrying those three commits, each with its diff, within the pull request diff budget

  @AC-CLOG-005.2
  Scenario: The producer's pull request notes are validated
    Given a producer has returned pull request notes that do not validate against the schema
    When the tooling reads them
    Then the tooling rejects them and writes nothing to the pull request

  @AC-CLOG-005.3
  Scenario: The notes are rendered as a notes block recording the head commit
    Given a producer has returned valid pull request notes for head commit 3f2a9c1
    When the tooling renders them
    Then the result is a notes block in the contract's grammar whose marker records head 3f2a9c1

  @AC-CLOG-005.4
  Scenario: The block is written into the pull request description
    Given a pull request description holding a summary paragraph and an earlier notes block
    When the maintainer runs release notes for that pull request
    Then the description's notes block is replaced by the new one and the rest of the description is unchanged

  @AC-CLOG-005.5
  Scenario: The block can be printed instead of written
    When the maintainer runs release notes asking for the block to be printed
    Then the tooling prints the notes block and makes no change to the pull request

  @AC-CLOG-005.6 @negative
  Scenario: A current block is not re-authored unless asked
    Given a pull request description whose notes block records the current head commit
    When the maintainer runs release notes without asking for authoring to be redone
    Then the tooling invokes no producer and leaves the block unchanged

  @AC-CLOG-005.7
  Scenario: A current, well-formed block passes the check
    Given a pull request description whose notes block is well formed and records the current head commit
    When release notes --check runs
    Then the check passes

  @AC-CLOG-005.8
  Scenario: A missing block fails the check
    Given a pull request description with no notes block
    When release notes --check runs
    Then the check fails and says the notes block is missing

  @AC-CLOG-005.9
  Scenario: A malformed block fails the check
    Given a pull request description whose notes block has an entry line outside the contract's grammar
    When release notes --check runs
    Then the check fails and names the offending line

  @AC-CLOG-005.10
  Scenario: A stale block fails the check
    Given a pull request description whose notes block records head commit 3f2a9c1
    And the pull request's diff has changed since 3f2a9c1
    When release notes --check runs
    Then the check fails and says the notes block is stale

  @AC-CLOG-005.11
  Scenario: A moved head with an unchanged diff passes the check
    Given a pull request description whose notes block records head commit 3f2a9c1
    And the pull request's head has moved since 3f2a9c1 without changing its diff
    When release notes --check runs
    Then the check passes

  @AC-CLOG-005.12 @negative
  Scenario: The check invokes no producer
    When release notes --check runs
    Then the tooling invokes no producer and needs no language model credential

  @AC-CLOG-005.13
  Scenario: Entries are rendered under the configured changelog headings
    Given the changelog settings name the breaking heading "💥 Breaking changes" and the groups FEATURE "🚀 Features" and FIX "🪲 Bugs"
    And a producer has returned pull request notes with one feature entry scoped to cut and one fix entry
    When the tooling renders them
    Then the block holds a "### 🚀 Features" section and then a "### 🪲 Bugs" section, each with its entry
    And each entry line reads "- **cut:** " or "- " followed by its summary, with no type prefix and no source reference

  @AC-CLOG-005.14 @negative
  Scenario: Work the changelog does not render is left out of the block
    Given the changelog settings render FEATURE and FIX and no other kind
    And a pull request whose changes include a feature and a CI configuration change
    When the producer authors the pull request notes
    Then the notes carry an entry for the feature and none for the CI change

  @AC-CLOG-005.15
  Scenario: An entry's section determines its kind
    Given the changelog settings name the breaking heading "💥 Breaking changes" and the group FEATURE "🚀 Features"
    And a notes block with one entry under "### 🚀 Features" and one entry, with a Migration paragraph, under "### 💥 Breaking changes"
    When the tooling parses the block
    Then the first entry has kind FEATURE and is not breaking
    And the second entry is breaking and has no kind

  @AC-CLOG-005.16 @negative
  Scenario: An unrecognised section heading makes the block malformed
    Given the changelog settings and the default headings do not include "Improvements"
    And a pull request description whose notes block has a "### Improvements" section
    When release notes --check runs
    Then the check fails, says the notes block is malformed, and names the heading
```

### FEAT-CLOG-006: Notes Blocks at Release Time

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-006
Feature: Notes Blocks at Release Time

  As a maintainer
  I want the notes reviewed on each pull request used when the release is cut
  So that reviewed prose and reviewed impact are not authored a second time from scratch

  A squash merge that uses the pull request description carries the notes
  block into the commit body. At release time the block is parsed from the
  body and used twice: as evidence the producer composes from, and as a
  reviewed impact signal the version computation reads. A commit with a block
  needs no diff, because someone already reviewed what it says against the
  code. A commit whose block cannot be parsed is treated as having none, and
  the failure is reported, because history cannot be corrected.

  @AC-CLOG-006.1
  Scenario: A notes block in a commit body is parsed into the commit's evidence
    Given a commit whose body carries a well-formed notes block with an impact of minor and two entries
    When the tooling builds an evidence bundle for a release
    Then that commit's evidence carries the parsed notes with the impact minor and both entries

  @AC-CLOG-006.2
  Scenario: A notes block's impact is a reviewed signal
    Given a commit whose body carries a notes block reading Release-Type: minor
    When the tooling classifies impact for the release
    Then that commit's signal is minor

  @AC-CLOG-006.3
  Scenario: A commit with a notes block needs no diff at release time
    Given a commit whose body carries a well-formed notes block
    When the tooling collects evidence for a release
    Then the tooling does not collect that commit's diff

  @AC-CLOG-006.4
  Scenario: The block ends at the next second-level heading or the end of the body
    Given a commit body carrying a notes block followed by Co-authored-by trailer lines
    When the tooling parses the notes block
    Then the trailer lines are not part of any entry

  @AC-CLOG-006.5
  Scenario: A malformed block in history is reported and treated as absent
    Given a commit whose body carries a notes block that does not parse
    When the tooling builds an evidence bundle for a release
    Then the tooling reports that commit's block as malformed
    And that commit's evidence carries its diff and no parsed notes
    And the block contributes no signal for that commit

  @AC-CLOG-006.6
  Scenario: A block under a default heading parses after the headings change
    Given the changelog settings name the group FEATURE "🚀 Features"
    And a commit whose body carries a notes block with an entry under "### Features"
    When the tooling builds an evidence bundle for a release
    Then that commit's evidence carries the entry with kind FEATURE
```

## References

- [[adr-003]] — the producer authors notes and proposes impact from
  evidence; reviewed records in git decide the version
- [[adr-004]] — changelog authoring is a replaceable step, authored from
  diffs
- [[contract-changelog]] — the evidence bundle, the corpus, and the notes
  block grammar exchanged across this feature
- [[frd-version-computation]] — how the proposal is reconciled
- [[repository-release-policy]] — how the notes block reaches git
- [[module-release-core]] — the module whose components realize this
  feature, among them [[component-commit-collector]],
  [[component-diff-collector]], [[component-pr-metadata-collector]],
  [[component-changelog-authoring-bridge]] and
  [[component-changelog-producer]]
