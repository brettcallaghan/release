package main_test

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// cutFixture is a repository released at v1.8.0 with the given commits
// since, pushed to a bare origin whose HEAD is main, and a stub gh first on
// PATH that records its arguments and answers with a pull request URL.
// Propagation writes VERSION, the one declared release path; the command
// producer answers with <root>/authored.json and records each run in
// <root>/runs.log. Every git run, the binary's included, is isolated from
// the user's configuration.
type cutFixture struct {
	t                *testing.T
	root, work, bare string
}

func newCutFixture(t *testing.T, subjects ...string) *cutFixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "Fixture", "GIT_AUTHOR_EMAIL": "fixture@example.com",
		"GIT_COMMITTER_NAME": "Fixture", "GIT_COMMITTER_EMAIL": "fixture@example.com",
	} {
		t.Setenv(k, v)
	}
	f := &cutFixture{t: t, root: t.TempDir()}
	f.work, f.bare = filepath.Join(f.root, "work"), filepath.Join(f.root, "origin.git")
	fixtureGit(t, f.root, "init", "-q", "--bare", "-b", "main", f.bare)
	fixtureGit(t, f.root, "init", "-q", "-b", "main", f.work)
	writeFile(t, f.work, "VERSION", "1.8.0\n")
	writeFile(t, f.work, "release.toml", `release_paths = ["VERSION"]

[commands]
propagate = 'printf "%s\n" "$RELEASE_VERSION" > VERSION'

[changelog]
producer = "command"
producer_command = "echo run >> `+filepath.ToSlash(filepath.Join(f.root, "runs.log"))+`; cat `+filepath.ToSlash(filepath.Join(f.root, "authored.json"))+`"
`)
	fixtureGit(t, f.work, "add", ".")
	fixtureGit(t, f.work, "commit", "-q", "-m", "Initial commit")
	fixtureGit(t, f.work, "tag", "-a", "-m", "v1.8.0", "v1.8.0")
	for _, s := range subjects {
		fixtureGit(t, f.work, "commit", "-q", "--allow-empty", "-m", s)
	}
	fixtureGit(t, f.work, "remote", "add", "origin", f.bare)
	fixtureGit(t, f.work, "push", "-q", "origin", "main", "--tags")
	fixtureGit(t, f.work, "remote", "set-head", "origin", "main")

	t.Setenv("PATH", ghStubDir(t)+string(os.PathListSeparator)+os.Getenv("PATH"))
	f.stubGh("")
	return f
}

var (
	ghOnce sync.Once
	ghDir  string
	ghErr  error
)

// ghStubDir builds internal/cut/testdata/ghstub once, as gh in a directory
// of its own: a real executable, so it runs as gh on every platform.
func ghStubDir(t *testing.T) string {
	t.Helper()
	ghOnce.Do(func() {
		if ghDir, ghErr = os.MkdirTemp("", "ghstub-*"); ghErr != nil {
			return
		}
		name := "gh"
		if runtime.GOOS == "windows" {
			name += ".exe"
		}
		out, err := exec.Command("go", "build", "-o", filepath.Join(ghDir, name), "../../internal/cut/testdata/ghstub").CombinedOutput()
		if err != nil {
			ghErr = fmt.Errorf("%v\n%s", err, out)
		}
	})
	if ghErr != nil {
		t.Fatalf("building the gh stub: %v", ghErr)
	}
	return ghDir
}

// stubGh makes gh fail with msg on standard error, or succeed WHEN msg is
// empty. Either way it records its arguments, one per line.
func (f *cutFixture) stubGh(msg string) {
	f.t.Helper()
	f.t.Setenv("GHSTUB_ARGS", filepath.Join(f.root, "gh.args"))
	f.t.Setenv("GHSTUB_FAIL", msg)
}

func (f *cutFixture) ghArgs() string {
	data, err := os.ReadFile(filepath.Join(f.root, "gh.args"))
	if os.IsNotExist(err) {
		return ""
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return string(data)
}

func (f *cutFixture) producerRuns() int {
	data, err := os.ReadFile(filepath.Join(f.root, "runs.log"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		f.t.Fatal(err)
	}
	return strings.Count(string(data), "run\n")
}

// writeAuthored makes the producer answer with one feature citing every
// commit in the range, carrying the bundle's sources through.
func (f *cutFixture) writeAuthored() {
	f.t.Helper()
	ev := evidenceAt(f.t, f.work)
	sources := []any{}
	for _, ce := range ev.Commits {
		sources = append(sources, citing(ce)...)
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": "1.0.0",
		"entries": []any{map[string]any{"kind": "FEATURE", "summary": "Compute for any release line.", "breaking": false, "derived": false,
			"sources": sources}},
		"sources": ev.Sources,
	})
	if err != nil {
		f.t.Fatal(err)
	}
	writeFile(f.t, f.root, "authored.json", string(data))
}

type cutAnswer struct {
	Version     string   `json:"version"`
	Branch      string   `json:"branch"`
	Commit      *string  `json:"commit"`
	PullRequest *string  `json:"pull_request"`
	Writes      []string `json:"writes"`
}

func (f *cutFixture) cut(args ...string) (cutAnswer, string, int) {
	f.t.Helper()
	stdout, stderr, code := runIn(f.t, f.work, append([]string{"cut", "--json"}, args...)...)
	var a cutAnswer
	if stdout != "" {
		if err := json.Unmarshal([]byte(stdout), &a); err != nil {
			f.t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
		}
	}
	return a, stderr, code
}

// refs is every ref in dir, so a test can show nothing was written.
func refs(t *testing.T, dir string) string {
	t.Helper()
	return gitOut(t, dir, "for-each-ref", "--format=%(refname) %(objectname)")
}

func (f *cutFixture) hasBranch(dir, name string) bool {
	return exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

// AC: AC-STG-001.1
func TestCutBranchesFromTheTargetTipWithTheReleaseCommitFirst(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	tip := strings.TrimSpace(gitOut(t, f.bare, "rev-parse", "main"))
	a, stderr, code := f.cut("minor", "--skip-notes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if a.Version != "1.9.0" || a.Branch != "cut/1.9.0" || a.Commit == nil {
		t.Fatalf("answer = %+v", a)
	}
	commits := strings.Fields(gitOut(t, f.bare, "rev-list", "--reverse", "main..cut/1.9.0"))
	if len(commits) != 1 || commits[0] != *a.Commit {
		t.Errorf("main..cut/1.9.0 on origin = %v, want just the release commit %s", commits, *a.Commit)
	}
	if parent := strings.TrimSpace(gitOut(t, f.bare, "rev-parse", *a.Commit+"^")); parent != tip {
		t.Errorf("the release commit's parent = %s, want the target tip %s", parent, tip)
	}
	if subject := gitOut(t, f.bare, "log", "-1", "--format=%s", *a.Commit); subject != "Release 1.9.0\n" {
		t.Errorf("subject = %q", subject)
	}
	want := []string{"create branch cut/1.9.0 at " + tip[:7], "commit " + (*a.Commit)[:7] + ` "Release 1.9.0" on cut/1.9.0`,
		"push cut/1.9.0 to origin", "open pull request https://github.com/o/r/pull/7"}
	if strings.Join(a.Writes, "\n") != strings.Join(want, "\n") {
		t.Errorf("writes =\n%s\nwant\n%s", strings.Join(a.Writes, "\n"), strings.Join(want, "\n"))
	}
	if a.PullRequest == nil || *a.PullRequest != "https://github.com/o/r/pull/7" {
		t.Errorf("pull_request = %v", a.PullRequest)
	}
}

// AC: AC-STG-001.2
func TestTheReleaseCommitTouchesOnlyDeclaredReleasePaths(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	a, stderr, code := f.cut("minor", "--skip-notes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	changed := gitOut(t, f.bare, "diff", "--name-only", *a.Commit+"^", *a.Commit)
	if changed != "CHANGELOG.md\nVERSION\n" {
		t.Errorf("the release commit modifies\n%swant CHANGELOG.md and the declared VERSION", changed)
	}
	if v := gitOut(t, f.bare, "show", *a.Commit+":VERSION"); v != "1.9.0\n" {
		t.Errorf("VERSION = %q, want the propagated version", v)
	}

	// A propagation that strays outside the declared paths is refused.
	writeFile(t, f.work, "release.toml", "release_paths = [\"VERSION\"]\n[commands]\npropagate = \"echo 2 > VERSION; echo x > main.go\"\n")
	before := refs(t, f.bare)
	_, stderr, code = f.cut("major", "--skip-notes", "--target", "main")
	if code != 1 || !strings.Contains(stderr, "main.go") {
		t.Errorf("exit = %d, stderr:\n%s\nwant a refusal naming main.go", code, stderr)
	}
	if refs(t, f.bare) != before {
		t.Error("a refused cut changed origin")
	}
}

// AC: AC-STG-001.3
func TestThePullRequestIsTitledForTheVersion(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	if _, stderr, code := f.cut("minor", "--skip-notes"); code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	args := f.ghArgs()
	for _, want := range []string{"pr\ncreate\n", "--title\nRelease 1.9.0\n", "--base\nmain\n", "--head\ncut/1.9.0\n"} {
		if !strings.Contains(args, want) {
			t.Errorf("gh was not run with %q:\n%s", want, args)
		}
	}
}

// AC: AC-STG-001.9
func TestThePullRequestBodyCarriesTheReleaseNotes(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	f.writeAuthored()
	if _, stderr, code := f.cut("minor"); code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	args := f.ghArgs()
	_, body, ok := strings.Cut(args, "--body\n")
	if !ok || !strings.HasPrefix(body, "## Release Notes\n\n### Features\n\n- Compute for any release line. (#12, ") {
		t.Errorf("gh --body =\n%s\nwant the rendered notes under ## Release Notes", body)
	}
}

// AC: AC-STG-001.4
func TestNothingReleasableMeansNoBranch(t *testing.T) {
	f := newCutFixture(t, "docs: describe the flag")
	before := refs(t, f.bare)
	a, stderr, code := f.cut()
	if code != 1 || !strings.Contains(stderr, "no release is warranted") {
		t.Errorf("exit = %d, stderr:\n%s\nwant a no-release refusal", code, stderr)
	}
	if a.Branch != "" || f.hasBranch(f.work, "cut/1.8.1") || refs(t, f.bare) != before {
		t.Error("a branch was created with nothing to release")
	}
}

// AC: AC-STG-001.6
func TestCuttingNeverModifiesTheTargetBranch(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	// The maintainer's checkout is on the target, with uncommitted work.
	writeFile(t, f.work, "VERSION", "work in progress\n")
	writeFile(t, f.work, "notes.txt", "untracked\n")
	remoteMain := gitOut(t, f.bare, "rev-parse", "main")
	localMain := gitOut(t, f.work, "rev-parse", "main")
	status := gitOut(t, f.work, "status", "--porcelain")
	if _, stderr, code := f.cut("minor", "--skip-notes"); code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if gitOut(t, f.bare, "rev-parse", "main") != remoteMain || gitOut(t, f.work, "rev-parse", "main") != localMain {
		t.Error("the target branch moved")
	}
	if gitOut(t, f.work, "symbolic-ref", "HEAD") != "refs/heads/main\n" || gitOut(t, f.work, "status", "--porcelain") != status {
		t.Error("the maintainer's checkout changed")
	}
	if data, _ := os.ReadFile(filepath.Join(f.work, "VERSION")); string(data) != "work in progress\n" {
		t.Errorf("VERSION = %q, want the uncommitted work byte for byte", data)
	}
}

// AC: AC-STG-001.7
func TestCuttingNeverTagsOrPublishes(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	tags, localTags := gitOut(t, f.bare, "tag"), gitOut(t, f.work, "tag")
	if _, stderr, code := f.cut("minor", "--skip-notes"); code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if gitOut(t, f.bare, "tag") != tags || gitOut(t, f.work, "tag") != localTags {
		t.Error("the cut created a tag")
	}
	// gh ran once, to open the pull request, and for nothing else: no
	// release, no merge.
	if args := f.ghArgs(); strings.Count(args, "pr\ncreate\n") != 1 || strings.Contains(args, "release") || strings.Contains(args, "merge") {
		t.Errorf("gh ran with:\n%s", args)
	}
}

// AC: AC-STG-001.8
func TestAPartialWriteIsReportedInFull(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	f.stubGh("HTTP 422: Validation Failed")
	a, stderr, code := f.cut("minor", "--skip-notes")
	if code != 3 {
		t.Fatalf("exit = %d, want 3\n%s", code, stderr)
	}
	if len(a.Writes) != 3 || !strings.HasPrefix(a.Writes[0], "create branch cut/1.9.0") ||
		!strings.HasPrefix(a.Writes[1], "commit ") || a.Writes[2] != "push cut/1.9.0 to origin" || a.PullRequest != nil {
		t.Errorf("answer = %+v, want the branch, the commit and the push", a)
	}
	for _, want := range []string{"HTTP 422", "create branch cut/1.9.0", "push cut/1.9.0 to origin",
		"gh pr create --base main --head cut/1.9.0", "git push origin --delete cut/1.9.0", "git branch -D cut/1.9.0"} {
		if !strings.Contains(stderr, want) {
			t.Errorf("stderr lacks %q:\n%s", want, stderr)
		}
	}
	if !f.hasBranch(f.bare, "cut/1.9.0") {
		t.Error("the pushed branch was deleted")
	}
}

func TestAFailedPushDeletesTheLocalBranchSoARerunSucceeds(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	hook := filepath.Join(f.bare, "hooks", "pre-receive")
	writeFile(t, filepath.Dir(hook), "pre-receive", "#!/bin/sh\necho no pushes today >&2\nexit 1\n")
	if err := os.Chmod(hook, 0o755); err != nil {
		t.Fatal(err)
	}
	a, stderr, code := f.cut("minor", "--skip-notes")
	if code != 3 || !strings.Contains(stderr, "no pushes today") {
		t.Fatalf("exit = %d, want 3\n%s", code, stderr)
	}
	if len(a.Writes) != 3 || a.Writes[2] != "delete branch cut/1.9.0 (local, created by this cut)" {
		t.Errorf("writes = %v, want the deletion reported", a.Writes)
	}
	if f.hasBranch(f.work, "cut/1.9.0") || f.ghArgs() != "" {
		t.Error("the local branch survived, or gh ran")
	}
	if err := os.Remove(hook); err != nil {
		t.Fatal(err)
	}
	if _, stderr, code := f.cut("minor", "--skip-notes"); code != 0 {
		t.Errorf("rerun: exit = %d\n%s", code, stderr)
	}
}

// AC: AC-STG-001.11
func TestANamedBumpSettlesTheImpact(t *testing.T) {
	f := newCutFixture(t, "fix(cli): handle empty input (#11)")
	a, stderr, code := f.cut("minor", "--skip-notes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if a.Version != "1.9.0" {
		t.Errorf("version = %s, want 1.9.0 from the named minor, not 1.8.1 from the fix", a.Version)
	}
}

// AC: AC-STG-001.12
func TestCuttingWithoutNotesInvokesNoProducer(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	a, stderr, code := f.cut("minor", "--skip-notes")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if f.producerRuns() != 0 {
		t.Error("--skip-notes invoked the producer")
	}
	entry := regexp.MustCompile(`(?m)^- add a --target flag \([0-9a-f]{7}, #12\)$`)
	if log := gitOut(t, f.bare, "show", *a.Commit+":CHANGELOG.md"); !entry.MatchString(log) {
		t.Errorf("CHANGELOG.md =\n%s\nwant the mechanical producer's entry", log)
	}
}

func TestADryRunChangesNothing(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	local, remote := refs(t, f.work), refs(t, f.bare)
	stdout, stderr, code := runIn(t, f.work, "cut", "minor", "--skip-notes", "--dry-run")
	if code != 0 {
		t.Fatalf("exit = %d\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "- open pull request \"Release 1.9.0\" from cut/1.9.0 into main\n") || strings.Contains(stdout, "commit:") {
		t.Errorf("stdout =\n%s", stdout)
	}
	// The fetch refreshes origin/main and nothing else.
	if refs(t, f.work) != local || refs(t, f.bare) != remote || f.ghArgs() != "" {
		t.Error("a dry run wrote")
	}
}

func TestStandardInputThatIsNotATerminalIsNeverPrompted(t *testing.T) {
	f := newCutFixture(t, "feat: add a --target flag (#12)")
	_, stderr, code := f.cut("minor", "--skip-notes")
	if code != 0 || strings.Contains(stderr, "Proceed?") {
		t.Errorf("exit = %d, stderr:\n%s\nwant no prompt", code, stderr)
	}
}
