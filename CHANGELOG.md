# Changelog

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
