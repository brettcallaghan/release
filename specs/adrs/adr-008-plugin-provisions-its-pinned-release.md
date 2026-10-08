---
title: "ADR-008: The Plugin Provisions Its Pinned Release"
description: The `release` plugin's SessionStart hook downloads and verifies
  the CLI release whose version the plugin manifest carries, and places it
  in the plugin's data directory, rather than bundling, building or
  delegating to a package manager.
slug: adr-008
type: adr
status: draft
updated_at: 2026-10-08T00:00:00Z
---

# ADR-008: The Plugin Provisions Its Pinned Release

**Status:** draft
**Date:** 2026-10-08

## Context

[[frd-plugin-distribution]] makes installing the `release` plugin the
whole of adopting the CLI: the session gains the binary at the plugin's
own version. Something has to put a binary for the session's platform on
its PATH, and the choice is constrained by:

- **The repository is private.** Release assets need a credential that can
  read it. Measured from a Claude Code cloud session on 2026-10-08, with
  the repository attached and the session's `GH_TOKEN` rejected by
  `gh auth status`:

  | Request | Result |
  |---|---|
  | `curl https://github.com/brettcallaghan/release/releases/download/v0.1.0/SHA256SUMS` | 404 |
  | `curl https://api.github.com/repos/brettcallaghan/release/releases/tags/v0.1.0` | 200 |
  | `curl -H 'Accept: application/octet-stream' https://api.github.com/repos/brettcallaghan/release/releases/assets/<id>` | 200, and the archive's SHA-256 matched SHA256SUMS |
  | `gh release download v0.1.0 -R brettcallaghan/release` | 403: GraphQL is refused in cloud sessions |

  The REST asset endpoint is the one route that worked. Whether it works
  in a cloud session that does **not** have this repository attached is
  **unverified**; a session in another repository settles it.
- **Six targets.** [[cli-distribution-policy]] publication already builds
  linux, darwin and windows on amd64 and arm64, with a SHA256SUMS file,
  for every release (`.github/workflows/release.yml`).
- **The binary must match the plugin's skills.** A skill written for one
  version's commands misleads an agent running another.
- **Cloud and local machines differ.** A cloud container is disposable
  and root-owned; a local machine is the person's own, with their package
  manager and PATH. Homebrew is not yet a published channel.
- **Downloads are verified before running** (CDP-014).

## Decision

The plugin will declare a `SessionStart` hook that provisions the CLI
release whose version is the plugin manifest's `version`. It selects the
archive for the session's OS and architecture, fetches it and SHA256SUMS
through the GitHub REST release-asset endpoint, verifies the checksum,
and extracts the binary to `${CLAUDE_PLUGIN_DATA}/bin/<version>/`, where
a later session reuses it without the network. It puts that directory on
the session's PATH; in a cloud session it also links the binary into
`/usr/local/bin`, because subagents and hook subprocesses do not reliably
inherit the session's environment. It fails open: any failure is one
line of cause and remedy, and the session starts.

The plugin manifest's version is written by `release cut` through the
repository's `commands.propagate`, and the publication workflow refuses a
merge commit whose plugin version differs from the stage branch's.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| Bundle all six binaries in the plugin directory | No network or credential at session start<br>Pinning is automatic | About 10 MB of binaries committed per release, forever, in a repository whose plugin is fetched by cloning<br>Every session downloads every platform's binary to use one | Repository and plugin size grow with every release, to save a download the release workflow already publishes |
| Build from source with `go install …@v<version>` | No release assets involved<br>Exact version by construction | Needs a Go toolchain in the session<br>A private module needs `GOPRIVATE` and a git credential<br>Minutes per cold session | Most sessions that cut a release are not Go projects, and the toolchain is the larger dependency |
| A package manager (Homebrew) | The idiomatic local install<br>The person's terminal gets the CLI too | Not yet published<br>Absent from cloud containers<br>Installs the formula's current version, not the plugin's | Cannot pin, and does not exist in the environment where most agent sessions run |
| Require a manual install | Nothing to build | The agent discovers the gap mid-task; the version drifts from the skills | Defeats the FRD's premise that installing the plugin is adoption |
| Download and verify the pinned release asset | One archive per session, cached across sessions<br>Exactly the plugin's version<br>Reuses the six archives and SHA256SUMS already published | Needs a credential for a private repository<br>Needs the GitHub API reachable at first use | N/A — selected |

## Consequences

**Positive:**

- Installing the plugin is the whole of adoption, in a cloud session and
  locally, with no Go toolchain.
- The binary an agent runs is the one its skills were written for.
- Nothing outside the plugin data directory changes on a local machine.
- The release workflow stays the only producer of binaries; provisioning
  is a consumer of what CDP-001 already declares.

**Negative:**

- A private repository makes first use depend on a credential; a session
  with none reports the remedy and runs without the CLI.
- The plugin and the CLI cannot be released separately. A skill-only fix
  is a CLI patch release.
- Between releases, the skills on the default branch can describe
  commands the pinned binary lacks. The cut-release skill defers flags to
  the binary's own `--help` for that reason.
- The hook is a Bash script. Windows sessions depend on the agent
  harness running hooks through a Bash it provides (**unverified**).
- A repository that builds the CLI from its own working tree, as this one
  does, must opt out of provisioning so the plugin's copy does not shadow
  its development build.

## Open Questions

- **Local sessions.** Session-only copy of the pinned version, as
  decided above, or an install through Homebrew once a formula exists,
  accepting that the installed version can differ from the pinned one?
- **Marketplace pinning.** The marketplace serves the plugin from the
  default branch. Pointing its source at the release tag would make the
  installed skills match the pinned binary exactly, at the cost of a
  marketplace edit in every release commit.
- **Sessions without this repository attached.** Whether the cloud
  session's repository access reaches the release assets of a repository
  it was not given; if not, a `GH_TOKEN` in the cloud environment is the
  credential.
- **`CLAUDE_ENV_FILE` in plugin hooks.** Whether a plugin's
  `SessionStart` hook receives it, as a project hook does (unverified).

## References

- [[frd-plugin-distribution]] — the requirements this provisions for
- [[cli-distribution-policy]] — CDP-001 (declared artifacts), CDP-011 and
  CDP-014 (checksums, verified before running)
- [[adr-007]] — the static Go binary that makes a single-file download
  sufficient
- [[adr-005]] — publication stays with the repository's own workflow
