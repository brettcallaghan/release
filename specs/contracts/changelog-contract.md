---
title: "Contract: Changelog Corpus"
description: The evidence a producer reads, the structured changelog it
  writes, and the pull request notes block, exchanged between producers that
  may vary and the deterministic tooling that validates and renders them.
slug: contract-changelog
type: contract
status: draft
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
has three parts:

- **The evidence bundle** — what a producer reads: the commits in a range,
  what git says about each, its reviewed notes block where it has one, and
  its diff where one was collected.
- **The corpus** — what a producer writes for a release: its proposed impact
  and the reasons for it, its narrative, and its itemized changes.
- **The `## Release notes` block** — what a producer writes for a pull
  request, as markdown a reviewer edits like prose, with a grammar the
  tooling parses when the squash merge has carried it into a commit body.

Provided by [[container-release-cli]]. Visibility: public.

Public rather than internal because any producer may read and write these:
the authoring skills vended to adopting repositories, a hand-edited file
when a release needs correcting, a person writing a notes block in a pull
request, or third-party tooling. The schemas and the block grammar are the
only things those producers share.

## Specification

| Artifact | Location | Normative for |
|----------|----------|---------------|
| Strata schema | `schemas/changelog.strata` | Shape of the corpus |
| Evidence schema | `schemas/evidence.strata` (new; lands with the implementation) | Shape of the evidence bundle and of pull request notes, as [Evidence Bundle](#evidence-bundle) states them |
| Collector records | `schemas/collection.strata` | Shape of the records the evidence carries |
| JSON Schema | generated: `strata build schemas/<name>.strata --target json-schema` | Shape, for validators |
| This document | — | Semantics, the notes block grammar, operations, change policy |

The Strata schemas are the single source for shape; the JSON Schema and
every language binding are generated from them and are never hand-edited.
Where this document states a shape the schemas do not yet carry, this
document is normative until they do.

## Corpus

An entry is a **change**, not a commit. Its `sources` cite the commits,
pull requests and issues that account for it, and the relation is
many-to-many: one commit may account for several entries, one entry may cite
several commits, and a commit that changed nothing a reader can observe may
be cited by no entry at all.

The corpus amends `schemas/changelog.strata` as follows:

- `Changelog.proposal_rationale: String?` — why the producer proposes
  `proposed_bump`, in a sentence or two a maintainer can confirm or reject.
- `Changelog.proposal_drivers: [ChangeSource]` — the sources of the changes
  that drove `proposed_bump`.
- `AdapterRun.budget: BudgetReport?` — on the run of the `DIFFS` adapter, the
  diff budget report of the evidence the corpus was authored from.
- `ChangelogEntry.summary` is one sentence stating the change from the
  reader's point of view. `ChangelogEntry.detail` is the expected home of a
  sentence or two explaining it, rendered beneath the summary.
- `ChangelogEntry.derived` is set only by the mechanical producer, for a
  summary it derived from a commit subject: nobody wrote the sentence.

## Evidence Bundle

The producer reads an `Evidence` instance. It is built by the tooling, never
by a producer, and states what git said without interpreting it.

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
  commit: String?                   # the commit, for a per-commit diff
  from: String?                     # the base, for a range diff
  to: String                        # the head
  files: [FileDiff]
  truncated: Bool                   # some patch text was cut by the budget
}
struct FileDiff {
  path: String
  patch: String                     # unified diff text for this file
  truncated: Bool
}
```

`schemas/evidence.strata` defines the bundle and pull request notes:

```
struct Evidence {
  schema_version: String
  purpose: EvidencePurpose          # PULL_REQUEST | RELEASE
  range: Range
  base_version: String?             # RELEASE only
  commits: [CommitEvidence]         # oldest first
  sources: [AdapterRun]             # the adapters that contributed
  budget: BudgetReport
}
struct Range {
  from: String?                     # full hash; absent for a line with no release
  to: String                        # full hash of the head
}
struct CommitEvidence {
  record: CommitRecord
  notes: PullRequestNotes?          # the parsed notes block, where present
  diff: DiffRecord?                 # every commit at PULL_REQUEST; at RELEASE,
                                    # a commit with no notes block
  signal: Bump?                     # the classifier's reading; absent when unclassified
}
struct BudgetReport {
  limit: Int                        # bytes of patch text permitted
  total: Int                        # bytes of patch text before the budget
  used: Int                         # bytes of patch text carried
  omitted: [OmittedDiff]
}
struct OmittedDiff {
  commit: String?
  path: String
  reason: OmissionReason            # EXCLUDED | BINARY | OVER_BUDGET
  truncated: Bool                   # part of the patch was carried
}
struct PullRequestNotes {           # also the pull request producer's output
  impact: Bump
  entries: [NoteEntry]
}
struct NoteEntry {
  kind: ChangeKind?                 # absent under the breaking and other headings
  scope: String?
  summary: String                   # one sentence, reader's point of view
  detail: String?                   # a sentence or two
  breaking: Bool
  migration: String?
}
```

`Bump`, `ChangeKind` and `AdapterRun` are the corpus schema's.

## Release Notes Block

A notes block is the markdown form of `PullRequestNotes`. It lives in a pull
request description, and under squash merging that uses the description, in
the commit body. Its entries render as the Changelog Renderer renders a
changelog section's body, so a reviewer sees the entries as the changelog
will show them. Example, under this repository's configured headings:

```markdown
## Release notes

<!-- release-notes: v1 head=3f2a9c1 -->
Release-Type: minor

### 🚀 Features

- **cut:** `release cut --dry-run` shows the changelog section the release commit would write.
- **cut:** `release cut` refuses a named bump over a range with nothing releasable unless `--force` is given.

  Pass `--force` to release anyway, for example for a rebuild.

### 🪲 Bugs

- **cut:** `release cut` checks `gh` authentication before writing anything.
```

The grammar, in order:

1. **Heading.** The block starts at a line reading exactly `## Release
   notes`.
2. **Marker.** The next non-blank line is
   `<!-- release-notes: v1 head=<hash> -->`, where `v1` is the grammar
   version and `<hash>` is the abbreviated or full hash of the pull request
   head the block was written for.
3. **Impact.** The next non-blank line is `Release-Type: <bump>`, where
   `<bump>` is one of `major`, `minor`, `patch` or `none`. It is the block's
   impact.
4. **Sections.** Entries appear under level-three headings,
   `### <heading>`, in the Changelog Renderer's order:
   `changelog.breaking_heading`, then each `changelog.groups` heading in
   configured order, then `changelog.other_heading`. A section with no
   entries is left out. The section gives each of its entries' kind:
   - under the breaking heading, an entry is breaking and has no `kind`;
   - under a group's heading, an entry has that group's `kind`;
   - under the other heading, an entry has no `kind`. `ChangeKind` has no
     value for other changes: the renderer puts there the kinds it does not
     recognise.
5. **Entries.** Each entry starts at a line at column zero reading `- `, an
   optional `**<scope>:** `, and the entry's one-sentence summary, which
   occupies the rest of the line. The line carries no type and no breaking
   mark, because its section gives both, and no source reference, because
   the block's source is its own pull request.
6. **Detail.** An entry may be followed by paragraphs indented by two
   spaces and separated from the entry and from each other by a blank line.
   Together they are the entry's detail. A blank line separates an entry
   carrying paragraphs from the next entry.
7. **Migration.** An indented paragraph beginning `Migration: ` is the
   entry's migration guidance. An entry has at most one, and it follows the
   detail paragraphs.
8. **End.** The block ends at the next line beginning `## `, or at the end
   of the text. Lines after it, such as `Co-authored-by:` trailers, are not
   part of it.

A parser maps a section heading to its meaning through the repository's
changelog configuration. It also accepts the default headings,
`Breaking changes`, `Features`, `Fixes`, `Performance`, `Reverts` and
`Other changes`, with the meanings the default configuration gives them, so
a block in history still parses after the repository changes its headings.
A configured heading takes precedence over a default heading it equals.

### Block guarantees

The provider SHALL accept a block whose lines end in LF or CRLF.

**Rationale:** Hosting platforms store pull request descriptions with CRLF
line endings, and the squash merge carries them into the commit body.

---

IF a text holds a `## Release notes` heading and the lines up to the block's
end do not follow the grammar, THEN the provider SHALL report the block as
malformed and name the first line that does not.

---

IF a text holds more than one `## Release notes` heading, THEN the provider
SHALL report the block as malformed.

---

IF a section heading in a block is neither a heading the repository's
changelog configuration declares nor a default heading, THEN the provider
SHALL report the block as malformed and name the heading.

**Rationale:** An unrecognised heading gives its entries no kind, and a
guessed kind would reach the changelog as reviewed.

---

WHEN the provider parses an entry, it SHALL take the entry's kind and
breaking flag from the section the entry is under, and from nothing else.

---

IF an entry under the breaking heading carries no `Migration:` paragraph,
THEN the provider SHALL report the block as malformed.

---

IF a block has an entry under the breaking heading and its impact is below
`major`, THEN the provider SHALL report the block as malformed.

**Rationale:** The impact line is the block's only signal. A breaking entry
under a lower impact would let a reviewed block understate its own content.

---

The provider SHALL render `PullRequestNotes`, under a repository's changelog
settings, into a block that parses back under the same settings to the same
`PullRequestNotes`, and SHALL render identical notes and settings
identically on every run.

---

A producer SHALL write into `PullRequestNotes` only entries the repository's
changelog renders: breaking entries, and entries whose kind a
`changelog.groups` entry names. Work of any other kind, such as `CHORE`,
`CI` or `TEST`, SHALL be left out of the block.

**Rationale:** The block shows what the changelog will show. An entry the
changelog drops would be reviewed and then never published.

---

The provider SHALL read only the block's impact line as its impact signal.

**Rationale:** One line a reviewer can see and change is the signal. The
entries' sections describe the changes, and the guarantee
above keeps them consistent with it.

---

WHEN the provider replaces a block in a pull request description, it SHALL
leave every line outside the block unchanged.

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

An evidence bundle SHALL carry no proposed impact, no changelog entry and no
`ChangeKind` the tooling assigned.

**Rationale:** The bundle is what git said. A kind or an impact guessed from
a commit subject would reach the producer as a fact ([[adr-003]]).

---

The provider SHALL record in an evidence bundle's `budget` every diff or file
it excluded, truncated or omitted, and why.

**Rationale:** A producer that cannot see what it was not shown writes notes
as though it saw everything.

---

The provider SHALL reject an authored corpus that carries no
`proposed_bump`, or a `proposed_bump` with no `proposal_rationale`.

---

The provider SHALL reject an authored corpus in which an entry's `sources`
or the `proposal_drivers` cite a pull request or commit absent from the
evidence bundle it was authored from.

**Rationale:** A source outside the evidence is one the producer invented,
and a reviewer following it finds nothing.

---

The provider SHALL treat `proposed_bump` as a proposal and SHALL NOT derive a
published version from it unless a person has confirmed it.

**Rationale:** The proposal is a model's reading of the evidence. It reaches
a version only through a confirmation recorded on the release commit
([[adr-003]], [[frd-version-computation]]).

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

Producers SHALL write each entry's `summary` as one sentence.

---

Producers SHALL set `migration` on every entry whose `breaking` flag is set.

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

The mechanical producer SHALL set an entry's `derived` flag WHEN it derived
the entry's `summary` from a commit subject, and SHALL clear it WHEN the
summary is prose carried from a notes block. Every other producer SHALL
clear it.

**Rationale:** A reviewer of the release notes can then find every line
nobody wrote.

---

The mechanical producer SHALL propose the impact computed from the reviewed
signals in the range, with the commits that determined it as
`proposal_drivers`.

**Rationale:** It exercises no judgement, so it has nothing to propose beyond
what the signals already say; where commits are unclassified, reconciliation
still asks a person ([[frd-version-computation]]).

---

Consumers SHALL tolerate unknown fields within a compatible
`schema_version`.

## Operational Expectations

- **Availability / SLO:** not applicable — the corpus, the bundle and the
  block are files and text, not services.
- **Latency:** rendering SHALL complete without network access.
- **AuthN/AuthZ:** not applicable.
- **Consumer obligations:** consumers SHALL validate an instance against
  the generated JSON Schema before rendering, and SHALL NOT infer meaning
  from field order.
- **Producer obligations:** producers SHALL write only fields the schema
  declares, and SHALL NOT embed rendered markdown in `summary`.

## Versioning and Change Policy

- **Versioning scheme:** semver on `schema_version`, carried in every
  instance, and the `v1` grammar version in every notes block marker.
- **Compatible changes:** the provider MAY add optional fields and
  MAY add enum variants without a major version change.
- **Breaking changes:** WHEN a required field is added, removed or
  re-typed, the provider SHALL increment the major `schema_version`. WHEN a
  block the current grammar accepts would no longer parse, the provider SHALL
  increment the grammar version and SHALL continue to parse every earlier
  version, because blocks in git history cannot be rewritten.
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
optional; `Changelog.proposal_rationale`, `Changelog.proposal_drivers` and
`AdapterRun.budget`; and the semantics of [Corpus](#corpus). Every later
change follows the policy above.

## Key Decisions

- [[adr-003]] — the producer authors notes and proposes impact from
  evidence; reviewed records in git decide the version, and a person
  confirms anything unreviewed.
- [[adr-004]] — changelog authoring is a replaceable step, which is what
  requires a structured intermediate rather than a render template.

## Open Questions

- [ ] Whether `IssueSource` should carry a tracker discriminator rather than
  relying on the identifier's own form — decided when the first adopter wires
  a second tracker.
- [ ] Whether a notes block written by hand, with no pull request head to
  record, may omit `head=` — decided with the `release notes --check`
  implementation.

## References

- `shared/c4-standards.md` — C4 mapping rules and contract visibility tiers
- [[container-release-cli]] — provider's blueprint
- [[frd-changelog-authoring]] — the feature that builds evidence and
  exchanges it with producers
- [[frd-version-computation]] — how the proposal and the block's impact are
  reconciled
- [[frd-changelog-rendering]] — the feature that consumes the corpus
- [[repository-release-policy]] — how the notes block reaches git
