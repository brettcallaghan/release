package gitread

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
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
		t.Errorf("direct commit pull_request = %v, want absent", direct.PullRequest)
	}
	if squash.PullRequest == nil || squash.PullRequest.Number != 87 || squash.PullRequest.Origin != releasemodel.PullRequestOriginSquash {
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
	if rec.PullRequest == nil || rec.PullRequest.Number != 9 {
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

// write puts content at path in the fixture's working tree.
func (f *fixture) write(path, content string) {
	f.t.Helper()
	full := filepath.Join(f.dir, path)
	if err := os.MkdirAll(filepath.Dir(full), 0o755); err != nil {
		f.t.Fatal(err)
	}
	if err := os.WriteFile(full, []byte(content), 0o644); err != nil {
		f.t.Fatal(err)
	}
}

// commitAll stages everything and commits it with msg.
func (f *fixture) commitAll(msg string) {
	f.t.Helper()
	f.git("add", "-A")
	f.git("commit", "-q", "--cleanup=verbatim", "-m", msg)
}

func strp(p *string) string {
	if p == nil {
		return "<nil>"
	}
	return *p
}

// AC: AC-CLOG-001.1
func TestCommitsSquashPullRequestTitleAndDescription(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("Fix the parser (#87)\n\nFirst paragraph.\n\nSecond paragraph.\n\nCo-Authored-By: A <a@example.com>\n")
	pr := f.commits("", "HEAD")[0].PullRequest
	if pr == nil || pr.Number != 87 || pr.Origin != releasemodel.PullRequestOriginSquash {
		t.Fatalf("pull_request = %+v", pr)
	}
	if strp(pr.Title) != "Fix the parser" {
		t.Errorf("title = %q", strp(pr.Title))
	}
	if strp(pr.Description) != "First paragraph.\n\nSecond paragraph." {
		t.Errorf("description = %q, want the body less its trailer paragraph", strp(pr.Description))
	}
}

// AC: AC-CLOG-001.1
func TestCommitsSquashWithoutDescriptionHasNoDescription(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("Fix the parser (#87)\n\nCo-Authored-By: A <a@example.com>\n")
	f.commitMsg("Bare squash (#88)")
	recs := f.commits("", "HEAD")
	for _, rec := range recs {
		if rec.PullRequest == nil || rec.PullRequest.Description != nil {
			t.Errorf("%q: description = %v, want absent", rec.Subject, rec.PullRequest)
		}
	}
	if strp(recs[1].PullRequest.Title) != "Fix the parser" {
		t.Errorf("title = %q", strp(recs[1].PullRequest.Title))
	}
}

// AC: AC-CLOG-001.14
func TestCommitsMergeCommitPullRequest(t *testing.T) {
	f := newFixture(t)
	f.commit("v1.0.0")
	f.git("checkout", "-q", "-b", "topic")
	f.commitMsg("topic work")
	f.git("checkout", "-q", "main")
	f.commitMsg("main work")
	f.git("merge", "-q", "--no-ff", "--cleanup=verbatim", "-m",
		"Merge pull request #42 from owner/topic\n\nAdd the thing\n\nIt does X.\n\nAnd Y.\n\nSigned-off-by: A <a@example.com>\n", "topic")

	rec := f.commits("v1.0.0", "main")[0]
	pr := rec.PullRequest
	if pr == nil || pr.Number != 42 || pr.Origin != releasemodel.PullRequestOriginMerge {
		t.Fatalf("pull_request = %+v", pr)
	}
	if strp(pr.Title) != "Add the thing" {
		t.Errorf("title = %q", strp(pr.Title))
	}
	if strp(pr.Description) != "It does X.\n\nAnd Y." {
		t.Errorf("description = %q", strp(pr.Description))
	}
	if rec.Subject != "Merge pull request #42 from owner/topic" {
		t.Errorf("subject = %q, want it unchanged", rec.Subject)
	}
	if len(rec.Files) != 0 {
		t.Errorf("merge commit files = %v, want none", rec.Files)
	}
}

// AC: AC-CLOG-001.6
func TestCommitsDirectCommitHasNoPullRequest(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("Merge branch 'topic' into main")
	f.commitMsg("Direct push (see #5)")
	for _, rec := range f.commits("", "HEAD") {
		if rec.PullRequest != nil {
			t.Errorf("%q: pull_request = %+v, want absent", rec.Subject, rec.PullRequest)
		}
	}
}

// AC: AC-CLOG-001.2
func TestCommitsSquashBodyKeptVerbatim(t *testing.T) {
	f := newFixture(t)
	body := "Line one.\n\n  Indented second.\n\nCo-Authored-By: A <a@example.com>\n"
	f.commitMsg("Subject (#9)\n\n" + body)
	rec := f.commits("", "HEAD")[0]
	if rec.Body != body {
		t.Errorf("body = %q, want %q", rec.Body, body)
	}
}

// AC: AC-CLOG-001.15
func TestCommitsAuthorDateAndConventionalParts(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("feat(cut)!: rename --dry to --dry-run\n\nRelease-Type: major\n")
	f.commitMsg("fix: plain fix\n\nBody.\n\nBREAKING CHANGE: the flag moved\n")
	f.commitMsg("Update the README")
	f.commitMsg("Not conventional\n\nBREAKING CHANGE: ignored without a conventional subject\n")
	f.commitMsg("chore(deps-dev): bump")
	recs := f.commits("", "HEAD")

	chore, nonConv, readme, fix, feat := recs[0], recs[1], recs[2], recs[3], recs[4]
	if feat.AuthorDate != "2026-01-01T00:00:00+00:00" {
		t.Errorf("author_date = %q", feat.AuthorDate)
	}
	c := feat.Conventional
	if c == nil || c.Type != "feat" || strp(c.Scope) != "cut" || !c.Breaking {
		t.Errorf("feat conventional = %+v", c)
	}
	if feat.Trailers["Release-Type"] != "major" {
		t.Errorf("trailers = %v", feat.Trailers)
	}
	if c := fix.Conventional; c == nil || c.Type != "fix" || c.Scope != nil || !c.Breaking {
		t.Errorf("fix conventional = %+v, want breaking from the body footer", c)
	}
	if c := chore.Conventional; c == nil || strp(c.Scope) != "deps-dev" || c.Breaking {
		t.Errorf("chore conventional = %+v", c)
	}
	if readme.Conventional != nil || nonConv.Conventional != nil {
		t.Errorf("non-conventional subjects carry %+v / %+v", readme.Conventional, nonConv.Conventional)
	}
}

// AC: AC-CLOG-001.8
func TestCommitsChangedFiles(t *testing.T) {
	f := newFixture(t)
	big := strings.Repeat("a line of content\n", 20)
	f.write("keep.txt", "one\ntwo\n")
	f.write("old/name.txt", big)
	f.write("gone.txt", "bye\n")
	f.write("blob.bin", "\x00\x01\x02")
	f.commitAll("base")
	f.git("tag", "v1.0.0")

	f.write("keep.txt", "one\nthree\nfour\n")
	f.write("added.txt", "new\nfile\n")
	if err := os.MkdirAll(filepath.Join(f.dir, "new"), 0o755); err != nil {
		t.Fatal(err)
	}
	f.git("mv", "old/name.txt", "new/name.txt")
	f.write("new/name.txt", big+"extra\n")
	f.git("rm", "-q", "gone.txt")
	f.write("blob.bin", "\x00\x03\x04\x05")
	f.write("added.bin", "\x00\x09")
	f.commitAll("change everything")

	rec := f.commits("v1.0.0", "HEAD")[0]
	got := map[string]releasemodel.FileChange{}
	for _, fc := range rec.Files {
		got[fc.Path] = fc
	}
	if len(got) != 6 {
		t.Fatalf("files = %+v, want 6", rec.Files)
	}
	check := func(path string, status releasemodel.FileStatus, prev string, add, del int64) {
		t.Helper()
		fc, ok := got[path]
		if !ok {
			t.Errorf("%s missing", path)
			return
		}
		if fc.Status != status || fc.Additions != add || fc.Deletions != del || (prev == "") != (fc.PreviousPath == nil) || (prev != "" && *fc.PreviousPath != prev) {
			t.Errorf("%s = %+v (previous %s), want %s +%d -%d previous %q", path, fc, strp(fc.PreviousPath), status, add, del, prev)
		}
	}
	check("keep.txt", releasemodel.FileStatusModified, "", 2, 1)
	check("added.txt", releasemodel.FileStatusAdded, "", 2, 0)
	check("new/name.txt", releasemodel.FileStatusRenamed, "old/name.txt", 1, 0)
	check("gone.txt", releasemodel.FileStatusDeleted, "", 0, 1)
	check("blob.bin", releasemodel.FileStatusModified, "", 0, 0)
	check("added.bin", releasemodel.FileStatusAdded, "", 0, 0)
}

// AC: AC-CLOG-001.8
func TestCommitsRootCommitFilesAndEmptyCommit(t *testing.T) {
	f := newFixture(t)
	f.write("a.txt", "x\n")
	f.commitAll("root")
	f.commitMsg("empty")
	recs := f.commits("", "HEAD")
	if len(recs[0].Files) != 0 {
		t.Errorf("empty commit files = %v", recs[0].Files)
	}
	if len(recs[1].Files) != 1 || recs[1].Files[0].Status != releasemodel.FileStatusAdded {
		t.Errorf("root commit files = %+v", recs[1].Files)
	}
}

func TestChangedFilesRejectsMalformedOutput(t *testing.T) {
	if _, _, _, _, err := nextNumstat([]string{"nonsense"}); err == nil {
		t.Error("want an error for a malformed numstat entry")
	}
	if _, _, _, _, err := nextNumstat([]string{"1\t2\t"}); err == nil {
		t.Error("want an error for a truncated rename entry")
	}
}
