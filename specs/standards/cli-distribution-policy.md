---
title: CLI Distribution Policy
description: How a project publishes and withdraws a release — declared
  artifacts, major-aware version resolution, integrity and provenance,
  channel coverage, and channel state.
slug: cli-distribution-policy
type: standards
status: draft
updated_at: 2026-10-06T00:00:00Z
version: "0.2"
---

# CLI Distribution Policy

How a released version reaches the people who install it. These are a
project's obligations, not the release tooling's: artifact construction and
registry publication stay with each adopting repository, which already owns
its toolchains and its registry credentials ([[adr-005]]).

The requirements are written against "the project" for that reason. In
practice they are discharged by a workflow the project maintains, invoked
after a release is tagged.

They begin at the tag. The branching and versioning rules that produce it
are in [[release-tooling-standards]] and [[repository-release-policy]].

Requirements carry stable identifiers so that tests, blueprints and reviews
can cite them individually. An identifier is never renumbered once assigned,
and a retired identifier is never reused: nothing validates a requirement
citation, so renumbering breaks references silently.

---

## Publication

**CDP-001:** A project SHALL declare the set of artifacts a release
publishes.

**Rationale:** Publication spans registries, archives and third-party
formulae that fail independently. Without a declared set there is no
definition of a complete release and no way to detect a partial one.

---

**CDP-002:** A project SHALL publish every declared artifact from a single
tag.

---

**CDP-003:** IF publication of a declared artifact fails, THEN the project
SHALL report which artifacts published and which did not.

**Rationale:** Partial publication is the expected failure, not an
exceptional one, and most publication steps are not idempotent. An operator
recovering from one needs the current state before retrying.

---

**CDP-004:** A project SHALL make every publication step re-runnable
without manual cleanup after a partial failure.

---

## Channel State

**CDP-005:** A project SHALL record, for each declared artifact of a
release, whether it is pending, published or failed.

**Rationale:** Third-party channels have review queues measured in days.
Without a pending state, an artifact awaiting review is indistinguishable
from one that failed, and a release looks incomplete for as long as the
queue takes.

---

**CDP-006:** A project SHALL NOT report a release as fully published WHILE
any declared artifact is pending.

---

## Version Resolution

**CDP-007:** An installer SHALL resolve a requested major version to the
latest supported release of that major.

---

**CDP-008:** An installer SHALL NOT resolve a requested major version to a
tag belonging to a different major.

**Rationale:** Supported majors release independently, so a maintenance
release of a superseded major can carry a later timestamp than the current
major's newest release. Resolving by recency would downgrade users of the
current major.

---

**CDP-009:** An installer SHALL NOT resolve a requested version to a
prerelease or a canary build unless one is explicitly requested.

---

**CDP-010:** WHERE a distribution channel holds only one current version,
the project SHALL publish the current major to that channel.

---

## Integrity and Provenance

**CDP-011:** A project SHALL publish a checksum for every binary artifact.

---

**CDP-012:** A project SHALL sign every release tag.

---

**CDP-013:** A project SHALL attach build provenance attestation to every
binary artifact.

---

**CDP-014:** An install script SHALL verify the checksum of a downloaded
artifact before executing it.

**Rationale:** An install script is commonly piped directly into a shell, so
it runs before the user can inspect anything. Verification inside the script
is the only check that happens at all.

---

**CDP-015:** A project SHALL restrict publication to workflows running in
the repository that owns the release.

**Rationale:** A workflow triggered from a fork must never reach a
publication credential.

---

## Channels

**CDP-016:** WHERE a channel supports distribution outside the stable
series, a project MAY publish builds from a release line to a prerelease or
canary channel.

---

**CDP-017:** A project SHALL publish to a prerelease or canary channel
using the same publication path as a stable release.

**Rationale:** Exercising the publication path continuously is the main
reason such a channel earns its keep — it is the part most likely to rot
between releases, because it only runs at release time and touches
credentials, signing and several registries. A separate workflow would
validate only itself.

---

**CDP-018:** A project SHALL NOT publish a canary build to a registry that
has no concept of a channel.

**Rationale:** Publishing there consumes a real version number in the
stable series for a build that was never released.

---

**CDP-019:** A project SHALL document which artifacts a prerelease or
canary channel covers.

**Rationale:** Coverage is necessarily partial, and a channel that silently
omits the primary artifact rehearses less of the publication path than it
appears to.

---

## Withdrawal

**CDP-020:** A project SHALL be able to withdraw a published version from
every channel that supports withdrawal.

---

**CDP-021:** WHEN a published version is withdrawn, the project SHALL
retain its tag.

---

**CDP-022:** WHEN a published version is withdrawn, the project SHALL
report every channel that does not support withdrawal and still carries it.

---

## References

- [[release-tooling-standards]] — what the tooling enforces
- [[repository-release-policy]] — the branching and support policy that
  produces a release
- [[adr-005]] — publication stays with adopting repositories
