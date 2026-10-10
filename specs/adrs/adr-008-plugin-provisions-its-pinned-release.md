---
title: "ADR-008: The Plugin Provisions Its Pinned Release"
description: When a `release` plugin skill needs the CLI, a script bundled
  with the skill downloads and verifies the release whose version the
  plugin manifest carries and caches it in the plugin's data directory,
  rather than bundling, building, delegating to a package manager, or
  provisioning at session start.
slug: adr-008
type: adr
status: approved
updated_at: 2026-10-10T00:00:00Z
---

# ADR-008: The Plugin Provisions Its Pinned Release

**Status:** approved
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

Every plugin skill that drives the CLI will ship an `install-cli` script
and run it as its first step. The script provisions the CLI release whose
version is the plugin manifest's `version`. It first looks for one already
installed, in order: the binary the repository's agent settings name, then
the copy in `${CLAUDE_PLUGIN_DATA}/bin/<version>/`. It never runs a
`release` it finds on the PATH. Only when neither is found does it select
the archive for the machine's OS and architecture, fetch it and SHA256SUMS
through the GitHub REST release-asset endpoint, verify the checksum, and
extract the binary to `${CLAUDE_PLUGIN_DATA}/bin/<version>/`, where a later
session reuses it without the network. The skill passes
`${CLAUDE_PLUGIN_DATA}` to the script as an argument: Claude Code
substitutes it in skill content but does not export it to the Bash tool.
The directory is `~/.claude/plugins/data/<plugin id>/`, one per plugin on
the machine whatever scope enabled it, so repositories pinning different
versions keep separate `bin/<version>/` copies side by side. It prints the binary's absolute
path, and the skill runs the CLI by that path, so a `release` earlier on
the PATH, such as a Homebrew install, is never run, whatever its version. In
a cloud session it also links the binary into `/usr/local/bin`. Nothing is
provisioned in a session that invokes no such skill.

A failure stops the skill with one line of cause and remedy; the rest of
the session is unaffected.

The plugin manifest's version is written by `release cut` through the
repository's `commands.propagate`, and `plugin/.claude-plugin/plugin.json`
is declared in `release_paths`, without which cut refuses the propagated
change. The publication workflow refuses a merge commit whose plugin
version differs from the stage branch's.

The marketplace pins the plugin to its release tag: the same propagation
writes the marketplace entry's source as this repository at `v<version>`,
so an installed plugin's skills are the ones published with the binary
they provision. `.claude-plugin/marketplace.json` is declared in
`release_paths` alongside the plugin manifest. The release commit names a
tag that exists once publication creates it; until then the marketplace
keeps serving the previous release.

Until the repository is public, a session reads the release assets with
the credential of an account that can read it, through the order in
[[frd-plugin-distribution]] FEAT-PLG-003. Once it is public, the same
REST requests succeed without one.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| Bundle all six binaries in the plugin directory | No network or credential at session start<br>Pinning is automatic | About 10 MB of binaries committed per release, forever, in a repository whose plugin is fetched by cloning<br>Every session downloads every platform's binary to use one | Repository and plugin size grow with every release, to save a download the release workflow already publishes |
| Build from source with `go install …@v<version>` | No release assets involved<br>Exact version by construction | Needs a Go toolchain in the session<br>A private module needs `GOPRIVATE` and a git credential<br>Minutes per cold session | Most sessions that cut a release are not Go projects, and the toolchain is the larger dependency |
| A package manager (Homebrew) | The idiomatic local install<br>The person's terminal gets the CLI too | Not yet published<br>Absent from cloud containers<br>Installs the formula's current version, not the plugin's | Cannot pin, and does not exist in the environment where most agent sessions run |
| A `SessionStart` hook | The CLI is on PATH before any skill runs<br>No step for a skill to forget | Every session pays a network check, including the many that never release<br>Plugin hooks run only in Claude Code; the skills also load in harnesses that ignore them<br>A hook's PATH change reaches subprocesses only through `CLAUDE_ENV_FILE`, unverified for plugin hooks | Cost on sessions that never use it, and a harness dependency the skills do not otherwise have |
| A `bin/release` launcher in the plugin, which Claude Code puts on the Bash tool's PATH | The agent types `release` and gets the plugin's CLI | Plugin `bin/` directories come after the person's own PATH, so a Homebrew `release` at another version wins<br>claude.ai and Cowork do not install a plugin with a top-level `bin/` | Does not pin where a person has the CLI installed, and blocks the account-synced route cloud sessions use |
| Install into a gitignored `bin/` in each repository | The person's terminal in that repository finds the same binary | Writes into the person's working tree<br>One download per repository per version<br>Every adopting repository must ignore the path | The plugin data directory is the documented home for caches that survive plugin updates, and a session needs nothing in the working tree |
| Reuse a matching `release` on the PATH | Saves one cached download per version where the person already has the CLI | Telling whether it matches means running an arbitrary program by that name, which needs a portable timeout, and macOS ships no `timeout` | Running an unknown program with a hand-rolled timeout costs more than one cached download per version saves |
| Require a manual install | Nothing to build | The agent discovers the gap mid-task; the version drifts from the skills | Defeats the FRD's premise that installing the plugin is adoption |
| Download and verify the pinned release asset when a skill needs it | One archive per machine and version, cached across sessions<br>Nothing happens in sessions that do not release<br>Works wherever the skill's script can run<br>Exactly the plugin's version<br>Reuses the six archives and SHA256SUMS already published | Needs a credential while the repository is private<br>Needs the GitHub API reachable at first use | N/A — selected |

## Consequences

**Positive:**

- Installing the plugin is the whole of adoption, in a cloud session and
  locally, with no Go toolchain.
- A session that never releases never touches the network for the CLI.
- The binary an agent runs is the one its skills were written for.
- Nothing outside the plugin data directory changes on a local machine.
  This holds once Homebrew is published too: a formula serves the
  person's terminal, and a session keeps its pinned copy even when the
  formula installed another version.
- The release workflow stays the only producer of binaries; provisioning
  is a consumer of what CDP-001 already declares.

**Negative:**

- While the repository is private, first use depends on a credential; a
  session with none reports the remedy and runs without the CLI.
- The plugin and the CLI cannot be released separately. A skill-only fix
  is a CLI patch release.
- Every release commit edits the marketplace file as well as the plugin
  manifest.
- The skills on the default branch can describe commands the last
  release lacks; the marketplace's tag pin keeps them from installed
  plugins until the next release.
- `install-cli` is a Bash script. Windows depends on the agent's shell
  being a Bash, as Claude Code's is on Windows (**unverified**).
- Every skill that drives the CLI carries its own copy of `install-cli`,
  because a skill is self-contained; a check keeps the copies identical.
- The first release skill in a session waits for a download.
- A repository that builds the CLI from its own working tree, as this one
  does, must point the skills at that build so the pinned copy does not
  shadow it.

## References

- [[frd-plugin-distribution]] — the requirements this provisions for
- [[cli-distribution-policy]] — CDP-001 (declared artifacts), CDP-011 and
  CDP-014 (checksums, verified before running)
- [[adr-007]] — the static Go binary that makes a single-file download
  sufficient
- [[adr-005]] — publication stays with the repository's own workflow
