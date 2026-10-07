---
title: "Feature Blueprint: Changelog Authoring"
description: How release core assembles a changelog corpus from git,
  exchanges it with an out-of-process producer, validates and caches what
  returns, and how `release changelog` drives it.
slug: fb-changelog-authoring
type: feature-blueprint
status: in-progress
updated_at: 2026-10-07T00:00:00Z
delivery: single-pr
plan: release-mng
frd: frd-changelog-authoring
---

# Changelog Authoring

## Overview

**FRD:** [[frd-changelog-authoring]]

The corpus schema first gains the provenance fields the FRD needs. Then
[[component-changelog-corpus-builder]] turns commit records into an
assembled corpus, [[component-changelog-authoring-bridge]] hands it to a
producer named in configuration and validates, compares and caches what
comes back, and `release changelog` drives the sequence and renders through
the changelog rendering feature. The hosting-platform adapter is specified
but deferred past the MVP. Delta blocks in dependency order: SCH, ACFG, CB,
PRM, BR, PROD, CLCMD.

## Element Deltas

### [[contract-changelog]] and `schemas/changelog.strata` (modified)

**Intent:** The FRD requires a corpus that records which adapters built it
(AC-CLOG-002.4), marks mechanically derived entries (AC-CLOG-002.3), and
carries no proposed impact until a producer sets one (AC-CLOG-002.5). The
schema today has no field for the first two and makes `proposed_bump`
required.

**Data:**
- `Changelog` SHALL gain `sources: [AdapterRun]`, where `AdapterRun` holds
  `adapter` (`commits`, `pull_requests` or `diffs`), `from` (the base
  commit hash, or absent for a line with no release) and `to` (the head
  commit hash).
- `ChangelogEntry` SHALL gain `derived: Bool`, true WHEN the entry was
  derived from a commit subject rather than author-written prose.
- `Changelog.proposed_bump` SHALL become optional: absent on an assembled
  corpus, and required of a producer by the bridge.
- The contract SHALL state each of these semantics. `schema_version` SHALL
  stay 1.0, amended in place: nothing has been released against it.

**Anchors:** `schemas/changelog.strata`;
`specs/contracts/changelog-contract.md`; the generated bindings (bead
release-8bt.2's generator).

### [[module-release-core]] (modified — authoring configuration)

**Intent:** The settings authoring reads, added to the configuration model
the tool configuration feature defines.

**Behavior:**
- The configuration SHALL gain `changelog.producer` (a producer name,
  default `claude`), `changelog.producer_command` (a command,
  unset by default, required WHEN the producer is `command`), and
  `adapters.pull_requests` and `adapters.diffs` (bools, default `false`).
- The module SHALL refuse `adapters.diffs = true` while no diff adapter is
  implemented, and `adapters.pull_requests = true` until the PRM delta
  lands.

**Anchors:** the release-core configuration model (tool configuration
feature, CFGM delta).

### `Changelog Corpus Builder` in [[module-release-core]] (added)

**Intent:** Implement [[component-changelog-corpus-builder]] with the
commit adapter: commit records in, an assembled corpus out.

**Behavior:**
- The component SHALL emit one entry per commit record, in record order,
  and SHALL record the commit adapter in `sources` with the range it read.
- An author-written block SHALL be a `## Release notes` heading in the
  commit body (the pull request description, under squash merge),
  running to the next `## ` heading or the end of the body; its first
  paragraph is the summary and the rest is the detail.
- WHEN a record's body carries an author-written block, the entry SHALL
  take its summary, and its detail WHEN present, from the block verbatim,
  with `derived` false.
- Otherwise the entry SHALL be derived from the subject, with `derived`
  true: a conventional-commit type maps to `ChangeKind` (`feat` FEATURE,
  `fix` FIX, `perf` PERFORMANCE, `revert` REVERT, `refactor` REFACTOR,
  `docs` DOCUMENTATION, `build` BUILD, `ci` CI, `test` TEST, `style` STYLE,
  anything else CHORE), the conventional scope becomes `scope`, and the
  summary is the subject without its type prefix and trailing ` (#<n>)`.
- An entry SHALL be `breaking` WHEN its commit carries a breaking-change
  marker as the bump classifier defines one, with `migration` taken from
  the `BREAKING CHANGE:` footer text WHEN present.
- An entry's `sources` SHALL hold the pull request WHEN the record has a
  number, and the commit otherwise.
- The component SHALL NOT set `proposed_bump` or `version`, and SHALL NOT
  enable an adapter that configuration does not.

**Anchors:** `internal/corpus` (new, per the component blueprint).

### `Pull Request Metadata Collector` in [[module-release-core]] (added)

**Intent:** Implement [[component-pr-metadata-collector]] as the
`pull_requests` adapter. Outside the MVP: every adopting repository
squash-merges with the description as the body, so this work order may be
deferred without holding up anything else.

**Behavior:**
- WHEN `adapters.pull_requests` is on and a record has a pull request number
  and an empty body, the builder SHALL fetch that pull request through the
  `gh` CLI and use its description as the body.
- The adapter SHALL NOT replace a body the commit already carries.
- IF `gh` is missing, unauthenticated or the platform is unreachable, THEN
  the builder SHALL return the commit-only corpus and report the
  degradation.

**Anchors:** `internal/platform` (new, per the component blueprint).

### `Changelog Authoring Bridge` in [[module-release-core]] (added)

**Intent:** Implement [[component-changelog-authoring-bridge]]: select the
configured producer, exchange the corpus, validate, compare, cache.

**Behavior:**
- The bridge SHALL invoke the producer named by `changelog.producer` and
  no other.
- The bridge SHALL validate a returned corpus against the JSON Schema
  generated from `schemas/changelog.strata`, and SHALL reject one that fails
  or declares an unimplemented major `schema_version`, before anything else
  reads it.
- The bridge SHALL reject a returned corpus with no `proposed_bump`.
- The bridge SHALL report agreement or disagreement between `proposed_bump`
  and the computed impact, and SHALL NOT resolve a disagreement. A
  maintainer confirms by naming the bump, which then governs and makes the
  proposal advisory (`schemas/changelog.strata`).
- The bridge SHALL cache an accepted corpus as
  `.git/release/corpus/<from>..<to>.json`, where `from` and `to` are the
  range's commit hashes (`root` for a line with no release), and SHALL reuse
  it without invoking a producer WHEN it exists.
- WHEN the maintainer asks to redo notes, the bridge SHALL ignore and
  replace the cached corpus.
- WHEN the maintainer asks to skip notes, the bridge SHALL NOT run, and the
  assembled corpus SHALL proceed unchanged.
- The bridge SHALL carry returned prose byte-for-byte.

**Anchors:** `internal/authoring` (new, per the component blueprint).

### `Changelog Producer` in [[module-release-core]] (added — MVP producers)

**Intent:** The producers the MVP ships, each conforming to
[[component-changelog-producer]]: `command` and `claude`, with `claude` the
default. `conventional` and `manual` are later work.

**Behavior:**
- The `command` producer SHALL run `changelog.producer_command` with the
  assembled corpus as JSON on standard input and read the authored corpus
  as JSON from standard output, failing on a non-zero exit.
- The `claude` producer SHALL run `claude -p` with a prompt invoking the
  `release:changelog` skill (the `changelog` skill of this repository's
  `release` plugin), the assembled corpus as JSON on
  standard input, and SHALL read the authored corpus as JSON from
  standard output, failing on a non-zero exit.
- Tests SHALL exercise the `claude` producer against a stub `claude` on
  `PATH`, never the real one.
- Every producer SHALL refuse, naming the missing prerequisite, WHEN it
  cannot run.

**Anchors:** `internal/authoring/producers` (new, per the component
blueprint).

### [[container-release-cli]] (modified — `release changelog`)

**Intent:** `release changelog [bump]` per [[contract-release-cli]]:
resolve the version, assemble, author, render, and print or write.

**Behavior:**
- The command SHALL resolve the version through the version computation
  feature's operation, using `bump` WHEN given; IF no release is warranted,
  THEN it SHALL exit 1.
- WHEN authoring returns a disagreement and no bump was named, the command
  SHALL print the computed and proposed impacts and exit 1, writing
  nothing.
- The command SHALL set the corpus `version`, render it through the
  changelog rendering feature's renderer, and answer per
  [[cli-output-standards]] with `version`, `corpus_path`, and `section`
  (the rendered changelog section).
- `--skip-notes`, `--redo-notes`, `--corpus <path>` and `--write` SHALL
  behave as the contract states; `--corpus` reads the corpus at that path
  instead of the cache WHEN it exists, and writes it there otherwise.
- `--write` SHALL place the section through the changelog writer and make
  no other change.

**Anchors:** `internal/cli`; `testdata/help/`, `testdata/cases/`.

### Relied-upon elements (unchanged)

- [[component-commit-collector]] — records, pull request numbers and bodies
  (version computation feature).
- [[component-bump-classifier]] — the computed impact and the
  breaking-change marker rules.
- [[component-changelog-renderer]], [[component-changelog-writer]] — the
  rendering and file placement (changelog rendering feature).

## Contracts

- [[contract-changelog]] (revised, drafted in place by the SCH delta) —
  corpus-level `sources`, entry-level `derived`, optional `proposed_bump`.

## Tracker Reconciliation

- release-8bt.2 — prerequisite: SCH regenerates the bindings and BR
  validates against the generated JSON Schema.
- release-8bt.4 — out-of-scope: the `release` plugin and its `changelog`
  skill, which the `claude` producer invokes, are that bead's; PROD only
  invokes it.
- release-cb2.4 — prerequisite: CB consumes the commit records that work
  order produces.
- release-cb2.7 — prerequisite: CLCMD resolves the version through that
  work order's operation.

## Sequencing Constraints

- The SCH delta SHALL land before the CB, PRM and BR deltas, and its
  schema, contract and generated bindings SHALL change together in it
  ([[module-release-core]] Change Guidance).
- The ACFG delta SHALL land before the CB, PRM, BR and PROD deltas.
- The CB, BR and PROD deltas SHALL land before the CLCMD delta.
- The changelog rendering feature's RND and WR deltas SHALL land before the
  CLCMD delta.

## AC Coverage

Legend: SCH = [[contract-changelog]] delta; ACFG = [[module-release-core]]
delta (— authoring configuration); CB = `Changelog Corpus Builder` delta;
PRM = `Pull Request Metadata Collector` delta; BR = `Changelog Authoring
Bridge` delta; PROD = `Changelog Producer` delta (— MVP producers); CLCMD =
[[container-release-cli]] delta (— `release changelog`).

| AC | Satisfied by | Verified by |
|----|--------------|-------------|
| AC-CLOG-001.1 | **CB (proves)** — via relied-upon Commit Collector | integration |
| AC-CLOG-001.2 | **CB (proves)** — via relied-upon Commit Collector | integration |
| AC-CLOG-001.3 | ACFG, **PRM (proves)** | integration |
| AC-CLOG-001.4 | **PRM (proves)** | integration |
| AC-CLOG-001.5 | **PRM (proves)** | integration |
| AC-CLOG-001.6 | **CB (proves)** — via relied-upon Commit Collector | integration |
| AC-CLOG-001.7 | ACFG, **CB (proves)** | unit |
| AC-CLOG-002.1 | SCH, **CB (proves)** | unit |
| AC-CLOG-002.2 | **CB (proves)** | unit |
| AC-CLOG-002.3 | SCH, **CB (proves)** | unit |
| AC-CLOG-002.4 | SCH, **CB (proves)** | unit |
| AC-CLOG-002.5 | SCH, **CB (proves)** | unit |
| AC-CLOG-003.1 | **BR (proves)** | unit |
| AC-CLOG-003.2 | **BR (proves)** | unit |
| AC-CLOG-003.3 | **BR (proves)** | unit |
| AC-CLOG-003.4 | BR, **CLCMD (proves)** | e2e |
| AC-CLOG-003.5 | BR, **CLCMD (proves)** | e2e |
| AC-CLOG-003.6 | **BR (proves)** | unit |
| AC-CLOG-003.7 | **BR (proves)**, PROD | unit |
| AC-CLOG-003.8 | ACFG, **BR (proves)**, PROD | unit |
| AC-CLOG-003.9 | **BR (proves)** | unit |
| AC-CLOG-003.10 | BR, **CLCMD (proves)** | e2e |
| AC-CLOG-003.11 | BR, **CLCMD (proves)** | e2e |

## Key Decisions

- [[adr-003]] — prose proposes, computation decides.
- [[adr-004]] — authoring is a replaceable step behind a structured corpus.
- [[adr-006]] — git through the `git` binary; the PR adapter likewise
  reaches the platform through `gh`.

## Definition of Done

- [ ] Every delta above is folded into its owning blueprint:
  - [ ] [[module-release-core]] — Interface records the authoring settings
  - [ ] [[component-changelog-corpus-builder]],
    [[component-changelog-authoring-bridge]],
    [[component-changelog-producer]] — Interfaces record the pinned rules
  - [ ] [[component-pr-metadata-collector]] — when PRM lands
  - [ ] [[container-release-cli]] — Command Surface records
    `release changelog`
- [ ] [[contract-changelog]] revision approved and its version updated
- [ ] Every AC in the coverage table is verified at its declared level
  and cited from its tests per the specs plugin's
  `shared/ac-traceability-standards.md`
- [ ] This blueprint's status → retired

## Open Questions

None.

## References

- `shared/c4-standards.md` — the module and component definitions these
  deltas are written against
- [[frd-changelog-authoring]] — the requirements this realizes
- [[contract-changelog]] — the corpus revised and exchanged
- [[fb-version-computation]], [[fb-tool-configuration]],
  [[fb-changelog-rendering]] — the features this builds on
