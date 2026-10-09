---
title: "Component Blueprint: Changelog Corpus Builder"
entity_name: Changelog Corpus Builder
description: Collected commit and pull-request records assembled into a
  changelog corpus, merging author-written notes where they exist.
slug: component-changelog-corpus-builder
type: component-blueprint
status: retired
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths: []
test_paths: []
---

# Changelog Corpus Builder

## Overview

**Retired.** [[component-evidence-builder]] replaces it as the producer's
input, and [[component-mechanical-producer]] takes over its model-free
entries. No code implements it: its package, `internal/corpus`, is gone, and
the changelog operation exchanges the evidence bundle instead. The rest of
this document describes the component as it was.

Assembles the structured corpus an author works from. It reads records
from the enabled source adapters, merges any author-written notes already
present in them, and emits an instance of [[contract-changelog]] with
whatever it could determine mechanically.

Author-written notes are the reason this component matters. Where a pull
request carried a release-notes block — in its description, and therefore
in the squash commit body — that block is already reviewed prose about a
small change written by someone who understood it. Merging those produces a
corpus that an author edits rather than one they write from nothing, which
is both cheaper and better.

Three adapters are specified, each with its own blueprint:

- [[component-commit-collector]] — always enabled, and the floor. For a
  squash-merging repository it already carries pull-request titles,
  descriptions and numbers.
- [[component-pr-metadata-collector]] — enriches from the hosting platform
  where the commits do not carry what is needed. Deferred beyond the first
  milestone.
- [[component-diff-collector]] — opt-in and deferred, because reading diffs is
  expensive and only pays where author-written notes are absent.

- **Role:** the corpus an author receives.
- **Scope:** in — adapter composition, note extraction, mechanical entry
  derivation; out — authoring prose, and rendering.
- **Composed into:** [[module-release-core]] — consumes the collectors,
  feeds [[component-changelog-authoring-bridge]]; see it for relationships.
- **Source & tests:** none; `internal/corpus` is removed.

## Interface

- **Surface:** given commit records and an enabled adapter set, return a
  changelog corpus conforming to [[contract-changelog]].
- **Guarantees:**
  - The component SHALL emit a corpus that validates against the schema
    [[contract-changelog]] names.
  - The component SHALL populate `sources` with every adapter that
    contributed and the range it read.
  - WHEN a record carries an author-written release-notes block, the
    component SHALL carry that prose into the corresponding entry rather
    than deriving a summary from the subject.
  - WHEN a record carries no such block, the component SHALL derive an
    entry from the subject and mark it as mechanically derived.
  - The component SHALL enable the commit adapter unconditionally.
  - The component SHALL NOT enable the diff adapter unless configured.
  - The component SHALL treat the enabled adapter set as configuration rather
    than inferring it from the repository's state.
  - The component SHALL NOT set `proposed_bump`.
- **Operation:** `corpus.Build` takes the commit records, the range they
  were read from (base and head commit hashes; no base for a line with no
  release) and the enabled adapter set, and returns a corpus declaring
  `schema_version` 1.0.0, one entry per record in record order, and `sources`
  holding the commit adapter with that range. It refuses an enabled adapter
  that is not implemented rather than skipping it.
- **Pinned rules:**
  - An author-written block is a level-two heading reading `Release notes`
    (any letter case) in the body, running to the next `## ` heading or the
    end of the body. Its first paragraph is the entry's `summary` and the
    rest its `detail`, both verbatim except for the blank lines around them,
    with `derived` false. WHEN the block runs to the end of the body, a final
    paragraph made only of the commit's trailers and breaking-change footers
    is left out; a block with no prose is no block.
  - Otherwise the `summary` is the subject's description, with `derived`
    true. A conventional-commit type maps to `kind` case-insensitively:
    `feat` FEATURE, `fix` FIX, `perf` PERFORMANCE, `revert` REVERT,
    `refactor` REFACTOR, `docs` DOCUMENTATION, `build` BUILD, `ci` CI,
    `test` TEST, `style` STYLE, and any other type, or a subject that is not
    conventional, CHORE. The conventional scope is `scope`. The summary is
    the subject without its type prefix and its trailing ` (#<n>)`.
  - `kind` and `scope` come from the subject whether or not a block exists.
  - `breaking` is the bump classifier's breaking-change marker; `migration`
    is the text of a `BREAKING CHANGE:` footer, with its continuation lines,
    WHEN one exists.
  - An entry's `sources` hold the pull request WHEN the record carries a
    number, and the commit otherwise.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the changelog sources a release reads.

## Change Guidance

WHEN an adapter is added, it SHALL record itself in `sources`.

---

An adapter SHALL NOT alter entries another adapter contributed.

**Rationale:** The corpus must say what it was built from, and an adapter
that silently rewrites another's output makes the provenance record false.

---

The component SHALL NOT set `proposed_bump`, which belongs to the author.

**Rationale:** A mechanically derived proposal would agree with the
classifier by construction and defeat the disagreement check in
[[adr-003]].

- **Verification gates:** the component's unit suite over fixture record
  sets, including records with and without author-written blocks, plus
  schema validation of every emitted corpus.

## Key Decisions

- [[adr-003]] — authored prose proposes impact; this component does not.
- [[adr-004]] — changelog authoring is a replaceable step, which is why
  this component stops at a structured corpus.

## Open Questions

None. A `## Release Notes` heading delimits the block, matching the
convention Auto established and the heading the stage pull request already
uses for its own notes. The block runs to the next heading of the same level
or to the end of the body.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[contract-changelog]] — the corpus this component emits
- [[frd-changelog-authoring]] — the feature this component serves
