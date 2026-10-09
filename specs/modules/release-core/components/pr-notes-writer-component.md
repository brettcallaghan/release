---
title: "Component Blueprint: PR Notes Writer"
entity_name: PR Notes Writer
description: A pull request's description read from the hosting platform,
  and a rendered `## Release notes` block written into it in place, leaving
  the rest of the description untouched.
slug: component-pr-notes-writer
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/prnotes
test_paths:
  - internal/prnotes
---

# PR Notes Writer

## Overview

The pull request description is where a notes block is reviewed. This
component puts the block there and reads it back: `release notes --write-pr`
writes it, and `release notes --check` reads the description it checks. It
reaches the hosting platform through `gh`, as the Stage Orchestrator does,
so it needs no credential of its own.

It is the only part of the pull-request flow that writes, and it writes one
thing: the block. Everything else in the description is the author's, and is
returned byte for byte. Where `gh` is missing or unauthenticated, the front
end prints the block for the person or agent opening the pull request to
paste instead.

- **Role:** the block into, and the description out of, a pull request.
- **Scope:** in — reading a description and its head commit, replacing or
  appending the block, writing the description; out — authoring, rendering
  and checking the block, and any other write to the platform.
- **Composed into:** [[module-release-core]] — renders through
  [[component-notes-codec]]; see the module for relationships.
- **Source & tests:** `internal/prnotes`.

## Interface

- **Surface:** `prnotes.Read` takes a repository and a pull request (a
  number, or the current branch's) and returns its description, its number
  and its head commit hash. `prnotes.Write` takes the same, the rendered
  block, and the head hash the block was authored from, and writes the
  description with the block in place.
- **Guarantees:**
  - WHEN the description holds a `## Release notes` block, the component
    SHALL replace that block and nothing else.
  - WHEN the description holds no block, the component SHALL append it,
    separated from the existing text by a blank line.
  - The component SHALL preserve every byte of the description outside the
    block.
  - IF the pull request's head differs from the head the block was authored
    from, THEN the component SHALL refuse to write, naming both.
  - IF `gh` is missing, or `gh auth status` scoped to `origin`'s host reports
    it unauthenticated, THEN the component SHALL refuse before reading, and
    name the prerequisite.
  - The component SHALL NOT write anything to the platform but the
    description of the pull request it was given.
  - The component SHALL NOT change the pull request's title, state, labels
    or reviewers.
- **Pinned rules:** the description is read with `gh pr view <pr> --json
  number,body,headRefOid` and written with `gh pr edit <pr> --body-file`,
  from a temporary file, so the description never passes through an
  argument list. The block's extent is the [[component-notes-codec]]
  grammar's.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the notes block.
- [[repository-release-policy]] — the notes block in every pull request
  into a release line.

## Change Guidance

WHEN another hosting platform is supported, it SHALL sit behind the same two
operations, so no caller learns which platform answered.

---

The component SHALL NOT become a prerequisite of a release.

**Rationale:** A release reads blocks from git. Writing them is a
convenience at pull-request time; a release cut where the platform is
unreachable loses nothing.

---

- **Verification gates:** the component's unit suite against a recorded `gh`,
  covering replacement, appending, a description with text after the block,
  a stale head, and a missing or unauthenticated `gh`.

## Key Decisions

- [[adr-003]] — the block is reviewed in the pull request description.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-notes-codec]] — the block's extent and rendering
- [[frd-changelog-authoring]] — the feature this component serves
