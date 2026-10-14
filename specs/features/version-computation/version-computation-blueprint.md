---
title: "Feature Blueprint: Version Computation"
description: How the release core gains semver, tag, commit, impact and
  version components, and how `release version` exposes them.
slug: feature-version-computation
type: feature-blueprint
status: in-progress
delivery: single-pr
frd: frd-version-computation
plan: release-cb2
---

# Version Computation

## Overview

**FRD:** [[frd-version-computation]]

The feature lands as five new components in [[module-release-core]], each a
package under `internal/`, plus one module-level operation that composes
them and one command in [[container-release-cli]] that prints the result.
The pure components ([[component-semver-core]],
[[component-bump-classifier]], [[component-version-calculator]]) take every
policy value as an input; the two git readers ([[component-tag-reader]],
[[component-commit-collector]]) are the only parts that touch a repository
([[adr-005]]). Delta blocks in dependency order: SV, CAN, TR, CC, BC, VC, RES,
CMD. CAN is outside the MVP and its work order may be deferred.

## Element Deltas

### `Semver Core` in [[module-release-core]] (added)

**Intent:** The version type every other component speaks, and the single
place every release name is derived and parsed back, so a tag and its stage
branch agree by construction. Written to the existing
[[component-semver-core]] blueprint, which is already in steady-state
vocabulary; this delta implements it.

**Behavior:**
- The component SHALL parse a version of the form
  `MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]` as defined by Semantic
  Versioning 2.0.0, and SHALL refuse any other string, including one
  carrying a leading `v`.
- The component SHALL derive a tag name as the tag prefix followed by the
  formatted version, and SHALL parse a tag name back to its version given
  the same prefix.
- The component SHALL derive a stage branch name as the stage prefix
  followed by the formatted version, with no tag prefix, and SHALL parse a
  stage branch name back to its version given the same prefix.
- The component SHALL take the tag prefix and the stage prefix as inputs;
  it SHALL NOT hold a default for either.
- The component SHALL derive a prerelease version for a planned version
  and an ordinal `n` as `<planned>-rc.<n>`.
- The component SHALL order versions by Semantic Versioning precedence,
  including prerelease precedence, and SHALL ignore build metadata when
  ordering.

**Anchors:** `internal/semver` (new). No existing code.

### `Semver Core` in [[module-release-core]] (added — canary identifiers)

**Intent:** Canary versions for arbitrary commits. Split from SV because
canaries are not part of the MVP: the work order this block becomes may be
deferred without holding up anything else in the feature.

**Behavior:**
- The component SHALL derive a canary version for a base version, an
  optional pull request number and a commit hash as
  `<base>-canary.<pr>.<hash7>`, or `<base>-canary.<hash7>` WHEN no pull
  request is given.
- `<hash7>` SHALL be the first seven characters of the hash in lower case,
  prefixed with `g` WHEN those characters are all digits, so the
  identifier is never a numeric identifier with a leading zero, which
  Semantic Versioning forbids.

**Anchors:** `internal/semver` (the SV delta's package).

### `Tag Reader` in [[module-release-core]] (added)

**Intent:** The base version of a release line as a git query over tags
reachable from the target ref, implementing [[component-tag-reader]].

**Behavior:**
- The component SHALL consider a tag a release tag WHEN its name is the
  configured tag prefix followed by a version, or is a bare version, and
  that version carries no prerelease and no build metadata.
- The component SHALL ignore every other tag, including prerelease and
  canary tags, without failing.
- The component SHALL consider only tags reachable from the target ref.
- WHEN no release tag is reachable, the component SHALL report the absence
  as a distinct result rather than as an error or as a version.
- WHEN two reachable release tags carry the same version, the component
  SHALL return the one whose name carries the configured prefix as the
  tag for that version.
- The component SHALL return, with the highest version, the name of the
  tag it was read from, so the commit range can start there.

**Anchors:** `internal/gitread` (new). No existing code. Git is reached by
running the `git` binary ([[adr-006]]).

### `Commit Collector` in [[module-release-core]] (added)

**Intent:** The commit records between the base tag and the target ref,
implementing [[component-commit-collector]] against the `CommitRecord`
shape in `schemas/collection.strata`. Version computation is the first
feature to need it; changelog authoring reuses it unchanged.

**Behavior:**
- The component SHALL collect the commits reachable from the target ref
  and not reachable from the base tag, or every commit reachable from the
  target ref WHEN there is no base tag.
- The component SHALL emit records in the order `git log` lists them,
  newest first.
- The component SHALL extract a pull request number from a subject ending
  in ` (#<n>)` and SHALL leave the subject otherwise unchanged.
- The component SHALL parse trailers from the final paragraph of the body
  using git's trailer rules, and WHEN a key repeats, SHALL keep the last
  value.

**Anchors:** `internal/gitread` (new); `schemas/collection.strata`
(`CommitRecord`), whose generated Go type this component emits.

### `Bump Classifier` in [[module-release-core]] (added)

**Intent:** Release impact from a named bump, an authored changelog's
entries, or commit signals, with everything that drove it, implementing
[[component-bump-classifier]].

**Behavior:**
- WHEN a bump is named, the component SHALL return it and report it as
  named.
- WHEN no bump is named, the component SHALL return the highest impact
  indicated by any enabled signal across the authored changelog's entries
  and the commit records, and SHALL report every entry and record that
  indicated that highest impact.
- The component SHALL map a `Release-Type` trailer of `major`, `minor`,
  `patch` or `none` to that impact, case-insensitively.
- The component SHALL map a conventional-commit subject type of `feat` to
  minor; `fix`, `perf` and `refactor` to patch; and every other type to
  no signal.
- The component SHALL map a breaking-change marker to major: a `!`
  immediately before the subject's colon, or a line in the body's final
  paragraph beginning `BREAKING CHANGE:` or `BREAKING-CHANGE:`. The
  component reads the footer from the body, because git's trailer rules
  do not accept a key containing a space.
- A `Release-Type: none` trailer SHALL indicate an impact of none and
  SHALL NOT lower the impact any other signal indicates, on the same
  commit or another: the highest impact wins.
- The component SHALL map an authored changelog entry marked `breaking` to
  major and an entry of kind `FEATURE` to minor, and every other entry to
  patch.
- The component SHALL NOT read an authored changelog's `proposed_bump`.
- The component SHALL take the enabled signal set as an input.

**Anchors:** `internal/bump` (new); `schemas/changelog.strata`
(`ChangelogEntry`, `Bump`), whose generated Go types it reads.

### `Version Calculator` in [[module-release-core]] (added)

**Intent:** Every version-policy rule, applied in one pure function,
implementing [[component-version-calculator]].

**Behavior:**
- The component SHALL take as inputs an optional base version, an impact,
  a graduation flag, and a policy carrying `leaps` and `pre1_remap`.
- WHEN no base version is supplied, the impact is not none and graduation
  is not requested, the component SHALL return 0.0.0 (RTS-031), and SHALL
  accept only 0.0.0 as a proposed version.
- WHEN the impact is none and graduation is not requested, the component
  SHALL return a no-release result rather than a version.
- WHEN graduation is requested and the base is below 1.0.0, the component
  SHALL return 1.0.0 whatever the impact.
- IF graduation is requested and the base is 1.0.0 or above, THEN the
  component SHALL return a policy violation.
- WHILE the base is below 1.0.0 and `pre1_remap` is on, the component
  SHALL map an impact of major to a minor increment.
- The component SHALL check a proposed version against a base under the
  policy, and SHALL return a policy violation WHEN the proposal increments
  any component by more than one while `leaps` is off.
- From a base version, the computed next version SHALL always increment
  exactly one component by one; `leaps` affects only whether a proposed version is accepted.
- The component SHALL derive a prerelease of a planned version through
  [[component-semver-core]].

**Anchors:** `internal/version` (new). No existing code.

### [[module-release-core]] (modified — version resolution operation)

**Intent:** The module's Interface promises a "resolve, classify, compute"
surface callable without staging. This delta adds the one operation that
composes TR, CC, BC and VC for a target ref, so the CLI holds no release
logic ([[container-release-cli]] Change Guidance) and the stage
orchestrator can reuse it later.

**Behavior:**
- The module SHALL expose a version resolution operation taking a
  repository path, a target ref, an optional named bump, an optional
  graduation flag and a policy, and returning the base version (or its
  absence), the impact, what determined it, and the next version or a
  no-release result.
- The module SHALL supply the default policy: tag prefix `v`, stage
  prefix `cut/`, `leaps` off, `pre1_remap` on, and every signal enabled:
  the `Release-Type` trailer, the conventional-commit type, and the
  breaking-change marker.
- The operation SHALL NOT write to the repository.

**Anchors:** `internal/` (new package for the operation is the
implementer's choice); the module Interface section.

### [[container-release-cli]] (modified — `release version`)

**Intent:** The first command with release behavior: `release version
[bump]` per [[contract-release-cli]], wired through the Command Surface and
Output Renderer to the RES operation.

**Behavior:**
- The command SHALL accept `bump` as `major`, `minor` or `patch`, and
  `--target` and `--json`, exactly as [[contract-release-cli]] lists them.
- The command SHALL compute with the RES default policy. Reading
  configuration is the Configuration Loader's, which
  [[frd-tool-configuration]] delivers and wires in later.
- WHEN `--target` is absent, the command SHALL use the repository's
  default branch, the branch `origin/HEAD` points to; IF `origin/HEAD` is
  unset, THEN it SHALL exit 2 naming `git remote set-head origin --auto`.
- The command SHALL answer with one mapping whose keys, in this order,
  are `base` (string, or null when no tag is reachable), `impact`, `named`
  (bool), `determined_by` (a sequence of mappings with `hash`, `subject`
  and `signal`), and `next` (string, or null when no release is
  warranted), rendered as YAML by default and as JSON under `--json`, per
  [[cli-output-standards]].
- WHEN no release is warranted, or the policy refuses the version, the
  command SHALL exit 1 after printing the answer.
- The command SHALL NOT access the network.

**Anchors:** `internal/cli/root.go` (command tree), `internal/cli/exit.go`
(exit statuses), `testdata/help/` (a new `release-version.txt` golden,
regenerated by `scripts/regenerate-snapshots.sh`), `testdata/cases/`.

### Relied-upon elements (unchanged)

- [[contract-release-cli]] — the `release version` surface and exit codes
  this feature implements without changing.
- [[cli-output-standards]] — YAML by default, JSON under `--json`, and
  the answer-alone rule the CMD delta follows.
- [[release-tooling-standards]] — RTS-010 to RTS-024 and RTS-030 to
  RTS-032, the rules the components enforce.
- `schemas/collection.strata` and `schemas/changelog.strata` — the record
  and corpus shapes CC emits and BC reads.

## Contracts

None. [[contract-release-cli]] already specifies `release version`; this
feature implements it unchanged.

## Tracker Reconciliation

- release-8bt.2 — prerequisite: CC and BC use the Go types generated from
  `schemas/*.strata`, so the generator lands before them.
- release-8bt.7 — out-of-scope: raising the coverage floor follows this
  feature; this feature does not raise it.

## Sequencing Constraints

- The SV delta SHALL land before the CAN, TR, VC and CMD deltas.
- The TR, CC, BC and VC deltas SHALL land before the RES delta.
- The RES delta SHALL land before the CMD delta.
- No lockstep-update rule in the owning blueprints' Change Guidance spans
  more than one delta block: derivation and inverse parse are both in SV,
  and no delta changes a record shape or a version-policy rule.

## AC Coverage

Legend: SV = `Semver Core` delta; CAN = `Semver Core` delta (— canary
identifiers); TR = `Tag Reader` delta; CC =
`Commit Collector` delta; BC = `Bump Classifier` delta; VC =
`Version Calculator` delta; RES = [[module-release-core]] delta (— version
resolution operation); CMD = [[container-release-cli]] delta (—
`release version`).

| AC | Satisfied by | Verified by |
|----|--------------|-------------|
| AC-VER-001.1 | SV, **TR (proves)** | unit |
| AC-VER-001.2 | **TR (proves)** | unit |
| AC-VER-001.3 | SV, **TR (proves)** | unit |
| AC-VER-001.4 | **TR (proves)** | unit |
| AC-VER-001.5 | TR, **CMD (proves)** | e2e |
| AC-VER-001.6 | TR, **VC (proves)** | unit |
| AC-VER-001.7 | SV, **TR (proves)** | unit |
| AC-VER-001.8 | TR, **VC (proves)**, RES | unit, snapshot |
| AC-VER-002.1 | CC, **BC (proves)** | unit |
| AC-VER-002.2 | **BC (proves)** | unit |
| AC-VER-002.3 | **BC (proves)**, RES, CMD | unit |
| AC-VER-002.4 | **BC (proves)** | unit |
| AC-VER-002.5 | **BC (proves)** | unit |
| AC-VER-003.1 | **VC (proves)** | unit |
| AC-VER-003.2 | **VC (proves)** | unit |
| AC-VER-003.3 | **VC (proves)** | unit |
| AC-VER-003.4 | **VC (proves)** | unit |
| AC-VER-003.5 | **VC (proves)** | unit |
| AC-VER-003.6 | **VC (proves)** | unit |
| AC-VER-003.7 | **VC (proves)**, RES, CMD | unit |
| AC-VER-004.1 | **SV (proves)** | unit |
| AC-VER-004.2 | **SV (proves)** | unit |
| AC-VER-004.3 | **SV (proves)** | unit |
| AC-VER-004.4 | SV, **CAN (proves)** | unit |
| AC-VER-004.5 | **SV (proves)** | unit |
| AC-VER-004.6 | **SV (proves)**, VC | unit |
| AC-VER-004.7 | **SV (proves)**, VC | unit |
| AC-VER-004.8 | SV, **CAN (proves)** | unit |
| AC-VER-004.9 | **SV (proves)** | unit |
| AC-VER-004.10 | SV, **CAN (proves)** | unit |

## Key Decisions

- [[adr-001]] — branch-agnostic; why TR and VC read nothing from a branch
  name.
- [[adr-003]] — the bump is computed; why BC never reads `proposed_bump`.
- [[adr-005]] — composable core with git I/O isolated; why SV, BC and VC
  are pure and RES composes them.
- [[adr-006]] (new) — git is reached through the `git`
  binary; why TR and CC run `git` rather than a Go library.

## Definition of Done

- [ ] Every delta above is folded into its owning blueprint:
  - [ ] [[component-semver-core]] — Interface records the `rc` and
    canary forms and the leading-`v` refusal. The `rc` form and the
    leading-`v` refusal are recorded; the canary form waits on the
    deferred canary item
  - [x] [[component-tag-reader]] — Interface records the release-tag rule
  - [x] [[component-commit-collector]] — Interface records the range and
    trailer rules
  - [x] [[component-bump-classifier]] — Interface records the signal
    mappings
  - [x] [[component-version-calculator]] — Interface records the
    graduation and proposed-version rules
  - [x] [[module-release-core]] — Interface records the version resolution
    operation and the default policy
  - [x] [[container-release-cli]] — Command Surface records
    `release version`
- [x] [[adr-006]] status → approved, and referenced from
  [[module-release-core]] Key Decisions
- [ ] Every AC in the coverage table is verified at its declared level
  and cited from its tests per the
  specs plugin's `shared/ac-traceability-standards.md`. All but
  AC-VER-004.4, AC-VER-004.8 and AC-VER-004.10 are; those three are
  proved by the deferred canary item
- [ ] This blueprint's status → retired. Held at in-progress: the canary
  identifiers item (CAN) is deferred outside the MVP

## Open Questions

None.

## References

- `shared/c4-standards.md` — the module and component definitions these
  deltas are written against
- [[frd-version-computation]] — the requirements this realizes
- [[release-tooling-standards]] — the rules the components enforce
- [[contract-release-cli]] — the command this feature implements
- [[cli-output-standards]] — the output format `release version` answers in
