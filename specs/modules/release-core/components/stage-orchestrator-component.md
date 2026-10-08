---
title: "Component Blueprint: Stage Orchestrator"
entity_name: Stage Orchestrator
description: The sequence that cuts a release — producing a stage branch and
  its pull request — and the only component that writes to a repository or a
  hosting platform.
slug: component-stage-orchestrator
type: component-blueprint
status: approved
updated_at: 2026-10-08T00:00:00Z
kind: standard
contracts_consumed:
  - contract-changelog
source_paths:
  - internal/cut
test_paths:
  - internal/cut
---

# Stage Orchestrator

## Overview

Runs the sequence behind `release cut`: resolve the base version, resolve the
impact, compute the next version, build the corpus, author it, render, write
the changelog, propagate the version across the repository's manifests,
commit, branch, and open the pull request. It holds no logic of its own beyond
ordering and failure handling.

The operation is called cutting a release, which is why the command reads
`release cut minor` and the default branch prefix is `cut/`. The documents
keep calling the branch a *stage branch* — that is the concept, whatever a
project names the prefix ([[adr-002]]).

Two paths through it. With a bump named on the command line, the impact is
settled before anything runs. Without one, the authored changelog supplies it
and [[component-bump-classifier]] resolves it from the entries. Either way
authoring can be skipped outright, which is the path for a release whose notes
are not worth a model's time.

It is the only component that writes. Everything upstream reads or computes,
so a failure part-way leaves state this component is responsible for
describing — and, where it can, for not creating. Every check that can fail is
performed before the first write.

Version propagation is delegated. Which files carry a version differs by
ecosystem, so the repository configures a command and this component runs it,
rather than the tooling learning any manifest format.

- **Role:** the cut sequence, and the only write surface.
- **Scope:** in — ordering, pre-flight checks, the commit, the branch, the
  pull request; out — every computation, which belongs to the components it
  calls, and verifying the merged result, which is the project's required
  checks.
- **Composed into:** [[module-release-core]] — depends on every other
  component; see the module for relationships.
- **Source & tests:** `internal/cut`.

## Interface

- **Surface:** given a target branch, an optional named bump, and a policy,
  produce a stage branch and open its pull request, or report why it did not.
- **Guarantees:**
  - The component SHALL perform every check that can fail before making any
    write.
  - WHEN the maintainer names a bump, the component SHALL use it rather than
    inferring one.
  - WHEN the maintainer requests staging without notes, the component SHALL
    skip the authoring exchange and proceed with a mechanically assembled
    corpus.
  - IF staging without notes is requested and no bump is named, THEN the
    component SHALL refuse, because neither source of impact is available.
  - The component SHALL create a stage branch whose first commit is the
    release commit.
  - The component SHALL run the repository's configured version-propagation
    command to write the version into its manifests.
  - The component SHALL confine its writes to the paths the project declared
    as release paths.
  - The component SHALL title the pull request for the version being released.
  - The component SHALL include the rendered release notes in the pull request
    body under a "Release Notes" heading.
  - WHEN no releasable change exists since the latest reachable tag, the
    component SHALL refuse without writing.
  - WHEN the authored proposal disagrees with the resolved impact, the
    component SHALL halt without writing.
  - IF a write fails part-way, THEN the component SHALL report every change it
    made.
  - The component SHALL NOT tag, publish, or merge.
  - The component SHALL NOT modify the target branch.
  - The component SHALL NOT require the stage branch to hold only one commit
    after it is created.

- **Writes:** a write is creating or moving a ref, creating a commit,
  pushing, or opening a pull request. Files changed in the scratch checkout
  are not writes, and neither is the fetch refreshing the remote-tracking
  ref `origin/<target>` and the tags that follow it, which every run makes,
  dry runs and refusals included.
- **Sequence:** the component first runs `git fetch origin <target>` and pins
  the release to that `origin/<target>` commit; the version, the commit range
  and the scratch checkout (a temporary detached `git worktree` at the pinned
  commit, removed on every exit) all use it, and the maintainer's working
  tree, index and current branch are never read for the release or changed.
  Refusals that need no repository come first: staging without notes and
  without a bump, an unconfigured `commands.propagate`, and, unless the run
  is a dry run or opens no pull request, a missing `gh`. Then, stopping at the
  first refusal or failure:
  1. resolve the version, with the named bump WHEN given; refuse WHEN no
     release is warranted;
  2. refuse WHEN the stage branch exists locally or on `origin`;
  3. run `commands.preflight` WHEN configured;
  4. assemble the corpus and, unless notes are skipped, author it; halt WHEN
     the proposal disagrees and no bump was named;
  5. set the corpus `version` and render it;
  6. in the scratch checkout, write the changelog section, then run
     `commands.propagate`;
  7. refuse WHEN a changed path is neither `CHANGELOG.md` nor matched by
     `release_paths` (equal to an entry, under an entry ending in `/`, or
     matched by an entry as a glob);
  8. run `commands.validate` WHEN configured, then repeat step 7's check,
     and refuse WHEN git has no committer identity;
  9. write: create the stage branch at the pinned commit, commit the changes
     as `Release <version>`, push the branch to `origin`, and open the pull
     request into the target through `gh`, titled `Release <version>` with
     the release body under `## Release Notes`, unless told not to.
- **Configured commands and the changelog producer** run in the scratch
  checkout with `RELEASE_VERSION` set to the version being released. The
  component's own git operations there (creating the checkout, the branch
  and the commit) run with no repository hooks, so the commit is exactly
  what was checked; a `CHANGELOG.md` that is a symbolic link is refused.
- **Push:** the stage branch is pushed with a lease requiring it to be
  absent on `origin`, so a branch created there since step 2 is never
  moved.
- **Partial writes:** IF the push fails, THEN the component deletes the local
  stage branch it created and reports both. IF opening the pull request
  fails, THEN it keeps the pushed branch, reports every write, and names the
  commands that finish (`gh pr create`) or discard
  (`git push origin --delete <branch>`, `git branch -D <branch>`) the
  release. It never deletes or changes anything on `origin`.
- **Dry run:** steps 1 to 8 run, and step 9's writes are reported, not made.

**Rationale for the last guarantee:** a maintainer correcting the notes during
review adds a commit, and a tool that treated that as malformed would make the
review surface unusable for the one thing it is for.

## Cross-Cutting Standards

This component SHALL comply with:

- [[release-tooling-standards]] — stage-branch shape, release-commit shape,
  and the refusal conditions.

## Change Guidance

WHEN a new failure condition is introduced, it SHALL be checked before the
first write if it can be.

**Rationale:** A condition checked after the commit exists turns a clean
refusal into a partial state a maintainer has to clean up by hand.

---

The component SHALL NOT modify the target release branch.

**Rationale:** Not modifying a release branch outside a reviewed pull request
is the principle the whole design rests on ([[adr-002]]). This is the one
component with the access to violate it.

---

The component SHALL NOT implement a computation that belongs to a component it
calls.

**Rationale:** Logic that migrates into the orchestrator stops being testable
without a repository, which is how the correctness-critical parts lose their
exhaustive tests.

---

The component SHALL NOT learn the format of any manifest.

**Rationale:** Version propagation is delegated to a configured command
precisely so that supporting a new ecosystem is a configuration change in one
repository rather than a release of the tooling.

---

- **Verification gates:** the end-to-end cut test against a fixture
  repository, covering the accepting path, a named bump, staging without
  notes, the nothing-to-release refusal, the disagreement halt, and a
  partial-write report.

## Key Decisions

- [[adr-002]] — a release is staged as a branch carrying the release commit
  and merged by pull request; nothing modifies a release branch directly.
- [[adr-003]] — disagreement between the resolved and proposed impact halts
  the release.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-bump-classifier]] — resolves the impact when none is named
- [[component-changelog-authoring-bridge]] — the authoring step this sequence
  may skip
- [[frd-release-staging]] — the feature this component serves
