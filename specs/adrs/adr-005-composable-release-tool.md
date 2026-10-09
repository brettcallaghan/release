---
title: "ADR-005: A Composable Library With a Thin CLI"
description: The release tooling is built as small, independently testable
  components behind a thin command surface, with artifact construction and
  registry publication left to adopting repositories.
slug: adr-005
type: adr
status: approved
updated_at: 2026-10-07T00:00:00Z
---

# ADR-005: A Composable Library With a Thin CLI

**Status:** approved
**Date:** 2026-10-06

## Context

[[adr-004]] establishes that no existing tool is adopted. This decision is
about the shape of what replaces it.

Three things constrain that shape. The tooling is branch-agnostic
([[adr-001]]), so it cannot be organised around a topology. A release must
exist as a reviewable artifact before publication ([[adr-002]]), so the
pipeline has a mandatory human checkpoint in the middle of it. And the
correctness-critical part — deciding a version number — is the one part that
cannot be corrected after the fact, so it has to be exhaustively testable
without a repository, a network, or a model.

Two further constraints come from how this will actually be used. Adopting
repositories already own their build toolchains and registry credentials, and
they differ: a Go module, a Cargo and npm monorepo, an editor extension.
And changelog authoring needs a language model, which is available on a
maintainer's workstation and would otherwise oblige every adopter to hold an
API credential and a per-call budget.

## Decision

Build a library of small, independently testable components behind a thin
command surface. Version computation, bump classification, changelog
assembly and rendering are pure functions of their inputs. Reading git and
the hosting platform, and writing branches and files, are separate components.

Artifact construction and registry publication are out of scope: adopting
repositories own those workflows. Language models run outside the tooling,
which exchanges a changelog corpus as a file and never calls one.

Impact signals are read from git — a release-type trailer, a
conventional-commit type, a breaking-change marker — so that a version is
reproducible from a clone. Labels on the hosting platform are not an
accepted primary signal.

## Alternatives Considered

| Option | Pros | Cons | Why rejected |
|--------|------|------|--------------|
| A monolithic release CLI owning the whole pipeline, publication included | One tool to install and configure<br>Adopters write no workflow<br>Matches what Auto offers | Publication is ecosystem-specific, so the tool would carry a plugin system for Cargo, npm, Homebrew, editor marketplaces and container registries<br>Requires registry credentials, and therefore becomes a credential-handling component<br>Duplicates what goreleaser, cargo-dist and existing repository workflows already do | The publication surface is larger than everything else combined and is already solved per ecosystem; owning it would dominate the tool's maintenance for no differentiating gain |
| Reusable platform workflows only, with no binary | Nothing to install or distribute<br>Trivially adoptable by any repository on the same hosting platform | Version computation cannot be unit-tested in workflow YAML, and it is the part where correctness matters most<br>Locks every adopter to one hosting platform<br>Logic becomes unrunnable on a workstation, where staging happens | The correctness-critical logic would be the least tested code in the system |
| A skill only, with no code | No tool to maintain<br>Changelog authoring is already a skill<br>Fastest to a working release | A version number produced by a model is not reproducible from the repository, which [[adr-003]] forbids<br>No deterministic verification of a release commit is possible | A version is the one output that cannot be corrected after publication, so it cannot depend on model sampling |
| A composable library behind a thin CLI, publication left to adopters | The correctness-critical components are pure and exhaustively testable<br>Adopters keep the build workflows and credentials they already have<br>Components can be recombined into a different release policy without forking<br>No model credential, no registry credential | A tool to maintain<br>Adopters write more workflow than a plugin ecosystem would require<br>Language-agnostic packaging — target matrices, checksum manifests, installer and formula generation — is deferred rather than designed | N/A — selected |

## Consequences

**Positive:**
- The components that decide a version are pure functions, so they can be
  tested exhaustively over the cross product of their inputs rather than
  through fixture repositories.
- Granularity is what makes a branch-agnostic tool useful: a project with a
  different release policy recombines the same primitives instead of
  configuring its way around opinions it does not share.
- No adopter needs an API credential to cut a release, because authoring
  happens where a model already exists.
- A version is derivable from a clone, offline, and checkable by a reviewer
  who has nothing but the repository.

**Negative:**
- Adopting repositories carry the artifact and publication workflow
  themselves, and the obligations in [[cli-distribution-policy]] fall on
  them rather than on the tool. Early adoption therefore costs more workflow
  authoring than installing a plugin would.
- Staging requires a workstation, because the authoring exchange needs a
  model that is not available in continuous delivery. An unattended release
  path does not exist and is not specified.
- Labels on the hosting platform are unavailable as the primary impact
  signal, so adopters who prefer labels must carry the signal into the commit
  instead.
- Splitting one pipeline into many components costs more interface surface
  than a single procedure would, and every seam is somewhere a future change
  has to stay consistent across two documents.

## References

- [[adr-001]] — the tooling is branch-agnostic, which is why composability
  rather than configuration is the flexibility mechanism
- [[adr-002]] — the reviewable release artifact the pipeline is built around
- [[adr-003]] — a version recomputable from git through the confirmed bump
  on the release commit, which sets the testability bar these components meet
- [[adr-004]] — why no existing tool is adopted
- [[release-tooling-standards]] — the requirements this shape must satisfy
- [[container-release-cli]] — the container this decision describes
