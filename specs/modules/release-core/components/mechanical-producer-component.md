---
title: "Component Blueprint: Mechanical Producer"
entity_name: Mechanical Producer
description: The changelog producer of last resort — entries derived from
  conventional commit subjects, unclassified commits listed under the other
  heading, with no language model.
slug: component-mechanical-producer
type: component-blueprint
status: approved
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/authoring/producers
test_paths:
  - internal/authoring/producers
---

# Mechanical Producer

## Overview

An instance of [[component-changelog-producer]] that needs nothing but the
evidence bundle. Each commit becomes one entry derived from its subject and
marked as derived, so a reader of the changelog can tell a sentence nobody
wrote from one a producer authored. A conventional subject gives the entry
its kind; a commit with no conventional type is listed under the other
heading with no kind.

It serves `--skip-notes`, continuous integration with no model, and
`changelog.producer = "mechanical"`. The bridge never selects it on its
own when the configured producer is unavailable or fails. It exercises no judgement and reads no
diff. Its kinds come from conventional subjects whether or not the
`conventional-type` signal is enabled. With that signal enabled, the
default, the suggestion [[component-bump-suggester]] derives from its
entries is the signal floor; with it disabled, the suggestion can exceed
the floor, and a person still confirms it. Where the suggestion is none
and an entry has no kind, the cut halts for a named bump.

- **Role:** a model-free structured changelog.
- **Scope:** in — mapping commit subjects to changelog entries; out —
  reading git or diffs, composing prose, merging, splitting or omitting
  changes, and suggesting a bump.
- **Composed into:** [[module-release-core]] — invoked by
  [[component-changelog-authoring-bridge]]; see the module for
  relationships.
- **Source & tests:** `producers.Mechanical` in
  `internal/authoring/producers`.

## Interface

- **Surface:** given an evidence bundle, return a structured changelog
  conforming to [[contract-changelog]].
- **Guarantees:**
  - The component SHALL be a pure function of the evidence bundle, and
    SHALL run in process.
  - The component SHALL derive one entry per commit, from its subject, with
    `derived` true.
  - The component SHALL cite every commit in exactly one entry, and SHALL
    list no commit in `omitted`.
  - WHEN a commit's subject has no conventional type, the component SHALL
    give its entry no `kind`, so it renders under the other heading.
  - The component SHALL NOT set `version`.
- **Pinned rules:**
  - A derived entry's `summary` is the subject's description: the subject
    without its conventional type prefix and its trailing ` (#<n>)`. A
    conventional type maps to `kind` case-insensitively: `feat` FEATURE,
    `fix` FIX, `perf` PERFORMANCE, `revert` REVERT, `refactor` REFACTOR,
    `docs` DOCUMENTATION, `build` BUILD, `ci` CI, `test` TEST, `style`
    STYLE, and any other type CHORE. The conventional scope is `scope`.
  - A derived entry's `breaking` is the commit's breaking-change marker as
    [[component-bump-classifier]] reads it; `migration` is the text of a
    `BREAKING CHANGE:` footer, with its continuation lines, WHEN one
    exists, and otherwise a sentence saying the commit is marked breaking
    and names no migration.
  - An entry's `sources` hold the commit, and the pull request WHEN the
    commit's record carries a number.
  - Entries follow the bundle's commit order, oldest first.
  - Corpus-level `sources` are the bundle's `sources`, copied unchanged.

## Cross-Cutting Standards

This component SHALL comply with:

- [[contract-changelog]] — the structured changelog semantics.

## Change Guidance

The component SHALL NOT take a kind from anything but a conventional type.

**Rationale:** It reads no diff, so any other kind would be a guess dressed
as a reading of the evidence. Taking kinds only from conventional types
keeps the suggestion derived from its entries equal to the signal floor, and
a range whose floor is none still halts for a person.

---

- **Verification gates:** the conformance suite every producer runs
  (`producertest.Run`), plus
  the component's unit suite over bundles with conventional and
  non-conventional subjects, squash subjects carrying ` (#<n>)`, and
  breaking markers with and without a footer.

## Key Decisions

- [[adr-003]] — without a model, entries come from conventional subjects
  and the suggestion is the signal floor alone.
- [[adr-004]] — conventional subjects are used where present and never
  required.

## Open Questions

None. [[contract-changelog]] makes `ChangelogEntry.kind` optional, so a
non-conventional commit's entry carries no kind and renders under the other
heading.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-producer]] — what every producer must do
- [[contract-changelog]] — the changelog this producer returns
- [[frd-changelog-authoring]] — the feature this component serves
