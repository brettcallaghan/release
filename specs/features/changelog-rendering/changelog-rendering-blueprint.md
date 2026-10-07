---
title: "Feature Blueprint: Changelog Rendering"
description: How release core renders one changelog corpus into every
  target in one pure pass, and places the changelog section idempotently.
slug: fb-changelog-rendering
type: feature-blueprint
status: draft
updated_at: 2026-10-07T00:00:00Z
delivery: single-pr
frd: frd-changelog-rendering
---

# Changelog Rendering

## Overview

**FRD:** [[frd-changelog-rendering]]

Two new release-core components and one configuration setting. The
[[component-changelog-renderer]] is a pure function from one corpus to every
target's markdown at once; the [[component-changelog-writer]] is the only
part that touches `CHANGELOG.md`. Group headings are configuration, added
to the model the tool configuration feature defines. No command lands here:
`release changelog` and `release cut` call these components and arrive with
the changelog authoring and release staging features. Delta blocks in
dependency order: HEAD, RND, WR.

## Element Deltas

### [[module-release-core]] (modified — changelog headings)

**Intent:** Let a project keep its changelog style by naming the heading
for each entry kind and the order groups appear in.

**Behavior:**
- The configuration SHALL gain a `changelog.groups` setting: an ordered
  list of mappings, each with a `kind` (a `ChangeKind` name) and a
  `heading`.
- The default `changelog.groups` SHALL be `FEATURE` "Features", `FIX`
  "Fixes", `PERFORMANCE` "Performance", `REVERT` "Reverts", in that order.
- The configuration SHALL gain `changelog.other_heading`, defaulting to
  "Other changes", and `changelog.breaking_heading`, defaulting to
  "Breaking changes".
- The module SHALL refuse a `changelog.groups` entry whose `kind` is not a
  `ChangeKind` name, or a kind listed twice.

**Anchors:** the configuration model the tool configuration feature adds
to release core (its CFGM delta).

### `Changelog Renderer` in [[module-release-core]] (added)

**Intent:** Implement [[component-changelog-renderer]]: one call renders
the changelog section and the release body from the same corpus, so the
two cannot differ.

**Behavior:**
- The component SHALL take a corpus, the `changelog` settings and an
  optional repository web URL, and SHALL return the changelog section and
  the release body together from that one corpus value.
- IF the corpus `version` is unset, or its `schema_version` has a major
  version the component does not implement, THEN the component SHALL
  refuse to render.
- The changelog section SHALL be laid out as in Open Questions (layout):
  a `## <version>` heading, then the title in bold and the summary WHEN
  present, then the breaking-changes group, then each configured group in
  configured order, then the other group.
- The release body SHALL be the changelog section without its
  `## <version>` heading line and the blank line after it.
- Every entry marked `breaking` SHALL render in the breaking-changes group
  with its `migration` beneath it, and SHALL NOT render again in its
  kind's group.
- An entry whose kind is not a `ChangeKind` name the component knows SHALL
  render in the other group. An entry of a known kind absent from
  `changelog.groups` SHALL NOT render.
- A group with no entries SHALL NOT render its heading.
- Each entry SHALL render as one list item: `**<scope>:** ` WHEN a scope is
  set, the summary, then its sources in parentheses, then `detail` as an
  indented paragraph WHEN set.
- WHEN a repository web URL is given, a pull request source SHALL render
  as `[#<n>](<url>/pull/<n>)` and a commit source as
  `[<hash7>](<url>/commit/<hash>)`; WHEN it is not, both SHALL render as
  plain `#<n>` and `<hash7>`. An issue source SHALL render as its id.
- Output SHALL use `\n` line endings and end with exactly one newline.

**Anchors:** `internal/render` (new, per the component blueprint); the
`Changelog` type generated from `schemas/changelog.strata`.

### `Changelog Writer` in [[module-release-core]] (added)

**Intent:** Implement [[component-changelog-writer]]: place a section in
`CHANGELOG.md` idempotently and touch nothing else.

**Behavior:**
- The component SHALL treat a line beginning `## ` whose first word,
  after an optional `[` and an optional `v`, parses as a version, as the
  start of that version's section, which runs to the next such line or the
  end of the file.
- WHEN the file holds a section for the same version, the component SHALL
  replace that section's bytes and nothing else.
- Otherwise the component SHALL insert the new section, followed by one
  blank line, immediately before the first existing section, leaving any
  preamble above it untouched; WHEN no section exists, after the preamble.
- WHEN `CHANGELOG.md` does not exist, the component SHALL create it as
  `# Changelog`, a blank line, and the section.
- The component SHALL write `CHANGELOG.md` in the repository root it is
  given, and SHALL NOT invoke git.

**Anchors:** `internal/changelogfile` (new, per the component blueprint).

### Relied-upon elements (unchanged)

- [[contract-changelog]] — the corpus semantics: order preserved, version
  required, unknown kinds tolerated, one instance for every artifact.
- `schemas/changelog.strata` — the corpus shape, through its generated Go
  types.

## Contracts

None. [[contract-changelog]] already specifies the corpus this consumes.

## Tracker Reconciliation

- release-8bt.2 — prerequisite: RND and WR use the Go types generated from
  `schemas/changelog.strata`.

## Sequencing Constraints

- The tool configuration feature's configuration-model delta SHALL land
  before the HEAD delta.
- The HEAD delta SHALL land before the RND delta.
- The renderer's golden files for every target SHALL be generated in the
  RND delta's change (component Change Guidance); no other lockstep rule
  spans delta blocks.

## AC Coverage

Legend: HEAD = [[module-release-core]] delta (— changelog headings); RND =
`Changelog Renderer` delta; WR = `Changelog Writer` delta.

| AC | Satisfied by | Verified by |
|----|--------------|-------------|
| AC-REND-001.1 | **RND (proves)** | unit |
| AC-REND-001.2 | **RND (proves)** | unit |
| AC-REND-001.3 | **RND (proves)** | unit |
| AC-REND-001.4 | **RND (proves)** | unit |
| AC-REND-001.5 | **RND (proves)** | unit |
| AC-REND-001.6 | **RND (proves)** | unit |
| AC-REND-001.7 | HEAD, **RND (proves)** | unit |
| AC-REND-002.1 | **WR (proves)** | unit |
| AC-REND-002.2 | **WR (proves)** | unit |
| AC-REND-002.3 | **WR (proves)** | unit |
| AC-REND-002.4 | **WR (proves)** | unit |
| AC-REND-002.5 | **WR (proves)** | unit |

## Key Decisions

- [[adr-003]] — one corpus renders every artifact.

## Definition of Done

- [ ] Every delta above is folded into its owning blueprint:
  - [ ] [[module-release-core]] — Interface records the `changelog`
    settings
  - [ ] [[component-changelog-renderer]] — Interface records the layout
  - [ ] [[component-changelog-writer]] — Interface records the section
    rule and the created file's shape
- [ ] Every AC in the coverage table is verified at its declared level
  and cited from its tests per the specs plugin's
  `shared/ac-traceability-standards.md`
- [ ] This blueprint's status → retired

## Open Questions

- [ ] **Approve the default layout.** The RND and WR behaviour lines pin a
  layout; it needs the owner's eye because every published changelog will
  look like it. For a corpus with a breaking `feat` and a `fix`:

  ```markdown
  ## 1.9.0

  **Faster resolution for large monorepos**

  Resolution now caches per package. Upgrade when convenient.

  ### Breaking changes

  - **cli:** `--target` is required ([#412](https://github.com/o/r/pull/412))

    Migration: pass `--target main` where you relied on the default.

  ### Fixes

  - Tags with a `v` prefix are recognised (#87)
  ```

  Choices inside it: no date in the heading (a date would be an input the
  renderer cannot compute purely); internal kinds (`REFACTOR`, `DOCUMENTATION`,
  `BUILD`, `CI`, `TEST`, `STYLE`, `CHORE`) left out by default, as the
  schema suggests; links only when a repository web URL is supplied,
  using GitHub's URL shapes. Blocks RND and WR. Decided by the project
  owner.

## References

- `shared/c4-standards.md` — the module and component definitions these
  deltas are written against
- [[frd-changelog-rendering]] — the requirements this realizes
- [[contract-changelog]] — the corpus consumed
- [[fb-tool-configuration]] — the configuration model HEAD extends
