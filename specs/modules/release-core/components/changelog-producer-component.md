---
title: "Component Blueprint: Changelog Producer"
entity_name: Changelog Producer
description: The component type that receives a release's evidence bundle
  and returns a structured changelog — a language model, a command, the
  mechanical producer, or a person.
slug: component-changelog-producer
type: component-blueprint
status: approved
updated_at: 2026-10-09T00:00:00Z
kind: pattern
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/authoring/producers
test_paths:
  - internal/authoring/producers
---

# Changelog Producer

## Overview

The plug point behind [[component-changelog-authoring-bridge]]. A producer
receives a release's evidence bundle and returns a **structured changelog**:
the changes in the release, each with a kind, a scope, a summary of one or
two full sentences, optional detail, migration text where it is breaking,
and the commits it is derived from; and the commits it leaves out, each
with a one-line reason. Entries are changes, not commits, so a producer may
split one commit into several entries or merge several commits into one.

A producer proposes no bump. [[component-bump-suggester]] derives the
suggestion from the entry kinds the producer chose, so the producer's
judgement reaches the version only through what it says changed, and only
once a person confirms it.

A language model reading the commits, their pull request descriptions and
their diffs is the default, reached by invoking the `claude` command on the
maintainer's workstation. The others exist because that one is not always
right or available. A command lets a project drive whatever it already has.
The mechanical producer needs no model at all. A person editing the result
in a text editor is a producer too, and is the fallback when a release needs
correcting by hand.

The bridge knows none of them apart. Every producer is reached the same way
and every result is validated the same way, which is what makes authoring
replaceable rather than merely configurable.

- **Role:** author a structured changelog from evidence.
- **Scope:** in — reading an evidence bundle and returning a structured
  changelog; out — collecting evidence, suggesting or deciding the bump,
  writing files, and opening or editing pull requests.
- **Composed into:** [[module-release-core]] — invoked by
  [[component-changelog-authoring-bridge]]; see the module for relationships.
- **Source & tests:** `internal/authoring/producers`.

## Interface

- **Surface:** given an evidence bundle, return a structured changelog
  conforming to [[contract-changelog]].
- **Guarantees:**
  - A producer SHALL return a changelog that validates against the schema
    [[contract-changelog]] names.
  - A producer SHALL copy the bundle's `sources` into the changelog
    unchanged.
  - A producer SHALL cite in every entry's `sources` the commits the entry
    is derived from.
  - A producer SHALL cite every commit in the bundle in at least one entry,
    or list it in `omitted` with a one-line reason.
  - A producer SHALL give every breaking entry a `migration`.
  - A producer SHALL NOT set `version`.
  - A producer SHALL NOT write to the repository or the hosting platform.
  - IF a producer's prerequisite is unavailable, THEN it SHALL refuse and name
    the prerequisite rather than returning a degraded changelog.

## Pattern Guidance

**Building a new instance:**

1. Accept an evidence bundle and return a structured changelog. Take no
   other input from the process environment.
2. Declare the configuration keys the producer needs, and nothing more.
3. Where authoring can fail, fail loudly: return an error rather than a
   result with empty prose.
4. Register the producer so a project can select it by name.

**Conformance — every instance:**

- SHALL be selectable by name in the repository's configuration.
- SHALL be deterministic in its handling of an empty bundle, returning a
  changelog with no entries and nothing omitted rather than inventing
  entries.
- SHALL account for every commit in the bundle: as an entry's source, or
  in `omitted` with a reason.
- SHALL NOT require network access unless its documentation says so.
- SHALL NOT read credentials the bridge did not pass it.

**Instances:**

- **Claude CLI producer** (`claude`, `producers.Claude` in
  `internal/authoring/producers`) — **the default.** Runs `claude -p` in a
  separate process with a prompt invoking the `release:changelog` skill of
  this repository's `release` plugin. The bundle is JSON on standard input,
  and the changelog is read as JSON from standard output, unwrapping one
  markdown code fence around the whole output if the model added one.
  Fails on a non-zero exit or empty output. Refuses, naming the `claude`
  command, WHEN it is not on `PATH`. Requires no API credential, because it
  uses the session the maintainer already has.
- **Command producer** (`command`, `producers.Command` in
  `internal/authoring/producers`) — runs `changelog.producer_command`
  through `sh -c` in the repository root, the bundle as JSON on standard
  input, and reads the changelog as JSON from standard output. Fails on a
  non-zero exit, reporting the command's standard error, or on empty
  output. Refuses, naming the missing prerequisite, WHEN
  `changelog.producer_command` is unset or `sh` is not on `PATH`. The
  escape hatch for any other model or tool.
- **Mechanical producer** (`mechanical`, `producers.Mechanical` in
  `internal/authoring/producers`) — [[component-mechanical-producer]].
  Entries from conventional subjects, in process, with no model. Serves
  `--skip-notes`, including continuous integration with no model.
- **Manual producer** (`internal/authoring/producers/manual`) — writes the
  bundle to a path and waits for a person to return the changelog.

## Cross-Cutting Standards

Every instance SHALL comply with:

- [[contract-changelog]] — the evidence and changelog semantics.

## Change Guidance

WHEN a producer is added, it SHALL be added to the Instances list above in
the same change.

---

A producer SHALL NOT be given responsibility for the version or the bump.

**Rationale:** [[adr-003]] keeps the version a person's confirmation of a
suggestion derived deterministically from the changelog and floored by git.
A producer that set `version` or a bump would make the version depend on
which producer a project selected.

---

WHEN the skill the Claude CLI producer invokes changes the shape it returns,
[[contract-changelog]] SHALL change in the same change.

---

- **Verification gates:** the conformance suite every producer runs, plus the
  individual producer's own tests. The conformance suite covers the empty
  bundle, schema validity of the result, every commit cited or omitted, and
  that `version` is left unset.

## Key Decisions

- [[adr-003]] — a producer authors a structured changelog from commit
  evidence; the bump is suggested from it and confirmed by a person.
- [[adr-004]] — changelog authoring is a replaceable step rather than a render
  template.

## Open Questions

None. The Claude CLI producer is the default. A machine without the `claude`
command gets a refusal naming the prerequisite, and the command stops and
names `--skip-notes`, so the changelog never depends silently on which tools
happened to be installed.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-authoring-bridge]] — the component that invokes a
  producer
- [[component-mechanical-producer]] — the model-free instance
- [[contract-changelog]] — the bundle a producer reads and the changelog it
  returns
- [[frd-changelog-authoring]] — the feature this component serves
