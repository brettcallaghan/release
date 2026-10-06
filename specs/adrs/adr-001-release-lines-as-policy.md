---
title: "ADR-001: Release Lines Are Policy, Not Tooling"
description: The tooling accepts any branch as a release line and does not
  interpret branch names; which branches carry which majors is a choice each
  project makes and maintains.
slug: adr-001
type: adr
status: draft
updated_at: 2026-10-06T00:00:00Z
---

# ADR-001: Release Lines Are Policy, Not Tooling

**Status:** draft
**Date:** 2026-10-06

## Context

A project releasing more than one major version needs somewhere to develop
the next one and somewhere to maintain the previous one. There are several
workable layouts, and which is right depends on how many majors a project
supports, how often it releases, and whether its consumers pin versions.

The question this decision answers is not which layout is best. It is
whether the tooling should know about layouts at all. A tool that encodes
one makes every adopter's branch structure a tooling concern — and makes a
project with a different structure unable to use it without changing how it
works.

The repositories adopting this model have different needs already: one is
pre-1.0 with a single major, another has no releases yet. Neither is served
by a tool with opinions about maintenance branches.

## Decision

The tooling is branch-agnostic. It accepts a target branch, does not
interpret its name, and enforces no relationship between branches and major
versions.

A project's recommended practice — the default branch carries the current
major, a dedicated `release/vN` branch carries a superseded one — is
documented as policy in [[repository-release-policy]] and is not enforced by
the tool. A project may release any major from any branch, and takes on
maintaining whatever branches it chooses.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| Tooling enforces trunk for the current major and `release/vN` for superseded ones | One layout, enforced, so a project cannot drift from it<br>The tool can refuse operations that violate the topology | Imposes a layout on every adopter<br>A project with any other structure cannot use the tool<br>Requires the tool to parse branch names and infer majors from them | The adopting repositories do not share one layout, and the capabilities this would enforce — retirement, support windows, propagation — are not needed by either of them yet |
| Tooling enforces a branch per minor version | Any historical minor can be patched | Many live branches<br>Back-port matrix grows with every minor | No consumer is expected to pin a minor, and the enforcement objection above applies with more force |
| Tooling is branch-agnostic with no documented practice | Maximum flexibility<br>Smallest tool | Every adopting project invents a layout, and the first one to need a maintenance branch gets no guidance | Flexibility without guidance is how three repositories end up with three incompatible release processes |
| Tooling is branch-agnostic; recommended practice is documented as policy | Adopters keep the layout they have<br>The tool stays small and its components stay composable<br>Projects that want the recommended layout have it written down | Nothing prevents a project from adopting an unworkable layout<br>Capabilities that depend on topology cannot be built into the tool | N/A — selected |

## Consequences

**Positive:**
- An adopting project keeps the branch structure it already has, and
  adoption costs no restructuring.
- The tool has no branch-name parsing, no major inference, and no topology
  model, which removes a whole class of configuration and a whole class of
  bug.
- Version computation rests on tag reachability alone, which is mechanical
  rather than conventional: a branch cut before a later tag existed cannot
  reach it, so a query scoped to a branch is already scoped to its line.
- The components stay composable. A project that wants a different policy
  assembles the same primitives differently rather than fighting a tool
  that disagrees with it.

**Negative:**
- Nothing stops a project from releasing two majors from one branch, or one
  major from two. The policy document says not to; the tool will do it.
- Capabilities that depend on knowing the topology — retiring a major,
  enforcing a support window, auditing unpropagated fixes — cannot live in
  the tool as specified. They become project practice, or tooling built
  later against a declared topology.
- A project adopting the recommended practice gets no enforcement of it, so
  branch protection and continuous delivery configuration carry that weight
  instead.

## References

- [[adr-002]] — how a release is triggered on a branch the tool knows
  nothing about
- [[release-tooling-standards]] — the branch-agnostic requirements this
  decision produces
- [[repository-release-policy]] — the recommended practice this decision
  moves out of the tool
