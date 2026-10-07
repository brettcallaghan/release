---
title: "ADR-006: Git Through the Git Binary"
description: The release core reads and writes repositories by running the
  installed `git` command, not through a Go reimplementation of git.
slug: adr-006
type: adr
status: approved
updated_at: 2026-10-07T00:00:00Z
---

# ADR-006: Git Through the Git Binary

**Status:** approved
**Date:** 2026-10-07

## Context

[[adr-005]] keeps git I/O in its own components so that everything else in
[[module-release-core]] is a pure function. Those components still have to
reach git somehow, and the choice is made once for all of them: the
[[component-tag-reader]] and [[component-commit-collector]] read, and the
[[component-stage-orchestrator]] writes a branch and pushes it.

Four things constrain the choice:

- **The answers must match git's.** The base version is the highest tag
  *reachable* from the target ([[release-tooling-standards]] RTS-010), and
  impact comes from trailers parsed out of commit bodies. A maintainer
  checks the tool's judgement by running `git log` and `git tag --merged`
  themselves; any disagreement between the tool and git is a wrong version.
- **The repository is the maintainer's own checkout,** with whatever their
  git configuration implies: worktrees, partial or shallow clones,
  `safe.directory`, credential helpers, signing, hooks.
- **Staging pushes with the maintainer's credentials.** The contract
  requires platform commands to use the platform CLI's own credentials, and
  a push must use whatever git already uses.
- **The binary is static and dependency-free** (the busybox smoke test in
  `.github/workflows/cli.yml`), and it already documents `git` as a
  run-time requirement.

## Decision

We will reach git by running the installed `git` command as a subprocess
with a fixed, non-interactive environment, and parse its plumbing output.
No Go library reimplements git inside the binary.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| go-git (pure Go git library) | No `git` needed on the machine<br>Typed objects instead of parsed text<br>Fixture repositories built in memory | A reimplementation, so its answers can differ from git's on edge cases (trailer parsing, shallow and partial clones, newer object formats)<br>Ignores much of the user's git configuration: credential helpers, signing, hooks, `safe.directory`<br>Large dependency tree | Fidelity: a version computed differently from what `git` reports is the failure this tooling exists to prevent, and pushes need the user's credential setup |
| libgit2 via cgo | Mature C library, closer to git than go-git | Requires cgo, breaking the static single binary<br>Same configuration gaps as go-git | Breaks the static, dependency-free binary the CLI ships as |
| Run the `git` binary | Answers are git's own by construction<br>Honours the user's configuration, credentials and hooks<br>No new dependency; the binary stays static | `git` must be installed, at a version new enough for the plumbing used<br>Output must be parsed, and a subprocess per query is slower | N/A — selected |

## Consequences

**Positive:**

- The base version and commit range are exactly what `git tag --merged`
  and `git log` show, so a maintainer can check every answer by hand.
- Trailers are parsed by git's own rules (`git interpret-trailers`), so a
  `Release-Type` trailer means the same to the tool as to git.
- Pushing a stage branch uses the maintainer's existing credentials and
  signing setup with no extra configuration.

**Negative:**

- `git` is a run-time dependency. The tool SHALL check for it and report a
  missing or too-old `git` as an invalid environment rather than failing
  mid-command.
- Every git call is a subprocess whose output is parsed; the readers must
  request machine-stable formats (`--format`, `-z`) and never parse
  porcelain meant for people.
- Component tests need real fixture repositories built with `git`, which
  makes them slower than in-memory fixtures would be.

## References

- [[adr-005]] — git I/O isolated in its own components
- [[release-tooling-standards]] — RTS-010 (reachable tags) and RTS-012
  (signals live in git)
- [[contract-release-cli]] — platform commands use the platform CLI's
  credentials
- [[component-tag-reader]], [[component-commit-collector]],
  [[component-stage-orchestrator]] — the components this governs
