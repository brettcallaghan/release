---
title: "Module Blueprint: Release Core"
entity_name: Release Core
description: The library that computes versions, assembles and renders
  changelogs, and stages releases — every component of the release process,
  behind one in-process interface.
slug: module-release-core
type: module-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
kind: standard
visibility: internal
contracts_consumed: []
source_paths:
  - internal
test_paths:
  - internal
version: "0.1"
---

# Release Core

## Overview

Every capability of the release process lives here: reading tags and commits,
classifying impact, computing versions, assembling and rendering changelogs,
and staging a release as a branch and a pull request. The command-line tool is
a wrapper around this module and holds no release logic of its own.

The separation exists so the release process is reachable by more than one
front end. A command surface is the first consumer; an HTTP service or a
tool-calling server is a plausible second, and neither should have to
reimplement version computation or re-derive what a release commit may
contain. Keeping the logic here means a second front end is a new wrapper
rather than a new implementation.

A module rather than a set of components because it crosses a boundary of
independent change: it is composed into a container that does not own it, and
it has internal structure worth documenting. It is `internal` for now —
unpublished, evolving in lockstep with its consumers — and becomes `public`
the first time something outside this repository depends on it.

- **Responsibility:** the whole release process as a library.
- **Out of scope:** argument parsing, terminal output, exit codes, and
  process lifetime, which belong to a front end; building artifacts and
  publishing them, which belong to adopting repositories; calling a language
  model, which happens outside the process entirely.
- **Source & tests:** `internal/`.

## Interface

- **Surface:** one operation per release capability — resolve a base version,
  classify impact, compute a next version, collect records, assemble a
  corpus, exchange it with a producer, render it, write a changelog, stage a
  release. Each is callable independently; the staging operation composes the
  rest.
- **Guarantees:**
  - The module SHALL expose every capability independently of the staging
    operation, so a consumer may use one without invoking the others.
  - The module SHALL return structured results rather than writing to a
    terminal.
  - The module SHALL NOT read process arguments or environment for anything
    a caller can pass.
  - The module SHALL NOT terminate the process.
  - The module SHALL NOT call a language model.

**Rationale for the first two:** a front end that has to parse the library's
human-readable output is a front end that breaks when the wording changes, and
a library that prints cannot serve a service.

## Components

### Semver Core

[[component-semver-core]] — version parsing, comparison, and every name
derived from a version.

### Tag Reader

[[component-tag-reader]] — release tags reachable from a ref, as versions.

### Commit Collector

[[component-commit-collector]] — commits between two refs, with pull-request
identity and trailers parsed from them.

### Pull Request Metadata Collector

[[component-pr-metadata-collector]] — pull-request metadata fetched from the
hosting platform. **Not part of the MVP:** every adopting repository
squash-merges, so this metadata already reaches the release through git.

### Diff Collector

[[component-diff-collector]] — code diffs between two refs, as changelog
material. **Not part of the MVP:** author-written notes make it unnecessary,
and it is the only adapter whose cost scales with how much changed.

### Bump Classifier

[[component-bump-classifier]] — release impact computed deterministically from
commit metadata.

### Version Calculator

[[component-version-calculator]] — the next version, under the project's
version policy.

### Changelog Corpus Builder

[[component-changelog-corpus-builder]] — collected records assembled into a
changelog corpus.

### Changelog Authoring Bridge

[[component-changelog-authoring-bridge]] — the seam across which a producer
receives a corpus and returns an authored one.

### Changelog Producer

[[component-changelog-producer]] — the component type that authors a corpus: a
language model, a command, a conventional-commit parser, or a person.

### Changelog Renderer

[[component-changelog-renderer]] — a corpus rendered to markdown, one
rendering per target.

### Changelog Writer

[[component-changelog-writer]] — a rendered section placed into the changelog
file.

### Stage Orchestrator

[[component-stage-orchestrator]] — the sequence that cuts a release, producing
a stage branch and its pull request.

## Pattern Guidance

Not applicable — this module is a single instance, not a pattern.

## Versioning and Compatibility

WHILE this module is `internal`, it MAY be refactored or broken in the same
change as its consumers, and no deprecation window applies.

---

WHEN this module becomes `public`, its interface SHALL be versioned under
semantic versioning and this section SHALL be rewritten to state its
compatibility obligations.

---

The module SHALL version the changelog corpus it exchanges independently of
itself, under [[contract-changelog]].

**Rationale:** The corpus is written by producers outside this repository, so
its compatibility is a contract concern rather than a library-versioning one.

## Cross-Cutting Standards

This module SHALL comply with:

- [[release-tooling-standards]] — stage-branch and release-commit shape,
  version computation, and the configuration surface.

The following govern the adopting project rather than this module, and it
SHALL NOT enforce them:

- [[repository-release-policy]] — branch roles, support window, propagation,
  readiness, and the verification of a release commit.
- [[cli-distribution-policy]] — publication, resolution, provenance,
  channels, withdrawal.

## Change Guidance

WHEN a version-policy rule changes, [[component-version-calculator]] and the
acceptance criteria in [[frd-version-computation]] SHALL be updated in the
same change.

---

WHEN the changelog schema changes, `schemas/changelog.strata`,
[[contract-changelog]] and every generated binding SHALL be updated in the
same change.

---

WHEN a collector's record shape changes, `schemas/collection.strata` SHALL be
updated in the same change.

---

A component that computes a version, classifies impact, or renders a corpus
SHALL be a pure function of its inputs.

**Rationale:** These are the parts whose correctness matters most and whose
failures are hardest to see. Purity is what makes them exhaustively testable
without fixtures or network.

---

A component in this module SHALL NOT call a language model.

**Rationale:** Authoring happens outside the process, where a model is already
available, so that adopters need no API credential and no per-call budget. A
component that reached for a model would reintroduce both.

---

WHEN a component reads from the hosting platform, it SHALL degrade to
git-only behaviour rather than failing.

**Rationale:** The release path must work offline and in repositories hosted
on a platform the tooling does not integrate with.

---

- **Verification gates:** the unit suites of every component changed, plus the
  end-to-end staging test against a fixture repository.

## Key Decisions

- [[adr-001]] — the tooling is branch-agnostic; which branches carry which
  majors is the adopting project's policy.
- [[adr-002]] — a release is staged as a branch carrying the release commit
  and merged by pull request.
- [[adr-003]] — the bump is computed deterministically; authored prose
  proposes but never decides.
- [[adr-005]] — the release process is a composable library behind a thin
  front end, which is what this module is.

## Open Questions

None. No second consumer is near-term, so the interface may keep changing
freely while this module is `internal`. The separation from the command
surface is kept because it costs nothing now and would be expensive to
introduce later, not because a second front end is planned.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container composing this module
- [[contract-changelog]] — the corpus this module exchanges
- [[frd-version-computation]], [[frd-changelog-authoring]],
  [[frd-changelog-rendering]], [[frd-release-staging]],
  [[frd-tool-configuration]] — the features this module realizes
