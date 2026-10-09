---
title: "FRD: Changelog Authoring"
description: Collecting the evidence of what changed from git — commit
  records, the pull request information commits carry, file lists and
  budgeted diffs — and exchanging it with an external producer that returns
  a structured changelog, which the tooling validates rather than trusts.
slug: frd-changelog-authoring
type: frd
status: approved
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
the pull request title and description a squash or merge commit carries,
the files each commit touched, and the diffs themselves.

This feature collects that evidence from git at release time and hands it to
a producer that can do the synthesis, then checks what comes back. The
tooling collects every commit since the base version: its record, the pull
request information git carries for it, its changed files, its diff within
the release's diff budget, and the signal the bump classifier read from it.
The producer returns a structured changelog whose entries are changes, not
commits, and accounts for every commit in the range: each is cited by an
entry or listed as omitted with a reason.

The producer is outside the tooling: a maintainer with a language model on
their workstation, a skill driving one, a configured command, or a person in
an editor. Without a model (`--skip-notes`), the mechanical producer
composes entries from conventional subjects. WHEN the configured producer
is unavailable or fails, the tooling reports the failure and stops, naming
`--skip-notes`; it never falls back to the mechanical producer on its own. The tooling never calls a model, so no
adopter needs an API credential to release. The structured changelog never
becomes a version on its own: the [[component-bump-suggester]] derives a
suggested bump from its entry kinds, and a person confirms it, as
[[frd-version-computation]] specifies ([[adr-003]]).

This feature SHALL comply with [[release-tooling-standards]],
[[repository-release-policy]] and [[contract-changelog]].

## Terminology

- **Evidence bundle** — what a producer reads: the commits in a range, each
  with its record, the pull request information git carries for it, its
  changed files, its diff where one was collected and the signal the bump
  classifier read, plus a report of what the diff budget left out. Defined
  by [[contract-changelog]].
- **Diff budget** — the most patch text, in bytes, one evidence bundle may
  carry. Diffs beyond it are truncated or omitted, and the omission is
  recorded.
- **Structured changelog** — what a producer returns: entries, each a change
  with a kind, a scope, a summary, optional detail, migration text where it
  is breaking, and the commits it is derived from, plus the omitted list.
  [[contract-changelog]] calls it the authored corpus.
- **Omitted list** — the commits a structured changelog cites in no entry,
  each with a one-line reason. It is shown for review and never rendered in
  the changelog.
- **Producer** — whatever authors a structured changelog: a language model
  driven by a skill, a configured command, the mechanical producer, or a
  person editing a file.
- **Mechanical producer** — the producer of last resort, built into the
  tooling. It composes entries from conventional commit subjects without
  judgement, and serves releases without notes and model-free automation.
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

  Evidence comes from git. Commit records are always collected, with their
  full bodies, trailers, any conventional type, scope and breaking marker,
  the pull request number, title and description where a squash or merge
  commit carries them, and the list of files each changed. Diffs are part of
  the default path: they are the only honest record of a change nobody
  described. They are collected within the release's byte budget, with
  lockfiles, vendored and generated paths excluded, and whatever the budget
  leaves out is reported rather than dropped silently. Pull-request metadata
  from the hosting platform is an optional enrichment, off by default, for
  commits whose body lacks the description; nothing depends on it.

  @AC-CLOG-001.1
  Scenario: A pull request number is read from a squash commit subject
    Given a squash commit whose subject ends with a pull request reference
    When the tooling collects commit records
    Then the record carries that pull request number

  @AC-CLOG-001.14
  Scenario: A pull request is read from a merge commit
    Given a merge commit whose subject names pull request 42 and whose body carries the pull request's title
    When the tooling collects commit records
    Then the record carries pull request number 42 and that title without consulting the hosting platform

  @AC-CLOG-001.2
  Scenario: A commit's full body is collected
    Given a squash commit whose body is the pull request description
    When the tooling collects commit records
    Then the record carries that body in full without consulting the hosting platform

  @AC-CLOG-001.15
  Scenario: A commit's trailers and conventional header are parsed into its record
    Given a commit with the subject "feat(cut)!: rename --dry to --dry-run" and the trailer "Release-Type: major"
    When the tooling collects commit records
    Then the record carries its hash, author, date and trailers
    And the record carries the conventional type "feat", the scope "cut" and a breaking marker

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
    When the tooling collects evidence for a release
    Then the evidence carries each commit's diff within the release diff budget

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
    And a commit carrying a pull request number and a body that holds no pull request description
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

  The evidence bundle is the producer's input. It carries every commit in the
  range, oldest first: what git said about it, the pull request information
  git carries for it, its changed files, the diff where one was collected,
  and the signal the bump classifier read from its records. It never
  classifies a change and never authors one: an entry, a kind or an impact
  of its own in the bundle would be a guess presented as data. It is kept
  with the structured changelog, so a reviewer can see what the producer was
  shown.

  @AC-CLOG-002.1
  Scenario: The evidence bundle validates against the schema
    When the tooling builds an evidence bundle
    Then the bundle validates against the evidence schema the changelog contract names

  @AC-CLOG-002.6
  Scenario: Every commit in the range is carried, oldest first
    Given a range of three commits
    When the tooling builds an evidence bundle for a release
    Then the bundle carries all three commits, oldest first, each with its subject, body, trailers, pull request information where git carries it, and changed files

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

  @AC-CLOG-002.11
  Scenario: Each commit carries the signal the bump classifier read
    Given a range of two commits, one carrying the trailer "Release-Type: minor" and one carrying no signal
    When the tooling builds an evidence bundle
    Then the first commit's evidence carries the signal minor
    And the second commit's evidence states that it carries no signal

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
  Scenario: The evidence bundle is kept with the structured changelog
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
  So that generated prose is checked before a person reviews it and never becomes a version unconfirmed

  The exchange is files, not calls. Producers are plugins and the default one
  runs outside the tooling, which is why no adopter needs an API credential to
  cut a release. The producer reads the evidence bundle and returns a
  structured changelog. What returns is validated against the schema and
  checked against the evidence it cites: every entry names its sources in
  the evidence, every breaking entry carries migration text, and every commit
  in the range is cited by an entry or listed as omitted with a reason, so
  nothing is dropped silently. The suggested bump is derived from the
  validated changelog, as the version computation feature specifies.
  Authoring is slow and may be paid for, so the result is cached against the
  commit range it describes.

  @AC-CLOG-003.12
  Scenario: The producer receives the evidence bundle
    Given a commit whose body describes four changes
    When the tooling performs an authoring exchange
    Then the producer's input is an evidence bundle carrying that commit's body in full

  @AC-CLOG-003.1
  Scenario: A returned structured changelog is validated before anything reads it
    Given a producer has returned a structured changelog that does not validate against the schema
    When the tooling reads it
    Then the tooling rejects it and no consumer receives it

  @AC-CLOG-003.2
  Scenario: An unimplemented schema version is rejected
    Given a returned structured changelog declaring a schema version the tooling does not implement
    When the tooling reads it
    Then the tooling rejects it

  @AC-CLOG-003.14 @negative
  Scenario: A source outside the evidence is rejected
    Given a returned structured changelog with an entry citing a commit that is not in the evidence bundle
    When the tooling reads it
    Then the tooling rejects it and no consumer receives it

  @AC-CLOG-003.15 @negative
  Scenario: An entry citing no source is rejected
    Given a returned structured changelog with an entry whose sources are empty
    When the tooling reads it
    Then the tooling rejects it and no consumer receives it

  @AC-CLOG-003.16 @negative
  Scenario: A commit neither cited nor omitted is rejected
    Given a range of three commits
    And a returned structured changelog whose entries cite two of them and whose omitted list is empty
    When the tooling reads it
    Then the tooling rejects it and names the commit it does not account for

  @AC-CLOG-003.17 @negative
  Scenario: An omitted commit without a reason is rejected
    Given a returned structured changelog that lists a commit as omitted with an empty reason
    When the tooling reads it
    Then the tooling rejects it and names that commit

  @AC-CLOG-003.18 @negative
  Scenario: A breaking entry without migration text is rejected
    Given a returned structured changelog with a breaking entry that carries no migration text
    When the tooling reads it
    Then the tooling rejects it and names that entry

  @AC-CLOG-003.19 @negative
  Scenario: A summary that is not one or two sentences is rejected
    Given a returned structured changelog with an entry whose summary is empty or runs to three sentences
    When the tooling reads it
    Then the tooling rejects it and names that entry

  @AC-CLOG-003.5 @negative
  Scenario: The structured changelog never determines the version unconfirmed
    Given a returned structured changelog with a breaking entry
    And the maintainer names no bump and has not confirmed the suggested bump
    When the tooling resolves the version
    Then the tooling returns no version and halts with the suggested bump for a person to confirm

  @AC-CLOG-003.6
  Scenario: Returned prose is preserved exactly
    Given a returned structured changelog whose prose contains markdown formatting and non-ASCII characters
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
  Scenario: A structured changelog is cached against its commit range
    Given an authoring exchange has completed for a commit range
    When the tooling performs an authoring exchange for the same range
    Then the tooling reuses the cached structured changelog without invoking a producer

  @AC-CLOG-003.10
  Scenario: A cached structured changelog can be discarded
    Given a cached structured changelog exists for the commit range being released
    When the maintainer requests that authoring be redone
    Then the tooling invokes the producer and replaces the cached structured changelog

  @AC-CLOG-003.11
  Scenario: Authoring can be skipped entirely
    Given the maintainer requests a release without notes
    When the tooling reaches the authoring exchange
    Then the tooling invokes no configured producer and proceeds with the structured changelog the mechanical producer composes
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
  kind and scope from the evidence, writes a summary of one or two full
  sentences and, where the change needs it, further detail, writes migration
  text for every breaking entry, and names the commits each entry is derived
  from. A commit no entry cites is listed as omitted with a one-line reason,
  which a reviewer sees and a reader of the changelog does not. The tooling
  accepts any composition that accounts for every commit and whose sources
  are in the evidence.

  @AC-CLOG-004.1
  Scenario: One commit may yield several entries
    Given a squash commit whose evidence shows four user-visible changes
    When the tooling reads a returned structured changelog with four entries each citing that commit
    Then the tooling accepts it and renders four entries

  @AC-CLOG-004.2
  Scenario: Several commits may yield one entry
    Given two commits that together deliver one user-visible change
    When the tooling reads a returned structured changelog with one entry citing both commits
    Then the tooling accepts it and renders one entry citing both

  @AC-CLOG-004.3
  Scenario: Internal-only work may be omitted with a reason
    Given a commit that changes only tests and continuous integration configuration
    When the tooling reads a returned structured changelog with no entry citing that commit and that commit listed as omitted with a reason
    Then the tooling accepts it

  @AC-CLOG-004.4
  Scenario: Omitted commits are shown for review with their reasons
    Given a returned structured changelog that lists one commit in the range as omitted with a reason
    When the tooling presents the structured changelog for review
    Then the tooling names that commit as omitted and shows its reason

  @AC-CLOG-004.13 @negative
  Scenario: Omission reasons are not rendered in the changelog
    Given a returned structured changelog that lists one commit as omitted with a reason
    When the tooling renders the changelog section
    Then the section carries neither that commit nor its reason

  @AC-CLOG-004.5
  Scenario: The producer sets each entry's kind from the evidence
    Given a commit whose subject carries no conventional-commit type
    And whose diff adds a command-line flag
    When the vended changelog producer composes the release
    Then the entry describing that flag has the kind feature

  @AC-CLOG-004.11
  Scenario: Each entry carries its kind, scope and sources
    When the vended changelog producer composes the release
    Then each entry carries a kind, a scope where the change has one, and the commits it is derived from

  @AC-CLOG-004.6
  Scenario: An entry states its change in one or two sentences, with optional detail
    When the vended changelog producer composes the release
    Then each entry's summary is one or two full sentences stating the change from the reader's point of view
    And any further explanation is carried in the entry's detail

  @AC-CLOG-004.12
  Scenario: A breaking entry says what to do about it
    Given a commit whose diff removes a command-line flag
    When the vended changelog producer composes the release
    Then the entry describing that removal is breaking and carries migration text naming what replaces the flag

  @AC-CLOG-004.10
  Scenario: The mechanical producer derives an entry from a conventional subject
    Given a commit whose subject is "fix(cut): refuse an existing stage branch"
    When the mechanical producer composes the release
    Then the structured changelog carries one entry for that commit with the kind fix and the scope cut, derived from its subject and marked as derived

  @AC-CLOG-004.14
  Scenario: The mechanical producer lists an unclassified commit with no kind
    Given a commit whose subject carries no conventional-commit type
    When the mechanical producer composes the release
    Then the structured changelog carries one entry for that commit, with no kind, derived from its subject and marked as derived
    And the rendered section places that entry under the other heading
```

## References

- [[adr-003]] — the producer authors a structured changelog from commit
  evidence; a person confirms the bump suggested from it
- [[adr-004]] — changelog authoring is a replaceable step, authored from
  diffs
- [[contract-changelog]] — the evidence bundle and the structured changelog
  exchanged across this feature
- [[frd-version-computation]] — how the suggested bump is derived from the
  structured changelog and confirmed
- [[repository-release-policy]] — how a squash merge carries the pull request
  title and description into git
- [[module-release-core]] — the module whose components realize this
  feature, among them [[component-commit-collector]],
  [[component-diff-collector]], [[component-pr-metadata-collector]],
  [[component-evidence-builder]], [[component-changelog-authoring-bridge]],
  [[component-changelog-producer]] and [[component-mechanical-producer]]
