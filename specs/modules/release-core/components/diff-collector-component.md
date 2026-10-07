---
title: "Component Blueprint: Diff Collector"
description: The code diff between two refs, collected as changelog material
  for releases where the commits and pull requests do not say enough.
slug: component-diff-collector
type: component-blueprint
status: draft
updated_at: 2026-10-06T00:00:00Z
kind: standard
contracts_provided: []
contracts_consumed: []
source_paths:
  - internal/gitread
test_paths:
  - internal/gitread
---

# Diff Collector

## Overview

Reads the code diff between the last release and the head of a target branch,
so a producer can describe what changed from the change itself rather than
from what someone wrote about it.

It is the most expensive adapter and the least often needed. Where
contributors write release notes into pull-request descriptions, the commits
already carry reviewed prose about every change, and reading diffs adds cost
without adding information. Where they do not, the diff is the only honest
source.

**Opt-in, and deferred beyond the first milestone.** A release that needs it
is a release whose pull-request discipline is not carrying its weight, and
fixing that is cheaper than reading every diff.

- **Role:** code diffs as changelog material.
- **Scope:** in — diff enumeration between two refs, with per-path
  granularity; out — interpreting a diff, which is a producer's job.
- **Composed into:** [[module-release-core]] — an adapter of
  [[component-changelog-corpus-builder]]; see the module for relationships.
- **Source & tests:** `internal/gitread`.

## Interface

- **Surface:** given two refs, return the diff between them, grouped by path.
- **Guarantees:**
  - The component SHALL return diffs only for paths the repository has not
    excluded.
  - The component SHALL report the total size of the diff before returning
    its content.
  - WHEN the diff exceeds a configured budget, the component SHALL report the
    excess rather than truncating silently.
  - The component SHALL NOT run unless the diff adapter is enabled.
  - The component SHALL NOT interpret a diff.

**Rationale for the size report:** a producer driven by a language model pays
per token, so the caller needs the cost before committing to it.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — the changelog sources a release reads.

## Change Guidance

The component SHALL remain disabled by default.

**Rationale:** Every other adapter is cheap and bounded. This one is neither,
and a default that silently reads every diff in a release makes the cost of a
release depend on how much changed.

---

- **Verification gates:** the component's unit suite against fixture
  repositories, including the exceeds-budget and excluded-path cases.

## Key Decisions

- [[adr-005]] — publication and artifact work stay with adopters, which is
  why this component reads diffs for authoring only and never for building.

## Open Questions

- [ ] What a sensible default diff budget is, in a unit a maintainer can
  reason about. Decided when the first producer that consumes diffs is built.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-corpus-builder]] — the component this adapter serves
- [[frd-changelog-authoring]] — the feature this component serves
