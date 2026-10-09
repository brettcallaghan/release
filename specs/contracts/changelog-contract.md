---
title: "Contract: Changelog Corpus"
description: The evidence a producer reads and the structured changelog it
  writes, exchanged between producers that may vary and the deterministic
  tooling that validates and renders them.
slug: contract-changelog
type: contract
status: approved
updated_at: 2026-10-09T00:00:00Z
provider: module-release-core
visibility: public
specs:
  - schemas/changelog.strata
version: "1.0"
---

# Changelog Corpus

The seam between a producer that may vary between runs — a person, or a
language model reading commits and diffs — and the tooling that may not. It
has two parts:

- **The evidence bundle** — what a producer reads: the commits in a release's
  range, what git says about each, including the pull request it carries,
  the files it changed, its diff within the release's budget, and the impact
  signal the Bump Classifier read from it.
- **The structured changelog** (the corpus) — what a producer writes: the
  release's changes, each with its kind, scope, summary, detail, migration
  guidance and the commits it derives from, and the commits it leaves out
  with the reason for each.

Provided by [[container-release-cli]]. Visibility: public.

Public rather than internal because any producer may read and write these:
the authoring skill vended to adopting repositories, a hand-edited file when a
release needs correcting, or third-party tooling. The schemas are the only
thing those producers share.

## Specification

| Artifact | Location | Normative for |
|----------|----------|---------------|
| Strata schema | `schemas/changelog.strata` | Shape of the corpus |
| Evidence schema | `schemas/evidence.strata` (new; lands with the implementation) | Shape of the evidence bundle, as [Evidence Bundle](#evidence-bundle) states it |
| Collector records | `schemas/collection.strata` | Shape of the records the evidence carries |
| JSON Schema | generated: `strata build schemas/<name>.strata --target json-schema` | Shape, for validators |
| This document | — | Semantics, rendering, operations, change policy |

The Strata schemas are the single source for shape; the JSON Schema and
every language binding are generated from them and are never hand-edited.
Where this document states a shape the schemas do not yet carry, this
document is normative until they do.

## Corpus

An entry is a **change**, not a commit. A producer may split one commit into
several entries, or merge several commits into one entry. Its `sources` cite
the commits it derives from, and may also cite the pull requests and issues
those commits record. Every commit in the evidence is either cited by at
least one entry or listed in `omitted` with a reason.

The corpus amends `schemas/changelog.strata` as follows:

- `Changelog.omitted: [OmittedCommit]?` — the commits no entry cites, each
  with why:

  ```
  struct OmittedCommit {
    hash: String                    # full hash, as the evidence carries it
    reason: String                  # one line; never rendered
  }
  ```

- `ChangelogEntry.kind: ChangeKind?` — optional. Every entry a model or a
  person authors carries a kind. Only the mechanical producer writes an
  entry with none, for a commit whose subject carries no conventional type;
  such an entry renders under the other heading.
- `AdapterRun.budget: BudgetReport?` — on the run of the `DIFFS` adapter, the
  diff budget report of the evidence the corpus was authored from.
- `ChangelogEntry.summary` is one or two full sentences stating the change
  from the reader's point of view. `ChangelogEntry.detail` is the home of
  anything longer, rendered beneath the summary.
- `ChangelogEntry.migration` is required WHEN `breaking` is set, and says
  what a consumer must do to adopt the change.
- `ChangelogEntry.derived` is set only by the mechanical producer, for a
  summary it derived from a commit subject: nobody wrote the sentence.
- `Changelog.proposed_bump` is left unset by every producer. The suggested
  bump is derived by the tooling from the entries' kinds and breaking flags
  ([[component-bump-suggester]]), so a producer states the impact of each
  change and never of the release.

## Evidence Bundle

The producer reads an `Evidence` instance. It is built by the tooling, never
by a producer, and states what git said without interpreting it. Every
record in it is read from git; no hosting platform is consulted unless the
optional pull request adapter is enabled.

`schemas/collection.strata` gains the records it carries:

```
struct CommitRecord {
  # ...existing fields...
  files: [FileChange]               # every file the commit changed
}
struct FileChange {
  path: String
  previous_path: String?            # set for a rename or copy
  status: FileStatus                # ADDED | MODIFIED | DELETED | RENAMED | COPIED
  additions: Int                    # lines added
  deletions: Int                    # lines deleted
}
struct DiffRecord {
  commit: String                    # the commit the diff is of
  files: [FileDiff]
  truncated: Bool                   # some patch text was cut by the budget
}
struct FileDiff {
  path: String
  patch: String                     # unified diff text for this file
  truncated: Bool
}
```

`schemas/evidence.strata` defines the bundle:

```
struct Evidence {
  schema_version: String
  purpose: EvidencePurpose          # RELEASE
  range: Range
  base_version: String?             # absent for a line with no release
  commits: [CommitEvidence]         # oldest first
  sources: [AdapterRun]             # the adapters that contributed
  budget: BudgetReport
}
struct Range {
  from: String?                     # full hash; absent for a line with no release
  to: String                        # full hash of the head
}
struct CommitEvidence {
  record: CommitRecord              # hash, subject, body, trailers, author, files
  conventional: Conventional?       # the parsed conventional subject, where it is one
  pull_request: PullRequestInfo?    # the pull request the commit carries, where it carries one
  diff: DiffRecord?                 # absent WHEN the Diff Collector did not run
  signal: Bump?                     # the classifier's reading; absent when unclassified
}
struct Conventional {
  type: String                      # as written, e.g. `feat`
  scope: String?
  breaking: Bool                    # a `!` marker or a `BREAKING CHANGE:` footer
}
struct PullRequestInfo {
  number: Int
  title: String?
  description: String?
  origin: PullRequestOrigin         # SQUASH | MERGE | PLATFORM
}
struct BudgetReport {
  limit: Int                        # bytes of patch text permitted
  total: Int                        # bytes of patch text before the budget
  used: Int                         # bytes of patch text carried
  omitted: [OmittedDiff]
}
struct OmittedDiff {
  commit: String
  path: String
  reason: OmissionReason            # EXCLUDED | BINARY | OVER_BUDGET
  truncated: Bool                   # part of the patch was carried
}
```

A `PullRequestInfo` read from a squash commit (`SQUASH`) takes its number
from the subject's trailing ` (#<n>)`, its title from the subject without
it, and its description from the body. One read from a merge commit
(`MERGE`) takes its number from the subject and its title and description
from the body. `PLATFORM` marks a title or description the optional pull
request adapter fetched because the commit body lacked it.

`Bump`, `ChangeKind` and `AdapterRun` are the corpus schema's.

`EvidencePurpose` names what the bundle is evidence for. Its only variant is
`RELEASE`.

## Rendering

The Changelog Renderer renders a corpus under the repository's `changelog`
settings ([[frd-changelog-rendering]]):

- Entries appear under level-three headings in this order:
  `changelog.breaking_heading`, then each `changelog.groups` heading in
  configured order, then `changelog.other_heading`. A heading with no
  entries is left out.
- A breaking entry renders under the breaking heading, whatever its kind,
  with its `migration` beneath it. Any other entry renders under its kind's
  group heading. An entry with no kind, or a kind `ChangeKind` does not
  declare, renders under the other heading. An entry of a declared kind no
  group names does not render.
- Each entry is one list item: `- `, then `**<scope>:** ` WHEN a scope is
  set, then the summary, then its sources. The item carries no kind prefix
  and no breaking mark, because its heading gives both. `detail` follows as
  an indented paragraph.
- `omitted` never renders.

## Guarantees

WHEN an instance declares a `schema_version` a consumer does not
implement, the consumer SHALL reject the instance.

**Rationale:** Rendering an instance partially produces release notes
that silently omit content, which is worse than failing to render.

---

The provider SHALL render every published artifact describing one release
from a single corpus instance.

**Rationale:** Two artifacts rendered from one instance cannot
contradict each other. Independently generated release notes and
changelog sections can, and have.

---

An evidence bundle SHALL carry no impact other than the signals read from
git, no changelog entry and no `ChangeKind` the tooling assigned.

**Rationale:** The bundle is what git said. A kind or an impact guessed from
a commit subject would reach the producer as a fact ([[adr-003]]).

---

The provider SHALL include in an evidence bundle every commit in the range,
including commits that carry no signal and commits whose diff the budget
omitted.

---

The provider SHALL record in an evidence bundle's `budget` every diff or file
it excluded, truncated or omitted, and why.

**Rationale:** A producer that cannot see what it was not shown writes notes
as though it saw everything.

---

The provider SHALL reject an authored corpus in which a commit of the
evidence bundle is neither cited by an entry's `sources` nor listed in
`omitted`.

**Rationale:** Nothing in the range is dropped silently. A reviewer of the
stage pull request can see every commit either described or set aside with a
reason.

---

The provider SHALL reject an authored corpus in which an entry's `sources`
cite no commit, or in which an entry's `sources` or `omitted` cite a commit
or pull request absent from the evidence bundle it was authored from.

**Rationale:** A source outside the evidence is one the producer invented,
and a reviewer following it finds nothing.

---

The provider SHALL reject an authored corpus that lists in `omitted` a
commit an entry also cites, or that lists a commit in `omitted` with an
empty `reason`.

---

The provider SHALL ignore `proposed_bump` in an authored corpus, and SHALL
NOT derive a published version from a producer's output except through the
suggested bump [[component-bump-suggester]] derives from its entries, once a
person has confirmed it.

**Rationale:** The entries' kinds are the model's reading of the evidence,
and each one is visible and reviewable. A release-level impact a producer
states on its own is neither. The suggestion reaches a version only through
a confirmation recorded on the release commit ([[adr-003]],
[[frd-version-computation]]).

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

Producers SHALL write each entry's `summary` as one or two full sentences.

---

Producers other than the mechanical producer SHALL set `kind` on every
entry.

**Rationale:** The suggested bump is derived from the kinds. An entry with
no kind contributes nothing to it, and WHERE the suggestion is none it makes
the cut halt for a named bump, so only the producer that reads no diff may
leave a commit unclassified.

---

Producers SHALL set `migration` on every entry whose `breaking` flag is set,
and the provider SHALL reject an authored corpus with a breaking entry that
carries none.

**Rationale:** A breaking change without migration guidance is an incident
report rather than a release note. There is no separate collection for
breaking changes: an entry carries the flag and its own guidance, so the two
cannot be recorded apart from each other.

---

The provider SHALL record in an evidence bundle's `sources` every adapter
that contributed to it, with the range it read: `from` is the base commit's
full hash, absent WHEN the release line has no release yet, and `to` is the
head commit's full hash.

---

Producers SHALL copy the evidence bundle's `sources` into the corpus's
`sources` unchanged.

**Rationale:** Provenance a producer can rewrite says what the producer
claimed, not what the corpus was built from.

---

The mechanical producer SHALL set an entry's `derived` flag, because it
derives every entry's `summary` from a commit subject. Every other producer
SHALL clear it.

**Rationale:** A reviewer of the release notes can then find every line
nobody wrote.

---

The mechanical producer SHALL derive one entry from each commit's subject,
take its `kind` from the commit's conventional type, and leave `kind` unset
for a commit with no conventional type, which then renders under the other
heading.

**Rationale:** It exercises no judgement, so it classifies only what the
commit already says. The suggested bump over its output is the signal floor
alone ([[component-bump-suggester]]).

---

Consumers SHALL tolerate unknown fields within a compatible
`schema_version`.

## Operational Expectations

- **Availability / SLO:** not applicable — the corpus and the bundle are
  files, not services.
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

### Amendments within 1.0

Schema version 1.0 is amended in place until the tooling's first public
release. An instance written before an amendment may not validate. The
amendments are: `Changelog.sources` (with `AdapterRun` and `Adapter`) and
`ChangelogEntry.derived` as required fields; `Changelog.proposed_bump` as
optional; `Changelog.omitted`, `ChangelogEntry.kind` as optional, and
`AdapterRun.budget`; `Evidence.purpose` as required; and the semantics of [Corpus](#corpus). Every later
change follows the policy above.

## Key Decisions

- [[adr-003]] — a model authors the structured changelog from evidence read
  from git, the tooling suggests a bump from its entries' kinds, and a
  person confirms the bump before it becomes a version.
- [[adr-004]] — changelog authoring is a replaceable step, which is what
  requires a structured intermediate rather than a render template.

## Open Questions

- [ ] Whether `IssueSource` should carry a tracker discriminator rather than
  relying on the identifier's own form — decided when the first adopter wires
  a second tracker.

## References

- `shared/c4-standards.md` — C4 mapping rules and contract visibility tiers
- [[container-release-cli]] — provider's blueprint
- [[frd-changelog-authoring]] — the feature that builds evidence and
  exchanges it with producers
- [[frd-version-computation]] — how the suggested bump is derived and
  confirmed
- [[frd-changelog-rendering]] — the feature that consumes the corpus
- [[component-bump-suggester]] — derives the suggested bump from the
  corpus's entries
