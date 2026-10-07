# release

<!-- coverage:badge:start -->
[![coverage](https://img.shields.io/badge/coverage-97.5%25-brightgreen)](#test-coverage)
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
| Unit tests | 521 / 571 | 91.2% |
| Snapshot cases | 362 / 571 | 63.4% |
| End-to-end (built binary) | 329 / 571 | 57.6% |
| **All layers combined** | **557 / 571** | **97.5%** |

| Package | Statements | Covered | Coverage |
|:---|---:|---:|---:|
| `release/cmd/release` | 1 | 1 | 100.0% |
| `release/internal/buildinfo` | 8 | 7 | 87.5% |
| `release/internal/bump` | 79 | 79 | 100.0% |
| `release/internal/cli` | 185 | 175 | 94.6% |
| `release/internal/gitread` | 96 | 95 | 99.0% |
| `release/internal/resolve` | 27 | 26 | 96.3% |
| `release/internal/semver` | 110 | 109 | 99.1% |
| `release/internal/version` | 65 | 65 | 100.0% |
| **Total** | **571** | **557** | **97.5%** |

Generated code is excluded from these figures: 278 statements in 1 file (`releasemodel.go`) carrying the `// Code generated … DO NOT EDIT.` marker. It is excluded from the coverage profile too, so every report derived from it agrees with this table.
<!-- coverage:table:end -->
