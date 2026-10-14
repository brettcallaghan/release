---
title: "Component Blueprint: Changelog Writer"
entity_name: Changelog Writer
description: A rendered section placed into CHANGELOG.md idempotently,
  without disturbing the entries already there.
slug: component-changelog-writer
type: component-blueprint
status: approved
kind: standard
contracts_consumed: []
source_paths:
  - internal/changelogfile
test_paths:
  - internal/changelogfile
---

# Changelog Writer

## Overview

Places a rendered section into `CHANGELOG.md`. Separate from
[[component-changelog-renderer]] so that rendering stays a pure function
and the only component touching the filesystem is this one.

Its one difficult requirement is idempotency. Staging may be re-run after
a correction, and a writer that appends unconditionally produces a file
with the same version twice — which a reviewer may well miss in a large
diff. So the writer recognises a section it has already placed for a
version and replaces it rather than adding another.

- **Role:** the `CHANGELOG.md` edit.
- **Scope:** in — locating the insertion point, replacing an existing
  section for the same version, creating the file; out — rendering, and
  committing.
- **Composed into:** [[module-release-core]] — consumes
  [[component-changelog-renderer]]; see it for relationships.
- **Source & tests:** `internal/changelogfile`.

## Interface

- **Surface:** `Write` takes a repository root, a version and a rendered
  section, and writes `CHANGELOG.md` in that root.
- **Section rule:** a line beginning `## ` whose first word, after an
  optional `[` (with its closing `]`) and an optional `v`, parses as a
  version starts that version's section, which runs to the next such line
  or the end of the file. `## 1.9.0`, `## v1.9.0 (date)` and
  `## [v1.9.0] - date` all start a 1.9.0 section; `## Unreleased` starts
  none.
  - A section for the same version is replaced in place: its bytes become
    the new section, followed by one blank line when another section
    follows.
  - Otherwise the new section and one blank line are inserted immediately
    before the first section, leaving the preamble above it untouched;
    with no section, the section follows the preamble, after one blank
    line, and an empty file takes the section alone.
  - A file that does not exist is created as `# Changelog`, a blank line,
    and the section.
- **Guarantees:**
  - The component SHALL insert a new section above every existing section.
  - WHEN a section for the same version is already present, the component
    SHALL replace it rather than inserting a second.
  - WHEN `CHANGELOG.md` does not exist, the component SHALL create it.
  - The component SHALL preserve existing content byte-for-byte outside the
    section it writes.
  - The component SHALL NOT commit, stage, or otherwise invoke git.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the release-commit shape, of which this
  file is one of two permitted members.

## Change Guidance

WHEN the section delimiter changes, the component SHALL still recognise
sections written under the previous delimiter.

**Rationale:** An existing `CHANGELOG.md` holds years of sections written
under whatever delimiter was current. A writer that stops recognising them
loses idempotency on exactly the releases most likely to be re-staged.

---

The component SHALL NOT rewrite or reformat existing sections.

- **Verification gates:** the component's unit suite over fixture files,
  including an absent file, an empty file, a file whose top section is the
  same version, and a file with hand-edited content below the top section.

## Key Decisions

- [[adr-002]] — a release commit touches only version manifests and the
  changelog, which is what makes this component's write surface narrow.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[frd-changelog-rendering]] — the feature this component serves
