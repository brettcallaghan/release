---
title: "Feature Blueprint: Release Staging"
description: How release core cuts a release as a stage branch and pull
  request, with every failable check before the first write, and how
  `release cut` drives it.
slug: fb-release-staging
type: feature-blueprint
status: in-progress
updated_at: 2026-10-07T00:00:00Z
delivery: single-pr
plan: release-elu
frd: frd-release-staging
---

# Release Staging

## Overview

**FRD:** [[frd-release-staging]]

[[component-stage-orchestrator]] is the one component that writes. It
composes the features before it: version resolution, the authoring bridge,
the renderer and writer, and the delegated command runner. It prepares the
release in a scratch checkout so every check, including the project's own
commands and the release-path check, runs before anything reaches the
repository. Only then does it create the branch, commit, push and open the
pull request. `release cut` is its front end. Delta blocks in dependency
order: ORCH, CUT.

## Element Deltas

### `Stage Orchestrator` in [[module-release-core]] (added)

**Intent:** Implement [[component-stage-orchestrator]]. The sequence is the
design: everything that can refuse runs before the first write, and a
write is defined narrowly so that holds.

**Behavior:**
- A write SHALL mean creating or moving a ref, creating a commit, pushing,
  or opening a pull request. Files changed in the scratch checkout are not
  writes.
- The orchestrator SHALL first run `git fetch origin <target>` and pin the
  release to that fetched `origin/<target>` commit. The version, the
  commit range and the scratch checkout SHALL all use that one commit,
  never the maintainer's current branch, `HEAD` or working tree.
- The scratch checkout SHALL be a temporary detached `git worktree` at the
  pinned commit, removed on every exit, success or failure. The
  maintainer's working tree, index and current branch SHALL NOT change,
  and uncommitted work in them SHALL NOT reach the release.
- The orchestrator SHALL run these steps in this order, stopping at the
  first refusal or failure:
  1. resolve the version through the version computation feature's
     operation, with the named bump WHEN given; refuse WHEN no release is
     warranted;
  2. refuse WHEN a branch named for the stage branch already exists
     locally or on `origin`;
  3. run `commands.preflight`;
  4. assemble the corpus and, unless notes are skipped, run the authoring
     bridge; halt WHEN it reports a disagreement and no bump was named;
  5. set the corpus `version` and render it;
  6. in the scratch checkout at the pinned commit, write the
     changelog section, then run `commands.propagate`, which SHALL be
     configured;
  7. refuse WHEN any changed path is neither `CHANGELOG.md` nor listed in
     `release_paths`;
  8. run `commands.validate` WHEN configured;
  9. write: create the stage branch at the pinned commit, commit the changes
     as its first commit with the message `Release <version>`, push it to
     `origin`, and open the pull request unless told not to.
- IF notes are skipped and no bump is named, THEN the orchestrator SHALL
  refuse before step 1.
- The pull request SHALL target the target branch, be titled
  `Release <version>`, and carry the release body under a `## Release
  Notes` heading.
- The orchestrator SHALL open the pull request through the `gh` CLI.
- IF the push fails, THEN the orchestrator SHALL delete the local stage
  branch it created in step 9, so a rerun is not refused by step 2, and
  SHALL report the failure and the deletion.
- IF opening the pull request fails, THEN the orchestrator SHALL report
  every write it made, SHALL NOT delete the pushed branch, and SHALL print
  the commands that finish (`gh pr create`) or discard
  (`git push origin --delete <branch>`, `git branch -D <branch>`) the
  release.
- The orchestrator SHALL NOT delete or change anything on `origin`.
- WHEN asked for a dry run, the orchestrator SHALL run steps 1 to 8 and
  report the writes of step 9 without making them.
- The orchestrator SHALL NOT create a tag, publish, merge, or move the
  target branch.

**Anchors:** `internal/cut` (new, per the component blueprint).

### [[container-release-cli]] (modified — `release cut`)

**Intent:** `release cut [bump]` per [[contract-release-cli]].

**Behavior:**
- The command SHALL accept the flags the contract lists and pass them to
  the orchestrator.
- WHILE standard input is a terminal and `--yes` is absent, the command
  SHALL show the version and the writes it will make, and prompt once
  before the first write.
- The command SHALL answer per [[cli-output-standards]] with `version`,
  `branch`, `commit`, `pull_request` (URL, or null), and `writes` (every
  write made, in order), and under `--dry-run` with `version`, `branch`
  and `writes` listing the writes it would make.
- The command SHALL exit 1 on a refusal, 3 WHEN a write failed after
  another succeeded, and 0 otherwise.

**Anchors:** `internal/cli`; `testdata/help/`, `testdata/cases/`.

### Relied-upon elements (unchanged)

- The version resolution operation (version computation feature).
- The Delegated Command Runner and configuration model (tool
  configuration feature).
- [[component-changelog-corpus-builder]],
  [[component-changelog-authoring-bridge]] (changelog authoring feature).
- [[component-changelog-renderer]], [[component-changelog-writer]]
  (changelog rendering feature).
- [[repository-release-policy]] — the project's own required checks, which
  verify a merged stage branch; the tooling does not.

## Contracts

None. [[contract-release-cli]] already specifies `release cut`.

## Tracker Reconciliation

- release-cb2.7 — prerequisite: ORCH resolves the version through that
  work order's operation.

## Sequencing Constraints

- The tool configuration, changelog rendering and changelog authoring
  features' deltas that ORCH composes SHALL land before the ORCH delta.
- The ORCH delta SHALL land before the CUT delta.

## AC Coverage

Legend: ORCH = `Stage Orchestrator` delta; CUT = [[container-release-cli]]
delta (— `release cut`).

| AC | Satisfied by | Verified by |
|----|--------------|-------------|
| AC-STG-001.1 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.2 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.3 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.4 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.5 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.6 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.7 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.8 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.9 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.10 | **ORCH (proves)** | manual |
| AC-STG-001.11 | ORCH, **CUT (proves)** | e2e |
| AC-STG-001.12 | ORCH, **CUT (proves)** | e2e |

The e2e cases run against a fixture repository with a bare `origin` and a
stub `gh` on `PATH`, per the component's verification gate.

## Key Decisions

- [[adr-002]] — stage branches merged by pull request are the release
  trigger.
- [[adr-003]] — a disagreement over impact halts the release.
- [[adr-006]] — git through the `git` binary; the platform through `gh`.

## Definition of Done

- [ ] Every delta above is folded into its owning blueprint:
  - [ ] [[component-stage-orchestrator]] — Interface records the step
    order and the definition of a write
  - [ ] [[container-release-cli]] — Command Surface records `release cut`
- [ ] Every AC in the coverage table is verified at its declared level
  and cited from its tests per the specs plugin's
  `shared/ac-traceability-standards.md`
- [ ] This blueprint's status → retired

## Open Questions

None.

## References

- `shared/c4-standards.md` — the module and component definitions these
  deltas are written against
- [[frd-release-staging]] — the requirements this realizes
- [[fb-version-computation]], [[fb-tool-configuration]],
  [[fb-changelog-rendering]], [[fb-changelog-authoring]] — the features
  this composes
