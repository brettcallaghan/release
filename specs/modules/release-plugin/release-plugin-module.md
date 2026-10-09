---
title: "Module Blueprint: Release Plugin"
entity_name: Release Plugin
technology: Claude Code plugin (skills, Bash)
description: The agent plugin published from this repository — the skills
  an agent uses to work with the release CLI, distributed through the
  repository's marketplace at the CLI's own version.
slug: module-release-plugin
type: module-blueprint
status: approved
updated_at: 2026-10-09T00:00:00Z
kind: standard
visibility: public
contracts_consumed:
  - contract-changelog
modules: []
source_paths:
  - plugin
test_paths:
  - internal/pluginscripts
version: "0.1"
---

# Release Plugin

## Overview

A Claude Code plugin, published from `plugin/` through the marketplace in
`.claude-plugin/marketplace.json`, that an agent harness loads into a
session. It carries no release logic: every rule lives in
[[module-release-core]] and reaches an agent through the
[[container-release-cli]] binary. The plugin holds the skills that tell an
agent how to drive that binary, and the producer skill the binary invokes
to author a changelog.

- **Responsibility:** skills, and the scripts and references a skill
  bundles.
- **Out of scope:** release logic, building or publishing the CLI, and
  anything a skill would need outside its own directory.

It is a module rather than a container because it is composed into an
agent harness's session and runs nothing of its own; it is public because
adopting repositories install it from the marketplace and update on their
own schedule.

## Tech Stack

- **Packaging:** a Claude Code plugin — `plugin/.claude-plugin/plugin.json`
  and `plugin/skills/<name>/SKILL.md`.
- **Scripts:** POSIX tools under Bash; no Python, `jq` or Go toolchain is
  required on the machine a skill runs on.

## Interface

Each skill is invoked as `release:<skill>`:

- `release:changelog` — authors a structured changelog from an evidence
  bundle, under [[contract-changelog]]. Invoked by the CLI's `claude`
  producer, never by a person.
- `release:cut-release` — cuts a release with the pinned CLI: provisions it,
  previews the version and changelog, confirms the bump with the person,
  and stages the release. Invoked by a person, or by an agent on their
  behalf.

## Components

### Changelog Skill

```component
entity_name: Changelog Skill
slug: component-changelog-skill
description: Authors a release's structured changelog from its evidence bundle
technology: Agent skill
```

`plugin/skills/changelog/`. Its references are byte-identical copies of
[[contract-changelog]] and the schemas it names, checked by
`scripts/check-skills.sh`.

### Cut Release Skill

```component
entity_name: Cut Release Skill
slug: component-cut-release-skill
description: Teaches an agent to cut a release with the pinned CLI, following the release standards
technology: Agent skill
```

`plugin/skills/cut-release/`, user-invocable. Its first step runs the
Install CLI Script and keeps the printed path; a failure there is reported
and stops the skill, which never builds, downloads or substitutes the CLI
another way. Every CLI command it directs runs through that path. It
teaches the workflow — the CLI's help, a `changelog` preview, the person's
confirmation of the bump, then `cut` — and states that the agent never
tags, publishes or merges. It defers to the CLI's `--help` for commands and
flags, and carries sections on commit conventions and on configuring a
repository. The plugin has no `hooks/` and no top-level `bin/`, so nothing
is provisioned in a session that runs no skill.

Its references are byte-identical copies of [[repository-release-policy]],
[[release-tooling-standards]] and [[cli-distribution-policy]], under the
same file names, checked by `scripts/check-skills.sh`; `SKILL.md` says when
to read each.

### Install CLI Script

```component
entity_name: Install CLI Script
slug: component-install-cli-script
description: Finds or downloads the CLI release the plugin pins and prints its path
technology: Bash
```

`plugin/skills/cut-release/scripts/install-cli`, run as
`bash <path> <data-dir>` with the plugin data directory; it lives with the
skill that runs it, so that skill stays self-contained. It is the only
provisioning code ([[adr-008]]): a skill runs it first and runs every CLI
step by the path it prints.

- **Pinned version:** the `version` of `.claude-plugin/plugin.json` three
  directories above the script, and nowhere else.
- **Lookup**, first hit wins and fetches nothing: `RELEASE_CLI` (relative
  paths resolved against the git toplevel; not executable is a failure,
  never a fall-through), then `<data-dir>/bin/<version>/release`, then a
  `release` on the PATH whose `--version` prints `release v<version>`.
- **Download:** the archive `release_<version>_<os>_<arch>` and
  `SHA256SUMS` of release `v<version>`, by asset id through the GitHub
  REST API, verified with `sha256sum` (or `shasum -a 256`) before `tar`
  extracts it under `<data-dir>/bin/.tmp-<pid>`; only the verified binary
  moves into `<data-dir>/bin/<version>/`. POSIX tools only.
- **Credential:** `GH_TOKEN`, `GITHUB_TOKEN`, `gh auth token`, or none —
  sent as a header read from standard input, to the API host only;
  redirects are followed without it.
- **Placement:** nothing outside `<data-dir>`, except that with
  `CLAUDE_CODE_REMOTE=true` the provisioned binary is linked as `release`
  into `/usr/local/bin`.
- **Result:** the binary's absolute path as the only line of standard
  output; on failure one line `install-cli: <cause>. <remedy>` on standard
  error and exit 1; exit 2 on a missing data directory.

`INSTALL_CLI_API_URL` and `INSTALL_CLI_LINK_DIR` exist for the
integration tests in `internal/pluginscripts`, which run the script
against an `httptest` server.

## Versioning and Compatibility

The plugin's version is the CLI's version: `release cut` writes it into
`plugin/.claude-plugin/plugin.json` through `scripts/propagate-version.sh`
([[adr-008]]), which also pins the `release` entry of
`.claude-plugin/marketplace.json` to the release's `v<version>` tag, so the
marketplace serves the plugin as released. A skill change ships in the next
CLI release.

## Cross-Cutting Standards

- This module SHALL comply with [[cli-distribution-policy]] for anything
  it downloads or runs.

## Change Guidance

- A skill SHALL be self-contained: every file it cites lives under its own
  directory, because a harness copies the skill on its own.
- A reference copied from this corpus SHALL be byte-identical to its
  source, and the pair SHALL be listed in `scripts/check-skills.sh`; the
  copy and its source change in the same commit.
- `scripts/check-skills.sh --strict` and `claude plugin validate plugin
  --strict` SHALL pass after every change.

## Key Decisions

- [[adr-008]] — the plugin provisions the CLI release it pins.

## References

- [[frd-plugin-distribution]] — what the plugin delivers to an adopting
  repository
