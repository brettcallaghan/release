package resolve

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/version"
)

type fixture struct {
	t   *testing.T
	dir string
}

func newFixture(t *testing.T) *fixture {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	f := &fixture{t: t, dir: t.TempDir()}
	f.git("init", "-q", "-b", "main")
	return f
}

func (f *fixture) git(args ...string) string {
	f.t.Helper()
	cmd := exec.Command("git", append([]string{"-C", f.dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull,
		"GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		f.t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
	return strings.TrimSpace(string(out))
}

func (f *fixture) commit(msg string) string {
	f.t.Helper()
	f.git("commit", "-q", "--allow-empty", "-m", msg)
	return f.git("rev-parse", "HEAD")
}

// state is every ref and the working tree's status, to prove a run wrote
// nothing.
func (f *fixture) state() string {
	return f.git("for-each-ref") + "\n" + f.git("status", "--porcelain", "--ignored") + "\n" + f.git("rev-parse", "HEAD")
}

func (f *fixture) resolve(named *releasemodel.Bump) Resolution {
	f.t.Helper()
	res, err := Resolve(context.Background(), Request{Dir: f.dir, Target: "HEAD", Named: named, Policy: DefaultPolicy()})
	if err != nil {
		f.t.Fatalf("Resolve: %v", err)
	}
	return res
}

func TestFeatAfterTagResolvesMinor(t *testing.T) {
	f := newFixture(t)
	f.commit("initial")
	f.git("tag", "-a", "-m", "v1.8.0", "v1.8.0")
	feat := f.commit("feat: add a flag")
	before := f.state()

	res := f.resolve(nil)

	if res.Base == nil || res.Base.String() != "1.8.0" || res.BaseTag != "v1.8.0" {
		t.Fatalf("base = %v (%q), want 1.8.0 (v1.8.0)", res.Base, res.BaseTag)
	}
	if res.Impact != releasemodel.BumpMinor || res.Named {
		t.Errorf("impact = %s (named %v), want inferred MINOR", res.Impact, res.Named)
	}
	want := []Driver{{Hash: feat, Subject: "feat: add a flag", Signal: bump.SignalConventionalType}}
	if len(res.DeterminedBy) != 1 || res.DeterminedBy[0] != want[0] {
		t.Errorf("determined by %+v, want %+v", res.DeterminedBy, want)
	}
	if res.Next.Kind != version.Release || res.Next.Version.String() != "1.9.0" {
		t.Errorf("next = %+v, want release 1.9.0", res.Next)
	}
	if after := f.state(); after != before {
		t.Errorf("the run changed the repository:\nbefore:\n%s\nafter:\n%s", before, after)
	}
}

// AC: AC-VER-001.6
func TestNoTagsResolvesZero(t *testing.T) {
	f := newFixture(t)
	f.commit("fix: handle empty input")
	res := f.resolve(nil)
	if res.Base != nil || res.BaseTag != "" {
		t.Fatalf("base = %v, want none", res.Base)
	}
	if res.Impact != releasemodel.BumpPatch {
		t.Errorf("impact = %s, want PATCH", res.Impact)
	}
	if res.Next.Kind != version.Release || res.Next.Version.String() != "0.0.0" {
		t.Errorf("next = %+v, want release 0.0.0", res.Next)
	}
}

func TestNamedBumpIsReportedAsNamed(t *testing.T) {
	f := newFixture(t)
	f.commit("initial")
	f.git("tag", "v2.0.0")
	f.commit("docs: readme")
	major := releasemodel.BumpMajor
	res := f.resolve(&major)
	if !res.Named || res.Impact != releasemodel.BumpMajor || len(res.DeterminedBy) != 0 {
		t.Fatalf("got %+v, want named MAJOR with no drivers", res)
	}
	if res.Next.Version.String() != "3.0.0" {
		t.Errorf("next = %s, want 3.0.0", res.Next.Version)
	}
}

func TestNoSignalIsNoRelease(t *testing.T) {
	f := newFixture(t)
	f.commit("initial")
	f.git("tag", "v1.0.0")
	f.commit("docs: readme")
	if res := f.resolve(nil); res.Next.Kind != version.NoRelease || res.Impact != releasemodel.BumpNone {
		t.Fatalf("got %+v, want no release", res)
	}
}

func TestGitFailuresAreErrors(t *testing.T) {
	f := newFixture(t)
	f.commit("initial")
	_, err := Resolve(context.Background(), Request{Dir: f.dir, Target: "no-such-ref", Policy: DefaultPolicy()})
	var gitErr *gitread.GitError
	if !errors.As(err, &gitErr) {
		t.Errorf("unknown target: err = %v, want a *GitError", err)
	}

	_, err = Resolve(context.Background(), Request{Dir: f.dir, Git: filepath.Join(t.TempDir(), "no-git"), Target: "HEAD", Policy: DefaultPolicy()})
	if !errors.Is(err, gitread.ErrGitUnavailable) {
		t.Errorf("missing git: err = %v, want ErrGitUnavailable", err)
	}
}

func TestDefaultPolicy(t *testing.T) {
	p := DefaultPolicy()
	if p.TagPrefix != "v" || p.StagePrefix != "cut/" || p.Version.Leaps || !p.Version.Pre1Remap {
		t.Errorf("default policy = %+v", p)
	}
	for _, s := range []bump.Signal{bump.SignalReleaseType, bump.SignalConventionalType, bump.SignalBreakingMarker} {
		if !p.Signals[s] {
			t.Errorf("signal %s is not enabled by default", s)
		}
	}
}

func TestToImpact(t *testing.T) {
	for b, want := range map[releasemodel.Bump]version.Impact{
		releasemodel.BumpNone:  version.ImpactNone,
		releasemodel.BumpPatch: version.ImpactPatch,
		releasemodel.BumpMinor: version.ImpactMinor,
		releasemodel.BumpMajor: version.ImpactMajor,
	} {
		if got, err := toImpact(b); err != nil || got != want {
			t.Errorf("toImpact(%s) = %v, %v; want %v", b, got, err, want)
		}
	}
	if _, err := toImpact("BOGUS"); err == nil {
		t.Error("an unknown bump must be an error")
	}
}

func TestUnknownNamedBumpIsAnError(t *testing.T) {
	f := newFixture(t)
	f.commit("initial")
	bogus := releasemodel.Bump("BOGUS")
	if _, err := Resolve(context.Background(), Request{Dir: f.dir, Target: "HEAD", Named: &bogus, Policy: DefaultPolicy()}); err == nil {
		t.Error("want an error for an unknown named bump")
	}
}
