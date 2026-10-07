---
title: "RFC: Pin What the Changelog Authoring Specs Left to the Implementer"
description: Close the gaps the changelog authoring implementers met in the
  corpus's schema_version form, the `release changelog` exit and output
  rules, where the command's logic lives, and the block and adapter rules
slug: rfc-0004-changelog-authoring-implementation-retro
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

# RFC: Pin What the Changelog Authoring Specs Left to the Implementer

## Summary

The changelog authoring feature (epic release-mng) was implemented in one
session, bead by bead, without swarm dispatch. Every MVP acceptance
criterion is cited, but the implementer had to decide several things the
documents answered wrongly or not at all: the form of `schema_version`,
what `release changelog` prints and exits on a disagreement or a failed
exchange, where its composition lives, and the edges of the release-notes
block rule. The proposals pin each in the documents.

## Provenance

| Field | Value |
|-------|-------|
| Origin project | brettcallaghan/release |
| Phase executed | Implemented the changelog authoring plan (beads release-mng.1–.3, .5–.8; .4 deferred) |
| Documents exercised | `specs/features/changelog-authoring/changelog-authoring-blueprint.md`, `changelog-authoring-frd.md`, `specs/contracts/changelog-contract.md`, `specs/contracts/release-cli-contract.md`, `specs/standards/cli-output-standards.md`, `changelog-corpus-builder-component.md`, `changelog-authoring-bridge-component.md`, `changelog-producer-component.md`, ADR-005 |
| Documents produced | `internal/schemacheck`, `internal/corpus`, `internal/authoring`, `internal/authoring/producers`, `internal/changelog`, `internal/cli/changelog.go`, the amended schema and contract, folds into the module, three component blueprints and the container blueprint |
| Skill(s) followed | retro-spec 1.5.0 (this retro); the implementing beads followed their own bead text |
| Templates / standards read | the AC traceability standard, cli-output-standards (corpus copy) |
| Plugin version | specs 0.25.0 |
| Session date | 2026-10-07 |

## Evidence

Harvested from the `retro:` comments on `bd list --label
fb-changelog-authoring`, plus this run's DoD verification.

1. Implementing the bridge (release-mng.5), the assembled corpus first
   declared `schema_version` `1.0`, as the contract's frontmatter
   (`version: "1.0"`) and the SCH delta ("`schema_version` SHALL stay
   1.0") read. The renderer parses `schema_version` as a full semantic
   version and refuses `1.0`, so the corpus had to declare `1.0.0`.
2. Implementing `release changelog` (release-mng.7), the CLCMD delta says
   "the command SHALL print the computed and proposed impacts and exit 1",
   without saying on which stream or in what shape. The command answers
   `computed`, `proposed` and `corpus_path` on standard output and
   explains on standard error, which is a second answer shape.
3. Same item: the release CLI contract names exit codes by kind but not
   for a producer that fails, a corpus the bridge rejects, a missing
   producer prerequisite, or `--skip-notes` combined with `--redo-notes`
   or `--corpus`. The implementation chose 1, 1, 2 and 2.
4. Same item: the bead's design anchors the command in `internal/cli`
   ("**Anchors:** `internal/cli`; `testdata/help/`, `testdata/cases/`"),
   but ADR-005 and the container blueprint keep release logic out of it.
   Resolve, assemble, exchange and render had to be composed in a new core
   package, `internal/changelog`, which no bead named.
5. Same item: the renderer takes an optional repository web URL, and no
   document says where a command gets one. The operation derives it from
   a GitHub `origin` remote.
6. Implementing the builder (release-mng.3), the CB delta says a block
   runs "to the next `## ` heading or the end of the body" and does not
   say whether a block ending the body includes the commit's trailer
   paragraph, which a squash body usually ends with. The builder drops a
   final paragraph made only of trailers and breaking-change footers. The
   delta also writes `## Release notes`, the component blueprint
   `## Release Notes`; the builder matches either case.
7. Implementing the schema (release-mng.1), the SCH delta spells the
   adapter values `commits`, `pull_requests`, `diffs`, while every enum in
   `schemas/changelog.strata` is upper case. The schema uses `COMMITS`,
   `PULL_REQUESTS`, `DIFFS`.
8. Implementing the producers (release-mng.6), the producer component
   lists one package per instance (`internal/authoring/producers/claude`,
   `.../command`) while the bead names one package,
   `internal/authoring/producers`. One package was used and the Instances
   list updated.
9. Implementing the bridge, the schema's enum still refuses an unknown
   `ChangeKind`, so the bridge rejects a corpus carrying a kind this
   release does not know, although the contract calls a new kind
   compatible. This is rfc-0003 P1, now reachable through
   `release changelog --corpus`.
10. AC-CLOG-002.4 asks that both the commit and pull request adapters are
    recorded; with the pull request adapter deferred (release-mng.4) only
    the commit half can be proved.
11. No document or bead named a JSON Schema validator, and the repository
    had none; `santhosh-tekuri/jsonschema/v6` was added.
12. AC-CLOG-003.7 ("does not issue any request to a language model
    service") has no natural unit-level proof; the test asserts the
    bridge's import graph holds no network package.
13. The real `claude` was on `PATH` in the session; a snapshot case left
    on the default producer would have run it.
14. The stub `claude` the PROD item requires had to run on all three CI
    platforms without `jq` or Python; the tests re-run the test binary.
15. Adding the authoring settings moved five `release config` snapshot
    cases and two CLI tests, which the ACFG item did not mention; the
    item also refuses an unknown producer name, which it did not list.
16. The contract approval acceptance criterion on release-mng.1 cannot be
    met by the implementer and rides the pull request review.
17. The `specs` CLI was not available, so the folds were not checked by
    `specs check --strict`.

## Proposals

### P1 — Spell `schema_version` as a full version everywhere

**Problem.** `changelog-authoring-blueprint.md`, SCH delta:
"`schema_version` SHALL stay 1.0, amended in place". The contract
frontmatter is `version: "1.0"` and its policy is "semver on
`schema_version`", while the renderer refuses anything but a full
`MAJOR.MINOR.PATCH` (finding 1).

**Change.** Add to the contract's Versioning section: "`schema_version`
SHALL be a full semantic version, `MAJOR.MINOR.PATCH`; this schema is
`1.0.0`." Change the SCH delta to "SHALL stay 1.0.0".

**Evidence.** Finding 1. The next producer written from the contract (a
person, or the `release:changelog` skill) would otherwise emit `1.0` and
be refused.

### P2 — State `release changelog`'s refusals in the CLI contract

**Problem.** `release-cli-contract.md`, `release changelog`: the section
lists flags and "Read-only unless `--write` is given", and nothing on the
disagreement answer or on exit codes for a failed exchange (findings 2
and 3).

**Change.** Add to that section: "WHEN the proposal disagrees with the
computed impact and no bump is named, the command SHALL answer
`computed`, `proposed` and `corpus_path`, explain on standard error,
write nothing, and exit 1. WHEN the producer fails or its corpus is
rejected, it SHALL exit 1. WHEN a producer prerequisite is missing, or
`--skip-notes` is combined with `--redo-notes` or `--corpus`, it SHALL
exit 2." This is what `internal/cli/changelog.go` does and the container
blueprint now records.

**Evidence.** Findings 2, 3. A script branching on the exit code needs
these fixed before `release cut` composes the same steps.

### P3 — Anchor a command's composition in the core, in the blueprint

**Problem.** The CLCMD delta's anchors are "`internal/cli`;
`testdata/help/`, `testdata/cases/`", but its behaviour (resolve,
assemble, author, render) is release logic, which ADR-005 places in the
core (finding 4).

**Change.** Amend the CLCMD delta's anchors to "`internal/changelog`
(the core's changelog operation), `internal/cli`, `testdata/help/`,
`testdata/cases/`", and add to the release-staging blueprint's command
delta, before it is groomed, that `release cut` composes
`changelog.Prepare` rather than its parts.

**Evidence.** Finding 4. release-elu.1 (the Stage Orchestrator) needs the
same composition; naming it avoids a second copy in the CLI.

### P4 — Say where the repository web URL comes from

**Problem.** No text states the source of the renderer's "optional
repository web URL" (finding 5).

**Change.** Add to the module's changelog operation: "The repository web
URL SHALL be derived from the `origin` remote WHEN it is a GitHub remote
(`https://github.com/<o>/<r>` or `git@github.com:<o>/<r>`), and sources
SHALL render as plain text otherwise." Or, if a setting is preferred,
add `changelog.repository_url` to the configuration model.

**Evidence.** Finding 5. Without a rule the next command could pass none
and publish a changelog without links.

### P5 — Ratify the block, heading and adapter spellings

**Problem.** CB delta: "running to the next `## ` heading or the end of
the body"; the component blueprint writes "`## Release Notes`"; the SCH
delta writes "`commits`, `pull_requests` or `diffs`" (findings 6, 7, 8).

**Change.** Amend the CB delta to the rule
`changelog-corpus-builder-component.md` now records ("a level-two
heading reading `Release notes` (any letter case) … WHEN the block runs
to the end of the body, a final paragraph made only of the commit's
trailers and breaking-change footers is left out"), the SCH delta to
`COMMITS`, `PULL_REQUESTS`, `DIFFS`, and the producer component's
instance paths to the single package (already folded).

**Evidence.** Findings 6–8.

## Considered and Not Proposed

- The unknown `ChangeKind` refusal (finding 9) — already rfc-0003 P1;
  this run adds only that `release changelog --corpus` now reaches it.
- AC-CLOG-002.4's pull request half (finding 10) — deferred with
  release-mng.4, not a document defect.
- The JSON Schema validator (finding 11) — an implementation choice the
  contract need not make.
- AC-CLOG-003.7's proof form (finding 12) — a negative criterion; the
  import-graph test is adequate.
- The real `claude` on `PATH` and the portable stub (findings 13, 14) —
  test practice, recorded in the tests themselves.
- ACFG's snapshot churn and unknown-producer refusal (finding 15) — the
  module Interface now records the refusal; churn is expected.
- Contract approval in review (finding 16) and the missing `specs` CLI
  (finding 17) — process and environment, not documents.

## Non-Goals

- No change to implemented behaviour: P1–P5 record what the code does,
  except P4's optional setting alternative.

## Adoption

The proposals are independent. P1 touches the changelog contract and the
SCH delta; P2 the release CLI contract; P3 the CLCMD delta and the
release staging blueprint; P4 the module blueprint (and, optionally, the
configuration model); P5 the authoring blueprint. All keep existing
documents and corpora valid; P1 states the form every corpus already
uses. Expected effect: `release cut` is groomed against documents that
already say what `release changelog` does.

## Resolution
