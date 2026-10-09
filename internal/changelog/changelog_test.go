package changelog

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/authoring/producers"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

func TestWebURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/o/r":       "https://github.com/o/r",
		"https://github.com/o/r.git":   "https://github.com/o/r",
		"https://github.com/o/r/":      "https://github.com/o/r",
		"git@github.com:o/r.git":       "https://github.com/o/r",
		"ssh://git@github.com/o/r.git": "https://github.com/o/r",
		"https://gitlab.com/o/r.git":   "",
		"/srv/git/r.git":               "",
		"https://github.com/o/r/extra": "",
		"  git@github.com:o/r.git\n":   "https://github.com/o/r",
	}
	for remote, want := range cases {
		if got := WebURL(remote); got != want {
			t.Errorf("WebURL(%q) = %q, want %q", remote, got, want)
		}
	}
}

// recorder is a producer that keeps the bundle it received and answers as
// the mechanical producer does.
type recorder struct{ received []byte }

func (r *recorder) Produce(ctx context.Context, evidence []byte) ([]byte, error) {
	r.received = evidence
	return producers.Mechanical{}.Produce(ctx, evidence)
}

// repo is a repository released at v1.8.0 with one feature since that adds
// a file, and a configuration selecting the recorder as its producer.
func repo(t *testing.T) (string, config.Config, *recorder) {
	t.Helper()
	gitBin, err := exec.LookPath("git")
	if err != nil {
		t.Skip("git not on PATH")
	}
	for k, v := range map[string]string{
		"GIT_CONFIG_GLOBAL": os.DevNull, "GIT_CONFIG_NOSYSTEM": "1",
		"GIT_AUTHOR_NAME": "Fixture", "GIT_AUTHOR_EMAIL": "fixture@example.com",
		"GIT_COMMITTER_NAME": "Fixture", "GIT_COMMITTER_EMAIL": "fixture@example.com",
	} {
		t.Setenv(k, v)
	}
	dir := t.TempDir()
	run := func(args ...string) {
		t.Helper()
		if out, err := exec.Command(gitBin, append([]string{"-C", dir}, args...)...).CombinedOutput(); err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
	}
	run("init", "-q", "-b", "main")
	run("commit", "-q", "--allow-empty", "-m", "Initial commit")
	run("tag", "-a", "-m", "v1.8.0", "v1.8.0")
	if err := os.WriteFile(filepath.Join(dir, "greet.txt"), []byte("Hello.\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	run("add", "greet.txt")
	run("commit", "-q", "-m", "feat: greet the user")
	cfg := config.Default()
	cfg.Changelog.Producer = "recorder"
	return dir, cfg, &recorder{}
}

func prepare(t *testing.T, dir string, cfg config.Config, p authoring.Producer) Result {
	t.Helper()
	res, err := Prepare(context.Background(), Request{
		Dir: dir, Target: "main", Config: cfg, Producers: map[string]authoring.Producer{"recorder": p},
	})
	if err != nil {
		t.Fatalf("Prepare: %v", err)
	}
	if res.Outcome != Prepared || res.Version != "1.9.0" {
		t.Fatalf("outcome %v, version %q; want 1.9.0 prepared", res.Outcome, res.Version)
	}
	return res
}

// received decodes the bundle the producer was given.
func (r *recorder) bundle(t *testing.T) releasemodel.Evidence {
	t.Helper()
	var ev releasemodel.Evidence
	if err := json.Unmarshal(r.received, &ev); err != nil {
		t.Fatalf("the producer received no bundle: %v\n%s", err, r.received)
	}
	return ev
}

func adapters(ev releasemodel.Evidence) []releasemodel.Adapter {
	var out []releasemodel.Adapter
	for _, s := range ev.Sources {
		out = append(out, s.Adapter)
	}
	return out
}

// AC: AC-CLOG-001.9
func TestDiffsAreCollectedByDefault(t *testing.T) {
	dir, cfg, p := repo(t)
	prepare(t, dir, cfg, p)
	ev := p.bundle(t)
	if len(ev.Commits) != 1 || ev.Commits[0].Diff == nil || len(ev.Commits[0].Diff.Files) != 1 ||
		!strings.Contains(ev.Commits[0].Diff.Files[0].Patch, "+Hello.") {
		t.Fatalf("commits = %+v, want the feature carrying its diff", ev.Commits)
	}
	if got := adapters(ev); len(got) != 2 || got[1] != releasemodel.AdapterDiffs {
		t.Errorf("adapters = %v, want COMMITS and DIFFS", got)
	}
	if ev.Budget.Limit != config.DefaultDiffBudget {
		t.Errorf("budget limit = %d, want changelog.diff_budget.release's default %d", ev.Budget.Limit, config.DefaultDiffBudget)
	}
}

// AC: AC-CFG-003.5
func TestTurningDiffsOffLeavesThemOutOfTheEvidence(t *testing.T) {
	dir, cfg, p := repo(t)
	cfg.Adapters.Diffs = false
	prepare(t, dir, cfg, p)
	ev := p.bundle(t)
	if len(ev.Commits) != 1 || ev.Commits[0].Diff != nil {
		t.Errorf("commits = %+v, want no diff", ev.Commits)
	}
	if got := adapters(ev); len(got) != 1 || got[0] != releasemodel.AdapterCommits {
		t.Errorf("adapters = %v, want COMMITS only", got)
	}
}

// AC: AC-CLOG-001.13
func TestTheHostingPlatformIsNotConsultedByDefault(t *testing.T) {
	dir, cfg, p := repo(t)
	gitBin, _ := exec.LookPath("git")
	// PATH holds git and nothing else: no gh, curl or any other client the
	// operation could reach the hosting platform through.
	bin := t.TempDir()
	if err := os.Symlink(gitBin, filepath.Join(bin, filepath.Base(gitBin))); err != nil {
		t.Skipf("cannot link git: %v", err)
	}
	t.Setenv("PATH", bin)
	if cfg.Adapters.PullRequests {
		t.Fatal("the default configuration enables the pull request adapter")
	}
	prepare(t, dir, cfg, p)
	if ev := p.bundle(t); ev.Commits[0].PullRequest != nil {
		t.Errorf("pull request = %+v, want none: the commit carries none and nothing else was asked", ev.Commits[0].PullRequest)
	}
}
