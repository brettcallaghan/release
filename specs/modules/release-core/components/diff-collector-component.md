---
title: "Component Blueprint: Diff Collector"
entity_name: Diff Collector
description: Per-commit code diffs collected as changelog evidence, leaving
  out excluded paths and keeping within a byte budget, with a report of
  everything it truncated or omitted.
slug: component-diff-collector
type: component-blueprint
status: approved
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
diff of every commit in the release range. A commit's subject and body say
what its author meant; the diff says what it did, and it is the only
account of a commit whose message says little.

Diffs are the one source whose size scales with how much changed, and a
model-backed producer pays per token. So the collector works within a byte
budget, leaves out paths that are noise to a reader (lockfiles, vendored
code, golden test output), and reports exactly what it cut, so the producer
knows what it was not shown and the changelog can say so.

- **Role:** code diffs as changelog evidence.
- **Scope:** in — per-commit diffs, path exclusion, the budget and the
  truncation report; out — interpreting a diff, which is a producer's job.
- **Composed into:** [[module-release-core]] — feeds
  [[component-evidence-builder]]; see the module for relationships.
- **Source & tests:** `internal/gitread`.

## Interface

- **Surface:** given a repository, the commits in the release range, the
  exclusion list (`changelog.diff_exclude`) and a budget in bytes
  (`changelog.diff_budget.release`), return a `DiffRecord` per commit, as
  `schemas/collection.strata` defines it, and a `BudgetReport` as
  [[contract-changelog]] defines it: the limit, the bytes of patch text
  before the budget and carried, and every file excluded, binary,
  truncated or omitted, with its reason.
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
  - A path the exclusion list matches has no `FileDiff` in its commit's
    `DiffRecord`. A binary file and a file of which no hunk fits do, with
    empty patches.
  - A `DiffRecord` is `truncated` WHEN the budget cut any of its files,
    including one kept with an empty patch.
  - The report's total counts the patch text of every file that is neither
    excluded nor binary, before the budget; its used counts the patch text
    carried. A file's report entries follow commit order, then path order.
  - A merge commit's diff is against its first parent, as every commit's is.
  - The component SHALL read each diff independent of the user's git diff
    configuration (submodule format, path prefixes, textconv, relative
    paths, hunk context, inter-hunk context, diff algorithm, indent
    heuristic), so the same repository yields the same `DiffRecord` under any
    such setting.
  - Exclusion entries match as `release_paths` entries do (equal to the
    entry, under an entry ending in `/`, or matched as a glob), with `**`
    matching any number of path segments.

**Rationale for the size report:** a producer driven by a language model pays
per token, and a reader of the changelog should be able to see when an
entry was written from part of a change.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the changelog sources a release reads.

## Change Guidance

WHEN the allocation of the budget changes, the budget report SHALL still
name every file that lost content.

**Rationale:** The producer cannot ask for what it was not shown. The report
is the only way it, or a reviewer of the changelog, learns that an entry
rests on part of a change.

---

- **Verification gates:** the component's unit suite against fixture
  repositories, including the excluded-path, truncated-hunk, omitted-file,
  binary-file and budget-redistribution cases.

## Key Decisions

- [[adr-003]] — the structured changelog is authored from commit evidence,
  and the diff is the evidence of what a commit did.
- [[adr-004]] — a changelog authored from diffs is why this tool was chosen
  over conventional-commit tooling.

## Open Questions

- [ ] Whether the default budget of 204800 bytes per release, and the
  default exclusion list, hold up against real releases. Decided by the
  project owner from the first releases cut with diffs.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into,
  and the configuration keys
- [[component-evidence-builder]] — the component this collector feeds
- [[frd-changelog-authoring]] — the feature this component serves
