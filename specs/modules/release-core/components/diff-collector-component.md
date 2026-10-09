---
title: "Component Blueprint: Diff Collector"
entity_name: Diff Collector
description: Per-commit code diffs collected as changelog evidence, leaving
  out excluded paths and keeping within a byte budget, with a report of
  everything it truncated or omitted.
slug: component-diff-collector
type: component-blueprint
status: draft
updated_at: 2026-10-09T00:00:00Z
kind: standard
contracts_consumed: []
source_paths:
  - internal/gitread
test_paths:
  - internal/gitread
---

# Diff Collector

## Overview

The code is the honest record of what changed, so a producer is shown the
diff. At pull request time it reads the diff of every commit on the pull
request, and the notes it authors are reviewed with that code. At release
time it reads the diffs only of the commits that reached git without a
reviewed notes block: the others are already described by notes a reviewer
read, and reading their diffs again would spend budget on what is known.

Diffs are the one source whose size scales with how much changed, and a
model-backed producer pays per token. So the collector works within a byte
budget, leaves out paths that are noise to a reader (lockfiles, vendored
code, golden test output), and reports exactly what it cut, so the producer
knows what it was not shown and the corpus can say so.

- **Role:** code diffs as changelog evidence.
- **Scope:** in — per-commit diffs, path exclusion, the budget and the
  truncation report; out — interpreting a diff, which is a producer's job,
  and choosing which commits need one, which is its caller's.
- **Composed into:** [[module-release-core]] — feeds
  [[component-evidence-builder]]; see the module for relationships.
- **Source & tests:** `internal/gitread`.

## Interface

- **Surface:** given a repository, the commits whose diffs are wanted, the
  exclusion list (`changelog.diff_exclude`) and a budget in bytes
  (`changelog.diff_budget.release` or `.pull_request`), return a
  `DiffRecord` per commit, as `schemas/collection.strata` defines it, and
  a `BudgetReport` as [[contract-changelog]] defines it: the limit, the
  bytes of patch text before the budget and carried, and every file
  excluded, binary, truncated or omitted, with its reason.
- **Guarantees:**
  - The component SHALL return a diff for each commit it was given, against
    its first parent.
  - The component SHALL NOT include the patch of a path the exclusion list
    matches, and SHALL list it in the report with reason `EXCLUDED`.
  - The component SHALL keep the patch text it returns within the budget.
  - The component SHALL report the total size of the patch text before the
    budget is applied.
  - WHEN a file's patch is cut to fit, the component SHALL mark it
    `truncated`, mark its `DiffRecord` `truncated`, and list it in the
    report with reason `OVER_BUDGET`.
  - The component SHALL NOT truncate silently.
  - The component SHALL NOT run WHEN `adapters.diffs` is `false`.
  - The component SHALL NOT interpret a diff.
  - The component SHALL NOT require network access.
- **Pinned rules:**
  - The budget is shared equally among the commits requested. A commit
    whose patch text is smaller than its share leaves the rest to the
    commits still over theirs, in commit order, until no commit is over or
    the budget is spent.
  - Within a commit, files are taken in path order. A file whose patch does
    not fit what remains is cut at the last whole hunk that does, and a
    file of which no hunk fits is kept with an empty patch and listed with
    reason `OVER_BUDGET` and `truncated` false.
  - A binary file has an empty patch and is listed with reason `BINARY`.
  - Exclusion entries match as `release_paths` entries do (equal to the
    entry, under an entry ending in `/`, or matched as a glob), with `**`
    matching any number of path segments.

**Rationale for the size report:** a producer driven by a language model pays
per token, and a reader of the corpus should be able to see when the notes
were written from part of a change.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the changelog sources a release reads.

## Change Guidance

WHEN the allocation of the budget changes, the budget report SHALL still
name every file that lost content.

**Rationale:** The producer cannot ask for what it was not shown. The report
is the only way it, or a reviewer of the corpus, learns that the notes rest
on part of a change.

---

- **Verification gates:** the component's unit suite against fixture
  repositories, including the excluded-path, truncated-hunk, omitted-file,
  binary-file and budget-redistribution cases.

## Key Decisions

- [[adr-003]] — notes are authored from evidence, and the diff is the
  evidence for a commit no one reviewed notes for.
- [[adr-004]] — notes authored from diffs are why this tool was chosen over
  conventional-commit tooling.

## Open Questions

- [ ] Whether the default budgets of 204800 bytes per release and 102400
  bytes per pull request, and the default exclusion list, hold up against
  real releases. Decided by the project owner from the first releases cut
  with diffs.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into,
  and the configuration keys
- [[component-evidence-builder]] — the component this collector feeds
- [[frd-changelog-authoring]] — the feature this component serves
