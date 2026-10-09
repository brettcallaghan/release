---
title: "Component Blueprint: Notes Codec"
entity_name: Notes Codec
description: The `## Release notes` block parsed from a commit body or pull
  request description into reviewed notes, and rendered from authored notes,
  under one grammar.
slug: component-notes-codec
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/notes
test_paths:
  - internal/notes
---

# Notes Codec

## Overview

The `## Release notes` block is where a pull request's release notes are
reviewed with its code, and, after a squash merge, where they reach git. It
has two readers with different needs: a reviewer, who edits it as prose, and
the release tooling, which reads its impact as a version signal and its
entries as changelog evidence. This component is the grammar that serves
both: readable markdown that parses to `PullRequestNotes`.

The entries render as the changelog section will: under the repository's
configured changelog headings, each section giving its entries' kind. A pull
request carrying three changes has three entries, and one `Release-Type:`
line for the pull request as a whole.

```markdown
## Release notes

<!-- release-notes: v1 head=3f2a9c1 -->
Release-Type: major

### 💥 Breaking changes

- **cut:** `release cut` refuses a named bump over an empty range.

  Pass `--force` to release anyway, for example for a rebuild.

  Migration: scripts cutting an empty range must add `--force`.

### 🚀 Features

- **cut:** `release cut --dry-run` shows the changelog section.
```

- **Role:** the one parser and renderer of the notes block.
- **Scope:** in — locating the block, parsing it, checking it, rendering it;
  out — authoring notes, reading or writing a pull request, and deciding
  what a block's impact means for a version.
- **Composed into:** [[module-release-core]] — feeds
  [[component-bump-classifier]] and [[component-evidence-builder]], and
  renders for [[component-pr-notes-writer]]; see the module for
  relationships.
- **Source & tests:** `internal/notes`.

## Interface

- **Surface:** every operation takes the repository's changelog settings
  (`changelog.breaking_heading`, `changelog.groups`,
  `changelog.other_heading`), which map section headings to kinds.
  `notes.Parse` takes a commit body or a pull request description and
  returns the block's `PullRequestNotes`, the head it is
  pinned to, and whether a block was found, or the line at which it stops
  following the grammar. `notes.Render` takes `PullRequestNotes` and a head
  commit hash and returns the block. It refuses an entry the settings do
  not render: one that is not breaking and whose kind no group names. `notes.Check` takes a description and
  whether the pull request's diff at its current head equals its diff at
  the head the marker records, and returns the parsed block, or why it is
  missing, malformed or stale. Comparing the diffs reads git, so the caller
  does it.
- **Guarantees:**
  - The component SHALL be a pure function of its inputs.
  - Parsing the output of `Render` under the same settings SHALL return the
    notes and head it was given.
  - The component SHALL name the first line that does not follow the
    grammar.
  - The component SHALL accept lines ending in LF or CRLF.
  - The component SHALL NOT treat a malformed block as absent: a caller
    learns that a block exists and why it does not parse.
  - The component SHALL leave out of the block a final paragraph made only
    of trailers, so `Co-authored-by:` lines a hosting platform appends to a
    squash commit never become notes.
  - The component SHALL NOT read or write a pull request or a repository.
- **Grammar (v1):** the grammar and the block guarantees are
  [[contract-changelog]]'s; this component implements them. In outline: the
  `## Release notes` heading; the marker
  `<!-- release-notes: v1 head=<hash> -->`; the `Release-Type: <bump>`
  line, which is the block's impact and its only signal; then `### <heading>`
  sections, each holding entries `- **<scope>:** <sentence>` with the scope
  optional, each entry followed by indented detail paragraphs and at most
  one `Migration: ` paragraph. A section's heading gives its entries' kind:
  the breaking heading makes them breaking with no kind, a group's heading
  gives that group's kind, and the other heading gives no kind. A second
  `## Release notes` heading, a section heading that is neither configured
  nor a default heading, a breaking entry with no `Migration:` paragraph,
  and a breaking entry under an impact below `major` each make the block
  malformed.
- **Pinned rules:**
  - A section heading resolves first against the settings, then against the
    default headings (`Breaking changes`, `Features`, `Fixes`,
    `Performance`, `Reverts`, `Other changes`) with the default
    configuration's meanings.
  - The block runs to the next line beginning `## ` or to the end of the
    text. WHEN it runs to the end, a final paragraph made only of trailers
    is not part of it.
  - The prose of `summary`, `detail` and `migration` is kept verbatim except
    for the list marker, the `Migration: ` label, the indentation, and the
    blank lines between paragraphs; `detail` paragraphs are joined by one
    blank line.
- **Check:** `notes.Check` reports, in this order of precedence:
  - `missing` — the description has no block;
  - `malformed` — the block does not follow the grammar, naming the line;
  - `stale` — the pull request's diff has changed since the head the marker
    records; a head that moved without changing the diff, such as a merge of
    the base branch, is not stale;
  - `current` otherwise.
- **Render:** the heading, a blank line, the marker with the full head
  hash, the `Release-Type:` line, then each non-empty section, in the
  Changelog Renderer's order, as a blank line, `### <heading>`, a blank line
  and its entries. An entry renders as the Changelog Renderer renders a list
  item, without sources: `- `, the bold scope and colon where there is a
  scope, the summary, then `detail` and `migration` as indented paragraphs,
  with a blank line after an entry carrying paragraphs. The rendered block
  ends with one newline and is identical for identical notes and settings.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the notes block as a reviewed signal and
  as changelog evidence.
- [[contract-changelog]] — the notes-block grammar and `PullRequestNotes`.

## Change Guidance

WHEN the grammar changes, the marker's version SHALL change with it, and the
component SHALL keep parsing every earlier version.

**Rationale:** Blocks live in commit bodies forever. A release cut after a
grammar change still reads every block merged before it.

---

The component SHALL NOT compare a block's impact with any other signal.

**Rationale:** Weighing one signal against another is the classifier's and
the reconciler's. A codec that did so would make the version depend on which
component happened to look first.

---

- **Verification gates:** the component's unit suite: round trips of
  rendered blocks under configured and default headings, each way a block
  is malformed, CRLF line endings, a
  block followed by trailers, a block followed by another heading, and each
  check outcome.

## Key Decisions

- [[adr-003]] — the notes block is the reviewed record of a pull request's
  impact and changes.
- [[adr-004]] — conventional-commit syntax is welcome and never required;
  the block uses none, because its sections give each entry's kind.

## Open Questions

- [ ] Whether a block written by hand may omit the marker's `head=`, and
  how `notes.Check` then reports it. Decided with [[contract-changelog]]'s
  open question of the same name.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[contract-changelog]] — the grammar and `PullRequestNotes`
- [[repository-release-policy]] — the block's review and its check in CI
- [[frd-changelog-authoring]] — the feature this component serves
