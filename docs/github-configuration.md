# GitHub configuration

How to configure a GitHub repository that releases with `release`. Each
setting names what it buys and what breaks without it. This is an operations
guide, not a specification: the requirements it serves are cited as
`RRP-nnn` ([repository release policy](../specs/standards/repository-release-policy.md))
and ADR numbers ([ADRs](../specs/adrs/README.md)).

Settings paths are GitHub's web UI, under the repository's **Settings** tab.

## At a glance

| Setting | Where | Value | Status |
|---|---|---|---|
| Squash merge commit message | General → Pull Requests | Pull request title and description | Recommended |
| Merge methods | General → Pull Requests | Squash only | Recommended |
| Branch up to date before merge | Rules → Rulesets (target branch) | On | Required (RRP-021) |
| Required status checks | Rules → Rulesets (target branch) | The repository's CI jobs | Required (RRP-016, RRP-035) |
| Release tags immutable | Rules → Rulesets (tags) | Block update and deletion of `v*` | Required (RRP-026) |
| Workflow permissions | Actions → General | Read repository contents | Recommended |
| Token for `release cut` | Where `release cut` runs | `gh` authenticated, Contents + Pull requests write | Required to open the PR |

## Pull requests and merging

### Squash with the pull request title and description

**General → Pull Requests → Allow squash merging → Default commit message:
"Pull request title and description".**

`release` reads git, not the hosting platform. With this setting a squash
commit is:

```
<pull request title> (#<number>)

<pull request description>
```

so the title, the number and the whole description reach the release through
`git log`, and the changelog author reads what the pull request said about
the change. With GitHub's default ("Default message"), a multi-commit pull
request squashes to a list of its commit subjects, and the author sees that
list instead of the description.

It is recommended, not required: `release` does not inspect the setting, and
works on whatever the commits carry. A repository that cannot change it gets
thinner changelog input, not a failure.

What follows from it:

- **Write the description for the changelog.** It becomes the commit body.
  Say what changed for a user and why; leave review chatter and checklists
  out, or keep them short.
- **Edit the commit message in the merge dialog** when the description has
  drifted from the final change. The dialog's text is what lands in git.
- **Trailers go at the end of the description**, as the last paragraph, so
  git parses them as trailers: `Release-Type: minor`, or a
  `BREAKING CHANGE: <what broke>` paragraph.
- **The title is the commit subject.** Where the repository uses
  conventional-commit types as bump signals, the pull request title carries
  the type: `feat: add --json to release version`.

### Squash only

**General → Pull Requests:** allow squash merging; clear "Allow merge
commits" and "Allow rebase merging".

One commit per pull request, whose subject ends ` (#<number>)`, is the shape
the commit collector reads best: every change maps to one pull request and its
description. Rebase merging copies the branch's commits without the pull
request number or description. Merge commits are read, but their subjects
name a branch, not a change.

A stage pull request holds one commit, so it lands the same under any method
(ADR-002).

### Delete head branches

**General → Pull Requests → Automatically delete head branches: on.**

Safe for stage branches: the release workflow checks out the merge commit,
not the branch. `release cut` refuses when the stage branch already exists on
`origin`, so a leftover `cut/<version>` from an abandoned stage pull request
blocks the next cut of that version until it is deleted.

## Rulesets

Prefer rulesets (**Rules → Rulesets**) over classic branch protection: they
cover tags, and several can apply to one branch.

### The target branch

A ruleset targeting the release branch (`main`, and any maintenance branches
the project releases from):

- **Require a pull request before merging.**
- **Require status checks to pass**, with the repository's CI jobs as
  required checks. The required-check set *is* release readiness (RRP-016);
  a stage pull request merges only when it passes (RRP-035). Include the full
  artifact build (RRP-019). In this repository that is every job of
  `.github/workflows/cli.yml` (the three `go test` legs, coverage,
  cross-compile, and the no-runtime smoke test) and the `checks.yml` plugin
  job.
- **Require branches to be up to date before merging** (RRP-021). A stage
  branch computed before another pull request merged has an incomplete
  changelog, and a wrong version if that pull request was breaking. When the
  target advances, re-cut rather than update: delete the stage branch and run
  `release cut` again.
- **Block force pushes.**

Workflows whose checks are required must run on every pull request. A
`paths:` filter that skips a pull request leaves its required check pending
forever. Skip expensive jobs on drafts with a job-level `if:` instead; a
skipped job still reports a check run, which a ruleset counts as passing.
`cli.yml` and `checks.yml` do this, and list `ready_for_review` in their
`pull_request` types so leaving draft starts the run.

### Release tags

A tag ruleset on `v*` (the configured `tag_prefix`) that **restricts updates
and deletions**. A published tag never moves (RRP-026): consumers, checksums
and package formulae resolve against it.

Leave **creation** open to GitHub Actions, or the release workflow cannot
tag. Restricting creation to the workflow alone means adding
`github-actions[bot]` (or the app that publishes) to the bypass list.

### Stage branches

The stage prefix (`stage_prefix`, `cut/` by default) records an intent to
release; it authorizes nothing (RRP-034). Anyone who can push a branch can
push `cut/9.9.9`. The publish workflow guards publication itself:

- it runs only for a merged pull request whose head is in this repository,
  never a fork;
- it verifies the merge commit: the branch names a plain version, the newest
  `CHANGELOG.md` section is that version, and the tag is free or already on
  that commit.

Keep the stage prefix distinct from any release-branch glob (`release/*`), or
rulesets and workflow filters meant for one catch the other (ADR-002).

## Actions

### Workflow permissions

**Actions → General → Workflow permissions: Read repository contents and
packages permissions.** Each workflow asks for what it needs: the publish job
in `release.yml` declares `contents: write` for itself, and every other job
reads.

"Allow GitHub Actions to create and approve pull requests" can stay off;
nothing in the release path opens a pull request from Actions.

### Why publication is one workflow

A tag pushed with `GITHUB_TOKEN` starts no other workflow, so an
`on: push: tags` workflow never fires after an Actions-made tag.
`release.yml` therefore tags, builds and publishes in one run, triggered by
the stage pull request closing as merged. Re-running a failed run finishes a
partial release: an existing tag on the merge commit is accepted, assets
upload with `--clobber`, and the release stays a draft until every asset is
attached.

## Credentials for `release cut`

`release cut` pushes the stage branch with git and opens the pull request
with `gh`. Before any write it runs `gh auth status --hostname <origin host>`
and refuses when `gh` is missing or unauthenticated, unless the run is
`--dry-run` or `--no-pull-request`.

- **On a workstation:** `gh auth login` once.
- **Headless (CI, an agent session):** set `GH_TOKEN` to a fine-grained
  personal access token scoped to the repository, with **Contents: read and
  write** (the push) and **Pull requests: read and write** (the pull
  request). The git push uses git's own credential, which must also be
  allowed to push `cut/*` branches.
- **Without a usable token:** `release cut --no-pull-request` pushes the
  stage branch and stops; open the pull request by hand, titled
  `Release <version>`, into the target branch, with the release notes under
  `## Release Notes`.

A pull request opened with `GITHUB_TOKEN` from inside Actions triggers no
workflows, so its required checks never run. Running `release cut` from a
workflow needs a personal access token or a GitHub App token.
