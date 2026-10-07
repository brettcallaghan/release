---
title: "RFC: Pin What the Changelog Rendering Specs Left to the Implementer"
description: Close three gaps the changelog rendering implementers met:
  unknown kinds the decoder refuses, a bracketed heading the section rule
  cannot parse, and layout choices the blueprint left open
slug: rfc-0003-changelog-rendering-implementation-retro
type: rfc
status: ready-for-review
updated_at: 2026-10-07T00:00:00Z
origin: brettcallaghan/release
skill: retro-spec
skill_version: "1.5.0"
model: claude-opus-5-5[1m]
effort: high
runs_observed: 1
severity: snag
plugin_version: "0.25.0"
---

# RFC: Pin What the Changelog Rendering Specs Left to the Implementer

## Summary

The changelog rendering feature (epic release-fgl) was implemented in one
session, bead by bead. Every acceptance criterion is proved, but three
times the implementer met a question the documents answered wrongly or not
at all: whether an unknown entry kind can reach the renderer, how a
bracketed `## [v1.2.0]` heading parses, and how the pinned layout spaces
items that carry paragraphs. The proposals pin each in the documents.

## Provenance

| Field | Value |
|-------|-------|
| Origin project | brettcallaghan/release |
| Phase executed | Implemented the changelog rendering plan (beads release-fgl.1–.4) |
| Documents exercised | `specs/features/changelog-rendering/changelog-rendering-blueprint.md`, `changelog-rendering-frd.md`, `specs/contracts/changelog-contract.md`, `schemas/changelog.strata`, `specs/modules/release-core/release-core-module.md`, `changelog-renderer-component.md`, `changelog-writer-component.md` |
| Documents produced | `internal/config` (changelog settings), `internal/render`, `internal/changelogfile`, `config/release.toml`, folds into the module and both component blueprints |
| Skill(s) followed | retro-spec 1.5.0 (this retro); the implementing beads followed their own bead text |
| Templates / standards read | release-tooling-standards (corpus copy), the AC traceability standard |
| Plugin version | specs 0.25.0 |
| Session date | 2026-10-07 |

## Evidence

Harvested from the `retro:` comments on `bd list --label
fb-changelog-rendering`, plus this run's DoD verification.

1. Implementing the renderer (release-fgl.2), AC-REND-001.4's unknown-kind
   case could not be fed from JSON. The contract says "the provider MAY
   add optional fields and MAY add enum variants without a major version
   change", and the blueprint relies on it as "unknown kinds tolerated",
   but the Go binding generated from `schemas/changelog.strata` refuses any
   `kind` outside the enum (`ChangeKind: unknown value`). The renderer's
   other group is reachable only from a corpus built in Go.
2. Implementing the writer (release-fgl.3), the WR delta says a heading's
   first word parses as a version "after an optional `[` and an optional
   `v`". Read literally, `## [v1.2.0]` leaves `1.2.0]`, which is not a
   version, yet the bead's fixtures require `## [v1.2.0]` headings. The
   implementation also strips the closing `]`.
3. Implementing the renderer, the RND delta pins one example with a single
   item per group. No text says whether items carrying `detail` or
   `migration` paragraphs are separated from the next item by a blank
   line, what the release body is for a corpus with no title, summary or
   entries, or whether `migration` renders on a non-breaking entry (the
   contract allows it: "Also used where adoption is optional but
   non-obvious"). The implementation chose a blank line after such an
   item, an empty body, and migration wherever set, and recorded all three
   in `changelog-renderer-component.md`.
4. Implementing the headings (release-fgl.1), no generated list of
   `ChangeKind` values exists, so `config.ChangeKinds` repeats the enum by
   hand.
5. The CRLF fixture for AC-REND-002.4 was normalised to LF by the
   repository's `.gitattributes` (`* text=auto eol=lf`) and needed a
   `-text` exception.
6. The `specs` CLI was not available in the session, so the folds were
   not checked by `specs check --strict`.

## Proposals

### P1 — Say how an unknown entry kind reaches a consumer

**Problem.** `changelog-contract.md`, Versioning and Change Policy: "the
provider MAY add optional fields and MAY add enum variants without a major
version change." No text says that a consumer must decode an unknown
`kind` rather than reject it, and the generated binding rejects it
(finding 1), so AC-REND-001.4 holds for the renderer function but not for
a corpus read from a file.

**Change.** Add a guarantee to the contract: "Consumers SHALL decode an
entry whose `kind` names a variant they do not implement, keeping its
spelling, and SHALL render it in the default group." Then either change
the decode step the authoring bridge will use (a lenient `kind` decode
before validation) or ask the Strata binding for an open-enum option, and
add an AC-REND-001.4 case that reads the corpus from JSON.

**Evidence.** Finding 1. The first command that reads a corpus file
(`release changelog`, release-mng.7) would otherwise refuse a corpus the
contract calls compatible.

### P2 — Strip the closing bracket in the section rule

**Problem.** The WR delta: "a line beginning `## ` whose first word, after
an optional `[` and an optional `v`, parses as a version" (finding 2).

**Change.** Reword to: "whose first word, with an optional enclosing `[`
and `]` removed and then an optional `v`, parses as a version". This is
what `internal/changelogfile` does and what
`changelog-writer-component.md` now records.

**Evidence.** Finding 2.

### P3 — Ratify the layout choices the example did not cover

**Problem.** The RND delta's layout ends at the 1.9.0 example and "Each
entry SHALL render as one list item: … then `detail` as an indented
paragraph WHEN set." No text covers spacing between items carrying
paragraphs, an empty corpus's body, or migration on a non-breaking entry
(finding 3).

**Change.** Accept or amend the three rules
`changelog-renderer-component.md` now states: "a blank line ends an item
carrying them", "a corpus with no title, summary or entries renders only
the heading, and an empty body", and "`detail` and then `Migration:
<migration>` follow as indented paragraphs when set". Whatever the review
picks, regenerate the goldens under `internal/render/testdata/`.

**Evidence.** Finding 3. The release body is published verbatim, so a
spacing choice is visible to every reader of a release.

## Considered and Not Proposed

- No generated list of change kinds (finding 4) — the fix lands in the
  Strata generator, outside this repository; the hand list is guarded by a
  test that every listed kind validates.
- `.gitattributes` normalising a CRLF fixture (finding 5) — repository
  configuration, now handled by a `-text` line; not a spec document.
- `specs check --strict` not run (finding 6) — environment, not documents.

## Non-Goals

- No change to the implemented behaviour for P2 and P3; P1 adds a decode
  rule for a consumer not yet built.

## Adoption

The proposals are independent. P1 touches the changelog contract and the
changelog authoring blueprint's beads; P2 and P3 touch the retired
changelog rendering blueprint (amend in place, or rely on the component
blueprints, which already record both). All are backward compatible:
existing documents and corpora stay valid.

## Resolution
