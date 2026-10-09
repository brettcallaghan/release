---
name: changelog
description: Authors a release's structured changelog. Reads the release's
  evidence bundle (each commit's record, pull request, changed files, diff and
  impact signal) as JSON on standard input, writes the release's changes and
  the commits it leaves out, and writes the authored changelog as JSON to
  standard output. Invoked by the release CLI's claude changelog producer as
  release:changelog.
version: "0.1"
user-invocable: false
---

# changelog

Turn a release's evidence bundle into a structured changelog.

## The exchange

- **Input:** the evidence bundle, as JSON, on standard input.
- **Output:** the authored changelog, as JSON, on standard output. Write the
  JSON document and nothing else: no prose before or after it, no code
  fence, no commentary. The caller parses standard output as the changelog
  and fails on anything that is not.

## Shape

The changelog's shape is `references/changelog.strata` and the evidence
bundle's is `references/evidence.strata`; `references/collection.strata`
holds the types they share. The semantics are
`references/changelog-contract.md`. All ship with this skill, relative to its
directory. Read them rather than relying on a copy here. Write only fields the
schema and the contract declare, and no rendered markdown in `summary`.

## What you author

- **Entries are changes, not commits.** Read each commit's subject, body,
  pull request, files and diff, and write one entry per change a reader of
  the changelog would recognise. Split a commit that made several changes;
  merge commits that together made one.
- **Each entry** has a `kind`, a `scope` where the change belongs to one
  area, a `summary` of one or two full sentences from the reader's point of
  view, `detail` only where the summary cannot stand alone, `breaking` where
  an upgrading consumer is broken, and `migration` on every breaking entry,
  naming what the consumer does or what replaces what was removed.
- **Choose `kind` from the evidence.** Take it from the commit's
  conventional type where it has one. Where the subject carries no type,
  read the diff and the message: a change that adds a flag, command, option
  or behaviour a user can invoke is `FEATURE`; `FIX` is a correction of
  behaviour that was wrong. Never leave a kind unset.
- **State what the tool does.** A summary describes the product's behaviour.
  This repository's own configuration, such as the values in its
  `config/release.toml`, is not the tool's default and is not product
  behaviour; do not write it as if it were.
- **Each entry's `sources`** cite the commits it derives from, by full hash
  from the bundle, and may add the pull request those commits record. Cite
  nothing the bundle does not hold.
- **`omitted`** lists every commit no entry cites, each with a one-line
  reason, such as internal refactoring or test-only work. Every commit in the
  bundle is cited or omitted; none is left out of both.
- **Cite a pull request** only where a commit in the bundle records it.
- **`title` and `summary`** introduce the release, where it has a theme
  worth naming.

A diff the bundle marks truncated, or a file its `budget` omitted, is one you
did not see in full: read the commit's message and file list for the rest,
and do not describe what you cannot see.

## What you never write

- `proposed_bump`: the release CLI suggests the bump from your entries'
  kinds and breaking flags. State each change's kind accurately; do not
  state the release's impact.
- `version`: the computed version is the tooling's.
- The changelog's `sources`: copy the bundle's `sources` unchanged, including
  any `DIFFS` budget report.
- `derived`: clear it on every entry. Only the mechanical producer sets it.

Authoring order is presentation order: put the entries a reader should see
first, first.
