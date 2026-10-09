---
title: "Component Blueprint: Changelog Producer"
entity_name: Changelog Producer
description: The component type that receives an evidence bundle and returns
  an authored changelog corpus or authored pull-request notes — a language
  model, a command, the mechanical producer, or a person.
slug: component-changelog-producer
type: component-blueprint
status: draft
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
receives an evidence bundle and returns what it authored from it. At pull
request time that is `PullRequestNotes`: one entry per user-visible change in
the pull request, and a proposed impact. At release time it is a changelog
corpus: the changes in the release, split, merged, classified, or omitted
where internal, and a proposed impact with the reasons for it.

A language model reading the diffs and the reviewed notes is the default,
reached by invoking the `claude` command on the maintainer's workstation. The
others exist because that one is not always right or available. A command
lets a project drive whatever it already has. The mechanical producer needs
no model at all. A person editing the result in a text editor is a producer
too, and is the fallback when a release needs correcting by hand.

The bridge knows none of them apart. Every producer is reached the same way
and every result is validated the same way, which is what makes authoring
replaceable rather than merely configurable.

- **Role:** author from evidence.
- **Scope:** in — reading an evidence bundle and returning an authored
  result; out — collecting evidence, deciding the version, writing files,
  and opening or editing pull requests.
- **Composed into:** [[module-release-core]] — invoked by
  [[component-changelog-authoring-bridge]]; see the module for relationships.
- **Source & tests:** `internal/authoring/producers`.

## Interface

- **Surface:** given an evidence bundle, return `PullRequestNotes` WHEN its
  `purpose` is `PULL_REQUEST` and a changelog corpus WHEN it is `RELEASE`,
  each conforming to [[contract-changelog]].
- **Guarantees:**
  - A producer SHALL return a result that validates against the schema
    [[contract-changelog]] names.
  - A producer SHALL set the proposed impact, and at release time
    `proposal_rationale` and `proposal_drivers`.
  - A producer SHALL copy the bundle's `sources` into the corpus unchanged.
  - A producer SHALL cite in every entry's `sources` the commits or pull
    requests the entry describes.
  - A producer SHALL carry the entries of a reviewed notes block forward
    without contradicting them; it MAY merge or reword them into the
    release's account of a change.
  - A producer SHALL NOT set `version`.
  - A producer SHALL NOT write to the repository or the hosting platform.
  - IF a producer's prerequisite is unavailable, THEN it SHALL refuse and name
    the prerequisite rather than degrading silently.

## Pattern Guidance

**Building a new instance:**

1. Accept an evidence bundle and return an authored result. Take no other
   input from the process environment.
2. Branch on the bundle's `purpose`, or refuse the purpose the producer
   does not serve.
3. Declare the configuration keys the producer needs, and nothing more.
4. Where authoring can fail, fail loudly: return an error rather than a
   result with empty prose.
5. Register the producer so a project can select it by name.

**Conformance — every instance:**

- SHALL be selectable by name in the repository's configuration.
- SHALL be deterministic in its handling of an empty bundle, returning an
  empty result rather than inventing entries.
- SHALL account for every commit in the bundle: as an entry's source, or
  as omitted internal work.
- SHALL NOT require network access unless its documentation says so.
- SHALL NOT read credentials the bridge did not pass it.

**Instances:**

- **Claude CLI producer** (`claude`, `producers.Claude` in
  `internal/authoring/producers`) — **the default.** Runs `claude -p` in a
  separate process with a prompt invoking a skill of this repository's
  `release` plugin: `release:notes` for a pull-request bundle and
  `release:changelog` for a release bundle. The bundle is JSON on standard
  input, and the result is read as JSON from standard output, unwrapping
  one markdown code fence around the whole output if the model added one.
  Fails on a non-zero exit or empty output. Refuses, naming the `claude`
  command, WHEN it is not on `PATH`. Requires no API credential, because it
  uses the session the maintainer already has.
- **Command producer** (`command`, `producers.Command` in
  `internal/authoring/producers`) — runs `changelog.producer_command`
  through `sh -c` in the repository root, the bundle as JSON on standard
  input, and reads the result as JSON from standard output; the command
  reads the bundle's `purpose` to know which to return. Fails on a non-zero
  exit, reporting the command's standard error, or on empty output.
  Refuses, naming the missing prerequisite, WHEN
  `changelog.producer_command` is unset or `sh` is not on `PATH`. The
  escape hatch for any other model or tool.
- **Mechanical producer** (`mechanical`, `producers.Mechanical` in
  `internal/authoring/producers`) — [[component-mechanical-producer]].
  Entries from reviewed notes blocks and conventional subjects, in
  process, with no model, proposing the computed impact. Serves release
  bundles only, `--skip-notes`, and continuous integration with no model.
- **Manual producer** (`internal/authoring/producers/manual`) — writes the
  bundle to a path and waits for a person to return the result.

## Cross-Cutting Standards

Every instance SHALL comply with:

- [[contract-changelog]] — the evidence, corpus and notes semantics.

## Change Guidance

WHEN a producer is added, it SHALL be added to the Instances list above in
the same change.

---

A producer SHALL NOT be given responsibility for the version.

**Rationale:** [[adr-003]] keeps the version a decision over reviewed
records, and the prose an authoring act. A producer that set `version` would
make the version depend on which producer a project selected.

---

WHEN a skill the Claude CLI producer invokes changes the shape it returns,
[[contract-changelog]] SHALL change in the same change.

---

- **Verification gates:** the conformance suite every producer runs, plus the
  individual producer's own tests. The conformance suite covers the empty
  bundle, schema validity of the result at each purpose it serves, and that
  `version` is left unset.

## Key Decisions

- [[adr-003]] — a producer authors from evidence and proposes impact with
  reasons; reviewed records and a person decide it.
- [[adr-004]] — changelog authoring is a replaceable step rather than a
  render template.

## Open Questions

None. The Claude CLI producer is the default. A machine without the `claude`
command gets a refusal naming the producer and the alternatives, rather than a
silent fallback that would make release notes depend on which tools happened to
be installed.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-changelog-authoring-bridge]] — the component that invokes a
  producer
- [[component-mechanical-producer]] — the model-free instance
- [[contract-changelog]] — the bundle a producer reads and the results it
  returns
- [[frd-changelog-authoring]] — the feature this component serves
