---
title: "Component Blueprint: Changelog Authoring Bridge"
entity_name: Changelog Authoring Bridge
description: The seam across which a selected producer receives a changelog
  corpus and returns an authored one, with the returned corpus validated
  against the schema and its proposed impact checked.
slug: component-changelog-authoring-bridge
type: component-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
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
producer, hands it the assembled corpus, takes an authored corpus back,
**validates that structured representation against the schema**, and compares
its `proposed_bump` against the computed impact.

Validation is the bridge's main job. Everything downstream — the renderer, the
changelog writer, the staging sequence — assumes a conforming corpus, so this
is the one place a malformed one is caught before reaching something that
would render it partially.

Producers are plugins rather than a fixed set: a language model, a configured
command, a conventional-commit parser, or a person editing a file.
[[component-changelog-producer]] governs what every producer must do, the same
way [[component-changelog-corpus-builder]] governs its adapters. The bridge
tells them apart only by name.

The default producer runs outside the process. A language model is available
on the workstation where a maintainer stages a release, and putting the call
inside the tooling would oblige every adopter to hold an API credential and a
per-call budget for something they already have. That ignorance is the
component's value: it makes authoring replaceable and keeps the tooling free
of model configuration.

Authoring is slow, and for a model producer it is paid for, so the bridge
caches what comes back. Re-staging after a correction reuses the authored
corpus rather than paying for it twice.

- **Role:** producer selection, the exchange, validation, and caching.
- **Scope:** in — selecting a producer, exchanging a corpus, schema
  validation, impact comparison, caching the result; out — calling a model
  directly, and composing prose.
- **Composed into:** [[module-release-core]] — between
  [[component-changelog-corpus-builder]] and
  [[component-changelog-renderer]]; see the module for relationships.
- **Source & tests:** `internal/authoring`.

## Interface

- **Surface:** select a producer, exchange a corpus with it, validate the
  result, cache it, and report agreement or disagreement with a computed
  impact.
- **Guarantees:**
  - The component SHALL validate a returned corpus against the schema
    [[contract-changelog]] names before any consumer reads it.
  - IF a returned corpus declares a `schema_version` the component does not
    implement, THEN the component SHALL reject it.
  - WHEN the returned `proposed_bump` differs from the computed impact, the
    component SHALL report a disagreement.
  - The component SHALL NOT resolve a disagreement between the returned
    `proposed_bump` and the computed impact.
  - The component SHALL preserve returned prose byte-for-byte.
  - The component SHALL cache an authored corpus keyed by the commit range it
    was authored from.
  - WHEN a cached corpus exists for the range being staged, the component
    SHALL reuse it rather than invoking a producer.
  - The component SHALL allow the maintainer to discard a cached corpus and
    author again.
  - WHEN the maintainer requests staging without notes, the component SHALL
    be skipped entirely.
  - The component SHALL NOT call a language model itself.
  - The component SHALL NOT require network access of its own.
- **Operation:** `authoring.Exchange` takes the assembled corpus, the
  computed impact, the configured producer name with the producers a
  configuration may select, the cache path, and the redo and skip requests.
  It answers with the accepted corpus, whether it was skipped, read from
  the cache or authored, the proposed and computed impacts, and whether they
  disagree.
- **Pinned rules:**
  - A producer receives the assembled corpus as indented JSON, HTML
    characters unescaped, and returns the authored corpus as JSON.
  - `Accept` rejects, before anything decodes the document for use, one
    that is not a JSON object, one whose `schema_version` is not a version
    or has a major other than 1, one that does not validate against the
    generated JSON Schema, and one carrying no `proposed_bump`. A cached or
    named corpus is accepted the same way as a returned one.
  - A returned, cached or named corpus whose corpus-level `sources` differ
    from the assembled corpus's is rejected: a producer carries them
    through, and a corpus for another range does not apply. Entry `sources`
    are not compared, because a producer may merge entries.
  - The cache file is `<git common dir>/release/corpus/<from>..<to>.json`,
    `from` and `to` the range's commit hashes and `from` `root` for a line
    with no release. The common directory is `git rev-parse
    --git-common-dir`, shared by every worktree. The file holds the
    producer's output byte for byte, and is replaced by an atomic rename
    of a fully written temporary file, so a failed write keeps the corpus
    already there.
  - WHEN the cache file exists and redo was not requested, the bridge reads
    it and invokes no producer. WHEN redo is requested, it invokes the
    producer and replaces the file only after accepting the result, so a
    failed redo keeps the earlier corpus.
  - WHEN skip is requested, the bridge reads no cache, invokes no producer,
    and returns the assembled corpus with no proposal and no disagreement.
  - A disagreement is `proposed_bump` differing from the computed impact.
    The bridge reports it and leaves the decision to its caller.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the halt-on-disagreement rule.
- [[contract-changelog]] — the corpus semantics it validates against.

## Change Guidance

The component SHALL NOT acquire a language-model client, an API credential,
or a network dependency.

**Rationale:** Authoring runs where a model already exists. Moving the call
inside the module would impose a credential and a budget on every adopter,
which is the cost this design exists to avoid.

---

The component SHALL NOT edit returned prose, including normalising whitespace
or rewrapping lines.

**Rationale:** Prose the tooling rewrites is prose nobody reviewed. The stage
pull request is the review surface, and it must show exactly what will be
published.

---

WHEN a producer is added, it SHALL conform to
[[component-changelog-producer]] rather than being special-cased here.

**Rationale:** A producer the bridge knows by more than its name is a producer
nothing else can replace.

---

The component SHALL NOT invalidate a cached corpus on anything but a change to
the commit range or an explicit request.

**Rationale:** A cache that clears on an unrelated change makes every re-run
of a failed staging cost again, which is the expense it exists to prevent.

---

- **Verification gates:** the component's unit suite, including rejection of
  an unknown schema version, the disagreement path, a byte-identical round
  trip of prose containing markdown and non-ASCII text, and a cache hit that
  does not invoke a producer.

## Key Decisions

- [[adr-003]] — a proposal is compared, never trusted.
- [[adr-004]] — changelog authoring is a replaceable step rather than a render
  template.

## Open Questions

None. The cache lives inside the repository under a path the project
gitignores. That keeps it discoverable by a maintainer wondering why
re-cutting was instant, keeps it out of commits, and means a clean checkout
starts empty — which is the correct behaviour, since a fresh clone has no
authored corpus to trust.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-producer]] — what every producer must do
- [[contract-changelog]] — the corpus exchanged across this seam
- [[frd-changelog-authoring]] — the feature this component serves
