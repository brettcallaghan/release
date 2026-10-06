---
title: "ADR-003: Deterministic Bump Computation with Authored Prose"
description: The version bump is computed from commit metadata; a language
  model writes changelog prose and proposes a bump, and disagreement halts
  the release for human confirmation.
slug: adr-003
type: adr
status: draft
updated_at: 2026-09-25T00:00:00Z
---

# ADR-003: Deterministic Bump Computation with Authored Prose

**Status:** draft
**Date:** 2026-09-25

## Context

Every release produces two artifacts that look similar and have
different requirements. The version number is correctness-critical:
dependency resolvers, installers and tap formulae consume it, a wrong
one cannot be withdrawn cleanly, and it must be reproducible from the
repository alone. The release notes are read by humans, and notes
derived from commit subjects are markedly worse than notes written
from the diff.

A language model writes the second well and cannot be trusted with the
first, because its output varies between runs and its reasoning is not
reproducible from the repository.

## Decision

The version bump is computed deterministically from commit metadata —
conventional-commit types and breaking-change markers, or equivalent
pull-request labels. A language model authors the changelog prose and
separately proposes a bump. WHEN the proposal disagrees with the
computed bump, the tooling halts and requires human confirmation. The
model is never the sole authority on a version number.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| The model decides the bump | No commit-message discipline needed<br>Reads intent from the diff, not just the subject line | The version is not reproducible from the repository<br>A sampling difference silently changes a published version | A version number must be derivable and checkable without re-running a model |
| Commit metadata only, no authored prose | Fully deterministic<br>No model dependency | Release notes are concatenated commit subjects, which is what maintainers hand-rewrite today | Loses the main reason to generate notes at all |
| A maintainer chooses the bump each time | Full human judgment | Unreproducible, undocumented, and easy to get wrong under time pressure | The failure it invites is exactly the one this decision exists to prevent |
| Deterministic bump, authored prose, halt on disagreement | Version is reproducible and checkable<br>Notes are diff-aware<br>The model acts as a second opinion on mis-typed commits | Requires conventional commits or labels<br>Two changelog artifacts to keep coherent | N/A — selected |

## Consequences

**Positive:**
- The version is reproducible from git alone and can be recomputed by a
  reviewer.
- Model unavailability degrades the quality of release notes without
  blocking or corrupting a release.
- The disagreement check turns the model into a detector for
  mis-labelled or mis-typed commits, which is where the deterministic
  classifier is weakest.

**Negative:**
- A machine-readable impact signal becomes mandatory in the commit
  history: a release-type trailer, a conventional-commit type, or a
  breaking-change marker. Every accepted signal lives in git, so the
  computation is reproducible from a clone — which rules out forge labels
  as the primary signal, since they live outside it.
- A commit carrying the wrong signal still produces a deterministic but
  wrong version whenever the model's proposal happens to agree with it.
- Two changelog artifacts exist — a terse changelog file and the prose
  release body — and they must not contradict each other.

## References

- [[adr-004]] — the engine that implements the deterministic half
- [[release-tooling-standards]] — version computation requirements
