---
title: "ADR-003: Reviewed Impact with Evidence-Authored Notes"
description: A language model authors release notes from commits, diffs and
  reviewed per-PR notes, and proposes impact with reasons; the version is
  computed from reviewed records in git, and a person confirms any impact
  that has none, which is committed as a Release-Type trailer.
slug: adr-003
type: adr
status: draft
updated_at: 2026-10-09T00:00:00Z
---

# ADR-003: Reviewed Impact with Evidence-Authored Notes

**Status:** draft
**Date:** 2026-10-09

## Context

Every release produces two artifacts that look similar and have different
requirements. The version number is correctness-critical: dependency
resolvers, installers and tap formulae consume it, a wrong one cannot be
withdrawn cleanly, and it must be reproducible from the repository alone. The
release notes are read by humans, and notes derived from commit subjects are
markedly worse than notes written from the diff.

A language model writes the notes well when it is shown what changed: commit
bodies, pull request descriptions and diffs. Shown only commit subjects, it
can do no better than the subjects. Its output varies between runs and its
reasoning is not reproducible from the repository, so it cannot be the
authority on a version on its own.

The adopting repositories do not use conventional commits consistently and
squash-merge their branches ([[adr-004]]). A version computed only from
per-commit signals would be `none` for most of their history, and requiring a
signal on every commit would reimpose the discipline [[adr-004]] chose this
tool to avoid.

## Decision

A language model authors release notes from evidence, at two points:

- **Pull request time** (`release notes`). The model reads the pull
  request's commits and diff and writes a `## Release notes` block into the
  pull request description: one entry per user-visible change, each with a
  kind, a sentence or two of prose, and a proposed impact for the pull
  request. The reviewer reviews the block with the code, and the squash merge
  carries it into the commit body.
- **Release time** (`release cut`). The model reads every commit since the
  base release tag, with its reviewed notes block, or its diff where it has
  none, and composes the release changelog. It proposes an impact for the
  release and gives its reasons.

The version is computed deterministically from reviewed records in git:
notes-block impacts reviewed in pull requests, `Release-Type:` trailers,
conventional-commit types and breaking-change markers. WHEN a commit in the
range carries none of these, or the model proposes a higher impact than the
computed one, a person confirms the impact before anything is pushed. A
maintainer may name the impact instead. The confirmed impact is committed as
a `Release-Type:` trailer on the release commit.

The model's output never becomes a version without a reviewed record in git:
either the pull request review of the notes block, or the confirmed trailer on
the release commit. The version is therefore recomputable from a clone without
re-running a model.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| A machine-readable signal in every commit is mandatory; the model only writes prose and is a second opinion on the computed bump | Fully deterministic<br>No confirmation step | The adopting repositories do not use conventional commits consistently, so most ranges compute `none` and the tool refuses before the model runs<br>Notes are built from commit subjects, which leaves the model nothing but subjects to work from | Contradicts [[adr-004]], which chose this tool because conventional commits are not used consistently and notes are expected to come from diffs |
| The model decides the impact alone | No commit or review discipline needed<br>Reads intent from the diff | The version is not reproducible from the repository<br>A sampling difference silently changes a published version | A version number must be derivable and checkable without re-running a model |
| Pull request labels carry the impact | Familiar from other tools<br>Set during review | Labels live on the hosting platform, not in git, so the version is not computable from a clone<br>Labels carry no prose, so the notes are still authored without review | A version signal outside git cannot be checked offline or by a reviewer who has only the repository |
| Commit metadata only, no authored notes | Fully deterministic<br>No model dependency | Release notes are concatenated commit subjects, which is what maintainers hand-rewrite today | Loses the main reason to generate notes at all |
| Reviewed records decide, the model authors from evidence and proposes, a person confirms the rest, and the confirmation is committed | Works with or without conventional commits<br>Notes are diff-aware and reviewed with the code<br>The version stays reproducible from a clone | A notes block to review on every pull request<br>A confirmation step when commits lack a reviewed record<br>Depends on squash merges carrying the pull request description | N/A — selected |

## Consequences

**Positive:**
- The version is reproducible from git alone: the reviewed records and the
  release commit's trailer are all a reviewer needs to recompute it.
- Conventional commits are welcome and never required. A repository without
  them releases by reviewing notes blocks, or by confirming a proposal.
- Release notes are authored from what changed, and the per-change prose is
  reviewed alongside the code it describes rather than after the fact.
- A model proposal above the computed impact halts for confirmation, so the
  model still detects mis-typed or mis-trailered commits.
- Model unavailability degrades the notes without corrupting a version: a
  maintainer names the impact, and a mechanical producer writes entries from
  notes blocks and conventional subjects.

**Negative:**
- Each pull request into a release line carries a notes block to author and
  review, and the repository's squash merge must use the pull request
  description as the commit body, or the block never reaches git
  ([[repository-release-policy]]).
- A release over commits without a reviewed record cannot complete
  non-interactively until a person, or an agent acting for one, names the
  impact.
- A reviewed record carrying the wrong impact still produces a deterministic
  but wrong version whenever the model's proposal is no higher.
- Two changelog artifacts exist — a terse changelog file and the prose
  release body — and they must not contradict each other.

## References

- [[adr-004]] — why conventional commits cannot be required of the adopting
  repositories
- [[release-tooling-standards]] — version computation and the release-commit
  trailer
- [[repository-release-policy]] — squash merges and the pull request notes
  block
