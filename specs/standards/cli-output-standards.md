---
title: Release CLI Output Standards
description: What every release command writes to standard output — YAML
  by default and JSON under --json, the answer alone, a document payload
  as itself, and the style rules that make both readable
slug: cli-output-standards
type: standards
status: approved
---

# Release CLI Output Standards

These standards govern what every command [[container-release-cli]]
exposes writes to standard output. They apply to every subcommand, present
and future, because what a command prints is the command surface's
business rather than any one subcommand's. [[contract-release-cli]] owns
which streams carry what and what each exit status means; these standards
own the shape of what standard output carries.

**The reader knows which command it ran and nothing else.** It may be a
maintainer at a terminal, a continuous-delivery job, or an agent driving
the tool from a skill. None of them holds this document, so the output
must need no key.

**The method is to stop inventing a format.** Structured output is YAML,
so a reader who has met YAML once can read it and a consumer already has
a parser for it. There is no line grammar to reverse-engineer, no column
convention, and no punctuation carrying meaning.

The same rules govern the Spec CLI and the swarm CLI. They are restated
here rather than cited because a corpus cannot reference a document in
another workspace.

---

## Renderings

### YAML by default, JSON under `--json`

WHERE a command's answer is structured, the CLI SHALL render it as YAML,
and SHALL render it as JSON when `--json` is passed.

**Rationale:** two renderings, one flag, no third form. YAML is the
default because a person and an agent both read it without a schema, and
because this CLI's values routinely carry multi-line text — a rendered
changelog section, a release note, a refusal's explanation — which
survives YAML intact instead of arriving as `\n` inside a quoted string.
JSON stays available so a consumer that already parses it need not
acquire a YAML parser.

---

Both renderings SHALL carry the same values in the same order.

**Rationale:** `--json` selects a syntax, never a depth or a subset. A
caller comparing the two forms is comparing two spellings of one answer.

---

Standard output SHALL carry no styling: no colour, no terminal control
sequence, and no decoration that is not part of a value.

**Rationale:** a styled answer is a different byte sequence from the one
a pipe receives, so the two would disagree, and an escape sequence is a
control character the [YAML style](#yaml-style) rules would have to
quote. Styling belongs to diagnostics on standard error, which
`--no-color` governs.

---

### A document payload is written as itself

WHERE a command's answer is a document rather than a set of properties,
the CLI SHALL write that document alone, with no envelope, no added
heading, and no trailing summary.

**Rationale:** wrapping a document in a mapping makes the caller unwrap
it, and quoting it into a scalar changes the bytes they asked for. The
test is what the caller does next: text they will write to a file or read
as prose is a payload; anything they would index by name is an answer.
`--help` and `--version` are payloads — usage text and the tool's own
version string — and `testdata/help/` already holds the help text byte
for byte.

---

### An artifact is not an answer

WHERE a command writes an artifact whose format another document
specifies, that format SHALL govern the artifact, and these standards
SHALL govern only what the command prints about it.

**Rationale:** the changelog corpus is shaped by [[contract-changelog]]
and the changelog file by the repository that owns it. What `release
changelog --write` *prints* about the section it wrote is an answer and
follows the rules above.

---

## The Answer

### The answer is the properties the command established

A command's answer SHALL be a YAML mapping whose keys are the properties
the command established and whose values are those properties' values,
and SHALL contain nothing else.

**Rationale:** "nothing else" is doing the work. No header naming the
command — the caller typed it. No count of what follows — the reader can
see the collection. No summary — a summary of a mapping in view is a
second copy of it. No outcome line — the exit status carries the outcome,
and a process printing its own exit status tells the caller what they
already hold.

---

WHERE a command exits non-zero having established properties, the CLI
SHALL render them as its answer.

**Rationale:** a refusal still knows things. `release version` refusing
an impact disagreement has computed both impacts, and `release cut`
exiting 3 owes the caller every change it made. Each is an answer in the
same shape as a success, so the caller parses one form and branches on
the exit status.

---

### A collection is a sequence of mappings, never formatted lines

WHERE an answer carries several of a thing, the CLI SHALL render them as
a sequence of mappings with one key per field.

**Rationale:** `abc1234  feat: add --target` is a format a reader must be
told about and a consumer must split on whitespace nothing enforces. A
mapping per change names every field it carries, and the fields a later
feature adds arrive as new keys rather than as a changed grammar.

---

### An empty answer is an answer

WHERE a query matches nothing, the CLI SHALL render the empty collection.

**Rationale:** nothing found is a fact about the repository. Whether that
fact is also a refusal — nothing to release — is the command's decision,
stated in [[contract-release-cli]] and carried by the exit status, never
by leaving the answer out.

---

### Order is the command's, and it is stable

Properties SHALL appear in the order the command established them, and a
collection SHALL appear in the order the command reports it.

**Rationale:** one ordering, chosen by the command, so two runs over
unchanged state produce byte-identical output and a diff of two runs is a
diff of what changed. Snapshot cases depend on it.

---

## Nothing Is Shortened

A command SHALL render every value it reports in full.

A command SHALL NOT truncate a value, SHALL NOT elide one behind a
description of it, and SHALL NOT answer with an instruction to run the
same command again differently.

**Rationale:** a line budget protects a terminal width nobody is looking
at. Worse, every form of shortening turns one invocation into two — and
the second costs a round trip, may observe changed state, and can only be
composed by a caller who decoded the elision notice.

---

WHERE a command's design says its output belongs in a file, the command
SHALL write the file and report a property whose name ends `_path` and
whose value is that file's path.

**Rationale:** a per-command design decision, taken when the command is
specified, and never a threshold that engages when a value grows large. A
dynamic rule would make one invocation's shape depend on what it happened
to find, so a caller could not know whether they held a value or a path
without checking.

---

## YAML Style

Valid YAML is not the bar; readable YAML is. Each rule below removes a
choice, so that two renderings of one value are the same bytes.

The output SHALL use block style throughout, indented two spaces per
level, with a sequence indented one level under the key introducing it.

---

An empty sequence SHALL be rendered `[]` and an empty mapping `{}`.

**Rationale:** the one exception to block style, and it is forced. A
block rendering of an empty collection is a key with nothing after it,
which reads as a key whose value is null — so an empty list, an empty
string and an unset key would be indistinguishable.

---

A multi-line string SHALL be trimmed of leading and trailing whitespace
and rendered as a block scalar. Whitespace inside the value SHALL NOT be
touched.

**Rationale:** the block scalar is the reason for choosing YAML — the
value appears as itself, on its own lines. Trimming keeps the block form
always available: a first line indented past the block's own indentation
needs an explicit indicator to survive, and a last line ending in
whitespace is read back differently by different parsers.

---

WHERE a block scalar cannot carry a multi-line string faithfully, the CLI
SHALL double-quote it instead. It cannot when the value carries a control
character other than a line break or a tab.

**Rationale:** correctness outranks readability. A block's only structure
is the line break, so a carriage return folds into the break beside it
and the value comes back shorter than it went in.

---

A string SHALL be rendered plain where it round-trips plain, and
double-quoted otherwise. It does not round-trip plain when it is empty,
has leading or trailing whitespace, begins with a YAML indicator
character, contains a control character, contains `: ` or ` #`, ends with
`:`, or reads as a number, a boolean or a null.

**Rationale:** quoting everything is safe and unreadable; quoting nothing
is readable and wrong. This is the exact set of cases where a plain scalar
parses back as something other than what was written. A version such as
`1.10` reads as a number and is therefore quoted; `1.10.0` is not.

---

Values SHALL be rendered as their own types: a number unquoted, a boolean
as `true` or `false`, an absent value as `null`.

---

The output SHALL NOT contain comments, anchors, aliases, explicit tags, a
document-start marker, or a document-end marker.

**Rationale:** each is a feature a reader must resolve before knowing
what the document says. An alias in particular defines a value somewhere
other than where it is read.

---

The output SHALL NOT wrap or fold a long line.

**Rationale:** folding changes the bytes of the value it carries, and any
wrapping decision depends on a terminal width that is not a property of
the data.

---

## What This Surface Does Not Adopt

Stated because the absence is a decision rather than an omission.

- **No result envelope and no `--full-result`.** The answer is the whole
  of standard output. `release cut --dry-run` reports its intended actions
  as its answer rather than as a layer beneath one. WHEN a command
  acquires a provenance layer a caller would audit separately, it acquires
  the flag with it.
- **No per-property verdict shape.** A refusal is carried by the exit
  status and explained by the properties the command established, not by
  a second vocabulary of verdicts beside them.

---

## References

- [[container-release-cli]] — the container whose command surface these
  standards govern; its Output Renderer applies them
- [[contract-release-cli]] — the streams, flags and exit statuses these
  standards assume
- [[contract-changelog]] — the corpus artifact these standards defer to
