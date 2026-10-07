---
title: "Component Blueprint: Changelog Producer"
description: The component type that receives a changelog corpus and returns
  an authored one — a language model, a command, a conventional-commit parser,
  or a person.
slug: component-changelog-producer
type: component-blueprint
status: approved
updated_at: 2026-10-07T00:00:00Z
kind: pattern
contracts_provided: []
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
receives an assembled corpus and returns an authored one: prose written,
entries grouped and summarised, a bump proposed.

There are several ways to author a changelog. A language model reading
pull-request descriptions is the default, reached by invoking the `claude`
command on the maintainer's workstation — the same mechanism Strata's existing
release script already uses, which is why it is the one producer this project
knows it can rely on.

The others exist because that one is not always right. A command lets a project
drive whatever it already has. A conventional-commit parser needs no model at
all and suits a project whose commit discipline already carries the
information. A person editing the corpus in a text editor is a producer too,
and is the fallback when a release needs correcting by hand.

The bridge knows none of them apart. Every producer is reached the same way
and every returned corpus is validated the same way, which is what makes
authoring replaceable rather than merely configurable.

- **Role:** author a changelog corpus.
- **Scope:** in — reading an assembled corpus and returning an authored one;
  out — deciding the version, writing files, and opening pull requests.
- **Composed into:** [[module-release-core]] — invoked by
  [[component-changelog-authoring-bridge]]; see the module for relationships.
- **Source & tests:** `internal/authoring/producers`.

## Interface

- **Surface:** given an assembled corpus, return an authored corpus
  conforming to [[contract-changelog]].
- **Guarantees:**
  - A producer SHALL return a corpus that validates against the schema
    [[contract-changelog]] names.
  - A producer SHALL set `proposed_bump`.
  - A producer SHALL preserve the sources of every entry it carries forward.
  - A producer SHALL NOT set `version`.
  - A producer SHALL NOT write to the repository.
  - IF a producer's prerequisite is unavailable, THEN it SHALL refuse and name
    the prerequisite rather than degrading silently.

## Pattern Guidance

**Building a new instance:**

1. Accept an assembled corpus and return an authored one. Take no other
   input from the process environment.
2. Declare the configuration keys the producer needs, and nothing more.
3. Where authoring can fail, fail loudly: return an error rather than a
   corpus with empty prose.
4. Register the producer so a project can select it by name.

**Conformance — every instance:**

- SHALL be selectable by name in the repository's configuration.
- SHALL be deterministic in its handling of an empty corpus, returning an
  empty authored corpus rather than inventing entries.
- SHALL carry every entry of the assembled corpus into its result, or state
  in its documentation which entries it drops and why.
- SHALL NOT require network access unless its documentation says so.
- SHALL NOT read credentials the bridge did not pass it.

**Instances:**

- **Claude CLI producer** (`internal/authoring/producers/claude`) — **the
  default.** Invokes the `claude` command on the workstation, passing the
  corpus and reading the authored result back. Requires no API credential,
  because it uses the session the maintainer already has.
- **Command producer** (`internal/authoring/producers/command`) — runs a
  configured command, passing the corpus on standard input and reading the
  result from standard output. The escape hatch for any other model or tool.
- **Conventional-commit producer**
  (`internal/authoring/producers/conventional`) — derives entries from
  conventional-commit subjects with no model involved.
- **Manual producer** (`internal/authoring/producers/manual`) — writes the
  corpus to a path and waits for a person to return it.

## Cross-Cutting Standards

Every instance SHALL comply with:

- [[contract-changelog]] — the corpus semantics.

## Change Guidance

WHEN a producer is added, it SHALL be added to the Instances list above in
the same change.

---

A producer SHALL NOT be given responsibility for the version.

**Rationale:** [[adr-003]] keeps the version a computation and the prose an
authoring act. A producer that set `version` would make the version depend on
which producer a project selected.

---

- **Verification gates:** the conformance suite every producer runs, plus the
  individual producer's own tests. The conformance suite covers the empty
  corpus, schema validity of the result, and that `version` is left unset.

## Key Decisions

- [[adr-003]] — authored prose proposes impact; computation decides it.
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
- [[contract-changelog]] — the corpus a producer returns
- [[frd-changelog-authoring]] — the feature this component serves
