---
title: "Component Blueprint: Changelog Authoring Bridge"
entity_name: Changelog Authoring Bridge
description: The seam across which a selected producer receives an evidence
  bundle and returns an authored changelog corpus or authored pull-request
  notes, validated against the schema, cached by range, and its proposed
  impact compared.
slug: component-changelog-authoring-bridge
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/authoring
test_paths:
  - internal/authoring
---

# Changelog Authoring Bridge

## Overview

The boundary between the module and whoever writes the prose. It selects a
producer, hands it an evidence bundle, takes the authored result back,
**validates that structured representation against the schema**, and
compares its proposed impact with the computed one.

There are two exchanges, told apart by the bundle's `purpose`. At pull
request time the producer returns `PullRequestNotes`, which the Notes Codec
renders as a block for review. At release time it returns a changelog
corpus, whose entries are changes rather than commits and whose proposal
comes with its reasons.

Validation is the bridge's main job. Everything downstream — the renderer, the
changelog writer, the reconciler, the staging sequence — assumes a conforming
result, so this is the one place a malformed one is caught before reaching
something that would render it partially.

Producers are plugins rather than a fixed set: a language model, a configured
command, the mechanical producer, or a person editing a file.
[[component-changelog-producer]] governs what every producer must do. The
bridge tells them apart only by name.

The default producer runs outside the process. A language model is available
on the workstation where a maintainer stages a release, and putting the call
inside the tooling would oblige every adopter to hold an API credential and a
per-call budget for something they already have. That ignorance is the
component's value: it makes authoring replaceable and keeps the tooling free
of model configuration.

Authoring is slow, and for a model producer it is paid for, so the bridge
caches what comes back. Re-staging after a correction reuses the authored
result rather than paying for it twice.

- **Role:** producer selection, the exchange, validation, caching, and the
  comparison of impacts.
- **Scope:** in — selecting a producer, exchanging a bundle, schema
  validation, impact comparison, caching the result; out — calling a model
  directly, composing prose, and deciding the impact.
- **Composed into:** [[module-release-core]] — between
  [[component-evidence-builder]] and [[component-notes-codec]] at pull
  request time, and [[component-changelog-renderer]] and
  [[component-impact-reconciler]] at release time; see the module for
  relationships.
- **Source & tests:** `internal/authoring`.

## Interface

- **Surface:** select a producer, exchange an evidence bundle with it,
  validate the result, cache it, and report agreement or disagreement with
  a computed impact.
- **Guarantees:**
  - The component SHALL validate a returned corpus, or returned
    `PullRequestNotes`, against the schema [[contract-changelog]] names
    before any consumer reads it.
  - IF a returned result declares a `schema_version` the component does not
    implement, THEN the component SHALL reject it.
  - WHEN the returned proposal differs from the computed impact, the
    component SHALL report a disagreement.
  - The component SHALL NOT resolve a disagreement; deciding is
    [[component-impact-reconciler]]'s.
  - The component SHALL preserve returned prose byte-for-byte.
  - The component SHALL cache an authored result keyed by its purpose and
    the commit range it was authored from.
  - WHEN a cached result exists for the range and purpose, the component
    SHALL reuse it rather than invoking a producer.
  - The component SHALL allow the maintainer to discard a cached result and
    author again.
  - WHEN the maintainer requests staging without notes, the component SHALL
    exchange the bundle with [[component-mechanical-producer]] in place of
    the configured producer, and SHALL neither read nor write the cache.
  - The component SHALL NOT call a language model itself.
  - The component SHALL NOT require network access of its own.
- **Operation:** `authoring.Exchange` takes the evidence bundle, the
  computed impact, the configured producer name with the producers a
  configuration may select, the cache path, and the redo and skip requests.
  It answers with the accepted corpus or notes, whether it was skipped,
  read from the cache or authored, the proposed and computed impacts with
  the proposal's rationale and drivers, and whether they disagree.
- **Pinned rules:**
  - A producer receives the evidence bundle as indented JSON, HTML
    characters unescaped, and returns its result as JSON.
  - `Accept` rejects, before anything decodes the document for use, one
    that is not a JSON object, one whose `schema_version` is not a version
    or has a major other than the one implemented, and one that does not
    validate against the generated JSON Schema. At release time it also
    rejects a corpus carrying no `proposed_bump` or no
    `proposal_rationale`. A cached or named result is accepted the same way
    as a returned one.
  - A returned, cached or named corpus whose corpus-level `sources` differ
    from the bundle's `sources` is rejected: a producer copies them
    unchanged, and a corpus for another range does not apply.
  - A corpus is rejected WHEN an entry's `sources`, or its
    `proposal_drivers`, cite a commit or pull request absent from the
    bundle. Entries are not matched to commits one for one, because an entry
    is a change: it may cite several commits, one commit may yield several
    entries, and internal work may be cited by none.
  - The cache file is `<git common dir>/release/<purpose>/<from>..<to>.json`,
    `purpose` `corpus` at release time and `notes` at pull request time,
    `from` and `to` the range's commit hashes and `from` `root` for a line
    with no release. The common directory is `git rev-parse
    --git-common-dir`, shared by every worktree. The file holds the
    producer's output byte for byte, and is replaced by an atomic rename
    of a fully written temporary file, so a failed write keeps the result
    already there.
  - WHEN the cache file exists and redo was not requested, the bridge reads
    it and invokes no producer. WHEN redo is requested, it invokes the
    producer and replaces the file only after accepting the result, so a
    failed redo keeps the earlier one.
  - A disagreement is a proposal differing from the computed impact. The
    bridge reports it and leaves the decision to its caller.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — showing a proposal and its reasons before
  anything is pushed.
- [[contract-changelog]] — the corpus, evidence and notes semantics it
  validates against.

## Change Guidance

The component SHALL NOT acquire a language-model client, an API credential,
or a network dependency.

**Rationale:** Authoring runs where a model already exists. Moving the call
inside the module would impose a credential and a budget on every adopter,
which is the cost this design exists to avoid.

---

The component SHALL NOT edit returned prose, including normalising whitespace
or rewrapping lines.

**Rationale:** Prose the tooling rewrites is prose nobody reviewed. The pull
request and the stage pull request are the review surfaces, and they must
show exactly what will be published.

---

WHEN a producer is added, it SHALL conform to
[[component-changelog-producer]] rather than being special-cased here.

**Rationale:** A producer the bridge knows by more than its name is a producer
nothing else can replace.

---

The component SHALL NOT invalidate a cached result on anything but a change to
the commit range or an explicit request.

**Rationale:** A cache that clears on an unrelated change makes every re-run
of a failed staging cost again, which is the expense it exists to prevent.

---

- **Verification gates:** the component's unit suite at both purposes,
  including rejection of an unknown schema version, a corpus without a
  rationale, the disagreement path, a byte-identical round trip of prose
  containing markdown and non-ASCII text, and a cache hit that does not
  invoke a producer.

## Key Decisions

- [[adr-003]] — a proposal is compared and shown with its reasons, never
  trusted on its own.
- [[adr-004]] — changelog authoring is a replaceable step rather than a render
  template.

## Open Questions

None. The cache lives inside the repository under a path the project
gitignores. That keeps it discoverable by a maintainer wondering why
re-cutting was instant, keeps it out of commits, and means a clean checkout
starts empty — which is the correct behaviour, since a fresh clone has no
authored result to trust.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-producer]] — what every producer must do
- [[contract-changelog]] — the bundle and results exchanged across this seam
- [[frd-changelog-authoring]] — the feature this component serves
