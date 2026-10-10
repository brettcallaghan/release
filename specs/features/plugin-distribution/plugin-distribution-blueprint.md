---
title: "Feature Blueprint: Plugin Distribution"
description: How the release plugin provisions the CLI it pins, teaches an
  agent to cut a release with the standards built in, and is released in
  lockstep with the CLI and pinned in the marketplace.
slug: feature-plugin-distribution
type: feature-blueprint
status: in-progress
updated_at: 2026-10-10T00:00:00Z
delivery: single-pr
frd: frd-plugin-distribution
plan: release-bi5
---

# Plugin Distribution

## Overview

**FRD:** [[frd-plugin-distribution]]

The [[module-release-plugin]] gains a `cut-release` skill and the
`install-cli` script it runs first ([[adr-008]]). The script finds or
downloads the CLI release the plugin's manifest names and prints its path;
the skill runs every CLI step by that path, and carries the release
standards as byte-identical references. This repository's release
configuration writes the version into the plugin manifest and pins the
marketplace entry to the release tag, and the publish workflow refuses a
release whose plugin version disagrees. Delta blocks in dependency order:
ICLI, SKILL, STD, PIN, GATE.

## Element Deltas

### `Install CLI Script` in [[module-release-plugin]] (added)

**Intent:** Provision the pinned CLI on demand, per [[adr-008]]: reuse
what is already installed, otherwise download, verify and cache, and
print the path the skill runs. The script is the only provisioning code;
the skill never reimplements any part of it.

**New components:**
- **Install CLI Script** (Bash) — Finds or downloads the CLI release the
  plugin pins and prints its path. Lives with the skill that runs it, so
  the skill stays self-contained.

  ```component
  entity_name: Install CLI Script
  slug: component-install-cli-script
  description: Finds or downloads the CLI release the plugin pins and prints its path
  technology: Bash
  ```

**Behavior:**
- The script SHALL live at `plugin/skills/cut-release/scripts/install-cli`,
  be executable, and be run as `bash <path> <data-dir>`, where
  `<data-dir>` is the plugin data directory. IF `<data-dir>` is missing or
  empty, THEN it SHALL exit 2 with a usage line.
- The pinned version SHALL be the `version` field of
  `.claude-plugin/plugin.json` under the plugin root, found relative to the
  script's own location (three directories up). The script SHALL NOT read
  it from anywhere else.
- The script SHALL look for an installed CLI in this order and stop at the
  first hit, fetching nothing:
  1. `RELEASE_CLI`, WHEN set (a repository sets it in the `env` of its
     `.claude/settings.json`): a path to an executable, relative paths
     resolved against `git rev-parse --show-toplevel` of the working
     directory. IF it is not an executable file, THEN the script SHALL
     fail naming `RELEASE_CLI`; it SHALL NOT fall through.
  2. `<data-dir>/bin/<version>/release` (`release.exe` on Windows), WHEN
     it exists and is executable.
- The script SHALL NOT run a `release` it finds on the PATH, whatever
  version it reports, and SHALL NOT change it.
- WHEN neither is found, the script SHALL select the archive
  `release_<version>_<os>_<arch>.tar.gz` (`.zip` for `windows`), mapping
  `uname -s` Linux→`linux`, Darwin→`darwin`, MINGW*/MSYS*/CYGWIN*→
  `windows`, and `uname -m` x86_64/amd64→`amd64`, aarch64/arm64→`arm64`.
  IF either maps to nothing, THEN it SHALL fail with "no release archive
  for <os>/<arch>".
- The script SHALL read the release `v<version>` from
  `GET <api>/repos/brettcallaghan/release/releases/tags/v<version>`, take
  the asset ids of the archive and `SHA256SUMS` from it, and download each
  with `GET <api>/repos/brettcallaghan/release/releases/assets/<id>` and
  `Accept: application/octet-stream`, following redirects. `<api>` SHALL
  default to `https://api.github.com` and be overridable only through
  `INSTALL_CLI_API_URL`, which exists for tests.
- The script SHALL parse the API response with POSIX tools; it SHALL NOT
  require `jq`, Python or Go.
- The credential SHALL be the first of `GH_TOKEN`, `GITHUB_TOKEN`, and the
  output of `gh auth token` WHEN `gh` is on the PATH and succeeds; with
  none, requests go without an `Authorization` header, which is how a
  session's own repository access is used.
- The credential SHALL be sent as an `Authorization` header to `<api>`
  only. Redirects SHALL be followed without forwarding it (curl `-L`
  without `--location-trusted`). The credential SHALL NOT appear in the
  script's output or in any file it writes, and SHALL NOT be passed on a
  command line (curl reads the header from standard input or a
  file descriptor).
- The archive SHALL be verified against its `SHA256SUMS` line with
  `sha256sum`, or `shasum -a 256` where that is absent, before it is
  extracted. IF it does not match, THEN the script SHALL delete it and fail
  with "checksum mismatch for <archive>".
- The script SHALL download and extract under `<data-dir>/bin/.tmp-<pid>`
  and move the binary into `<data-dir>/bin/<version>/` only after
  verification, removing the temporary directory on every exit. Extraction
  SHALL use `tar` for both archive kinds. Versions other than the pinned
  one SHALL be left in place.
- WHEN `CLAUDE_CODE_REMOTE` is `true`, the script SHALL also link the
  binary as `release` into `/usr/local/bin` (overridable through
  `INSTALL_CLI_LINK_DIR`, for tests), replacing any link there. Otherwise
  it SHALL create or change nothing outside `<data-dir>`.
- On success the script SHALL print the binary's absolute path as its only
  line on standard output and exit 0.
- On failure it SHALL print one line, `install-cli: <cause>. <remedy>`,
  on standard error and exit 1. The causes and remedies:

  | Cause | Remedy named |
  |---|---|
  | no release archive for the platform | use linux, darwin or windows on amd64 or arm64 |
  | the release cannot be reached (network error) | check that the session can reach `api.github.com` |
  | the release is not readable (401, 403 or 404) | set `GH_TOKEN` to a token that can read `brettcallaghan/release`, or run `gh auth login` |
  | checksum mismatch | retry; a repeated mismatch is a corrupted release to report |
  | `RELEASE_CLI` is not an executable | fix or unset `RELEASE_CLI` |

**Anchors:** `scripts/build-release.sh` (archive names and layout: one
binary at the archive root); `.github/workflows/release.yml` (the assets a
release carries); `internal/buildinfo` (what `release --version` prints).
Tests go in a new test-only Go package, `internal/pluginscripts`, that
runs the script against an `httptest` server and a fake `gh` on the PATH;
the tests SHALL skip on Windows and WHEN `bash` or `curl` is absent.

### `Cut Release Skill` in [[module-release-plugin]] (added)

**Intent:** The plugin's one skill for people: teach an agent with no
prior knowledge to cut a release with the CLI, and the judgement calls
around it. The CLI's own `--help` is the reference for commands and
flags; the skill does not restate them.

**New components:**
- **Cut Release Skill** (Agent skill) — Teaches an agent to cut a release
  with the pinned CLI, following the release standards.

  ```component
  entity_name: Cut Release Skill
  slug: component-cut-release-skill
  description: Teaches an agent to cut a release with the pinned CLI, following the release standards
  technology: Agent skill
  ```

**Behavior:**
- The skill SHALL live at `plugin/skills/cut-release/SKILL.md`, named
  `cut-release`, user-invocable, with a description that triggers on
  cutting, staging or preparing a release and on setting up a repository's
  release process.
- Its first step SHALL run
  `bash "${CLAUDE_PLUGIN_ROOT}/skills/cut-release/scripts/install-cli" "${CLAUDE_PLUGIN_DATA}"`
  and keep the printed path. IF the script exits non-zero, THEN the skill
  SHALL tell the person the script's message and stop; it SHALL NOT build,
  download or substitute the CLI another way.
- Every CLI command the skill directs SHALL be run through the kept path,
  never as a bare `release`.
- The workflow it teaches SHALL be: read `<cli> --help` and the help of the
  command about to run; preview with `<cli> changelog`; show the person the
  suggested bump and its reasons and get their confirmation or their named
  bump; then `<cli> cut` with the confirmed bump, which opens the stage
  pull request.
- The skill SHALL state that the agent never creates a tag, publishes a
  release or merges the stage pull request; publication is the
  repository's workflow.
- The skill SHALL carry sections on commit conventions (the
  `Release-Type:` trailer and conventional types as bump signals) and on
  configuring a repository (`config/release.toml` or `release.toml`, the
  configuration keys from `<cli> config --help`), pointing at its
  references for the rules.
- The skill body SHALL stay under `scripts/check-skills.sh`'s byte limit.
- The plugin SHALL have no `hooks/` directory and no top-level `bin/`.

**Anchors:** `plugin/skills/changelog/SKILL.md` (frontmatter shape);
`testdata/help/` (the help text the skill defers to);
`scripts/check-skills.sh` (the invariants a skill must pass).

### `Cut Release Skill` in [[module-release-plugin]] (modified — standards references)

**Intent:** Build the release standards into the skill as byte-identical
copies, so its advice cannot drift from the corpus.

**Behavior:**
- `plugin/skills/cut-release/references/` SHALL hold byte-identical copies
  of `specs/standards/repository-release-policy.md`,
  `specs/standards/release-tooling-standards.md` and
  `specs/standards/cli-distribution-policy.md`, under the same file names.
- Each pair SHALL be added to the vendored-copy list in
  `scripts/check-skills.sh`, so a source edited without its copy fails the
  check naming the file.
- `SKILL.md` SHALL say when to read each reference: repository release
  policy when setting up or reviewing a repository, release tooling
  standards for bump and changelog rules, the distribution policy for
  publishing artifacts. The first version covers the command-line
  tool/library and continuously deployed service archetypes the policy
  already describes.

**Anchors:** `scripts/check-skills.sh` (the list holding
`plugin/skills/changelog/references/changelog-contract.md`).

### `scripts/propagate-version.sh` (modified — marketplace pin)

**Intent:** Pin the marketplace to each release's tag in the same release
commit that writes the plugin version.

**Behavior:**
- The script SHALL keep writing `RELEASE_VERSION` into
  `plugin/.claude-plugin/plugin.json`.
- The script SHALL also set the `release` entry's `source` in
  `.claude-plugin/marketplace.json` to
  `{"source": "git-subdir", "url": "brettcallaghan/release", "path": "plugin", "ref": "v<version>"}`,
  leaving the file's other content unchanged. IF the entry is missing,
  THEN the script SHALL fail.
- `config/release.toml`'s `release_paths` SHALL list
  `.claude-plugin/marketplace.json` alongside the plugin manifest.
- `claude plugin validate . --strict` SHALL pass on the written file.

**Anchors:** `scripts/propagate-version.sh`, `config/release.toml`,
`.claude-plugin/marketplace.json`. Tests go in `internal/pluginscripts`,
running the script in a temporary copy of the two files.

### `.github/workflows/release.yml` (modified — plugin version gate)

**Intent:** Refuse to publish a release whose plugin does not carry its
version.

**Behavior:**
- The "Verify the release commit" step SHALL fail, naming both versions,
  WHEN the `version` in `plugin/.claude-plugin/plugin.json` at the merge
  commit differs from the stage branch's version. It SHALL fail before the
  tag step, so nothing is tagged or published.

**Anchors:** `.github/workflows/release.yml`, the `Verify the release
commit` step.

### Relied-upon elements (unchanged)

- [[container-release-cli]] — `release --version`, `changelog` and `cut`,
  whose help the skill defers to.
- [[contract-release-cli]] — the normative help output.
- `scripts/build-release.sh` — archive names and layout.
- `.github/workflows/release.yml` — publishes the six archives and
  `SHA256SUMS` that provisioning downloads.
- [[module-release-plugin#Changelog Skill]] — unchanged; it drives no CLI,
  so it carries no `install-cli`.

## Contracts

None. The skill invocation names are the plugin's interface, recorded in
[[module-release-plugin]].

## Tracker Reconciliation

- release-jkm — subsumed by the whole plan: its spec stage is complete and
  the plan's epic carries the implementation.
- release-zl0 — out-of-scope: per-bead checks omitting
  `scripts/check-skills.sh`. Beads touching the skill or its references
  run `scripts/check-skills.sh --strict` themselves.
- release-8bt.19, release-8bt.20, release-8bt.21, release-8bt.22,
  release-1uh, release-dgt, release-ems — out-of-scope: defects in
  `scripts/check-skills.sh`'s parser and comments. The STD delta adds
  entries to its vendored-copy list only and does not change the parser.
- release-v0w — out-of-scope: the plugin's unused `specs` dependency. This
  feature does not change `dependencies`.
- release-1xa — out-of-scope: the changelog skill's contract copy.
- release-ovh — out-of-scope: tag signing. The GATE delta adds a check
  before the tag step and does not change tagging.

## Sequencing Constraints

- The ICLI delta SHALL land before the SKILL delta, which runs it.
- The SKILL delta SHALL land before the STD delta, which adds to its
  directory.
- The PIN and GATE deltas are independent of the others and of each other.

## AC Coverage

Legend: ICLI = `Install CLI Script` delta; SKILL = `Cut Release Skill`
delta; STD = `Cut Release Skill` delta (— standards references);
PIN = `scripts/propagate-version.sh` delta (— marketplace pin);
GATE = `.github/workflows/release.yml` delta (— plugin version gate).

| AC | Satisfied by | Verified by |
|----|--------------|-------------|
| AC-PLG-001.1 | ICLI | integration |
| AC-PLG-001.2 | ICLI | integration |
| AC-PLG-001.3 | ICLI | integration |
| AC-PLG-001.4 | SKILL | manual |
| AC-PLG-001.5 | ICLI | integration |
| AC-PLG-001.6 | ICLI | integration |
| AC-PLG-001.7 | SKILL | unit |
| AC-PLG-002.1 | ICLI | integration |
| AC-PLG-002.2 | ICLI | integration |
| AC-PLG-002.3 | ICLI | integration |
| AC-PLG-002.4 | ICLI | integration |
| AC-PLG-002.5 | ICLI | integration |
| AC-PLG-003.1 | ICLI | integration |
| AC-PLG-003.2 | ICLI | integration |
| AC-PLG-003.3 | ICLI | integration |
| AC-PLG-003.4 | ICLI | integration |
| AC-PLG-004.1 | PIN | integration |
| AC-PLG-004.2 | GATE | manual |
| AC-PLG-004.3 | PIN | integration |
| AC-PLG-005.1 | ICLI, **SKILL (proves)** | manual |
| AC-PLG-005.2 | ICLI, **SKILL (proves)** | manual |
| AC-PLG-005.3 | SKILL | manual |
| AC-PLG-006.1 | STD | integration |
| AC-PLG-006.2 | STD | integration |
| AC-PLG-007.1 | SKILL | manual |
| AC-PLG-007.2 | **SKILL (proves)**, PIN | manual |
| AC-PLG-007.3 | SKILL | manual |

AC-PLG-001.7 is proven by a unit test asserting the plugin has no
`hooks/` directory and no hook in its manifest. AC-PLG-006.1 and 006.2 are
proven by `scripts/check-skills.sh --strict` passing on the copies and
failing, naming the file, on a test edit of one source. The manual rows
run once the plugin is published, from `agent-plugins` as the first
adopter.

## Key Decisions

- [[adr-008]] — the plugin provisions its pinned release on demand, into
  the plugin data directory.

## Definition of Done

- [ ] Every delta above is folded into its owning blueprint:
  - [ ] [[module-release-plugin]] — Components gains the Install CLI
    Script and Cut Release Skill; Interface gains `release:cut-release`
- [ ] [[adr-008]] is referenced from [[module-release-plugin]]'s Key
  Decisions (already true)
- [ ] Every AC in the coverage table is verified at its declared level
  and cited from its tests per the AC traceability standards
- [ ] Every behaviour this blueprint decided that no FRD criterion asks
  for is lifted into the FRD as a criterion and cited from its test
- [ ] release-jkm is closed
- [ ] No live document references this blueprint (`specs refs
  feature-plugin-distribution` names no live document)
- [ ] This blueprint's status → retired
- [ ] With this blueprint already retired, `specs check --strict` over
  the corpus is green

## Open Questions

None.

## References

- [[frd-plugin-distribution]] — the requirements this realizes
