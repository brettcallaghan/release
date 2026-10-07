---
title: "ADR-007: Go for the Release CLI"
description: The release CLI and its core module are written in Go, built as
  one static binary that cross-compiles from a single runner.
slug: adr-007
type: adr
status: approved
updated_at: 2026-10-07T00:00:00Z
---

# ADR-007: Go for the Release CLI

**Status:** approved
**Date:** 2026-10-07

## Context

[[container-release-cli]] and the [[module-release-core]] it calls are one
program, and the implementation language is chosen once for both
([[adr-005]]). Three things constrain the choice:

- **One static binary per platform.** [[cli-distribution-policy]] has the
  project publish a declared set of artifacts from a single tag. The binary
  therefore has to build for every declared target from one runner and run
  with no interpreter, runtime or package manager on the machine. The
  busybox smoke test in `.github/workflows/cli.yml` runs it in an image
  holding nothing else.
- **Git through the git binary.** [[adr-006]] has the core run the installed
  `git` as a subprocess. The language needs argv-vector subprocess execution
  with the child's exit status and both output streams read separately.
- **A tool that runs where releases are cut.** Maintainers use Linux and
  macOS; continuous-delivery jobs run in minimal containers; Windows is in
  the test matrix.

The same language question was decided for the swarm CLI in swarm's
[ADR-023](https://github.com/brettcallaghan/swarm/blob/main/specs/adrs/ADR-023-go-for-the-swarm-cli.md),
which chose Go over a shell language, Python and Rust on the same
distribution and subprocess constraints. This decision relies on that
reasoning rather than repeating it. ADR-023 is outside this workspace, so it
is cited by URL.

## Decision

We will write the release CLI and its core module in Go, as one module
(`github.com/brettcallaghan/release`) building one binary, `release`, from
`cmd/release`. The toolchain version is the `go` directive in `go.mod`.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| Bash | Zero install on Linux and macOS | Absent on Windows without a compatibility layer; a pipeline hides the exit status of a failing step and a reader that closes early inverts it under `pipefail`; version and changelog logic would be untestable as pure functions | The tool's whole judgement is a version computed from git; shell reintroduces the failure class it exists to prevent |
| Python | Rich standard library; readable | Needs an interpreter on every target machine, which the busybox smoke test and [[cli-distribution-policy]] rule out; interpreter startup on every invocation | Breaks the dependency-free binary |
| Rust | Static binary, fast start | A second toolchain for contributors and CI with no benefit over Go for a subprocess-driven tool | Cost with nothing to offset it |
| TypeScript on Node | Large ecosystem | Needs a runtime on the target; a single-file executable embeds one | Breaks the dependency-free binary |
| Go | Static binary; one-runner cross-compile to every target; argv-vector subprocesses with separate exit status and streams in the standard library; one toolchain | Binary size larger than a C equivalent; error handling is verbose | N/A — selected |

## Consequences

**Positive:**

- One `GOOS`/`GOARCH` cross-compile produces every declared artifact, and
  the binary runs in an image containing nothing else.
- Subprocess calls to `git` and the platform CLI cannot suffer shell quoting
  or pipe-status faults, because arguments are an argv vector and the exit
  status is the process's own.
- Components are packages with exported functions, so the pure components of
  [[adr-005]] are unit-testable without a harness.

**Negative:**

- Contributors need the Go toolchain at the `go.mod` version. Another
  version changes the statement counts that `scripts/coverage.sh` commits.
- Every dependency joins one module graph and one binary.

## References

- [[adr-005]] — a composable library with a thin CLI
- [[adr-006]] — git through the git binary
- [[cli-distribution-policy]] — declared artifacts from a single tag
- [[container-release-cli]], [[module-release-core]] — what this governs
- [ADR-023: Go for the Swarm CLI](https://github.com/brettcallaghan/swarm/blob/main/specs/adrs/ADR-023-go-for-the-swarm-cli.md)
  — the precedent, outside this workspace
