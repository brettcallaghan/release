---
title: "Component Blueprint: Stage Orchestrator"
entity_name: Stage Orchestrator
description: The sequence that cuts a release — producing a release commit
  carrying the confirmed bump as a trailer, a stage branch, and a pull
  request showing the changelog, the suggested bump and its reasons — and the
  only component that writes to a repository.
slug: component-stage-orchestrator
type: component-blueprint
status: approved
updated_at: 2026-10-09T00:00:00Z
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

Runs the sequence behind `release cut`: resolve the base version and the
signal floor, build the evidence, author the structured changelog, suggest
and confirm the bump, compute the next version, render, write the
changelog, propagate the version across the repository's manifests, commit,
branch, and open the pull request. It holds no logic of its own beyond
ordering and failure handling.

The operation is called cutting a release, which is why the command reads
`release cut minor` and the default branch prefix is `cut/`. The documents
keep calling the branch a *stage branch* — that is the concept, whatever a
project names the prefix ([[adr-002]]).

Two paths through it. With a bump named on the command line, the bump is
settled by a person before anything is authored, subject only to the
signal floor. Without one, [[component-bump-classifier]] computes the floor
from the explicit signals in the range, the producer authors the structured
changelog, and [[component-bump-suggester]] derives the suggested bump from
it and halts until a person confirms. Either way the confirmed bump is
committed as a `Release-Type:` trailer on the release commit, so the version
stays recomputable from a clone without re-running a model, and the stage
pull request shows the changelog, the suggestion and its reasons.
Reviewing the stage pull request is where a person reviews the model's
changelog before anything is published. Authoring can be skipped, in which
case [[component-mechanical-producer]] writes the changelog and the
suggestion is the signal floor alone.

It is the only component that writes to the repository. Everything upstream
reads or computes, so a failure part-way leaves state this component is
responsible for describing — and, where it can, for not creating. Every check
that can fail is performed before the first write.

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
  - WHEN the maintainer names a bump at or above the signal floor, the
    component SHALL use it, warning WHEN it is below the suggested bump.
  - IF the maintainer names a bump below the signal floor and the release is
    not forced, THEN the component SHALL refuse without writing.
  - WHEN the maintainer requests staging without notes, the component SHALL
    author the structured changelog with [[component-mechanical-producer]].
  - WHEN [[component-bump-suggester]] halts, the component SHALL halt
    without writing, returning the suggested bump, its reasons, the signal
    floor and the unclassified commits as its result.
  - IF a forced release is requested and no bump is named, THEN the
    component SHALL refuse, because there is no named bump to force.
  - The component SHALL create a stage branch whose first commit is the
    release commit.
  - The component SHALL end the release commit's message with the confirmed
    bump as a `Release-Type:` trailer.
  - The component SHALL run the repository's configured version-propagation
    command to write the version into its manifests.
  - The component SHALL confine its writes to the paths the project declared
    as release paths.
  - The component SHALL title the pull request for the version being released.
  - The component SHALL include the rendered release notes in the pull request
    body under a "Release Notes" heading.
  - The component SHALL show in the pull request body the confirmed bump and
    who decided it, the suggested bump with its reasons, and the signal floor
    with the commits that set it.
  - WHEN no releasable change exists since the latest reachable tag, the
    component SHALL refuse without writing, unless a bump is named and the
    release is forced. A range releasable only because an entry has no kind
    is releasable: its halt for a named bump is answered by naming one,
    without a force. The refusal SHALL say whether the range holds no
    commits or how many commits it holds, and WHEN a bump was named SHALL
    name how to force the release.
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
  Refusals that need no repository come first: forcing without a bump, an
  unconfigured `commands.propagate`,
  and, unless the run is a dry run or opens no pull request, a missing `gh` or
  one that `gh auth status`, scoped to `origin`'s host, reports
  unauthenticated. Then, stopping at the
  first refusal or failure:
  1. resolve the base version and the signal floor, with the unclassified
     commits; refuse WHEN the range holds no commits and the release is not
     forced; WHEN a bump is named, refuse WHEN it is below the floor and the
     release is not forced, then compute the version and refuse WHEN the
     stage branch exists locally or on `origin`;
  2. run `commands.preflight` WHEN configured;
  3. collect the diffs, build the evidence bundle and author the structured
     changelog, with the mechanical producer WHEN notes are skipped; WHEN
     the configured producer is unavailable or fails, stop with its failure,
     naming `--skip-notes`;
  4. suggest the bump; refuse WHEN the range holds no releasable change and
     the release is not forced with a named bump; otherwise decide it from
     the named bump or the confirmation, halting WHEN the suggester halts;
  5. compute the version from the confirmed bump, refusing WHEN the
     policy does, and refuse WHEN the stage branch exists locally or on
     `origin`;
  6. set the corpus `version` and render it;
  7. in the scratch checkout, write the changelog section, then run
     `commands.propagate`;
  8. refuse WHEN a changed path is neither `CHANGELOG.md` nor matched by
     `release_paths` (equal to an entry, under an entry ending in `/`, or
     matched by an entry as a glob);
  9. run `commands.validate` WHEN configured, then repeat step 8's check,
     and refuse WHEN git has no committer identity;
  10. write: create the stage branch at the pinned commit, commit the
      changes as `Release <version>` with the trailer
      `Release-Type: <confirmed bump>`, push the branch to `origin`, and
      open the pull request into the target through `gh`, titled
      `Release <version>`, with the rendered changelog under
      `## Release Notes`, followed under `## Bump` by the confirmed bump and
      whether it was named or confirmed, the suggestion and its reasons, the
      signal floor with the commits that set it, and the unclassified
      commits, and, WHEN the changelog omits any commit, each omitted commit
      with its subject and reason under `## Omitted commits`, unless told not
      to.
- **Configured commands and the changelog producer** run in the scratch
  checkout with `RELEASE_VERSION` set to the version being released WHEN a
  bump is named, and unset otherwise, since the version is not known until
  the suggestion is confirmed. The
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
- **Dry run:** steps 1 to 9 run, and step 10's writes are reported, not made.
- **Confirmation:** the component does not prompt. Its input carries an
  optional named bump and whether the suggestion is accepted. `release cut`
  on a terminal receives the halt, prompts, and calls again with the answer
  as an acceptance or a named bump; `--yes` passes the acceptance on the
  first call; a non-interactive call without `--yes` ends at the halt,
  which the front end reports with the exit status [[contract-release-cli]]
  names. The authored changelog is read from the cache on the second call,
  so confirming costs no second authoring and the suggestion confirmed is
  the one shown.

**Rationale for the last guarantee:** a maintainer correcting the changelog
during review adds a commit, and a tool that treated that as malformed would
make the review surface unusable for the one thing it is for.

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
  notes, the nothing-to-release refusal, the halt for confirmation and the
  accepted suggestion, a named bump below the suggestion (warned) and below
  the signal floor (refused, and forced), the halt for a named bump when
  the suggestion is none and an entry has no kind, the `Release-Type:` trailer on
  the release commit, and a partial-write report.

## Key Decisions

- [[adr-002]] — a release is staged as a branch carrying the release commit
  and merged by pull request; nothing modifies a release branch directly.
- [[adr-003]] — the bump is suggested from the authored changelog, floored
  by git signals and confirmed by a person, and the confirmed bump is
  committed on the release commit.

## Open Questions

None.

## References

- `shared/c4-standards.md` — C4 mapping rules and diagram conventions
- [[module-release-core]] — the module this component is composed into
- [[component-bump-classifier]] — computes the signal floor from git
- [[component-bump-suggester]] — suggests the bump and decides it, or halts
  for a person to confirm
- [[component-changelog-authoring-bridge]] — the authoring step this sequence
  may skip
- [[frd-release-staging]] — the feature this component serves
