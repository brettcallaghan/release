# Changelog

## 0.1.0

**Release staging and publishing arrive in the release CLI**

This release adds the first end-to-end release workflow to the release CLI. `release cut` stages a release as a branch and pull request, and merging that stage pull request now publishes a GitHub release.

The CLI also defaults to YAML output. The release specification corpus, the five MVP feature blueprints and the project scaffolding (tests, coverage, CI and issue tracking) underpin this work.

### 🚀 Features

- **cli:** `release cut` stages a release as a branch and a pull request ([#10](https://github.com/brettcallaghan/release/pull/10))
- **cli:** Merging a stage pull request publishes a GitHub release ([#11](https://github.com/brettcallaghan/release/pull/11))
- **cli:** Command output defaults to YAML, with `--json` available for JSON ([#4](https://github.com/brettcallaghan/release/pull/4))
