package gitread

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

// commitMsg adds an empty commit with the full message given.
func (f *fixture) commitMsg(msg string) string {
	f.t.Helper()
	f.git("commit", "-q", "--allow-empty", "--cleanup=verbatim", "-m", msg)
	return f.git("rev-parse", "HEAD")
}

func (f *fixture) commits(base, ref string) []releasemodel.CommitRecord {
	f.t.Helper()
	recs, err := f.repo().Commits(context.Background(), base, ref)
	if err != nil {
		f.t.Fatalf("Commits(%q, %q): %v", base, ref, err)
	}
	return recs
}

func TestCommitsBetweenBaseTagAndRefNewestFirst(t *testing.T) {
	f := newFixture(t)
	f.commit("v1.0.0")
	a := f.commitMsg("first after tag")
	b := f.commitMsg("second after tag")
	c := f.commitMsg("third after tag")

	recs := f.commits("v1.0.0", "HEAD")
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3: %+v", len(recs), recs)
	}
	for i, want := range []string{c, b, a} {
		if recs[i].Hash != want {
			t.Errorf("record %d hash = %s, want %s", i, recs[i].Hash, want)
		}
	}
	if recs[0].Subject != "third after tag" || recs[0].AuthorName != "Fixture" || recs[0].AuthorEmail != "fixture@example.com" {
		t.Errorf("record 0 = %+v", recs[0])
	}
}

func TestCommitsWithoutBaseCoverEverythingReachable(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("one")
	f.commitMsg("two")
	if recs := f.commits("", "HEAD"); len(recs) != 2 {
		t.Fatalf("got %d records, want 2", len(recs))
	}
}

func TestCommitsPullRequestNumberFromSquashSubject(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("Fix the parser (#87)\n\nThe pull request description.\n")
	f.commitMsg("Direct push to the release line")
	recs := f.commits("", "HEAD")

	direct, squash := recs[0], recs[1]
	if direct.PullRequest != nil {
		t.Errorf("direct commit pull_request = %d, want absent", *direct.PullRequest)
	}
	if squash.PullRequest == nil || *squash.PullRequest != 87 {
		t.Fatalf("squash pull_request = %v, want 87", squash.PullRequest)
	}
	if squash.Subject != "Fix the parser (#87)" {
		t.Errorf("squash subject = %q, subject must be left unchanged", squash.Subject)
	}
	if squash.Body != "The pull request description.\n" {
		t.Errorf("squash body = %q", squash.Body)
	}
}

func TestCommitsSquashWithoutDescription(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("Add a flag (#9)")
	rec := f.commits("", "HEAD")[0]
	if rec.Body != "" || len(rec.Trailers) != 0 {
		t.Errorf("body = %q, trailers = %v; want both empty", rec.Body, rec.Trailers)
	}
	if rec.PullRequest == nil || *rec.PullRequest != 9 {
		t.Errorf("pull_request = %v, want 9", rec.PullRequest)
	}
}

func TestCommitsTrailersFromFinalParagraph(t *testing.T) {
	f := newFixture(t)
	body := "Some prose.\n\nNot-A-Trailer: this paragraph is not the last\n\nRelease-Type: patch\nCo-Authored-By: A <a@example.com>\nRelease-Type: minor\n"
	f.commitMsg("Change something\n\n" + body)
	rec := f.commits("", "HEAD")[0]

	if got := rec.Trailers["Release-Type"]; got != "minor" {
		t.Errorf("Release-Type = %q, want minor (last value wins)", got)
	}
	if got := rec.Trailers["Co-Authored-By"]; got != "A <a@example.com>" {
		t.Errorf("Co-Authored-By = %q", got)
	}
	if _, ok := rec.Trailers["Not-A-Trailer"]; ok {
		t.Errorf("a key outside the final paragraph was parsed as a trailer")
	}
	if rec.Body != body {
		t.Errorf("body = %q, want it verbatim %q", rec.Body, body)
	}
}

func TestCommitsIncludeMergeCommits(t *testing.T) {
	f := newFixture(t)
	f.commit("v1.0.0")
	f.git("checkout", "-q", "-b", "topic")
	f.commitMsg("topic work")
	f.git("checkout", "-q", "main")
	f.commitMsg("main work")
	f.git("merge", "-q", "--no-ff", "-m", "Merge branch 'topic'", "topic")

	recs := f.commits("v1.0.0", "main")
	if len(recs) != 3 {
		t.Fatalf("got %d records, want 3 (merge, main work, topic work)", len(recs))
	}
	if recs[0].Subject != "Merge branch 'topic'" {
		t.Errorf("newest record = %q, want the merge commit", recs[0].Subject)
	}
}

func TestCommitsUnknownRefIsGitError(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("one")
	_, err := f.repo().Commits(context.Background(), "no-such-tag", "HEAD")
	var gitErr *GitError
	if !errors.As(err, &gitErr) {
		t.Fatalf("err = %v, want a *GitError", err)
	}
}

func TestCommitsGitMissing(t *testing.T) {
	r := Repo{Dir: t.TempDir(), Git: filepath.Join(t.TempDir(), "no-git")}
	_, err := r.Commits(context.Background(), "", "HEAD")
	if !errors.Is(err, ErrGitUnavailable) {
		t.Fatalf("err = %v, want ErrGitUnavailable", err)
	}
}

func TestParseCommitRejectsMalformedRecord(t *testing.T) {
	if _, err := parseCommit("only" + fieldSep + "two"); err == nil {
		t.Fatal("want an error for a record with the wrong field count")
	}
}

func TestPullRequestNumberOverflowIsAbsent(t *testing.T) {
	if n := pullRequestNumber("x (#99999999999999999999)"); n != nil {
		t.Errorf("got %d, want absent", *n)
	}
}

func TestDefaultBranchIsWhereOriginHEADPoints(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("one")
	if _, found, err := f.repo().DefaultBranch(context.Background()); err != nil || found {
		t.Fatalf("origin/HEAD unset: found = %v, err = %v; want not found, no error", found, err)
	}
	f.git("update-ref", "refs/remotes/origin/trunk", "HEAD")
	f.git("symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")
	ref, found, err := f.repo().DefaultBranch(context.Background())
	if err != nil || !found || ref != "refs/remotes/origin/trunk" {
		t.Fatalf("got %q, %v, %v; want refs/remotes/origin/trunk", ref, found, err)
	}
}

func TestDefaultBranchOutsideARepositoryIsAnError(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	_, _, err := Repo{Dir: filepath.Join(t.TempDir(), "missing")}.DefaultBranch(context.Background())
	if err == nil {
		t.Fatal("want an error outside a repository")
	}
}

func TestToplevelIsTheWorkingTreeRoot(t *testing.T) {
	f := newFixture(t)
	sub := filepath.Join(f.dir, "a", "b")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	dir, found, err := Repo{Dir: sub}.Toplevel(context.Background())
	if err != nil || !found {
		t.Fatalf("Toplevel = %q, %v, %v; want found", dir, found, err)
	}
	want, _ := filepath.EvalSymlinks(f.dir)
	if got, _ := filepath.EvalSymlinks(dir); got != want {
		t.Errorf("Toplevel = %q, want %q", got, want)
	}

	if _, found, err := (Repo{Dir: t.TempDir()}).Toplevel(context.Background()); err != nil || found {
		t.Errorf("outside a working tree: found = %v, err = %v; want not found and no error", found, err)
	}
}
