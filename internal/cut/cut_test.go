package cut

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/authoring/producers"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// fixture is a working repository released at v1.8.0 with a feature since,
// pushed to a bare origin, and a stub gh that records its arguments.
type fixture struct {
	t                *testing.T
	root, work, bare string
	gh               string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	// Isolate every git run, the cut's own included, from the user's
	// configuration, and give the release commit an identity.
	t.Setenv("GIT_CONFIG_GLOBAL", os.DevNull)
	t.Setenv("GIT_CONFIG_NOSYSTEM", "1")
	for _, k := range []string{"GIT_AUTHOR_NAME", "GIT_COMMITTER_NAME"} {
		t.Setenv(k, "Fixture")
	}
	for _, k := range []string{"GIT_AUTHOR_EMAIL", "GIT_COMMITTER_EMAIL"} {
		t.Setenv(k, "fixture@example.com")
	}
	f := &fixture{t: t, root: t.TempDir()}
	f.bare = filepath.Join(f.root, "origin.git")
	f.work = filepath.Join(f.root, "work")
	f.git(f.root, "init", "-q", "--bare", "-b", "main", f.bare)
	f.git(f.root, "init", "-q", "-b", "main", f.work)
	f.write("VERSION", "1.8.0\n")
	f.git(f.work, "add", ".")
	f.git(f.work, "commit", "-q", "-m", "Initial commit")
	f.git(f.work, "tag", "-a", "-m", "v1.8.0", "v1.8.0")
	f.git(f.work, "commit", "-q", "--allow-empty", "-m", "feat: add a --target flag (#12)")
	f.git(f.work, "remote", "add", "origin", f.bare)
	f.git(f.work, "push", "-q", "origin", "main", "--tags")
	f.gh = ghStub(t)
	f.stubGh("")
	return f
}

func (f *fixture) git(dir string, args ...string) string {
	f.t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return string(out)
}

func (f *fixture) write(name, content string) {
	f.t.Helper()
	path := filepath.Join(f.work, name)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// stubGh makes gh fail with msg on standard error, or succeed WHEN msg is
// empty. Either way it records its arguments, one per line.
func (f *fixture) stubGh(msg string) {
	f.t.Helper()
	f.t.Setenv("GHSTUB_ARGS", filepath.Join(f.root, "gh.args"))
	f.t.Setenv("GHSTUB_FAIL", msg)
}

func (f *fixture) ghArgs() []string {
	data, err := os.ReadFile(filepath.Join(f.root, "gh.args"))
	if os.IsNotExist(err) {
		return nil
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.Split(strings.TrimSuffix(string(data), "\n"), "\n")
}

var (
	ghOnce sync.Once
	ghPath string
	ghErr  error
)

// ghStub builds testdata/ghstub once: a real executable, so it runs as gh
// on every platform.
func ghStub(t *testing.T) string {
	t.Helper()
	ghOnce.Do(func() {
		dir, err := os.MkdirTemp("", "ghstub-*")
		if err != nil {
			ghErr = err
			return
		}
		ghPath = filepath.Join(dir, "gh")
		if runtime.GOOS == "windows" {
			ghPath += ".exe"
		}
		if out, err := exec.Command("go", "build", "-o", ghPath, "./testdata/ghstub").CombinedOutput(); err != nil {
			ghErr = fmt.Errorf("%v\n%s", err, out)
		}
	})
	if ghErr != nil {
		t.Fatalf("building the gh stub: %v", ghErr)
	}
	return ghPath
}

func ptr(s string) *string { return &s }

func minor() *releasemodel.Bump { b := releasemodel.BumpMinor; return &b }

// request cuts main with the default configuration, propagation writing
// VERSION, and VERSION declared.
func (f *fixture) request() Request {
	cfg := config.Default()
	cfg.ReleasePaths = []string{"VERSION"}
	cfg.Commands.Propagate = ptr(`printf '%s\n' "$RELEASE_VERSION" > VERSION`)
	cfg.Changelog.Producer = config.ProducerCommandName
	cfg.Changelog.ProducerCommand = ptr("exit 7")
	return Request{
		Dir: f.work, Gh: f.gh, Target: "main", Config: cfg, Named: minor(), Skip: true,
		Producers: func(dir string, env []string) map[string]authoring.Producer {
			return producers.Registry(cfg.Changelog, dir, env...)
		},
	}
}

func (f *fixture) hasRef(dir, ref string) bool {
	err := exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", ref).Run()
	return err == nil
}

func TestACutCommitsTheReleaseOntoAStageBranchAndOpensItsPullRequest(t *testing.T) {
	f := newFixture(t)
	res, err := Cut(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if res.Version != "1.9.0" || res.Branch != "cut/1.9.0" || res.PullRequest != "https://github.com/o/r/pull/7" {
		t.Errorf("result = %+v", res)
	}
	actions := []string{}
	for _, w := range res.Writes {
		actions = append(actions, w.Action)
	}
	if strings.Join(actions, ",") != "create branch,commit,push,open pull request" {
		t.Errorf("writes = %v", res.Writes)
	}
	if got := strings.TrimSpace(f.git(f.bare, "rev-parse", "cut/1.9.0")); got != res.Commit {
		t.Errorf("origin's cut/1.9.0 = %s, want the release commit %s", got, res.Commit)
	}
	if parent := strings.TrimSpace(f.git(f.bare, "rev-parse", res.Commit+"^")); parent != res.Base {
		t.Errorf("the release commit's parent = %s, want the target tip %s", parent, res.Base)
	}
	if v := f.git(f.bare, "show", res.Commit+":VERSION"); v != "1.9.0\n" {
		t.Errorf("VERSION in the release commit = %q, want the propagated 1.9.0", v)
	}
	args := strings.Join(f.ghArgs(), "\n")
	for _, want := range []string{"pr\ncreate", "--base\nmain", "--head\ncut/1.9.0", "--title\nRelease 1.9.0", "--body\n## Release Notes\n"} {
		if !strings.Contains(args, want) {
			t.Errorf("gh args lack %q:\n%s", want, args)
		}
	}
	if out := f.git(f.work, "worktree", "list", "--porcelain"); strings.Count(out, "worktree ") != 1 {
		t.Errorf("the scratch checkout was not removed:\n%s", out)
	}
}

func TestTheMaintainersCheckoutIsNeitherReadNorChanged(t *testing.T) {
	f := newFixture(t)
	// An unpushed commit and uncommitted work, staged and not: none of it
	// may reach the release, and all of it must survive.
	f.git(f.work, "commit", "-q", "--allow-empty", "-m", "feat!: an unpushed break")
	f.write("VERSION", "dirty, staged\n")
	f.git(f.work, "add", "VERSION")
	f.write("VERSION", "dirty, unstaged\n")
	f.write("scratch.txt", "untracked\n")
	head := f.git(f.work, "rev-parse", "HEAD")
	status := f.git(f.work, "status", "--porcelain")
	index := f.git(f.work, "diff", "--cached")

	res, err := Cut(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if res.Version != "1.9.0" {
		t.Errorf("version = %s, want 1.9.0 from origin's tip, not 2.0.0 from the unpushed break", res.Version)
	}
	if got := f.git(f.work, "rev-parse", "HEAD"); got != head {
		t.Errorf("HEAD moved: %s -> %s", head, got)
	}
	if got := f.git(f.work, "symbolic-ref", "HEAD"); got != "refs/heads/main\n" {
		t.Errorf("current branch = %s", got)
	}
	if got := f.git(f.work, "status", "--porcelain"); got != status {
		t.Errorf("status changed:\n%s\nwant\n%s", got, status)
	}
	if got := f.git(f.work, "diff", "--cached"); got != index {
		t.Errorf("index changed")
	}
	for name, want := range map[string]string{"VERSION": "dirty, unstaged\n", "scratch.txt": "untracked\n"} {
		if got, _ := os.ReadFile(filepath.Join(f.work, name)); string(got) != want {
			t.Errorf("%s = %q, want %q byte for byte", name, got, want)
		}
	}
	if _, err := os.Stat(filepath.Join(f.work, "CHANGELOG.md")); !os.IsNotExist(err) {
		t.Errorf("CHANGELOG.md appeared in the maintainer's tree")
	}
	if strings.Contains(f.git(f.bare, "log", "--format=%s", res.Commit), "unpushed") {
		t.Errorf("the unpushed commit reached the release")
	}
}

func TestNothingIsWrittenOnARefusal(t *testing.T) {
	cases := map[string]struct {
		setup func(*fixture, *Request)
		want  string
	}{
		"skip without a bump":    {func(_ *fixture, r *Request) { r.Named = nil }, "needs a named bump"},
		"propagate unconfigured": {func(_ *fixture, r *Request) { r.Config.Commands.Propagate = nil }, "commands.propagate is unconfigured"},
		"preflight fails": {func(_ *fixture, r *Request) {
			r.Config.Commands.Preflight = []string{"true", "echo preflight said no; exit 4"}
		}, "commands.preflight failed"},
		"propagate fails": {func(_ *fixture, r *Request) { r.Config.Commands.Propagate = ptr("exit 5") }, "commands.propagate failed"},
		"validate fails": {func(_ *fixture, r *Request) {
			r.Config.Commands.Validate = ptr(`test "$(cat VERSION)" = 2.0.0`)
		}, "commands.validate failed"},
		"undeclared path": {func(_ *fixture, r *Request) {
			r.Config.Commands.Propagate = ptr("echo x > src.go; echo 1.9.0 > VERSION")
		}, "src.go"},
		"undeclared path left by validate": {func(_ *fixture, r *Request) {
			r.Config.Commands.Validate = ptr("echo x > main.go")
		}, "main.go"},
		"undeclared path left by validate, dry run": {func(_ *fixture, r *Request) {
			r.Config.Commands.Validate = ptr("echo x > main.go")
			r.DryRun = true
		}, "main.go"},
		"changelog symlink": {func(f *fixture, _ *Request) {
			outside := filepath.Join(f.root, "outside.md")
			if err := os.WriteFile(outside, []byte("untouched\n"), 0o644); err != nil {
				f.t.Fatal(err)
			}
			if err := os.Symlink(outside, filepath.Join(f.work, "CHANGELOG.md")); err != nil {
				f.t.Skip("no symbolic links here")
			}
			f.git(f.work, "add", "CHANGELOG.md")
			f.git(f.work, "commit", "-q", "-m", "chore: link the changelog")
			f.git(f.work, "push", "-q", "origin", "main")
			f.t.Cleanup(func() {
				if got, _ := os.ReadFile(outside); string(got) != "untouched\n" {
					f.t.Errorf("the write followed the link: %q", got)
				}
			})
		}, "symbolic link"},
		"stage branch exists locally": {func(f *fixture, _ *Request) { f.git(f.work, "branch", "cut/1.9.0") }, "already exists locally"},
		"stage branch exists on origin": {func(f *fixture, _ *Request) {
			f.git(f.work, "push", "-q", "origin", "main:refs/heads/cut/1.9.0")
		}, "already exists on origin"},
		"gh unavailable": {func(f *fixture, r *Request) { r.Gh = filepath.Join(f.root, "no-gh") }, "gh is not available"},
		"declined": {func(_ *fixture, r *Request) {
			r.Confirm = func(Result) (bool, error) { return false, nil }
		}, "declined"},
	}
	for name, c := range cases {
		t.Run(name, func(t *testing.T) {
			f := newFixture(t)
			req := f.request()
			c.setup(f, &req)
			refs := f.git(f.bare, "show-ref")
			res, err := Cut(context.Background(), req)
			var r *Refusal
			if !errors.As(err, &r) || !strings.Contains(r.Reason, c.want) {
				t.Fatalf("err = %v, want a refusal naming %q", err, c.want)
			}
			if len(res.Writes) != 0 {
				t.Errorf("writes = %v on a refusal", res.Writes)
			}
			if f.hasRef(f.work, "refs/heads/cut/1.9.0") && !strings.Contains(name, "exists locally") {
				t.Error("a refusal left a local stage branch")
			}
			if got := f.git(f.bare, "show-ref"); got != refs {
				t.Errorf("origin changed on a refusal:\n%s", got)
			}
			if f.ghArgs() != nil {
				t.Error("gh ran on a refusal")
			}
		})
	}
}

func TestAFailingCommandsOutputIsCarried(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	req.Config.Commands.Preflight = []string{"echo preflight said no; exit 4"}
	_, err := Cut(context.Background(), req)
	var r *Refusal
	if !errors.As(err, &r) || string(r.Output) != "preflight said no\n" {
		t.Errorf("err = %v, want the command's output carried", err)
	}
}

func TestNoReleaseRefusesWithoutABranch(t *testing.T) {
	f := newFixture(t)
	f.git(f.work, "tag", "-a", "-m", "v1.9.0", "v1.9.0")
	f.git(f.work, "push", "-q", "origin", "--tags")
	req := f.request()
	req.Named, req.Skip = nil, false
	_, err := Cut(context.Background(), req)
	var n *NoRelease
	if !errors.As(err, &n) || !strings.Contains(n.Error(), "no release is warranted") {
		t.Fatalf("err = %v, want no release", err)
	}
	if f.hasRef(f.work, "refs/heads/cut/1.9.0") || f.hasRef(f.bare, "refs/heads/cut/1.9.0") {
		t.Error("a stage branch was created")
	}
}

// writeAuthored makes the command producer answer with a corpus proposing
// proposed.
func (f *fixture) writeAuthored(req *Request, proposed string) {
	f.t.Helper()
	c := map[string]any{
		"schema_version": "1.0.0",
		"proposed_bump":  proposed,
		"entries": []any{map[string]any{"kind": "FEATURE", "summary": "Compute for any release line.", "breaking": false, "derived": false,
			"sources": []any{map[string]any{"__type": "PullRequest", "number": 12}}}},
		"sources": []any{map[string]any{"adapter": "COMMITS",
			"from": strings.TrimSpace(f.git(f.work, "rev-parse", "v1.8.0^{commit}")),
			"to":   strings.TrimSpace(f.git(f.work, "rev-parse", "HEAD"))}},
	}
	data, err := json.Marshal(c)
	if err != nil {
		f.t.Fatal(err)
	}
	path := filepath.Join(f.root, "authored.json")
	if err := os.WriteFile(path, data, 0o644); err != nil {
		f.t.Fatal(err)
	}
	req.Config.Changelog.ProducerCommand = ptr("cat " + path)
	changelog := req.Config.Changelog
	req.Producers = func(dir string, env []string) map[string]authoring.Producer {
		return producers.Registry(changelog, dir, env...)
	}
	req.Named, req.Skip = nil, false
}

func TestADisagreementHaltsBeforeAnyWrite(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	f.writeAuthored(&req, "MAJOR")
	res, err := Cut(context.Background(), req)
	var d *Disagreement
	if !errors.As(err, &d) || !strings.Contains(d.Error(), "release cut minor") {
		t.Fatalf("err = %v, want a disagreement naming the confirming command", err)
	}
	if len(res.Writes) != 0 || f.hasRef(f.work, "refs/heads/cut/1.9.0") {
		t.Errorf("a halted cut wrote: %v", res.Writes)
	}
}

func TestAnAgreeingProposalIsAuthoredIntoTheRelease(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	f.writeAuthored(&req, "MINOR")
	res, err := Cut(context.Background(), req)
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if log := f.git(f.bare, "show", res.Commit+":CHANGELOG.md"); !strings.Contains(log, "Compute for any release line.") {
		t.Errorf("CHANGELOG.md lacks the authored prose:\n%s", log)
	}
}

func TestADryRunReportsTheWritesAndMakesNone(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	req.DryRun = true
	req.Gh = filepath.Join(f.root, "no-gh") // a dry run needs no gh
	refs := f.git(f.bare, "show-ref")
	res, err := Cut(context.Background(), req)
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if len(res.Writes) != 4 || res.Writes[3].Action != "open pull request" || res.Commit != "" {
		t.Errorf("result = %+v, want four planned writes and no commit", res)
	}
	if f.hasRef(f.work, "refs/heads/cut/1.9.0") || f.git(f.bare, "show-ref") != refs {
		t.Error("a dry run wrote")
	}
}

func TestConfirmationSeesThePlanBeforeTheFirstWrite(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	req.NoPullRequest = true
	var asked []Write
	req.Confirm = func(plan Result) (bool, error) {
		asked = plan.Writes
		if f.hasRef(f.work, "refs/heads/"+plan.Branch) {
			t.Error("asked after a write")
		}
		return true, nil
	}
	res, err := Cut(context.Background(), req)
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if len(asked) != 3 || len(res.Writes) != 3 || res.PullRequest != "" || f.ghArgs() != nil {
		t.Errorf("asked %v, wrote %v; want three writes and no pull request", asked, res.Writes)
	}
}

func TestAFailedPushDeletesOnlyTheLocalBranchItCreated(t *testing.T) {
	f := newFixture(t)
	hook := filepath.Join(f.bare, "hooks", "pre-receive")
	if err := os.WriteFile(hook, []byte("#!/bin/sh\necho refused by origin >&2\nexit 1\n"), 0o755); err != nil {
		t.Fatal(err)
	}
	_, err := Cut(context.Background(), f.request())
	var w *WriteFailure
	if !errors.As(err, &w) || !strings.Contains(w.Error(), "pushing cut/1.9.0") {
		t.Fatalf("err = %v, want a write failure on the push", err)
	}
	actions := []string{}
	for _, x := range w.Writes {
		actions = append(actions, x.Action)
	}
	if strings.Join(actions, ",") != "create branch,commit,delete branch" {
		t.Errorf("reported writes = %v", w.Writes)
	}
	if f.hasRef(f.work, "refs/heads/cut/1.9.0") {
		t.Error("the local stage branch survived the failed push")
	}
	if f.ghArgs() != nil {
		t.Error("gh ran after a failed push")
	}

	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if _, err := Cut(context.Background(), f.request()); err != nil {
		t.Errorf("a rerun after the push failure: %v", err)
	}
}

func TestAFailedPullRequestKeepsThePushedBranchAndSaysHowToFinish(t *testing.T) {
	f := newFixture(t)
	f.stubGh("HTTP 422: no")
	res, err := Cut(context.Background(), f.request())
	var w *WriteFailure
	if !errors.As(err, &w) || !strings.Contains(w.Error(), "HTTP 422") {
		t.Fatalf("err = %v, want a write failure carrying gh's error", err)
	}
	if len(w.Writes) != 3 || w.Writes[2].Action != "push" || res.PullRequest != "" {
		t.Errorf("reported writes = %v", w.Writes)
	}
	remedy := strings.Join(w.Remedy, "\n")
	for _, want := range []string{"gh pr create --base main --head cut/1.9.0", "git push origin --delete cut/1.9.0", "git branch -D cut/1.9.0"} {
		if !strings.Contains(remedy, want) {
			t.Errorf("remedy lacks %q:\n%s", want, remedy)
		}
	}
	if !f.hasRef(f.work, "refs/heads/cut/1.9.0") || !f.hasRef(f.bare, "refs/heads/cut/1.9.0") {
		t.Error("the pushed stage branch was deleted")
	}
}

func TestTheTargetIsNotModifiedAndNothingIsTagged(t *testing.T) {
	f := newFixture(t)
	main := f.git(f.bare, "rev-parse", "main")
	tags := f.git(f.bare, "tag")
	localTags := f.git(f.work, "tag")
	if _, err := Cut(context.Background(), f.request()); err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if f.git(f.bare, "rev-parse", "main") != main || f.git(f.bare, "tag") != tags || f.git(f.work, "tag") != localTags {
		t.Error("the cut moved the target or created a tag")
	}
}

// AC: AC-STG-001.10
func TestACorrectionCommitKeepsTheReleaseCommitFirst(t *testing.T) {
	// Manual-level AC: the project's required checks judge the stage branch.
	// What the tooling owes them is a branch whose first commit after the
	// target is the release commit however many corrections follow, and a
	// tool that never requires the branch to hold one commit.
	f := newFixture(t)
	res, err := Cut(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	clone := filepath.Join(f.root, "review")
	f.git(f.root, "clone", "-q", "--branch", "cut/1.9.0", f.bare, clone)
	if err := os.WriteFile(filepath.Join(clone, "CHANGELOG.md"), []byte("# Changelog\n\n## 1.9.0\n\nCorrected notes.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	f.git(clone, "commit", "-q", "-am", "Correct the release notes")
	f.git(clone, "push", "-q", "origin", "cut/1.9.0")

	commits := strings.Fields(f.git(f.bare, "rev-list", "--reverse", "main..cut/1.9.0"))
	if len(commits) != 2 || commits[0] != res.Commit {
		t.Errorf("main..cut/1.9.0 = %v, want the release commit %s first", commits, res.Commit)
	}
	changed := strings.Fields(f.git(f.bare, "diff", "--name-only", "main", "cut/1.9.0"))
	if undeclared(changed, []string{"VERSION"}) != nil {
		t.Errorf("the corrected branch modifies undeclared paths: %v", changed)
	}
}

func TestReleasePathsMatchExactlyByDirectoryOrByGlob(t *testing.T) {
	paths := []string{"VERSION", "./pkg/", "crates/*/Cargo.toml"}
	got := undeclared([]string{"CHANGELOG.md", "VERSION", "pkg/a/b.json", "crates/x/Cargo.toml", "crates/x/src/lib.rs", "VERSION.bak"}, paths)
	if strings.Join(got, ",") != "crates/x/src/lib.rs,VERSION.bak" {
		t.Errorf("undeclared = %v", got)
	}
}

func TestTheProducerSeesTheReleaseVersion(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	f.writeAuthored(&req, "MINOR")
	command := `test "$RELEASE_VERSION" = 1.9.0 && ` + *req.Config.Changelog.ProducerCommand
	req.Config.Changelog.ProducerCommand = &command
	changelog := req.Config.Changelog
	req.Producers = func(dir string, env []string) map[string]authoring.Producer {
		return producers.Registry(changelog, dir, env...)
	}
	if _, err := Cut(context.Background(), req); err != nil {
		t.Errorf("Cut: %v; want the producer to see RELEASE_VERSION=1.9.0", err)
	}
}

func TestRepositoryHooksDoNotTouchTheReleaseCommit(t *testing.T) {
	f := newFixture(t)
	hooks := filepath.Join(f.work, ".git", "hooks")
	for name, body := range map[string]string{
		"prepare-commit-msg": "echo rewritten > \"$1\"; echo stray > stray.txt; git add stray.txt",
		"post-checkout":      "exit 1",
	} {
		if err := os.WriteFile(filepath.Join(hooks, name), []byte("#!/bin/sh\n"+body+"\n"), 0o755); err != nil {
			t.Fatal(err)
		}
	}
	res, err := Cut(context.Background(), f.request())
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if subject := f.git(f.bare, "log", "-1", "--format=%s", res.Commit); subject != "Release 1.9.0\n" {
		t.Errorf("subject = %q, want the release message unrewritten", subject)
	}
	if changed := f.git(f.bare, "diff", "--name-only", res.Base, res.Commit); changed != "CHANGELOG.md\nVERSION\n" {
		t.Errorf("the release commit modifies\n%s", changed)
	}
}

func TestAStageBranchCreatedOnOriginMidCutIsNeverMoved(t *testing.T) {
	f := newFixture(t)
	req := f.request()
	req.Confirm = func(plan Result) (bool, error) {
		// Someone else stages the same version after step 2.
		f.git(f.work, "push", "-q", "origin", "main:refs/heads/"+plan.Branch)
		return true, nil
	}
	theirs := ""
	_, err := Cut(context.Background(), req)
	theirs = f.git(f.bare, "rev-parse", "main")
	var w *WriteFailure
	if !errors.As(err, &w) || !strings.Contains(w.Error(), "pushing cut/1.9.0") {
		t.Fatalf("err = %v, want the push refused", err)
	}
	if got := f.git(f.bare, "rev-parse", "cut/1.9.0"); got != theirs {
		t.Errorf("origin's cut/1.9.0 = %s, want it left at %s", got, theirs)
	}
}

func TestTheFetchRefreshesAStaleTrackingRefEvenOnADryRun(t *testing.T) {
	f := newFixture(t)
	stale := strings.TrimSpace(f.git(f.work, "rev-parse", "origin/main"))
	other := filepath.Join(f.root, "other")
	f.git(f.root, "clone", "-q", f.bare, other)
	f.git(other, "commit", "-q", "--allow-empty", "-m", "fix: land elsewhere (#13)")
	f.git(other, "push", "-q", "origin", "main")
	tip := strings.TrimSpace(f.git(f.bare, "rev-parse", "main"))

	req := f.request()
	req.DryRun = true
	res, err := Cut(context.Background(), req)
	if err != nil {
		t.Fatalf("Cut: %v", err)
	}
	if res.Base != tip || res.Base == stale {
		t.Errorf("base = %s, want origin's tip %s, not the stale %s", res.Base, tip, stale)
	}
	// The documented exception: the remote-tracking ref mirrors origin.
	if got := strings.TrimSpace(f.git(f.work, "rev-parse", "origin/main")); got != tip {
		t.Errorf("origin/main = %s, want it refreshed to %s", got, tip)
	}
	if f.git(f.work, "rev-parse", "main") != stale+"\n" {
		t.Error("the local target branch moved")
	}
}
