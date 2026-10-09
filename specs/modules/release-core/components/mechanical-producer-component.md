---
title: "Component Blueprint: Mechanical Producer"
entity_name: Mechanical Producer
description: The changelog producer of last resort — entries taken from
  reviewed notes blocks and derived from conventional commit subjects, with no
  language model, proposing only the computed impact.
slug: component-mechanical-producer
type: component-blueprint
status: draft
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
evidence bundle. Where a commit carries a reviewed notes block, its entries
become changelog entries as they were reviewed. Where it does not, an entry
is derived from the commit subject and marked as derived, so a reader of the
corpus can tell a reviewed sentence from one nobody wrote.

It serves `--skip-notes`, continuous integration with no model, and
`changelog.producer = "mechanical"` for a project whose notes blocks already
say everything. It exercises no judgement, so its proposal is the impact
computed from the reviewed signals, and it never raises it: where commits
carry no signal, [[component-impact-reconciler]] still asks a person.

- **Role:** a model-free authored corpus.
- **Scope:** in — mapping notes entries and commit subjects to corpus
  entries, and restating the computed impact as its proposal; out — reading
  git, composing prose, merging or omitting changes, and judging impact.
- **Composed into:** [[module-release-core]] — invoked by
  [[component-changelog-authoring-bridge]]; see the module for
  relationships.
- **Source & tests:** `producers.Mechanical` in
  `internal/authoring/producers`.

## Interface

- **Surface:** given a release evidence bundle, return a corpus conforming
  to [[contract-changelog]].
- **Guarantees:**
  - The component SHALL be a pure function of the evidence bundle, and
    SHALL run in process.
  - The component SHALL carry every entry of every parsed notes block into
    the corpus, verbatim, with `derived` false.
  - WHEN a commit has no notes block, or one that does not parse, the
    component SHALL derive one entry from its subject with `derived` true.
  - The component SHALL omit no commit.
  - The component SHALL set `proposed_bump` to the impact computed from the
    bundle's commit signals, `proposal_drivers` to the commits that
    determined it, and `proposal_rationale` to a sentence saying it is the
    computed impact and how many commits carry no signal.
  - The component SHALL NOT set `version`.
  - WHEN given a pull-request evidence bundle, the component SHALL refuse,
    because pull-request notes are prose for a reviewer and a restatement of
    subjects gives the reviewer nothing to review.
- **Pinned rules:**
  - A notes entry maps field for field: `kind`, `scope`, `summary`,
    `detail`, `breaking` and `migration`. A breaking notes entry has no
    `kind` and takes CHORE, which the renderer places under the breaking
    heading like any breaking entry. Its `sources` hold the pull
    request WHEN the commit's record carries a number, and the commit
    otherwise.
  - A derived entry's `summary` is the subject's description: the subject
    without its conventional type prefix and its trailing ` (#<n>)`. A
    conventional type maps to `kind` case-insensitively: `feat` FEATURE,
    `fix` FIX, `perf` PERFORMANCE, `revert` REVERT, `refactor` REFACTOR,
    `docs` DOCUMENTATION, `build` BUILD, `ci` CI, `test` TEST, `style`
    STYLE, and any other type, or a subject that is not conventional,
    CHORE. The conventional
    scope is `scope`.
  - A derived entry's `breaking` is the commit's breaking-change marker as
    [[component-bump-classifier]] reads it; `migration` is the text of a
    `BREAKING CHANGE:` footer, with its continuation lines, WHEN one exists.
  - Entries follow the bundle's commit order, oldest first, and a block's
    entries keep their order within it.
  - Corpus-level `sources` are the bundle's `sources`, copied unchanged.
  - The computed impact is the highest `signal` in the bundle, and none
    WHEN no commit carries one.

## Cross-Cutting Standards

This component SHALL comply with:

- [[contract-changelog]] — the corpus semantics.

## Change Guidance

The component SHALL NOT propose an impact higher or lower than the computed
one.

**Rationale:** It reads no diff, so any other proposal would be a guess
dressed as a reading of the evidence. Restating the computed impact keeps
the reconciler's decision where it belongs: a range whose commits all carry
reviewed signals is decided by them, and one with unclassified commits still
halts for a person.

---

- **Verification gates:** the conformance suite every producer runs, plus
  the component's unit suite over bundles with blocks, without blocks, with
  malformed blocks, and with breaking footers.

## Key Decisions

- [[adr-003]] — a proposal is the reading of evidence; this producer reads
  none, so it restates what the reviewed records compute.
- [[adr-004]] — conventional subjects are used where present and never
  required.

## Open Questions

- [ ] What corpus `kind` a notes entry from the other heading takes. It
  has no `kind`, `ChangelogEntry.kind` is required, and the renderer puts
  under the other heading only kinds `ChangeKind` does not declare, so
  every declared kind either moves the entry to a group or drops it. A
  producer never writes such an entry; one written by hand reaches this
  rule.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-producer]] — what every producer must do
- [[contract-changelog]] — the corpus this producer returns
- [[frd-changelog-authoring]] — the feature this component serves
