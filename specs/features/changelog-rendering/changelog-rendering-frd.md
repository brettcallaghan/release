---
title: "FRD: Changelog Rendering"
description: Rendering one changelog corpus into every published artifact
  deterministically, and maintaining CHANGELOG.md idempotently.
slug: frd-changelog-rendering
type: frd
status: approved
updated_at: 2026-10-07T00:00:00Z
sources:
  - adr-003
  - contract-changelog
  - release-tooling-standards
---

# Changelog Rendering

## Overview

A release describes itself in more than one place — a section in
`CHANGELOG.md`, a release body on the hosting platform — and those
descriptions have a habit of drifting apart when each is generated
separately. Rendering every
one of them from a single corpus in a single pass makes the drift
impossible rather than merely discouraged.

Rendering is also the half of changelog handling that must never vary.
Authoring may produce different prose on different runs; rendering the same
corpus must produce the same bytes every time, so that a stage pull request
diff means what it appears to mean and so the output can be tested against
golden files.

This feature SHALL comply with [[release-tooling-standards]] and
[[contract-changelog]].

## Terminology

- **Target** — one rendering destination: the changelog file, or a release
  body on the hosting platform.
- **Group** — a set of entries rendered together under one heading,
  determined by entry kind. Entries marked breaking form a group of their own
  regardless of kind.
- **Section** — the rendered markdown describing one release.

## Requirements

### FEAT-REND-001: Deterministic Rendering

**Acceptance Criteria:**

```gherkin
@FEAT-REND-001
Feature: Deterministic Rendering

  As a maintainer
  I want every artifact rendered from one corpus in one pass
  So that a release cannot describe itself two different ways

  Determinism is what makes the rendering reviewable. A stage pull request
  shows a diff of exactly what will be published, and a golden-file test can
  hold the renderer to it.

  @AC-REND-001.1
  Scenario: Identical input renders identical output
    Given a corpus
    When the tooling renders it twice for the same target
    Then the two renderings are byte-identical

  @AC-REND-001.2
  Scenario: Every target is rendered from one corpus instance
    When the tooling renders a release
    Then the changelog section and the release body are rendered from the same corpus instance in the same invocation

  @AC-REND-001.3
  Scenario: Entry order within a group is preserved
    Given a corpus whose entries of one kind appear in a deliberate order
    When the tooling renders it
    Then those entries appear in that order within their group

  @AC-REND-001.4
  Scenario: An unrecognised entry kind still renders
    Given a corpus containing an entry whose kind the renderer does not recognise
    When the tooling renders it
    Then the entry appears in a default group and the rendering succeeds

  @AC-REND-001.5
  Scenario: Entries marked breaking are rendered as their own section
    Given a corpus containing an entry whose breaking flag is set and which carries migration guidance
    When the tooling renders it
    Then the rendering contains a breaking-changes section carrying that entry and its migration guidance

  @AC-REND-001.6 @negative
  Scenario: A corpus with no resolved version does not render
    Given a corpus whose version is unset
    When the tooling renders it
    Then the tooling refuses to render

  @AC-REND-001.7
  Scenario: Section headings follow the project's configuration
    Given the repository configures a heading for a given entry kind
    When the tooling renders a corpus containing entries of that kind
    Then those entries appear under the configured heading
```

### FEAT-REND-002: Changelog File Maintenance

**Acceptance Criteria:**

```gherkin
@FEAT-REND-002
Feature: Changelog File Maintenance

  As a maintainer
  I want a rendered section placed into the changelog without disturbing what is there
  So that re-staging a release corrects the file rather than duplicating in it

  Staging is re-run after corrections, so writing must be idempotent. A
  writer that always appends produces a file carrying one version twice,
  which a reviewer reading a large diff will miss.

  @AC-REND-002.1
  Scenario: A new section is placed above existing sections
    Given a changelog file containing sections for earlier releases
    When the tooling writes a section for a newer release
    Then the new section appears above every existing section

  @AC-REND-002.2
  Scenario: Re-staging replaces rather than duplicates
    Given a changelog file whose topmost section is for version 1.9.0
    When the tooling writes a section for version 1.9.0 again
    Then the file contains exactly one section for version 1.9.0

  @AC-REND-002.3
  Scenario: An absent changelog file is created
    Given the repository has no changelog file
    When the tooling writes a section
    Then the tooling creates the file containing that section

  @AC-REND-002.4
  Scenario: Existing content is preserved exactly
    Given a changelog file containing hand-edited sections for earlier releases
    When the tooling writes a section for a newer release
    Then the earlier sections are unchanged byte-for-byte

  @AC-REND-002.5 @negative
  Scenario: Writing the changelog does not touch git
    When the tooling writes the changelog file
    Then the tooling does not stage, commit, or tag anything
```

## References

- [[adr-003]] — one corpus renders every artifact
- [[contract-changelog]] — the corpus this feature consumes
- [[component-changelog-renderer]], [[component-changelog-writer]] — the
  components realizing this feature
