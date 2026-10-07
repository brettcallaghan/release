---
title: "RFC: Pin What the Version Computation Specs Left to the Implementer"
description: Close four gaps in the version computation blueprint and its
  component specs that implementers of release-cb2 had to decide alone
slug: rfc-0001-version-computation-implementation-retro
type: rfc
status: ready-for-review
updated_at: 2026-10-07T00:00:00Z
origin: brettcallaghan/release
skill: retro-spec
skill_version: "1.5.0"
model: claude-opus-5-5[1m]
effort: high
runs_observed: 2
severity: snag
plugin_version: "0.25.0"
---

# RFC: Pin What the Version Computation Specs Left to the Implementer

## Summary

The version computation feature (epic release-cb2) was implemented bead by
bead across two sessions. Every bead landed, but four times an implementer
had to choose between two readings of the specs, or found the specs
disagreeing with each other. The proposals pin those choices in the
documents, so the next consumer (the tool configuration and release staging
features, which reuse the same operation and default branch) reads the
answer instead of re-deciding it.

## Provenance

| Field | Value |
|-------|-------|
| Origin project | brettcallaghan/release |
| Phase executed | Implemented the version computation plan (beads release-cb2.1, .3–.10) |
| Documents exercised | `specs/features/version-computation/version-computation-blueprint.md`, `version-computation-frd.md`, `specs/modules/release-core/components/bump-classifier-component.md`, `specs/standards/release-tooling-standards.md`, `specs/contracts/release-cli-contract.md` |
| Documents produced | `internal/{semver,gitread,bump,version,resolve}`, `internal/cli/version.go`, folds into the component, module and container blueprints |
| Skill(s) followed | retro-spec 1.5.0 (this retro); the implementing beads followed their own bead text |
| Templates / standards read | release-tooling-standards (corpus copy), cli-output-standards 0.1 |
| Plugin version | specs 0.25.0 |
| Session date | 2026-10-07 |

## Evidence

Harvested from the `retro:` comments on `bd list --label
fb-version-computation`, plus this run's DoD verification.

1. Implementing the Bump Classifier (release-cb2.5), the component spec and
   the blueprint disagreed on where the impact comes from when a changelog
   is supplied. The implementer followed the blueprint. Root cause:
   `bump-classifier-component.md` Interface: "WHEN no bump is named and an
   authored changelog is supplied, the component SHALL resolve the impact
   from its entries." versus the blueprint's BC delta: "the highest impact
   indicated by any enabled signal across the authored changelog's entries
   and the commit records".
2. The same bead did not say whether changelog entries are subject to the
   enabled signal set; the implementer chose "always read when supplied".
   No text exists where the rule should live: the blueprint lists three
   commit signals for the default policy and is silent on entries.
3. Implementing `release version` (release-cb2.8), the default target was
   ambiguous. Root cause: blueprint CMD delta: "the command SHALL use the
   repository's default branch, the branch `origin/HEAD` points to". That
   reads as either the local branch of that name or the remote-tracking ref
   `refs/remotes/origin/<branch>`. The implementer chose the remote-tracking
   ref, so a CI checkout with no local branch resolves. The same sentence
   is in `specs/features/tool-configuration/tool-configuration-blueprint.md`
   ("The repository's default branch SHALL be the branch `origin/HEAD`
   points to"), so the next feature meets the same fork.
4. The Tag Reader (release-cb2.3) cites AC-VER-001.4 ("A line with no
   release yet resolves to 0.0.0") on a test that proves only that no tag
   is "not found"; the 0.0.0 substitution is the Version Calculator's.
   Root cause: the blueprint's AC Coverage table row "AC-VER-001.4 |
   **TR (proves)** | unit" assigns the proof to a component that cannot
   produce 0.0.0.
5. release-tooling-standards RTS-018 ("WHERE a project configures the
   pre-1.0 remapping off, the release tooling SHALL map a bump of major to
   a major increment below 1.0.0") contradicts RTS-019 and the groomed
   decision cb2.6 implemented. Already filed as release-bar.
6. ADR-006 requires a missing or too-old git to be reported as an invalid
   environment; no bead owned the version check. Already filed as
   release-2hy.
7. release-cb2.7 was recorded as blocked by release-cb2.9, a docs-only
   item it does not depend on, so cb2.7 could not close until cb2.9 did.
8. The snapshot harness had no way to build a fixture git repository
   (`input/` cannot carry a nested `.git`), though release-cb2.8's
   acceptance criteria required snapshot cases over fixture repositories.
   The harness gained a `git` key in `case.toml`.

## Proposals

### P1 — Reconcile the Bump Classifier's Interface with the blueprint

**Problem.** `bump-classifier-component.md` Interface says an authored
changelog resolves the impact "from its entries", which reads as entries
only; the blueprint says entries and commits together, highest wins
(finding 1). Nothing says whether entries obey the enabled signal set
(finding 2).

**Change.** Replace the guarantee with: "WHEN no bump is named, the
component SHALL return the highest impact indicated by the authored
changelog's entries, when one is supplied, and by any enabled signal in
the commit records." Add: "The enabled signal set SHALL govern commit
signals only; an authored changelog's entries are read whenever one is
supplied." (The implementation already behaves this way; the Signal
mappings bullet folded by cb2.5 records the mappings.)

**Evidence.** Findings 1 and 2. The changelog authoring feature
(release-mng) is the next consumer of the classifier and would meet the
same contradiction.

### P2 — Name the ref the default branch resolves to

**Problem.** "the branch `origin/HEAD` points to" in the version
computation blueprint's CMD delta and in the tool configuration
blueprint admits two readings (finding 3).

**Change.** In both blueprints, replace the sentence with: "the
repository's default branch SHALL be the remote-tracking ref
`origin/HEAD` points to (for example `refs/remotes/origin/main`), read
locally without fetching; a local branch of the same name is not
consulted." Mirror it in the release CLI contract's `--target` row:
"Defaults to the configured target, then the remote-tracking ref
`origin/HEAD` points to".

**Evidence.** Finding 3. The tool configuration feature implements the
same default and would otherwise re-decide it, possibly the other way.

### P3 — Move AC-VER-001.4's proof to the operation that produces 0.0.0

**Problem.** The AC Coverage row "AC-VER-001.4 | **TR (proves)** | unit"
names a component that can only report absence (finding 4).

**Change.** Change the row to "AC-VER-001.4 | TR, **VC (proves)** | unit"
and add a `// AC: AC-VER-001.4` citation to the Version Calculator test
that computes from a nil base. Keep the Tag Reader's citation as a
participating test.

**Evidence.** Finding 4. An auditor tracing AC-VER-001.4 today finds a
test that never mentions 0.0.0.

## Considered and Not Proposed

- RTS-018 contradicting RTS-019 (finding 5) — a standards content fix,
  already tracked as release-bar; an RFC would duplicate it.
- Git version check unowned (finding 6) — already tracked as release-2hy.
- cb2.7 blocked by cb2.9 (finding 7) — a dependency edge in the tracker,
  not a document defect; if it recurs across plans it is a grooming-skill
  finding for retro-skill.
- Snapshot harness lacked git fixtures (finding 8) — fixed in code by
  release-cb2.8; the testing standard does not enumerate harness keys, so
  there is no document to amend. Candidate for retro-skill if the specs
  plugin's snapshot harness template lacks it too.
- The `specs` CLI was not built in either session, so spec folds were not
  checked by `specs check --strict` — environment, not documents.

## Non-Goals

- No change to the implemented behaviour: every proposal records what the
  code already does.
- No change to the canary identifiers item (release-cb2.2), which stays
  deferred.

## Adoption

The three proposals are independent. P1 touches
`bump-classifier-component.md`; P2 touches both feature blueprints and the
release CLI contract; P3 touches the version computation blueprint and one
test. All are backward compatible: existing documents stay valid, and only
P3 adds a test citation. Together they leave no implementer-chosen reading
in the specs the next two features consume.

## Resolution
