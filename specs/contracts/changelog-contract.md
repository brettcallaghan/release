---
title: "Contract: Changelog Corpus"
description: The structured representation of a release's changelog, written
  by an authoring producer and consumed by deterministic renderers.
slug: contract-changelog
type: contract
status: approved
updated_at: 2026-10-07T00:00:00Z
provider: module-release-core
visibility: public
specs:
  - schemas/changelog.strata
version: "1.0"
---

# Changelog Corpus

The structured form of a release's changelog: its proposed impact, its
narrative, its itemized entries, its breaking changes, and the inputs it
was authored from. It is the seam between a producer that may vary between
runs — a person, or a language model reading commits, pull requests and
diffs — and consumers that may not. Provided by [[container-release-cli]].
Visibility: public.

Public rather than internal because any producer may write an instance:
the authoring skill vended to adopting repositories, a hand-edited file
when a release needs correcting, or third-party tooling. The schema is the
only thing those producers share.

## Specification

| Artifact | Location | Normative for |
|----------|----------|---------------|
| Strata schema | `schemas/changelog.strata` | Shape |
| Collector records | `schemas/collection.strata` | Shape of the inputs a corpus is built from |
| JSON Schema | generated: `strata build schemas/changelog.strata --target json-schema` | Shape, for validators |
| This document | — | Semantics, operations, change policy |

The Strata schema is the single source; the JSON Schema and every
language binding are generated from it and are never hand-edited.

## Guarantees

WHEN an instance declares a `schema_version` a consumer does not
implement, the consumer SHALL reject the instance.

**Rationale:** Rendering an instance partially produces release notes
that silently omit content, which is worse than failing to render.

---

The provider SHALL render every published artifact describing one release
from a single instance.

**Rationale:** Two artifacts rendered from one instance cannot
contradict each other. Independently generated release notes and
changelog sections can, and have.

---

The provider SHALL treat `proposed_bump` as a proposal.

---

The provider SHALL NOT derive a published version from `proposed_bump`.

---

IF `proposed_bump` disagrees with the bump computed from commit metadata,
THEN the provider SHALL halt and require human confirmation.

**Rationale:** Disagreement usually means a change was mis-typed or
mis-trailered, which is the failure the deterministic classifier cannot
detect on its own ([[adr-003]]).

---

The provider SHALL require `version` to be set before rendering.

---

Renderers SHALL preserve the order of `entries` within each rendered
group.

**Rationale:** Authoring order is presentation order. A renderer that
re-sorts discards the author's judgement about what matters most.

---

The provider SHALL render an instance identically on every run given
identical input.

---

Producers SHALL set `migration` on every entry whose `breaking` flag is set.

**Rationale:** A breaking change without migration guidance is an incident
report rather than a release note. There is no separate collection for
breaking changes: an entry carries the flag and its own guidance, so the two
cannot be recorded apart from each other.

---

Consumers SHALL tolerate unknown fields within a compatible
`schema_version`.

## Operational Expectations

- **Availability / SLO:** not applicable — the corpus is a file, not a
  service.
- **Latency:** rendering SHALL complete without network access.
- **AuthN/AuthZ:** not applicable.
- **Consumer obligations:** consumers SHALL validate an instance against
  the generated JSON Schema before rendering, and SHALL NOT infer meaning
  from field order.
- **Producer obligations:** producers SHALL write only fields the schema
  declares, and SHALL NOT embed rendered markdown in `summary`.

## Versioning and Change Policy

- **Versioning scheme:** semver on `schema_version`, carried in every
  instance.
- **Compatible changes:** the provider MAY add optional fields and
  MAY add enum variants without a major version change.
- **Breaking changes:** WHEN a required field is added, removed or
  re-typed, the provider SHALL increment the major `schema_version`.
- **Deprecation:** WHEN a major `schema_version` is superseded, consumers
  SHALL continue to accept the prior major for one release cycle of the
  tooling.

A new enum variant is a compatible change only because consumers render
an unrecognized `ChangeKind` into a default section rather than failing.
That behaviour is itself a guarantee and SHALL NOT be removed without a
major version change.

## Key Decisions

- [[adr-003]] — the bump is computed deterministically; authored prose
  proposes but never decides.
- [[adr-004]] — changelog authoring is a replaceable step, which is what
  requires a structured intermediate rather than a render template.

## Open Questions

- [ ] Whether `IssueSource` should carry a tracker discriminator rather than
  relying on the identifier's own form — decided when the first adopter wires
  a second tracker.

## References

- `shared/c4-standards.md` — C4 mapping rules and contract visibility tiers
- [[container-release-cli]] — provider's blueprint
- [[frd-changelog-authoring]] — the feature that produces instances
- [[frd-changelog-rendering]] — the feature that consumes them
