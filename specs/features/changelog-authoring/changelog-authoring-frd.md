---
title: "FRD: Changelog Authoring"
description: Collecting commit and pull-request metadata, assembling it
  into a structured changelog corpus, and exchanging that corpus with an
  external author whose proposed impact is checked rather than trusted.
slug: frd-changelog-authoring
type: frd
status: approved
updated_at: 2026-10-07T00:00:00Z
sources:
  - adr-003
  - adr-005
  - contract-changelog
  - release-tooling-standards
---

# Changelog Authoring

## Overview

Release notes assembled from commit subjects read like a commit log,
because that is what they are. Good notes say what changed from the
reader's point of view, group related work, and explain what to do about
anything that breaks — which is synthesis, not formatting.

This feature gathers the raw material and hands it to someone who can do
that synthesis, then checks what comes back. The author is deliberately
outside the tooling: a maintainer with a language model on their
workstation, a skill driving one, or a person in an editor. The tooling
never calls a model, so no adopter needs an API credential to release.

The cheapest high-quality input is metadata that already exists. Pull
request titles and descriptions are written by the author and read by a
reviewer, and where a repository squashes with the description as the
commit body, they are in git already. The collection requirements below
exist to make sure that material is used.

This feature SHALL comply with [[release-tooling-standards]] and
[[contract-changelog]].

## Terminology

- **Corpus** — the structured changelog defined by [[contract-changelog]].
- **Adapter** — one source of changelog material: commits, pull-request
  metadata, or diffs.
- **Author-written block** — a release-notes section a contributor wrote
  into a pull request description, and therefore into the squash commit
  body.
- **Mechanically derived entry** — an entry the tooling produced from a
  commit subject because no author-written block existed for it.
- **Authored corpus** — a corpus returned by a producer, carrying prose and a
  proposed impact.
- **Producer** — whatever authors a corpus: a language model, a configured
  command, a conventional-commit parser, or a person editing a file.

## Requirements

### FEAT-CLOG-001: Changelog Source Collection

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-001
Feature: Changelog Source Collection

  As a maintainer
  I want pull request titles and descriptions collected as changelog material
  So that authoring starts from reviewed prose rather than from commit subjects

  Squash merging puts most of this in git already: the commit subject carries
  the pull request title and its number, and where the repository squashes
  using the description, the description is the commit body. The hosting platform is
  consulted only for what the commits do not carry.

  @AC-CLOG-001.1
  Scenario: A pull request number is read from a squash commit subject
    Given a squash commit whose subject ends with a pull request reference
    When the tooling collects commit records
    Then the record carries that pull request number

  @AC-CLOG-001.2
  Scenario: A pull request description carried in the commit body is collected
    Given a squash commit whose body is the pull request description
    When the tooling collects commit records
    Then the record carries that description without consulting the hosting platform

  @AC-CLOG-001.3
  Scenario: Missing descriptions are fetched from the hosting platform
    Given a commit carrying a pull request number and no description in its body
    When the tooling collects changelog material
    Then the tooling fetches the title and description for that pull request from the hosting platform

  @AC-CLOG-001.4 @negative
  Scenario: Fetched metadata never overwrites what the commit carried
    Given a commit whose body already carries a pull request description
    When the tooling collects changelog material with the hosting platform reachable
    Then the tooling does not replace the description carried in the commit

  @AC-CLOG-001.5
  Scenario: An unreachable hosting platform degrades rather than failing
    Given the hosting platform is unreachable
    When the tooling collects changelog material
    Then the tooling returns the commit records unchanged and reports the degradation

  @AC-CLOG-001.6
  Scenario: A commit with no pull request is still collected
    Given a commit pushed directly to the release line, carrying no pull request number
    When the tooling collects commit records
    Then the record is present with its pull request number absent

  @AC-CLOG-001.7
  Scenario: Reading diffs is opt-in
    Given the repository has not enabled the diff adapter
    When the tooling collects changelog material
    Then the tooling does not read diffs
```

### FEAT-CLOG-002: Corpus Assembly

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-002
Feature: Corpus Assembly

  As a maintainer
  I want the collected material assembled into a structured corpus
  So that an author edits a draft rather than writing from nothing

  Where a contributor wrote a release-notes block, that prose is already
  reviewed and already about a small change someone understood. Carrying it
  through is what makes authoring cheap, and it is what makes reading diffs
  optional rather than necessary.

  @AC-CLOG-002.1
  Scenario: The assembled corpus validates against the schema
    When the tooling assembles a corpus
    Then the corpus validates against the schema the changelog contract names

  @AC-CLOG-002.2
  Scenario: An author-written block becomes the entry's prose
    Given a record carrying an author-written release-notes block
    When the tooling assembles a corpus
    Then the corresponding entry carries that prose rather than a summary derived from the subject

  @AC-CLOG-002.3
  Scenario: A record with no block yields a mechanically derived entry
    Given a record carrying no author-written release-notes block
    When the tooling assembles a corpus
    Then the corresponding entry is derived from the commit subject and is marked as mechanically derived

  @AC-CLOG-002.4
  Scenario: Every contributing adapter is recorded
    Given the commit and pull request adapters both contributed records
    When the tooling assembles a corpus
    Then the corpus records both adapters and the range each read

  @AC-CLOG-002.5 @negative
  Scenario: Assembly does not propose an impact
    When the tooling assembles a corpus
    Then the corpus carries no proposed impact
```

### FEAT-CLOG-003: Authoring Exchange

**Acceptance Criteria:**

```gherkin
@FEAT-CLOG-003
Feature: Authoring Exchange

  As a maintainer
  I want the tooling to exchange a corpus with an author and check what returns
  So that generated prose is reviewed and a generated version number is impossible

  The exchange is files, not calls. Producers are plugins and the default one
  runs outside the tooling, which is why no adopter needs an API credential to
  cut a release. What returns is validated against the schema and its proposed
  impact is compared against the computed one. Authoring is slow and may be
  paid for, so the result is cached against the commit range it describes.

  @AC-CLOG-003.1
  Scenario: A returned corpus is validated before anything reads it
    Given an author has returned a corpus that does not validate against the schema
    When the tooling reads it
    Then the tooling rejects it and no consumer receives it

  @AC-CLOG-003.2
  Scenario: An unimplemented schema version is rejected
    Given a returned corpus declaring a schema version the tooling does not implement
    When the tooling reads it
    Then the tooling rejects it

  @AC-CLOG-003.3
  Scenario: Agreement between the proposed and computed impact proceeds
    Given a returned corpus proposing an impact of minor
    And the computed impact is minor
    When the tooling compares them
    Then the tooling proceeds

  @AC-CLOG-003.4
  Scenario: Disagreement halts for human confirmation
    Given a returned corpus proposing an impact of minor
    And the computed impact is patch
    When the tooling compares them
    Then the tooling halts and requires human confirmation

  @AC-CLOG-003.5 @negative
  Scenario: A proposal never determines the version
    Given a returned corpus proposing an impact of major
    When the tooling resolves the version after human confirmation of the computed impact
    Then the version is computed from the computed impact and not from the proposal

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
    Given the maintainer requests a release without notes and names a bump
    When the tooling reaches the authoring exchange
    Then the tooling skips it and proceeds with the mechanically assembled corpus
```

## References

- [[adr-003]] — authored prose proposes impact; computation decides it
- [[adr-004]] — changelog authoring is a replaceable step
- [[contract-changelog]] — the corpus exchanged across this feature
- [[component-commit-collector]], [[component-pr-metadata-collector]],
  [[component-changelog-corpus-builder]],
  [[component-changelog-authoring-bridge]] — the components realizing this
  feature
