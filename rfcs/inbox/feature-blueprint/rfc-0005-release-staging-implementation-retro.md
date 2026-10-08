---
title: "RFC: Pin What the Release Staging Specs Left to the Implementer"
description: Close the gaps the release staging implementers met in how
  configured commands learn the version and where they run, how
  release_paths match, which steps are optional, and what the cut answer
  reports about the impact
slug: rfc-0005-release-staging-implementation-retro
type: rfc
status: ready-for-review
updated_at: 2026-10-08T00:00:00Z
origin: brettcallaghan/release
skill: retro-spec
skill_version: "1.5.0"
model: claude-opus-5-5[1m]
effort: high
runs_observed: 1
severity: snag
plugin_version: "0.25.0"
---

# RFC: Pin What the Release Staging Specs Left to the Implementer

## Summary

The release staging feature (epic release-elu) was implemented in one
session, bead by bead, without swarm dispatch. Every acceptance criterion
is cited, but the implementer had to decide how a propagation command
learns the version, where configured commands and producers run, how a
`release_paths` entry matches a path, whether preflight is optional, and
whether the cut answer reports what determined the impact. The proposals
pin each in the documents.

## Provenance

| Field | Value |
|-------|-------|
| Origin project | brettcallaghan/release |
| Phase executed | Implemented the release staging plan (beads release-elu.1, .2, .3) |
| Documents exercised | `specs/features/release-staging/release-staging-blueprint.md`, `release-staging-frd.md`, `specs/modules/release-core/components/stage-orchestrator-component.md`, `specs/contracts/release-cli-contract.md` (1.1), `specs/features/tool-configuration/tool-configuration-blueprint.md`, `specs/standards/release-tooling-standards.md` |
| Documents produced | `internal/cut`, `internal/cli/cut.go`, an `Env` field on `internal/delegate`, ten snapshot cases, e2e tests, folds into the component and container blueprints |
| Skill(s) followed | retro-spec 1.5.0 (this retro); the implementing beads followed their own bead text |
| Plugin | specs 0.25.0 |

## Evidence

Harvested from the `retro:` comments on `bd list --label
fb-release-staging`, plus this run's DoD verification, which found no
further friction.

1. Implementing the orchestrator (release-elu.1), step 6 says "run
   `commands.propagate`, which SHALL be configured", and the component
   blueprint says the component "SHALL run the repository's configured
   version-propagation command to write the version into its manifests".
   No document says how the command learns the version. The
   implementation sets `RELEASE_VERSION` in its environment, which needed
   an `Env` field on the Delegated Command Runner.
2. Same item: the tool configuration blueprint types `release_paths` as
   "list of paths", and step 7 says "refuse WHEN any changed path is
   neither `CHANGELOG.md` nor listed in `release_paths`". Whether an
   entry can name a directory or a glob is unstated. The implementation
   accepts an exact path, a directory prefix for an entry ending in `/`,
   and a `path.Match` glob.
3. Same item: step 3 reads "run `commands.preflight`" unconditionally,
   while step 8 reads "run `commands.validate` WHEN configured". The
   Delegated Command Runner reports an empty preflight list as
   `Unconfigured`, "never a success", so a literal reading refuses every
   repository without preflight commands. Preflight was made optional.
4. Same item: the blueprint pins the version, range and scratch checkout
   to `origin/<target>` and keeps the maintainer's tree out of the
   release, but does not say where configured commands and the producer
   run. The implementation runs both in the scratch checkout, so a
   `producer_command` reading an untracked file in the maintainer's tree
   (as the changelog snapshot cases do) no longer sees it (release-elu.2).
5. Implementing `release cut` (release-elu.2), the contract guarantees
   "The provider SHALL report, for every version it computes, what
   determined the impact", but the CUT delta lists the answer keys as
   "`version`, `branch`, `commit`, `pull_request` … and `writes`", with no
   `determined_by`. The implementation follows the delta, so `release
   cut` reports no driver.
6. Same item: `changelog.Prepare` resolves, assembles, authors and
   renders in one call, while the blueprint orders the stage-branch check
   and preflight between resolving and assembling. Keeping the order took
   a separate `resolve.Resolve` call before `Prepare`.
7. Same item: a terminal check on `os.ModeCharDevice` treats the null
   device as a terminal, so every e2e run prompted; `golang.org/x/term`
   was added.
8. Same item: the snapshot harness ran the command under the user's git
   environment, and cut checks for a committer identity even on a dry
   run. The harness now applies its fixture environment to the command.

## Proposals

### P1 — Say what a configured command is given, and where it runs

**Evidence:** findings 1, 4.

**Problem:** step 6 reads "run `commands.propagate`, which SHALL be
configured", and no text in the blueprint, the component blueprint or the
tool configuration blueprint says how the command learns the version or
in which directory it, the other configured commands, and the producer
run.

**Change:** add to the ORCH delta's Behavior, and fold into the component
blueprint's Interface:

> Configured commands and the changelog producer SHALL run in the scratch
> checkout, with `RELEASE_VERSION` set to the version being released,
> without a tag prefix. They SHALL NOT see the maintainer's working tree.

Add `RELEASE_VERSION` to the tool configuration blueprint's description of
`commands.propagate`.

**Backward compatibility:** states what the implementation does; no
configuration changes.

### P2 — Define how a `release_paths` entry matches

**Evidence:** finding 2.

**Problem:** the tool configuration blueprint's table row reads
"`release_paths` | list of paths | empty", and step 7 reads "listed in
`release_paths`"; no text says whether an entry can name a directory or a
pattern.

**Change:** replace the row's type with "list of repository-relative
paths or patterns", and add under it:

> An entry matches a changed path equal to it, any path under it WHEN it
> ends in `/`, or any path it matches as a glob (`*`, `?`, `[...]`, not
> crossing `/`).

**Backward compatibility:** every exact-path list keeps its meaning.

### P3 — Mark preflight optional, as validate is

**Evidence:** finding 3.

**Problem:** step 3 reads "run `commands.preflight`;" while step 8 reads
"run `commands.validate` WHEN configured;".

**Change:** reword step 3 to "run `commands.preflight` WHEN configured;".

**Backward compatibility:** matches the implementation and the defaults
(`preflight` is `[]`).

### P4 — Reconcile the cut answer with the contract's driver guarantee

**Evidence:** finding 5.

**Problem:** the contract reads "The provider SHALL report, for every
version it computes, what determined the impact"; the CUT delta lists the
answer keys without `determined_by`.

**Change:** add `determined_by` (each `hash`, `subject`, `signal`, as
`release version` answers) after `version` in the CUT answer, in both the
real and the `--dry-run` answer, and in the container blueprint's Command
Surface. Adding a property is a compatible change under the contract.

**Backward compatibility:** additive; existing consumers are unaffected.

## Considered and Not Proposed

- Finding 6 (re-resolving before `Prepare`): one extra read, and the
  implementation note in `internal/cut` explains it. Splitting the
  changelog operation would cost more than it saves.
- Finding 7 (terminal detection): an implementation detail no spec should
  carry; one-off.
- Finding 8 (harness environment): test infrastructure, fixed in the
  harness; no document misled.
- AC-STG-001.10's manual verification raised no friction; recorded on
  release-elu.1.

## Non-Goals

- Changing the step order, the definition of a write, or the partial
  write rules, which held up in implementation.

## Adoption

The proposals are independent. P1 touches the release staging blueprint,
the component blueprint and the tool configuration blueprint; P2 the tool
configuration blueprint; P3 the release staging blueprint; P4 the release
staging blueprint and the container blueprint, and needs a code change
(adding `determined_by` to the cut answer). P1 to P3 state what the code
already does.

## Resolution
