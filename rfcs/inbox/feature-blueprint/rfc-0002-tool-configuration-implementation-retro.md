---
title: "RFC: Pin What the Tool Configuration Specs Left to the Implementer"
description: Close two gaps in the tool configuration blueprint that
  implementers of release-0qc had to decide alone, and carry one
  acceptance criterion forward to the feature that can prove it
slug: rfc-0002-tool-configuration-implementation-retro
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

# RFC: Pin What the Tool Configuration Specs Left to the Implementer

## Summary

The tool configuration feature (epic release-0qc) was implemented in one
session, bead by bead. Every bead landed, but three times the implementer
met a question the blueprint did not answer: who adds the global flags the
loader depends on, what `release config` does without a default branch,
and how to prove "refused before any write" before any command writes.
The proposals pin the first two in the documents and hand the third to the
feature that introduces the first writing command.

## Provenance

| Field | Value |
|-------|-------|
| Origin project | brettcallaghan/release |
| Phase executed | Implemented the tool configuration plan (beads release-0qc.1–.6) |
| Documents exercised | `specs/features/tool-configuration/tool-configuration-blueprint.md`, `tool-configuration-frd.md`, `specs/contracts/release-cli-contract.md`, `specs/modules/release-core/release-core-module.md`, `specs/containers/release-cli/release-cli-container.md` |
| Documents produced | `internal/config`, `internal/delegate`, `internal/cli/config.go`, folds into the module and container blueprints, `delegated-command-runner-component.md` |
| Skill(s) followed | retro-spec 1.5.0 (this retro); the implementing beads followed their own bead text |
| Templates / standards read | release-tooling-standards (corpus copy), cli-output-standards 0.1 |
| Plugin version | specs 0.25.0 |
| Session date | 2026-10-07 |

## Evidence

Harvested from the `retro:` comments on `bd list --label
fb-tool-configuration`, plus this run's DoD verification.

1. Implementing the Configuration Loader (release-0qc.3), the repository
   root comes from a flag no work order created. Root cause: the LOAD
   delta: "relative to the repository root, where the repository root is
   that of `-C/--repo` or the working directory". The contract lists
   `-C, --repo` and `--config` under Global flags, but no delta in either
   MVP blueprint adds them. The loader bead added both.
2. Implementing `release config` (release-0qc.4), the answer must include
   `target`, whose default is "the repository's default branch". No text
   says what the command does WHEN `origin/HEAD` is unset and no target is
   configured. The CFGM delta says "a command needing it SHALL refuse with
   exit 2"; the implementer read `release config` as needing it and exits
   2, so `release config` cannot report a configuration in a clone with no
   `origin/HEAD`.
3. AC-CFG-002.3 ("When the maintainer runs a command that would write to
   the repository / Then the tooling refuses before making any write") is
   assigned "LOAD (proves) | integration", but no command writes until
   `release cut` (epic release-elu). It was proved with `release version`
   and `release config` at the process boundary plus an in-process probe
   command; the first real writer is not covered.
4. The `specs` CLI was not available in the session, so the folds were
   not checked by `specs check --strict`.

## Proposals

### P1 — Say whether `release config` needs the default branch

**Problem.** The CMDCFG delta: "The command SHALL answer with one mapping
holding every setting in the order of the CFGM table". No text exists for
the case where `target`'s default cannot be resolved (finding 2).

**Change.** Add to the CMDCFG delta: "IF no `target` is configured and
`origin/HEAD` is unset, THEN `release config` SHALL exit 2 naming `git
remote set-head origin --auto`, like every command that needs the default
branch." (This is what the implementation does, and what
`release-cli-container.md` now records.) The alternative, answering
`target: null`, would let `release config` validate a file in any clone;
whichever the review picks, the blueprint should say it.

**Evidence.** Finding 2.

### P2 — Re-prove AC-CFG-002.3 with the first writing command

**Problem.** The AC Coverage row "AC-CFG-002.3 | CFGM, **LOAD (proves)** |
integration" names a scenario ("a command that would write") that no
command in this feature performs (finding 3).

**Change.** In the release staging blueprint's AC Coverage, add
"AC-CFG-002.3 | **CUT (proves)** | integration" and a test on `release cut`
with a refused configuration asserting no branch, commit or push. Keep the
existing e2e test as the participating proof.

**Evidence.** Finding 3. A refusal path added inside `release cut` after
its own first write would pass every current test.

## Considered and Not Proposed

- Global flags unowned (finding 1) — the loader bead added them and the
  container blueprint now records them, so no later feature meets the
  gap; recurrence is not plausible.
- `specs check --strict` not run (finding 4) — environment, not
  documents.

## Non-Goals

- No change to the implemented behaviour: P1 records what the code does,
  and P2 adds a test to a feature not yet built.

## Adoption

The proposals are independent. P1 touches the tool configuration
blueprint (now retired; amend in place or record in the container
blueprint, which already says it). P2 touches the release staging
blueprint and its groomed beads. Both are backward compatible: existing
documents stay valid.

## Resolution
