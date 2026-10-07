package corpus

import (
	"context"
	"os"
	"os/exec"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// The integration tier: a fixture repository read by the real Commit
// Collector, assembled by the builder.

func gitFixture(t *testing.T) (dir string, git func(args ...string) string) {
	t.Helper()
	dir = t.TempDir()
	git = func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
		cmd.Env = append(os.Environ(),
			"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
			"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
			"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
		)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
		}
		return strings.TrimSpace(string(out))
	}
	git("init", "-q", "-b", "main")
	return dir, git
}

func assemble(t *testing.T, dir, from, to string) releasemodel.Changelog {
	t.Helper()
	recs, err := gitread.Repo{Dir: dir}.Commits(context.Background(), from, to)
	if err != nil {
		t.Fatal(err)
	}
	return build(t, Input{Records: recs, Range: Range{From: from, To: to}})
}

// AC: AC-CLOG-001.1
func TestPullRequestNumberFromASquashSubjectReachesTheEntry(t *testing.T) {
	dir, git := gitFixture(t)
	git("commit", "-q", "--allow-empty", "-m", "feat: add a --target flag (#12)")
	head := git("rev-parse", "HEAD")

	c := assemble(t, dir, "", head)
	if len(c.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(c.Entries))
	}
	pr, ok := c.Entries[0].Sources[0].AsPullRequest()
	if !ok || pr.Number != 12 {
		t.Errorf("sources = %+v, want pull request 12", c.Entries[0].Sources)
	}
	if c.Entries[0].Summary != "add a --target flag" {
		t.Errorf("summary = %q", c.Entries[0].Summary)
	}
}

// AC: AC-CLOG-001.2
func TestADescriptionCarriedInTheBodyIsCollectedWithoutThePlatform(t *testing.T) {
	dir, git := gitFixture(t)
	git("commit", "-q", "--allow-empty", "-m", "Initial commit")
	base := git("rev-parse", "HEAD")
	desc := "## Summary\n\nWhy this matters.\n\n## Release notes\n\nA maintainer can compute a version for any release line.\n\nPass `--target <branch>`."
	git("commit", "-q", "--allow-empty", "--cleanup=verbatim", "-m", "feat(cli): add --target (#12)\n\n"+desc)
	head := git("rev-parse", "HEAD")

	// No hosting platform is configured or reachable from the fixture: the
	// prose can only have come from the commit body.
	c := assemble(t, dir, base, head)
	e := c.Entries[0]
	if e.Summary != "A maintainer can compute a version for any release line." || deref(e.Detail) != "Pass `--target <branch>`." || e.Derived {
		t.Errorf("entry = summary %q, detail %q, derived %v", e.Summary, deref(e.Detail), e.Derived)
	}
	if len(c.Sources) != 1 || c.Sources[0].Adapter != releasemodel.AdapterCommits {
		t.Errorf("sources = %+v, want the commit adapter alone", c.Sources)
	}
}

// AC: AC-CLOG-001.6
func TestACommitWithNoPullRequestIsStillCollected(t *testing.T) {
	dir, git := gitFixture(t)
	git("commit", "-q", "--allow-empty", "-m", "fix: pushed straight to main")
	head := git("rev-parse", "HEAD")

	c := assemble(t, dir, "", head)
	if len(c.Entries) != 1 {
		t.Fatalf("entries = %d, want 1", len(c.Entries))
	}
	src, ok := c.Entries[0].Sources[0].AsCommit()
	if !ok || src.Hash != head {
		t.Errorf("sources = %+v, want commit %s", c.Entries[0].Sources, head)
	}
}
