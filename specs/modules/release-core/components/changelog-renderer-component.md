---
title: "Component Blueprint: Changelog Renderer"
entity_name: Changelog Renderer
description: A changelog corpus rendered to markdown, one rendering per
  target, deterministically and from a single input.
slug: component-changelog-renderer
type: component-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/render
test_paths:
  - internal/render
---

# Changelog Renderer

## Overview

Turns one corpus into markdown, once per target: a `CHANGELOG.md` section and
a release body on the hosting platform. Both renderings come from the same
instance, which is what structurally prevents a release's two public
descriptions
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
- **Composed into:** [[module-release-core]] — consumed by
  [[component-changelog-writer]] and by release creation; see it for
  relationships.
- **Source & tests:** `internal/render`.

## Interface

- **Surface:** `Render` takes a corpus, the `changelog` settings and an
  optional repository web URL, and returns every target together from that
  one corpus value: the changelog section and the release body.
- **Layout:** the section is a `## <version>` heading with no date, then
  the title in bold and the summary when present, then the breaking-changes
  group, each configured group in configured order, and the other group,
  each group a `### <heading>` and a list. The release body is the section
  without its heading line and the blank line after it.
  - An entry is one list item: `**<scope>:** ` when scoped, the summary,
    and its sources in parentheses; `detail` and then `Migration:
    <migration>` follow as indented paragraphs when set, and a blank line
    ends an item carrying them.
  - A breaking entry renders only in the breaking-changes group, whatever
    its kind. An entry with no kind, or of a kind the component does not know,
    renders in the other group; an entry of a known kind absent from `changelog.groups`
    does not render. A group with no entries renders no heading.
  - With a repository web URL, a pull request renders as
    `[#<n>](<url>/pull/<n>)` and a commit as
    `[<hash7>](<url>/commit/<hash>)`; without one, as `#<n>` and
    `<hash7>`. An issue renders as its id.
  - Output uses `\n` line endings and ends with exactly one newline; a
    corpus with no title, summary or entries renders only the heading, and
    an empty body.
- **Guarantees:**
  - The component SHALL produce byte-identical output for identical input.
  - The component SHALL render every target from one corpus instance.
  - The component SHALL preserve the relative order of entries within each
    rendered group.
  - WHEN an entry declares a `kind` the component does not recognise, the
    component SHALL render it into a default group rather than failing.
  - The component SHALL render a breaking-changes section whenever the
    corpus declares one.
  - IF `version` is unset, or `schema_version` has a major other than 1,
    THEN the component SHALL refuse to render.
  - The component SHALL NOT render the corpus's `omitted` list, its commits
    or their reasons.
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
  target, including the unrecognised-`kind` case, the unkinded case, the
  breaking-changes section, a corpus with an omitted list, and a corpus
  with no entries.

## Key Decisions

- [[adr-003]] — one corpus renders every artifact, so the artifacts cannot
  contradict one another.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[contract-changelog]] — the corpus this component consumes
- [[frd-changelog-rendering]] — the feature this component serves
