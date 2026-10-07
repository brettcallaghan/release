package gitread

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// fixture is a real repository built with git in a temporary directory,
// isolated from the user's and the system's git configuration.
type fixture struct {
	t   *testing.T
	dir string
	n   int
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

// commit adds an empty commit on the current branch and tags it with each
// name given; the first tag is annotated, to cover both tag kinds.
func (f *fixture) commit(tags ...string) {
	f.t.Helper()
	f.n++
	f.git("commit", "-q", "--allow-empty", "-m", "commit "+string(rune('a'+f.n)))
	for i, tag := range tags {
		if i == 0 {
			f.git("tag", "-a", "-m", tag, tag)
		} else {
			f.git("tag", tag)
		}
	}
}

func (f *fixture) repo() Repo { return Repo{Dir: f.dir} }

func (f *fixture) latest(ref, prefix string) (ReleaseTag, bool) {
	f.t.Helper()
	tag, found, err := f.repo().LatestReleaseTag(context.Background(), ref, prefix)
	if err != nil {
		f.t.Fatalf("LatestReleaseTag(%q, %q): %v", ref, prefix, err)
	}
	return tag, found
}

func wantLatest(t *testing.T, f *fixture, ref, prefix, wantName, wantVersion string) {
	t.Helper()
	tag, found := f.latest(ref, prefix)
	if !found {
		t.Fatalf("LatestReleaseTag(%q, %q): no release tag found, want %s", ref, prefix, wantName)
	}
	if tag.Name != wantName || tag.Version.String() != wantVersion {
		t.Errorf("LatestReleaseTag(%q, %q) = %s (%s), want %s (%s)", ref, prefix, tag.Name, tag.Version, wantName, wantVersion)
	}
}

// AC: AC-VER-001.1
func TestLatestIsHighestReachableReleaseTag(t *testing.T) {
	f := newFixture(t)
	f.commit("1.6.0")
	f.commit("1.8.0")
	f.commit("1.7.0") // tagged later in history but lower in version
	wantLatest(t, f, "main", "", "1.8.0", "1.8.0")

	tags, err := f.repo().ReleaseTags(context.Background(), "main", "")
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, tag := range tags {
		names = append(names, tag.Name)
	}
	if got := strings.Join(names, " "); got != "1.6.0 1.7.0 1.8.0" {
		t.Errorf("ReleaseTags = %q, want ascending 1.6.0 1.7.0 1.8.0", got)
	}
}

// AC: AC-VER-001.2
func TestUnreachableTagIsNotConsidered(t *testing.T) {
	f := newFixture(t)
	f.commit("1.6.0")
	f.commit("1.7.0")
	f.commit("1.8.0")
	f.git("branch", "line-1")
	f.commit("2.0.0") // on main only, after line-1 was cut
	wantLatest(t, f, "line-1", "", "1.8.0", "1.8.0")
	wantLatest(t, f, "main", "", "2.0.0", "2.0.0")
}

// AC: AC-VER-001.3
func TestNonReleaseTagsAreIgnored(t *testing.T) {
	f := newFixture(t)
	f.commit("1.8.0")
	f.commit("nightly-2026-10-01", "v1.9.0-rc.1", "1.9.0-rc.2", "1.9.0+build.1",
		"1.9.0-canary.12.abc1234", "v1.9", "latest")
	wantLatest(t, f, "main", "v", "1.8.0", "1.8.0")
	wantLatest(t, f, "main", "", "1.8.0", "1.8.0")
}

// AC: AC-VER-001.4
func TestNoReachableReleaseTagIsAbsenceNotError(t *testing.T) {
	f := newFixture(t)
	f.commit()
	f.commit("nightly-2026-10-01", "v1.0.0-rc.1")
	tag, found, err := f.repo().LatestReleaseTag(context.Background(), "main", "v")
	if err != nil {
		t.Fatalf("err = %v, want nil", err)
	}
	if found {
		t.Fatalf("found %s, want no release tag", tag.Name)
	}
	if tag.Name != "" {
		t.Errorf("tag = %+v, want the zero value", tag)
	}
	tags, err := f.repo().ReleaseTags(context.Background(), "main", "v")
	if err != nil || len(tags) != 0 {
		t.Errorf("ReleaseTags = %v, %v; want empty, nil", tags, err)
	}
}

func TestNoTagsAtAll(t *testing.T) {
	f := newFixture(t)
	f.commit()
	if tag, found := f.latest("main", "v"); found {
		t.Errorf("found %s in a repository with no tags", tag.Name)
	}
}

// AC: AC-VER-001.7
func TestTagRecognisedWithOrWithoutPrefix(t *testing.T) {
	f := newFixture(t)
	f.commit("v1.8.0")
	wantLatest(t, f, "main", "v", "v1.8.0", "1.8.0")

	g := newFixture(t)
	g.commit("v1.7.0")
	g.commit("1.8.0")
	wantLatest(t, g, "main", "v", "1.8.0", "1.8.0")
}

func TestSameVersionPrefersPrefixedTag(t *testing.T) {
	for _, order := range [][]string{{"1.8.0", "v1.8.0"}, {"v1.8.0", "1.8.0"}} {
		f := newFixture(t)
		f.commit(order...)
		wantLatest(t, f, "main", "v", "v1.8.0", "1.8.0")
		tags, err := f.repo().ReleaseTags(context.Background(), "main", "v")
		if err != nil || len(tags) != 1 {
			t.Errorf("ReleaseTags = %v, %v; want one entry for 1.8.0", tags, err)
		}
	}
}

// A prefix can itself begin a bare version's spelling: with prefix "1", the
// derived name of 1.2.3 is "11.2.3", and the bare "1.2.3" also starts with
// "1". The derived name still wins.
func TestSameVersionPrefersExactDerivedName(t *testing.T) {
	for _, order := range [][]string{{"1.2.3", "11.2.3"}, {"11.2.3", "1.2.3"}} {
		f := newFixture(t)
		f.commit(order...)
		wantLatest(t, f, "main", "1", "11.2.3", "1.2.3")
	}
}

func TestBranchNameDoesNotConstrainMajor(t *testing.T) {
	f := newFixture(t)
	f.commit("1.9.0")
	f.commit("2.3.0")
	f.git("branch", "release/v1")
	wantLatest(t, f, "release/v1", "v", "2.3.0", "2.3.0")
}

func TestUnknownRefIsGitError(t *testing.T) {
	f := newFixture(t)
	f.commit("1.0.0")
	_, _, err := f.repo().LatestReleaseTag(context.Background(), "no-such-branch", "v")
	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("err = %v, want *GitError", err)
	}
	if errors.Is(err, ErrGitUnavailable) || !strings.Contains(gitErr.Error(), "for-each-ref") {
		t.Errorf("err = %v", err)
	}
}

func TestMissingGitIsUnavailable(t *testing.T) {
	r := Repo{Dir: t.TempDir(), Git: filepath.Join(t.TempDir(), "no-such-git")}
	_, _, err := r.LatestReleaseTag(context.Background(), "main", "v")
	if !errors.Is(err, ErrGitUnavailable) {
		t.Fatalf("err = %v, want ErrGitUnavailable", err)
	}
}

func TestCancelledContextIsReported(t *testing.T) {
	f := newFixture(t)
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	_, _, err := f.repo().LatestReleaseTag(ctx, "main", "v")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("err = %v, want context.Canceled", err)
	}
}

func TestParseReleaseTag(t *testing.T) {
	cases := []struct {
		prefix, name string
		ok           bool
		version      string
	}{
		{"v", "v1.8.0", true, "1.8.0"},
		{"v", "1.8.0", true, "1.8.0"},
		{"", "1.8.0", true, "1.8.0"},
		{"", "v1.8.0", false, ""},
		{"release-", "release-2.0.0", true, "2.0.0"},
		{"v", "vv1.8.0", false, ""},
		{"v", "v1.8.0-rc.1", false, ""},
		{"v", "v1.8.0+meta", false, ""},
		{"v", "nightly-2026-10-01", false, ""},
	}
	for _, c := range cases {
		tag, ok := ParseReleaseTag(c.prefix, c.name)
		if ok != c.ok || (ok && (tag.Version.String() != c.version || tag.Name != c.name)) {
			t.Errorf("ParseReleaseTag(%q, %q) = %+v, %v; want %s, %v", c.prefix, c.name, tag, ok, c.version, c.ok)
		}
	}
}

func TestGitErrorMessage(t *testing.T) {
	e := &GitError{Args: []string{"x"}, ExitCode: 3}
	if got := e.Error(); got != "git x: exit status 3: no output" {
		t.Errorf("Error() = %q", got)
	}
}
