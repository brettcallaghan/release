# Changelog

## 0.3.0

**Commits-first changelogs with a confirmed bump**

Release notes are now authored from the evidence in git: every commit's message, files and diff. The bump is suggested from the entries and confirmed by a person, then recorded on the release commit.

### 💥 Breaking changes

- **cut:** Without a named bump, `release cut` stops in non-interactive use and asks for confirmation instead of proceeding on the computed impact. `release changelog --write` likewise requires a named bump. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))

  Migration: Name the bump (major, minor or patch) in scripts and CI, or pass `--yes` to `release cut` to accept the suggestion.

- **changelog:** Changelog producers now receive an evidence bundle and return a structured changelog, and a producer failure stops the command with no fallback. The authored changelog is cached under `.git/release/changelog/`, and the authored `proposed_bump` is ignored. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))

  Migration: Update any `producer_command` to read the evidence bundle on standard input and write the structured changelog to standard output, and rerun with `--redo-notes` to replace corpus caches kept under `.git/release/corpus/`.

### 🚀 Features

- **changelog:** The changelog is authored from each commit's message, pull request text, changed files and diff, with one entry per change. Every commit is either cited by an entry or omitted with a one-line reason. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))
- **changelog:** `release changelog` suggests a bump from the entries' kinds, raised to the signal floor, and gives the reasons. A named bump below the suggestion proceeds with a warning, and one below the floor is refused unless `--force` is given. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))
- **cut:** `release cut` asks you to confirm the suggested bump at a terminal, or accepts it with `--yes`. The confirmed bump is recorded as a `Release-Type` trailer on the release commit and shown in the stage pull request. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))
- **version:** `release version` reports commits that carry no release signal and refuses a named bump below the signal floor. A `revert` commit now counts as a patch change. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))
- **config:** Commit diffs are collected by default within a configurable release budget, with configurable path exclusions. A budget report records what was cut or left out. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))
- **changelog:** A built-in mechanical producer composes a changelog from conventional commit subjects without a model. `--skip-notes` uses it and no longer needs a named bump. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))

### 🪲 Bugs

- **changelog:** The changed-file list of a merge commit is taken against its first parent. Commit author dates are written in RFC 3339 under every git version. ([4cf0697](https://github.com/brettcallaghan/release/commit/4cf06975ab5b56668cf051dd89fa7f26bdf804bb), [#18](https://github.com/brettcallaghan/release/pull/18))

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
