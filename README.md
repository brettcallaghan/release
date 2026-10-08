# release

<!-- coverage:badge:start -->
[![coverage](https://img.shields.io/badge/coverage-94.0%25-brightgreen)](#test-coverage)
<!-- coverage:badge:end -->

A command-line tool that computes the next version from git history,
assembles and renders its changelog, and stages the release on a stage branch
for review. It is paired with a Claude Code plugin that authors the changelog
prose.

The specification corpus under [`specs/`](specs/) is the source of truth: the
features' FRDs, the [container](specs/containers/release-cli/release-cli-container.md)
and [module](specs/modules/release-core/release-core-module.md) blueprints, and
the [command surface contract](specs/contracts/release-cli-contract.md).

## Install

```bash
go install github.com/brettcallaghan/release/cmd/release@latest
```

## Development

```bash
go test ./...                    # unit, snapshot and end-to-end tests
scripts/ci.sh                    # everything a push must pass
scripts/regenerate-snapshots.sh  # rewrite testdata/ from the implementation
scripts/coverage.sh              # measure coverage and update this README
```

Work is tracked with [beads](https://github.com/gastownhall/beads) on the
`beads/data` branch. After cloning, run `./scripts/beads-setup.sh` once; see
`AGENTS.md`.

## Test coverage

Three layers exercise the CLI, measured separately and merged:

- **Unit** — each package's own tests.
- **Snapshot** — the data-driven cases under `testdata/cases/` and the golden
  help under `testdata/help/`, replayed in-process against the command tree.
- **End-to-end** — the `release` binary, built with `go build -cover` and run
  as a process.

A statement is covered if any layer reaches it.

<!-- coverage:table:start -->
| Test layer | Statements covered | Coverage |
|:---|---:|---:|
| Unit tests | 1,486 / 1,714 | 86.7% |
| Snapshot cases | 1,133 / 1,714 | 66.1% |
| End-to-end (built binary) | 1,013 / 1,714 | 59.1% |
| **All layers combined** | **1,611 / 1,714** | **94.0%** |

| Package | Statements | Covered | Coverage |
|:---|---:|---:|---:|
| `release/cmd/release` | 1 | 1 | 100.0% |
| `release/internal/authoring` | 97 | 83 | 85.6% |
| `release/internal/authoring/producers` | 58 | 56 | 96.6% |
| `release/internal/buildinfo` | 8 | 7 | 87.5% |
| `release/internal/bump` | 94 | 94 | 100.0% |
| `release/internal/changelog` | 49 | 42 | 85.7% |
| `release/internal/changelogfile` | 62 | 62 | 100.0% |
| `release/internal/cli` | 439 | 403 | 91.8% |
| `release/internal/config` | 139 | 138 | 99.3% |
| `release/internal/corpus` | 83 | 82 | 98.8% |
| `release/internal/cut` | 206 | 176 | 85.4% |
| `release/internal/delegate` | 32 | 32 | 100.0% |
| `release/internal/gitread` | 122 | 118 | 96.7% |
| `release/internal/render` | 91 | 90 | 98.9% |
| `release/internal/resolve` | 28 | 27 | 96.4% |
| `release/internal/schemacheck` | 20 | 15 | 75.0% |
| `release/internal/semver` | 110 | 110 | 100.0% |
| `release/internal/version` | 75 | 75 | 100.0% |
| **Total** | **1,714** | **1,611** | **94.0%** |

Generated code is excluded from these figures: 251 statements in 1 file (`releasemodel.go`) carrying the `// Code generated … DO NOT EDIT.` marker. It is excluded from the coverage profile too, so every report derived from it agrees with this table.
<!-- coverage:table:end -->
