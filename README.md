# release

<!-- coverage:badge:start -->
[![coverage](https://img.shields.io/badge/coverage-93.8%25-brightgreen)](#test-coverage)
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
| Unit tests | 2,161 / 2,535 | 85.2% |
| Snapshot cases | 1,689 / 2,535 | 66.6% |
| End-to-end (built binary) | 1,410 / 2,535 | 55.6% |
| **All layers combined** | **2,379 / 2,535** | **93.8%** |

| Package | Statements | Covered | Coverage |
|:---|---:|---:|---:|
| `release/cmd/release` | 1 | 1 | 100.0% |
| `release/internal/authoring` | 169 | 154 | 91.1% |
| `release/internal/authoring/producers` | 97 | 94 | 96.9% |
| `release/internal/authoring/producers/producertest` | 53 | 41 | 77.4% |
| `release/internal/buildinfo` | 8 | 7 | 87.5% |
| `release/internal/bump` | 85 | 85 | 100.0% |
| `release/internal/changelog` | 87 | 71 | 81.6% |
| `release/internal/changelogfile` | 62 | 62 | 100.0% |
| `release/internal/cli` | 576 | 534 | 92.7% |
| `release/internal/config` | 149 | 148 | 99.3% |
| `release/internal/cut` | 323 | 285 | 88.2% |
| `release/internal/delegate` | 32 | 32 | 100.0% |
| `release/internal/evidence` | 49 | 47 | 95.9% |
| `release/internal/gitread` | 439 | 421 | 95.9% |
| `release/internal/render` | 95 | 94 | 98.9% |
| `release/internal/resolve` | 39 | 38 | 97.4% |
| `release/internal/schemacheck` | 25 | 19 | 76.0% |
| `release/internal/semver` | 110 | 110 | 100.0% |
| `release/internal/suggest` | 61 | 61 | 100.0% |
| `release/internal/version` | 75 | 75 | 100.0% |
| **Total** | **2,535** | **2,379** | **93.8%** |

Generated code is excluded from these figures: 513 statements in 1 file (`releasemodel.go`) carrying the `// Code generated … DO NOT EDIT.` marker. It is excluded from the coverage profile too, so every report derived from it agrees with this table.
<!-- coverage:table:end -->
