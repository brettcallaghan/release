---
title: "ADR-003: AI-Authored Changelog with a Suggested, Confirmed Bump"
description: A language model authors a structured changelog from commit
  evidence in git; the bump is suggested from the changelog's entry kinds,
  floored by explicit git signals, and confirmed by a person; the confirmed
  bump is committed as a Release-Type trailer so the version is recomputable
  from git.
slug: adr-003
type: adr
status: approved
---

# ADR-003: AI-Authored Changelog with a Suggested, Confirmed Bump

**Status:** approved
**Date:** 2026-10-09

## Context

Every release produces two artifacts that look similar and have different
requirements. The version number is correctness-critical: dependency
resolvers, installers and tap formulae consume it, a wrong one cannot be
withdrawn cleanly, and it must be reproducible from the repository alone. The
changelog is read by humans, and a changelog derived from commit subjects is
markedly worse than one written from the diff.

A language model writes the changelog well when it is shown what changed:
commit bodies, the pull request title and description that a squash or merge
commit carries, the changed files and the diffs. Shown only commit subjects,
it can do no better than the subjects. Its output varies between runs and its
reasoning is not reproducible from the repository, so it cannot be the
authority on a version on its own.

The adopting repositories do not use conventional commits consistently and
squash-merge their branches ([[adr-004]]). A version computed only from
per-commit signals would be `none` for most of their history, and requiring a
signal on every commit would reimpose the discipline [[adr-004]] chose this
tool to avoid.

## Decision

At release time (`release cut`), the tool collects evidence for every commit
since the base release tag from git alone: the commit record (subject, body,
trailers, author, date, any conventional type, scope and breaking marker), the
pull request number, title and description where the commit carries them, the
changed files, and the diff within a per-release budget.

A language model authors a **structured changelog** from that evidence. Its
entries are changes, not commits: each has a kind, a scope, a summary of one
or two sentences, migration text when it is breaking, and the commits it is
derived from. Every commit in the range is either cited by an entry or listed
as omitted with a reason, so nothing is dropped silently.

The tool derives a **suggested bump** from the changelog's entry kinds: any
breaking entry suggests major, else any feature minor, else any fix,
performance, refactoring or revert entry patch, else none. The suggestion is raised to
the **signal floor**, the bump the explicit records in git require on their
own: `Release-Type:` trailers, conventional-commit types and breaking-change
markers. The suggestion is shown with its reasons.

A person confirms the suggestion, or names a bump instead. A named bump below
the signal floor is refused unless forced; a named bump below the suggestion
proceeds with a warning, because a person decided. The confirmed bump is
committed as a `Release-Type:` trailer on the release commit, and the stage
pull request shows the changelog, the suggestion and its reasons for review
before anything is published.

The model's output never becomes a version without a person's confirmation
recorded in git. The version is therefore recomputable from a clone without
re-running a model. Without a model, a mechanical producer writes entries
from conventional subjects, the suggestion is the signal floor alone, and a
range whose floor is `none` halts for a named bump WHEN it holds a commit
with no conventional type, whose entry has no kind.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| A machine-readable signal in every commit is mandatory; the model only writes prose | Fully deterministic<br>No confirmation step | The adopting repositories do not use conventional commits consistently, so most ranges compute `none` and the tool refuses before the model runs | Contradicts [[adr-004]], which chose this tool because conventional commits are not used consistently |
| The model decides the bump alone | No commit discipline needed<br>Reads intent from the diff | The version is not reproducible from the repository<br>A sampling difference silently changes a published version | A version number must be derivable and checkable without re-running a model |
| Pull request labels carry the bump | Familiar from other tools<br>Set during review | Labels live on the hosting platform, not in git, so the version is not computable from a clone | A version signal outside git cannot be checked offline or by a reviewer who has only the repository |
| Commit metadata only, no authored changelog | Fully deterministic<br>No model dependency | The changelog is concatenated commit subjects, which is what maintainers hand-rewrite today | Loses the main reason to generate a changelog at all |
| A reviewed per-pull-request notes block is a required input | Prose is reviewed with the code it describes<br>Each pull request carries its own proposed bump | Makes the release core depend on a pull-request-time step, a required check and the repository's squash-merge settings<br>A release cannot run on history that predates the block | Rejected for now: the core works from commits alone. Kept as a deferred extension (below) |
| The model authors a structured changelog from commit evidence, the bump is suggested from its kinds and floored by git signals, a person confirms, and the confirmation is committed | Works with or without conventional commits<br>The changelog is diff-aware<br>Needs nothing at pull request time<br>The version stays reproducible from a clone | A confirmation step on every release without a named bump<br>The changelog is reviewed in the stage pull request, after the code | N/A — selected |

## Consequences

**Positive:**
- The version is reproducible from git alone: the release commit's
  `Release-Type:` trailer is all a reviewer needs to recompute it.
- Conventional commits are welcome and never required. They raise the signal
  floor, which the suggestion cannot go below.
- The changelog is authored from what changed, and every commit is accounted
  for, either in an entry or as an explicit omission.
- Model unavailability degrades the changelog without corrupting a version: a
  mechanical producer writes entries from conventional subjects, and a person
  names the bump when the signal floor is `none`.

**Negative:**
- A release cannot complete non-interactively until a person, or an agent
  acting for one, confirms the suggestion or names a bump.
- The changelog prose is reviewed in the stage pull request, by someone who
  may not have written or reviewed the changes it describes.
- A confirmed bump that is wrong still produces a deterministic but wrong
  version.
- Two changelog artifacts exist — a terse changelog file and the prose
  release body — and they must not contradict each other.

## Future Extension: Pull Request Release Notes (Deferred)

A later `release notes` command may author a reviewed `## Release notes` block
per pull request from its diff, and the squash merge would carry it into git.
The evidence builder would then attach a parsed block to its commit's
evidence, and the bump classifier could read its impact as a reviewed signal.
The core flow works without it, and nothing in the core requires it.

## References

- [[adr-004]] — why conventional commits cannot be required of the adopting
  repositories
- [[release-tooling-standards]] — the signal floor, the suggested bump, its
  confirmation and the release-commit trailer
- [[repository-release-policy]] — squash merges that carry the pull request
  description into git
