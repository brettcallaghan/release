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

## Versioning and Compatibility

The plugin's version is the CLI's version: `release cut` writes it into
`plugin/.claude-plugin/plugin.json` through `scripts/propagate-version.sh`
([[adr-008]]). A skill change ships in the next CLI release.

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
