---
name: changelog
description: Authors a release's changelog corpus. Reads the assembled corpus
  (commit, pull request and diff records) as JSON on standard input, fills in
  the authored prose, and writes the authored corpus as JSON to standard
  output. Invoked by the release CLI's claude changelog producer as
  release:changelog.
version: "0.1"
user-invocable: false
---

# changelog

Turn an assembled changelog corpus into an authored one.

## The exchange

- **Input:** the assembled corpus, as JSON, on standard input.
- **Output:** the authored corpus, as JSON, on standard output. Write the JSON
  document and nothing else: no prose before or after it, no code fence, no
  commentary. The caller parses standard output as the corpus and fails on
  anything that is not.

## Shape

The shape is `references/changelog.strata`; the semantics are
`references/changelog-contract.md`. Both ship with this skill, relative to its
directory. Read them rather than relying on a copy here. Write only fields the schema declares, and no rendered markdown in
`summary`.

## What you author

The prose a person would otherwise write: `title`, `summary`, each entry's
`summary`, `detail` and `migration`, and `proposed_bump` as a proposal. The
corpus arrives without `proposed_bump`; return it with one, or the release
CLI rejects the result.

## What you never change

- `version`: the computed version is the tooling's. Leave it as it arrived.
- Any field derived from the collected records rather than authored:
  `schema_version`, the corpus's `sources` and each entry's `sources`. Carry
  them through unchanged.
- An entry's `derived` flag, unless you replace its summary with your own
  prose: then clear it.
- Set `migration` on every entry whose `breaking` flag is set.

Authoring order is presentation order: put the entries a reader should see
first, first.
