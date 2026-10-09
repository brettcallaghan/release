---
name: cut-release
description: Cuts a release with the release CLI at the version this plugin
  pins. Provisions that CLI first, previews the next version and its
  changelog, confirms the bump with the person, then stages the release as a
  stage branch and pull request. Use when asked to cut, stage, prepare or
  ship a release, to preview what the next release would be, or to set up or
  review a repository's release process, its release configuration or its
  commit conventions.
version: "0.1"
user-invocable: true
---

# cut-release

Cut a release with the `release` CLI. The CLI computes the version, authors
the changelog and stages the release; this skill is the order to run it in
and the judgement calls around it. The CLI's own `--help` is the reference
for every command and flag — read it rather than guessing.

## 1. Provision the CLI — always first

```sh
bash "${CLAUDE_PLUGIN_ROOT}/skills/cut-release/scripts/install-cli" "${CLAUDE_PLUGIN_DATA}"
```

On success it prints one line: the absolute path of the CLI at exactly this
plugin's version. **Keep that path; it is `<cli>` below.** Run every CLI
command through it — never as a bare `release`. A `release` on the PATH may
be another version (a package-manager install, an old build), and a skill
that ran it would describe one binary while running another.

The script reuses what is already there — a repository's own build named by
`RELEASE_CLI`, an earlier download in the plugin data directory, or a
`release` on the PATH that reports the pinned version — and otherwise
downloads the pinned release, verifies its checksum and caches it.

**If it exits non-zero, tell the person its message — one line naming the
cause and the remedy — and stop.** Do not build the CLI from source, download
it another way, or run a different `release`. A checksum mismatch, an
unreadable release or an unsupported platform is the person's to fix, and a
substitute binary is exactly what the pinned version exists to prevent.

## 2. Read the help

Run `<cli> --help`, then `<cli> <command> --help` for each command before
you first run it. The help text is the CLI's contract: what the command
reads, what it writes, and what each exit status means.

## 3. Preview

```sh
<cli> changelog
```

It answers with the next version, the suggested bump with its reasons (the
changelog entries and commit signals that drove it), and the rendered
changelog section. It writes nothing. Exit 1 means no release is warranted:
say so and stop, unless the person wants to name a bump anyway.

`<cli> config` shows the effective configuration — useful when the preview
surprises you (wrong target branch, unexpected groups).

## 4. Confirm the bump with the person

Show the person the version, the suggested bump and its reasons, and the
changelog section. Ask them to confirm the suggestion or name another bump
(`major`, `minor` or `patch`). **Do not decide it yourself**: a version
number is a person's decision, recorded on the release commit as its
`Release-Type:` trailer.

- A bump below the suggestion proceeds with a warning; show it to them.
- A bump below the signal floor — what the commits' own records require —
  is refused without `--force`. Pass `--force` only when the person
  explicitly asks to override those records, having seen which commits set
  the floor.

## 5. Cut

```sh
<cli> cut <bump>
```

with the bump the person confirmed. A named bump is a decision, so `cut`
does not ask for it again. It prepares the release in a scratch checkout,
writes `CHANGELOG.md`, runs the repository's propagate and validate
commands, commits `Release <version>` on a stage branch, pushes it and opens
the stage pull request. Your working tree is never touched. `--dry-run`
reports every intended action first if the person wants to see them.

Report the pull request's URL. Exit 1 means `cut` refused and wrote nothing;
exit 3 means a write failed after another succeeded — relay the writes it
reports and let the person decide.

## What you never do

**You never create a tag, publish a release, or merge the stage pull
request.** Publication is the repository's own workflow, run when a person
merges the stage pull request after review. Hand over the pull request and
stop there.

## Commit conventions

The CLI reads the bump signals from git, nowhere else (not pull request
labels, not issue trackers):

- **Conventional types** in the commit subject, `type(scope)!: description`:
  `feat` signals minor; `fix`, `perf`, `refactor` and `revert` signal patch;
  other types (`docs`, `chore`, `test`, …) signal nothing.
- **Breaking changes**: a `!` before the subject's colon, or a
  `BREAKING CHANGE:` line in the body's final paragraph, signals major.
- **A `Release-Type:` trailer** (`major`, `minor`, `patch` or `none`) states
  a commit's impact outright. `none` never lowers what another signal says.

The highest signal across the range is the **signal floor**: the release is
never suggested below it. A commit carrying no signal is reported as
unclassified, and the authored changelog decides what it means. When
reviewing a repository's commits, point out subjects that would mislead
these readers — a breaking change written as `fix:`, a feature as `chore:`.

## Configuring a repository

The configuration is `release.toml` or `config/release.toml` at the
repository root (or the file `--config` names). Read `<cli> config --help`
for how it is found and `<cli> config` for every key with its effective
value; `<cli> config` exits 2 when the file is refused, naming why.

The keys a new repository most often sets: `target`, the release line's
branch; `release_paths`, every file the release commit may change besides
`CHANGELOG.md`; `commands.propagate`, which writes the version (from
`RELEASE_VERSION`) into those files; and `commands.validate`, which must pass
before anything is pushed. A file the propagate command changes that
`release_paths` does not list makes `cut` refuse.

To make the skills run the repository's own build of the CLI instead of the
pinned release — when the repository *is* the CLI, say — set `RELEASE_CLI`
in the `env` of its `.claude/settings.json` to the binary's path, relative
to the repository root.
