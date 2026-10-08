// Package cut is the Stage Orchestrator: it cuts a release as a stage branch
// holding the release commit, pushes it, and opens its pull request. It is
// the only release-core component that writes to a repository or a hosting
// platform.
//
// A write is creating or moving a ref, creating a commit, pushing, or opening
// a pull request. Every check that can refuse runs before the first write:
// the release is prepared in a scratch checkout (a detached git worktree at
// the fetched origin/<target> commit), so the project's own commands and the
// release-path check run before anything reaches the repository. The
// maintainer's working tree, index and current branch are never read for the
// release and never changed.
//
// Every computation belongs to a component this one calls: version
// resolution, the changelog operation, the Changelog Writer and the Delegated
// Command Runner. It learns no manifest format; propagation is the
// repository's configured command.
package cut

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"strings"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/changelog"
	"github.com/brettcallaghan/release/internal/changelogfile"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/delegate"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/resolve"
	"github.com/brettcallaghan/release/internal/semver"
	"github.com/brettcallaghan/release/internal/version"
)

// Request is the input to Cut.
type Request struct {
	// Dir is the repository root.
	Dir string
	// Git and Gh are the git and gh executables; empty means each from PATH.
	Git, Gh string
	// Sh is the shell configured commands run through; empty means "sh".
	Sh string
	// Target is the target branch's name on origin.
	Target string
	// Named is the bump the maintainer named, or nil to classify one.
	Named *releasemodel.Bump
	// Config is the loaded configuration.
	Config config.Config
	// Producers returns every changelog producer, by name, for the checkout
	// at dir, adding env to each producer process's environment. It is
	// called with the scratch checkout and RELEASE_VERSION.
	Producers func(dir string, env []string) map[string]authoring.Producer
	// Redo discards a cached corpus and authors again.
	Redo bool
	// Skip stages without an authoring exchange. It requires Named.
	Skip bool
	// NoPullRequest stops after the push.
	NoPullRequest bool
	// DryRun runs every check and reports the writes without making them.
	DryRun bool
	// Confirm, WHEN set, is asked once before the first write, with the
	// writes planned. Declining is a refusal and nothing is written.
	Confirm func(Result) (bool, error)
}

// Write is one write, made or planned.
type Write struct {
	// Action is one of "create branch", "commit", "push", "open pull
	// request" and "delete branch".
	Action string
	// Detail names what was written.
	Detail string
}

func (w Write) String() string { return w.Action + " " + w.Detail }

// Result is what a cut did, or under DryRun would do.
type Result struct {
	// Version is the staged version, without a tag prefix.
	Version string
	// Branch is the stage branch.
	Branch string
	// Target is the target branch, and Base the origin/<target> commit the
	// release is pinned to.
	Target, Base string
	// Commit is the release commit; empty under DryRun.
	Commit string
	// PullRequest is the pull request's URL; empty WHEN none was opened.
	PullRequest string
	// Writes is every write made, in order, or under DryRun every write
	// planned.
	Writes []Write
	// Resolution is the version resolution.
	Resolution resolve.Resolution
	// Changelog is the changelog operation's result. Unset WHEN the cut
	// refused before it.
	Changelog changelog.Result
}

// Refusal is a stated, expected condition under which nothing is written.
type Refusal struct {
	Reason string
	// Output is a refusing command's output, in full, WHEN one refused.
	Output []byte
}

func (r *Refusal) Error() string { return r.Reason }

// NoRelease is the refusal WHEN version resolution answers no release, or
// the version policy refuses the version.
type NoRelease struct {
	Next version.Result
}

func (r *NoRelease) Error() string {
	if r.Next.Kind == version.NoRelease {
		return "no release is warranted: " + r.Next.Reason
	}
	if r.Next.Rule != "" {
		return fmt.Sprintf("the version policy refuses this release (%s): %s", r.Next.Rule, r.Next.Reason)
	}
	return "the version policy refuses this release: " + r.Next.Reason
}

// Disagreement is the refusal WHEN the authored proposal differs from the
// computed impact and no bump was named.
type Disagreement struct {
	Computed, Proposed releasemodel.Bump
}

func (d *Disagreement) Error() string {
	computed := strings.ToLower(string(d.Computed))
	return fmt.Sprintf("the authored corpus proposes %s, but the commits indicate %s; "+
		"confirm the impact by naming it (release cut %s), or correct the commits",
		strings.ToLower(string(d.Proposed)), computed, computed)
}

// WriteFailure is a write that failed after Writes were made. Remedy lists
// the commands that finish or discard the release.
type WriteFailure struct {
	Err    error
	Writes []Write
	Remedy []string
}

func (f *WriteFailure) Error() string { return f.Err.Error() }
func (f *WriteFailure) Unwrap() error { return f.Err }

// Cut runs the cut sequence. A refusal is a *Refusal, *NoRelease or
// *Disagreement, and nothing was written; a failed write after another
// succeeded is a *WriteFailure; any other error means the repository, a
// producer or a tool failed before any write. The Result describes what was
// reached, including on a refusal or failure.
func Cut(ctx context.Context, req Request) (res Result, err error) {
	res.Target = req.Target
	if req.Skip && req.Named == nil {
		return res, &Refusal{Reason: "staging without notes needs a named bump: name major, minor or patch with --skip-notes"}
	}
	if req.Config.Commands.Propagate == nil {
		return res, &Refusal{Reason: "commands.propagate is unconfigured: cutting a release writes the version into the repository's manifests through it; " +
			"configure it in the release configuration (a command that needs no manifest change can be `true`)"}
	}
	if !req.NoPullRequest && !req.DryRun {
		if _, err := exec.LookPath(orDefault(req.Gh, "gh")); err != nil {
			return res, &Refusal{Reason: "gh is not available to open the pull request: install the GitHub CLI, or pass --no-pull-request"}
		}
	}

	g := git{bin: req.Git, dir: req.Dir}
	// Pin the release to the fetched tip: the version, the range and the
	// scratch checkout all describe this one commit.
	remoteRef := "refs/remotes/origin/" + req.Target
	if _, err := g.run(ctx, "fetch", "--quiet", "origin", "+refs/heads/"+req.Target+":"+remoteRef); err != nil {
		return res, fmt.Errorf("fetching %s from origin: %w", req.Target, err)
	}
	if res.Base, err = g.out(ctx, "rev-parse", "--verify", "--end-of-options", remoteRef+"^{commit}"); err != nil {
		return res, err
	}

	// 1. Resolve the version.
	if res.Resolution, err = resolve.Resolve(ctx, resolve.Request{
		Dir: req.Dir, Git: req.Git, Target: res.Base, Named: req.Named, Policy: req.Config.Policy(),
	}); err != nil {
		return res, err
	}
	if res.Resolution.Next.Kind != version.Release {
		return res, &NoRelease{Next: res.Resolution.Next}
	}
	v := res.Resolution.Next.Version
	res.Version = v.String()
	res.Branch = semver.StageBranchName(req.Config.StagePrefix, v)

	// 2. Refuse a stage branch that already exists, here or on origin.
	if exists, err := g.ok(ctx, "show-ref", "--verify", "--quiet", "refs/heads/"+res.Branch); err != nil {
		return res, err
	} else if exists {
		return res, &Refusal{Reason: fmt.Sprintf("the stage branch %s already exists locally; delete it (git branch -D %s) or release another version", res.Branch, res.Branch)}
	}
	remote, err := g.out(ctx, "ls-remote", "--heads", "origin", "refs/heads/"+res.Branch)
	if err != nil {
		return res, fmt.Errorf("checking origin for %s: %w", res.Branch, err)
	}
	if remote != "" {
		return res, &Refusal{Reason: fmt.Sprintf("the stage branch %s already exists on origin; close its pull request and delete it there, or release another version", res.Branch)}
	}

	scratch, err := os.MkdirTemp("", "release-cut-*")
	if err != nil {
		return res, err
	}
	wt := filepath.Join(scratch, "checkout")
	defer func() {
		// Removed on every exit. A failure here leaves only scratch state,
		// which git worktree prune reclaims, so it does not mask the cut's
		// own outcome.
		_, _ = g.run(context.WithoutCancel(ctx), "worktree", "remove", "--force", wt)
		_ = os.RemoveAll(scratch)
	}()
	if _, err := g.run(ctx, "-c", noHooks, "worktree", "add", "--quiet", "--detach", wt, res.Base); err != nil {
		return res, fmt.Errorf("creating the scratch checkout: %w", err)
	}
	env := []string{"RELEASE_VERSION=" + res.Version}
	sh := delegate.Shell{Sh: req.Sh, Env: env}

	// 3. Preflight, WHEN configured.
	if len(req.Config.Commands.Preflight) > 0 {
		if err := step(sh.RunAll(ctx, "preflight", req.Config.Commands.Preflight, wt)); err != nil {
			return res, err
		}
	}

	// 4, 5. Assemble, author unless skipped, set the version and render.
	var producers map[string]authoring.Producer
	if req.Producers != nil {
		producers = req.Producers(wt, env)
	}
	if res.Changelog, err = changelog.Prepare(ctx, changelog.Request{
		Dir: req.Dir, Git: req.Git, Target: res.Base, Named: req.Named, Config: req.Config,
		Producers: producers, Redo: req.Redo, Skip: req.Skip,
	}); err != nil {
		return res, err
	}
	switch res.Changelog.Outcome {
	case changelog.NoRelease:
		return res, &NoRelease{Next: res.Changelog.Resolution.Next}
	case changelog.Disagreement:
		return res, &Disagreement{Computed: res.Changelog.Authoring.Computed, Proposed: *res.Changelog.Authoring.Proposed}
	}

	// 6. Write the changelog section, then propagate the version. A
	// symbolic link would carry the write out of the scratch checkout.
	if info, err := os.Lstat(filepath.Join(wt, changelogfile.Name)); err == nil && info.Mode()&os.ModeSymlink != 0 {
		return res, &Refusal{Reason: changelogfile.Name + " is a symbolic link; the release writes the changelog only as a regular file"}
	}
	if err := changelogfile.Write(wt, res.Version, res.Changelog.Rendering.Section); err != nil {
		return res, err
	}
	if err := step(sh.Run(ctx, "propagate", req.Config.Commands.Propagate, wt)); err != nil {
		return res, err
	}

	// 7. Confine the release commit to the declared release paths.
	if err := confine(ctx, g.in(wt), req.Config.ReleasePaths); err != nil {
		return res, err
	}

	// 8. Validate, then confine again: validation may change files too, and
	// the commit takes the tree as it is left.
	if req.Config.Commands.Validate != nil {
		if err := step(sh.Run(ctx, "validate", req.Config.Commands.Validate, wt)); err != nil {
			return res, err
		}
		if err := confine(ctx, g.in(wt), req.Config.ReleasePaths); err != nil {
			return res, err
		}
	}
	// The commit needs an identity; without one it would fail after the
	// branch exists.
	if _, err := g.in(wt).out(ctx, "var", "GIT_COMMITTER_IDENT"); err != nil {
		return res, &Refusal{Reason: "git has no committer identity to make the release commit with: set user.name and user.email"}
	}

	res.Writes = planned(res, req)
	if req.DryRun {
		return res, nil
	}
	if req.Confirm != nil {
		ok, err := req.Confirm(res)
		if err != nil {
			return res, err
		}
		if !ok {
			res.Writes = nil
			return res, &Refusal{Reason: "declined; nothing was written"}
		}
	}
	res.Writes = nil
	return res, write(ctx, req, g, wt, &res)
}

// planned is the writes step 9 makes.
func planned(res Result, req Request) []Write {
	w := []Write{
		{"create branch", fmt.Sprintf("%s at %s", res.Branch, short(res.Base))},
		{"commit", fmt.Sprintf("%q on %s", "Release "+res.Version, res.Branch)},
		{"push", res.Branch + " to origin"},
	}
	if !req.NoPullRequest {
		w = append(w, Write{"open pull request", fmt.Sprintf("%q from %s into %s", "Release "+res.Version, res.Branch, res.Target)})
	}
	return w
}

// write is step 9: create the stage branch at the pinned commit, commit the
// release onto it, push it, and open the pull request.
func write(ctx context.Context, req Request, g git, wt string, res *Result) error {
	wg := g.in(wt)
	if _, err := wg.run(ctx, "-c", noHooks, "switch", "--quiet", "--create", res.Branch); err != nil {
		return fmt.Errorf("creating the stage branch %s: %w", res.Branch, err)
	}
	res.Writes = append(res.Writes, Write{"create branch", fmt.Sprintf("%s at %s", res.Branch, short(res.Base))})
	discard := []string{"git branch -D " + res.Branch}

	// No hooks: the commit is exactly the tree and message steps 6 to 8
	// checked. --no-verify alone still runs prepare-commit-msg, which can
	// rewrite the message or stage more files.
	if _, err := wg.run(ctx, "add", "--all"); err != nil {
		return &WriteFailure{Err: fmt.Errorf("staging the release: %w", err), Writes: res.Writes, Remedy: discard}
	}
	if _, err := wg.run(ctx, "-c", noHooks, "commit", "--quiet", "--no-verify", "-m", "Release "+res.Version); err != nil {
		return &WriteFailure{Err: fmt.Errorf("committing the release: %w", err), Writes: res.Writes, Remedy: discard}
	}
	commit, err := wg.out(ctx, "rev-parse", "HEAD")
	if err != nil {
		return &WriteFailure{Err: err, Writes: res.Writes, Remedy: discard}
	}
	res.Commit = commit
	res.Writes = append(res.Writes, Write{"commit", fmt.Sprintf("%s %q on %s", short(commit), "Release "+res.Version, res.Branch)})
	// Release the branch from the scratch checkout, so it can be deleted
	// WHEN the push fails and is not left checked out once the cut returns.
	if _, err := wg.run(ctx, "-c", noHooks, "switch", "--quiet", "--detach"); err != nil {
		return &WriteFailure{Err: err, Writes: res.Writes, Remedy: discard}
	}

	ref := "refs/heads/" + res.Branch
	// The lease with an empty expected value pushes only WHILE origin has no
	// such branch, so a branch created there since step 2 is never moved.
	if _, err := g.run(ctx, "push", "--quiet", "--force-with-lease="+ref+":", "origin", ref+":"+ref); err != nil {
		failure := &WriteFailure{Err: fmt.Errorf("pushing %s to origin: %w", res.Branch, err), Writes: res.Writes}
		// Delete only the branch this cut created, so a rerun is not refused
		// for it. Nothing on origin is touched.
		if _, derr := g.run(context.WithoutCancel(ctx), "branch", "--quiet", "-D", res.Branch); derr != nil {
			failure.Remedy = discard
			failure.Err = fmt.Errorf("%w; deleting the local stage branch also failed: %v", failure.Err, derr)
		} else {
			failure.Writes = append(failure.Writes, Write{"delete branch", res.Branch + " (local, created by this cut)"})
		}
		res.Writes = failure.Writes
		res.Commit = ""
		return failure
	}
	res.Writes = append(res.Writes, Write{"push", res.Branch + " to origin"})
	if req.NoPullRequest {
		return nil
	}

	title := "Release " + res.Version
	body := "## Release Notes\n\n" + res.Changelog.Rendering.Body
	out, err := ghRun(ctx, req, "pr", "create", "--base", res.Target, "--head", res.Branch, "--title", title, "--body", body)
	if err != nil {
		return &WriteFailure{
			Err:    fmt.Errorf("opening the pull request: %w", err),
			Writes: res.Writes,
			Remedy: []string{
				fmt.Sprintf("gh pr create --base %s --head %s --title %q --body-file <notes>", res.Target, res.Branch, title),
				"git push origin --delete " + res.Branch,
				"git branch -D " + res.Branch,
			},
		}
	}
	res.PullRequest = lastLine(out)
	res.Writes = append(res.Writes, Write{"open pull request", res.PullRequest})
	return nil
}

// step turns a delegated step's failure into a refusal. Every step is run
// only WHEN configured, propagation having been required up front.
func step(r delegate.Result, err error) error {
	if err != nil || r.Outcome != delegate.Failed {
		return err
	}
	return &Refusal{
		Reason: fmt.Sprintf("commands.%s failed: %q exited %d", r.Step, r.Command, r.ExitCode),
		Output: append(append([]byte{}, r.Stdout...), r.Stderr...),
	}
}

// noHooks points git at no hooks directory, for the scratch checkout's own
// operations.
var noHooks = "core.hooksPath=" + os.DevNull

// confine refuses WHEN the checkout changes a path that is neither the
// changelog nor declared.
func confine(ctx context.Context, wt git, releasePaths []string) error {
	changed, err := wt.changedPaths(ctx)
	if err != nil {
		return err
	}
	if stray := undeclared(changed, releasePaths); len(stray) > 0 {
		return &Refusal{Reason: fmt.Sprintf("the release would modify paths that are neither %s nor in release_paths: %s",
			changelogfile.Name, strings.Join(stray, ", "))}
	}
	return nil
}

// undeclared is every changed path that is neither the changelog nor
// matched by a release path. A release path matches a path equal to it, a
// path under it WHEN it ends in "/", or a path it matches as a glob.
func undeclared(changed, releasePaths []string) []string {
	var out []string
	for _, p := range changed {
		if p == changelogfile.Name || declared(p, releasePaths) {
			continue
		}
		out = append(out, p)
	}
	return out
}

func declared(p string, releasePaths []string) bool {
	for _, rp := range releasePaths {
		rp = strings.TrimPrefix(rp, "./")
		if p == rp || strings.HasSuffix(rp, "/") && strings.HasPrefix(p, rp) {
			return true
		}
		if ok, err := path.Match(rp, p); err == nil && ok {
			return true
		}
	}
	return false
}

func short(hash string) string {
	if len(hash) > 7 {
		return hash[:7]
	}
	return hash
}

func lastLine(s string) string {
	lines := strings.Split(strings.TrimSpace(s), "\n")
	return strings.TrimSpace(lines[len(lines)-1])
}

func orDefault(s, d string) string {
	if s == "" {
		return d
	}
	return s
}

// ghRun runs gh in the repository root, so it resolves the repository from
// origin, and returns its standard output.
func ghRun(ctx context.Context, req Request, args ...string) (string, error) {
	cmd := exec.CommandContext(ctx, orDefault(req.Gh, "gh"), args...)
	cmd.Dir = req.Dir
	cmd.Env = append(os.Environ(), "GH_PROMPT_DISABLED=1", "NO_COLOR=1")
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		msg := strings.TrimSpace(stderr.String())
		if msg == "" {
			msg = "no output"
		}
		return "", fmt.Errorf("gh %s: %v: %s", args[0]+" "+args[1], err, msg)
	}
	return stdout.String(), nil
}

// git runs the git binary in one directory, non-interactively.
type git struct {
	bin, dir string
}

func (g git) in(dir string) git { return git{bin: g.bin, dir: dir} }

var gitEnv = []string{"GIT_TERMINAL_PROMPT=0", "GIT_PAGER=cat", "LC_ALL=C", "LANG=C"}

func (g git) run(ctx context.Context, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, orDefault(g.bin, "git"), append([]string{"-C", g.dir}, args...)...)
	cmd.Env = append(os.Environ(), gitEnv...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return nil, &GitError{Args: args, ExitCode: exit.ExitCode(), Stderr: stderr.String()}
		}
		return nil, fmt.Errorf("git is not available: %w", err)
	}
	return stdout.Bytes(), nil
}

func (g git) out(ctx context.Context, args ...string) (string, error) {
	b, err := g.run(ctx, args...)
	return strings.TrimSpace(string(b)), err
}

// ok runs a git query whose exit status 1 means "no".
func (g git) ok(ctx context.Context, args ...string) (bool, error) {
	_, err := g.run(ctx, args...)
	var gitErr *GitError
	if errors.As(err, &gitErr) && gitErr.ExitCode == 1 {
		return false, nil
	}
	return err == nil, err
}

// changedPaths is every path changed in the checkout against HEAD, tracked
// or not, with forward slashes. A rename contributes both its paths.
func (g git) changedPaths(ctx context.Context) ([]string, error) {
	out, err := g.run(ctx, "status", "--porcelain=v1", "-z", "--untracked-files=all", "--no-renames")
	if err != nil {
		return nil, err
	}
	var paths []string
	for _, rec := range strings.Split(string(out), "\x00") {
		if len(rec) > 3 {
			paths = append(paths, rec[3:])
		}
	}
	return paths, nil
}

// GitError is a git invocation that exited unsuccessfully.
type GitError struct {
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *GitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = "no output"
	}
	return fmt.Sprintf("git %s: exit status %d: %s", strings.Join(e.Args, " "), e.ExitCode, msg)
}
