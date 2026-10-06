---
title: "Component Blueprint: Changelog Renderer"
description: A changelog corpus rendered to markdown, one rendering per
  target, deterministically and from a single input.
slug: component-changelog-renderer
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed:
  - contract-changelog
source_paths:
  - cli/release/internal/render
test_paths:
  - cli/release/internal/render
---

# Changelog Renderer

## Overview

Turns one corpus into markdown, once per target: a `CHANGELOG.md` section
and a forge release body. Both renderings come from the same instance,
which is what structurally prevents a release's two public descriptions
from disagreeing — a failure recorded as a cost in [[adr-003]] and removed
here.

The renderer is pure and deterministic. Given a corpus it produces the same
bytes every time, which makes it testable with golden files and makes a
rendering diff meaningful during review of a stage pull request.

Section titles, their order, and any decoration are configuration, so a
project can keep the changelog style it already publishes.

- **Role:** markdown for every target, from one corpus.
- **Scope:** in — grouping, ordering, formatting, per-target layout; out —
  authoring prose, writing files, and deciding a version.
- **Composed into:** [[container-release-cli]] — consumed by
  [[component-changelog-writer]] and by release creation; see it for
  relationships.
- **Source & tests:** `cli/release/internal/render`.

## Interface

- **Surface:** given a corpus and a target, return rendered markdown.
- **Guarantees:**
  - The component SHALL produce byte-identical output for identical input.
  - The component SHALL render every target from one corpus instance.
  - The component SHALL preserve the relative order of entries within each
    rendered group.
  - WHEN an entry declares a `kind` the component does not recognise, the
    component SHALL render it into a default group rather than failing.
  - The component SHALL render a breaking-changes section whenever the
    corpus declares one.
  - IF `version` is unset, THEN the component SHALL refuse to render.
  - The component SHALL NOT read the filesystem, git, or the network.

## Cross-Cutting Standards

This component SHALL comply with:

- [[contract-changelog]] — the semantics of the corpus it consumes.

## Change Guidance

WHEN rendering changes, the golden files for every target SHALL be
regenerated in the same change.

---

The component SHALL render every target from the same corpus instance in
the same invocation.

**Rationale:** Rendering targets separately, from separately built corpora,
is exactly how a release body and a changelog section come to disagree.

---

The component SHALL NOT re-order entries within a group.

**Rationale:** Authoring order is the author's judgement about what matters
most. A renderer that sorts discards it silently.

- **Verification gates:** the component's golden-file suite across every
  target, including the unrecognised-`kind` case, the breaking-changes
  section, and a corpus with no entries.

## Key Decisions

- [[adr-003]] — one corpus renders every artifact, so the artifacts cannot
  contradict one another.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[container-release-cli]] — the container this component is composed into
- [[contract-changelog]] — the corpus this component consumes
- [[frd-changelog-rendering]] — the feature this component serves
