# Changelog

## 0.3.0

**Commits-first changelogs, a confirmed bump, and the cut-release plugin skill**

The changelog is now authored from the evidence in git: every commit's message, pull request text, changed files and diff. The release CLI suggests the bump from what the entries say, and a person confirms it before `release cut` stages anything. The release plugin also gains a `cut-release` skill that provisions the CLI version the plugin pins.

Scripts that run `release cut` without naming a bump now stop at a confirmation. Scripts that name a bump below what the commits require are refused.

### 💥 Breaking changes

- **cut:** `release cut` asks a person to confirm the suggested bump, or to name a different one, before it writes anything. The confirmed bump is recorded on the release commit as a `Release-Type` trailer, and the stage pull request lists the suggestion, its reasons and the commits that were omitted. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))

  On a terminal it prompts. `--yes` accepts the suggestion without a prompt. Without a terminal or `--yes`, it halts and reports what it would suggest. It also refuses if the target branch moved while the prompt was open.

  Migration: Name the bump explicitly, as in `release cut minor`, or pass `--yes` to accept the suggestion in non-interactive runs. The disagreement halt between a producer's proposed bump and the computed one is gone, and `proposed_bump` is ignored.

- **version:** `release version` reports the commits that carry no bump signal. It refuses a named bump that is below the floor the commits' types and trailers require. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))

  Migration: Name a bump at or above the floor that `release version` reports.

### 🚀 Features

- **changelog:** The changelog is authored from commit evidence: each commit's message, the pull request text git carries, its changed files and its diff, within a configurable size budget. Every commit is either cited by an entry or listed as omitted with a reason. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))

  Entries with no kind render under the other heading, and omitted commits are never rendered. A producer that fails stops the command and does not fall back to another producer.

- **changelog:** `release changelog` suggests the next bump from the kinds and breaking flags of the authored entries, raised to the floor that commit types and trailers set. It reports the suggestion, the floor, the reasons for each, and any commits that carried no signal. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))

  `--write` now requires a bump to be named. `--force` overrides a refusal, such as a named bump below the floor.

- **changelog:** A model-free mechanical producer builds a structured changelog from conventional commit subjects. `--skip-notes` uses it, so skipping the authoring step still gives a cited, rendered changelog. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))
- **config:** The configuration gains a diff budget and diff exclusions, an option to turn diff evidence off, and a `mechanical` producer. Diff evidence is on by default, and golden snapshot files are excluded by default. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))
- **plugin:** The release plugin adds a `cut-release` skill that walks an agent through previewing, confirming and staging a release. It first runs a bundled `install-cli` script that provisions the CLI version the plugin pins, verifies its checksum and caches it, and never runs a `release` found on the PATH. ([#15](https://github.com/brettcallaghan/release/pull/15), [09ce17c](https://github.com/brettcallaghan/release/commit/09ce17c378002f2a29d519a41ad6ee6842a15a36))

  The skill bundles the release standards as references. The plugin runs nothing at session start; provisioning happens only when the skill is used.

### 🪲 Bugs

- **changelog:** The evidence a producer receives no longer varies with the machine's git setup. Diffs ignore user `diff.*` settings, merge commits list their files against the first parent, and author dates are written as RFC 3339 under every git version. ([#18](https://github.com/brettcallaghan/release/pull/18), [4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb))

## 0.2.0

**Safer cuts and a 0.0.0 first release**

This release tightens `release cut` after the first dogfood release, 0.1.0. A named bump over a range with nothing releasable is now refused unless `--force` is given. The cut also checks GitHub authentication up front, shows the rendered section in `--dry-run`, and releases `0.0.0` first in a repository with no tags.

### 🪲 Bugs

- **cut:** A named bump over a range with nothing releasable is refused unless you pass --force ([#16](https://github.com/brettcallaghan/release/pull/16))

  Previously a named bump could cut a release from commits that carry no releasable change. The cut now stops, and `--force` overrides the refusal.

- **cut:** Cut checks GitHub authentication first, --dry-run shows the changelog section, and a repository with no tags releases 0.0.0 first ([#14](https://github.com/brettcallaghan/release/pull/14))

  The release target defaults to the main branch, and the generated release notes are trimmed.

## 0.1.0

**Release staging and publishing arrive in the release CLI**

This release adds the first end-to-end release workflow to the release CLI. `release cut` stages a release as a branch and pull request, and merging that stage pull request now publishes a GitHub release.

The CLI also defaults to YAML output. The release specification corpus, the five MVP feature blueprints and the project scaffolding (tests, coverage, CI and issue tracking) underpin this work.

### 🚀 Features

- **cli:** `release cut` stages a release as a branch and a pull request ([#10](https://github.com/brettcallaghan/release/pull/10))
- **cli:** Merging a stage pull request publishes a GitHub release ([#11](https://github.com/brettcallaghan/release/pull/11))
- **cli:** Command output defaults to YAML, with `--json` available for JSON ([#4](https://github.com/brettcallaghan/release/pull/4))
