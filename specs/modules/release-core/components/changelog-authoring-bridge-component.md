---
title: "Component Blueprint: Changelog Authoring Bridge"
entity_name: Changelog Authoring Bridge
description: The seam across which a selected producer receives a release's
  evidence bundle and returns a structured changelog, validated against the
  schema and the bundle, and cached by range.
slug: component-changelog-authoring-bridge
type: component-blueprint
status: approved
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
producer, hands it the release's evidence bundle, takes the **structured
changelog** back, and **validates it against the schema and against the
bundle** before anything reads it.

Validation is the bridge's main job. Everything downstream — the Bump
Suggester, the renderer, the changelog writer, the staging sequence —
assumes a conforming changelog, so this is the one place a malformed one is
caught before reaching something that would suggest a bump from it or
render it partially. Beyond its shape, the bridge checks what makes the
changelog an account of the range: its entries are changes, each breaking
entry says how to migrate, and every commit in the range is either cited by
an entry or listed as omitted with a reason. Nothing is dropped silently.

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
caches what comes back. Re-staging after a correction, or calling again with
a person's confirmation, reuses the authored changelog rather than paying
for it twice, and the suggestion derived from it stays the one the person
saw.

- **Role:** producer selection, the exchange, validation and caching.
- **Scope:** in — selecting a producer, exchanging a bundle, validating the
  changelog against the schema and the bundle, caching the result; out —
  calling a model directly, composing prose, and suggesting or deciding a
  bump.
- **Composed into:** [[module-release-core]] — between
  [[component-evidence-builder]] and both [[component-bump-suggester]] and
  [[component-changelog-renderer]]; see the module for relationships.
- **Source & tests:** `internal/authoring`.

## Interface

- **Surface:** select a producer, exchange an evidence bundle with it,
  validate the structured changelog it returns, and cache it.
- **Guarantees:**
  - The component SHALL validate a returned changelog against the schema
    [[contract-changelog]] names, and against the validation rules below,
    before any consumer reads it.
  - IF a returned changelog declares a `schema_version` the component does
    not implement, THEN the component SHALL reject it.
  - IF a commit in the bundle is neither cited by an entry's `sources` nor
    listed in `omitted` with a reason, THEN the component SHALL reject the
    changelog and name the commit.
  - IF a breaking entry carries no `migration`, THEN the component SHALL
    reject the changelog and name the entry.
  - The component SHALL preserve returned prose byte-for-byte.
  - The component SHALL cache an accepted changelog keyed by the commit
    range it was authored from.
  - The component SHALL keep the evidence bundle the producer was given
    beside the cached changelog, as `<from>..<to>.evidence.json`.
  - WHEN a cached changelog exists for the range, the component SHALL reuse
    it rather than invoking a producer.
  - The component SHALL allow the maintainer to discard a cached changelog
    and author again.
  - WHEN the maintainer requests staging without notes (`--skip-notes`),
    the component SHALL exchange the bundle with
    [[component-mechanical-producer]] in place of the configured producer,
    report that it did so, and neither read nor write the cache.
  - WHEN the configured producer is unavailable or fails, the component
    SHALL report the failure and stop, naming `--skip-notes` as the way to
    author the changelog with the mechanical producer, and SHALL NOT fall
    back to another producer on its own.
  - The component SHALL NOT suggest or decide a bump.
  - The component SHALL NOT call a language model itself.
  - The component SHALL NOT require network access of its own.
- **Operation:** `authoring.ExchangeEvidence` takes the evidence bundle, the
  configured producer name with the producers a configuration may select
  (the registry includes `mechanical`), the cache path, and the redo and
  skip requests. It answers with the accepted changelog, which producer
  authored it, and whether it was authored mechanically, read from the
  cache, or authored by the configured producer. A producer that fails or
  is unavailable is returned as `*ProducerFailure`, which names
  `--skip-notes`. `authoring.AcceptChangelog` is the validation alone.
  `proposed_bump` is ignored: neither compared nor rejected.
  `authoring.ChangelogCachePath` names a range's cache file.
- **Validation rules:**
  - **Shape.** `AcceptChangelog` rejects, before anything decodes the document for
    use, one that is not a JSON object, one whose `schema_version` is not a
    version or has a major other than the one implemented, and one that
    does not validate against the generated JSON Schema. A cached or named
    changelog is accepted the same way as a returned one.
  - **Range.** A changelog whose corpus-level `sources` differ from the
    bundle's `sources` is rejected: a producer copies them unchanged, and a
    changelog for another range does not apply.
  - **Entries are changes.** Entries are not matched to commits one for
    one. An entry may cite several commits, and one commit may be cited by
    several entries. Every entry cites at least one source, and an entry
    citing a commit absent from the bundle, or a pull request no commit in
    the bundle carries, is rejected.
  - **Entry fields.** Each entry has a `kind` from `ChangeKind`, or none
    for the other heading; an optional `scope`; a non-empty `summary`
    holding no blank line and no more than two sentences (a sentence end
    is `.`, `!` or `?` followed by whitespace and an uppercase letter, or
    by the end of the summary); an optional `detail`; and `migration` WHEN it
    is `breaking`.
  - **Every commit accounted for.** Every commit in the bundle is cited by
    at least one entry or listed in `omitted`, each `omitted` item naming a
    commit in the bundle with a non-empty one-line reason. `omitted` is not
    rendered.
  - **Rejection** names every rule broken, not only the first, so a
    producer or a person correcting by hand fixes them in one pass.
- **Cache:**
  - The cache file is `<git common dir>/release/changelog/<from>..<to>.json`,
    `from` and `to` the range's commit hashes and `from` `root` for a line
    with no release. The common directory is `git rev-parse
    --git-common-dir`, shared by every worktree. The file holds the
    producer's output byte for byte, and is replaced by an atomic rename
    of a fully written temporary file, so a failed write keeps the
    changelog already there.
  - The evidence bundle the producer was given is kept beside it, written
    whenever a producer authors, as the cache file's name with
    `.evidence.json` in place of `.json`; for a maintainer-named changelog
    `x.json` it is `x.evidence.json`.
  - WHEN the cache file exists and redo was not requested, the bridge reads
    it and invokes no producer. WHEN redo is requested, it invokes the
    producer and replaces the file only after accepting the result, so a
    failed redo keeps the earlier one.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the authored changelog a suggestion is
  derived from, and releasability.
- [[contract-changelog]] — the evidence and changelog semantics it
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

**Rationale:** Prose the tooling rewrites is prose nobody reviewed. The stage
pull request is the review surface, and it must show exactly what will be
published.

---

The component SHALL NOT relax the every-commit-accounted-for rule.

**Rationale:** It looks like strictness a producer could be spared, but it
is what makes a changelog's silence meaningful: a commit absent from every
entry is one the producer decided, with a stated reason, does not matter to
a reader, not one it missed.

---

WHEN a producer is added, it SHALL conform to
[[component-changelog-producer]] rather than being special-cased here.

**Rationale:** A producer the bridge knows by more than its name is a producer
nothing else can replace.

---

The component SHALL NOT invalidate a cached changelog on anything but a
change to the commit range or an explicit request.

**Rationale:** A cache that clears on an unrelated change makes every re-run
of a failed staging cost again, which is the expense it exists to prevent,
and would let the suggestion change between a halt and its confirmation.

---

- **Verification gates:** the component's unit suite, including rejection
  of an unknown schema version, a changelog for another range, an entry
  citing a commit outside the bundle, a breaking entry without migration, a
  commit neither cited nor omitted, an omission without a reason, a
  byte-identical round trip of prose containing markdown and non-ASCII
  text, a cache hit that does not invoke a producer, the mechanical
  producer under a skip request, and a reported failure naming
  `--skip-notes`, with no other producer invoked, when the configured
  producer is unavailable or fails.

## Key Decisions

- [[adr-003]] — a language model authors a structured changelog from commit
  evidence, with every commit cited or omitted with a reason.
- [[adr-004]] — changelog authoring is a replaceable step rather than a render
  template.

## Open Questions

None. The cache lives inside the repository under a path the project
gitignores. That keeps it discoverable by a maintainer wondering why
re-cutting was instant, keeps it out of commits, and means a clean checkout
starts empty — which is the correct behaviour, since a fresh clone has no
authored changelog to trust.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-producer]] — what every producer must do
- [[contract-changelog]] — the bundle and changelog exchanged across this
  seam
- [[frd-changelog-authoring]] — the feature this component serves
