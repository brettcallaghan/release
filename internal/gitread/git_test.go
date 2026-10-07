package gitread

import (
	"context"
	"errors"
	"os"
	"path/filepath"
	"testing"
)

func TestCommitHashPeelsATag(t *testing.T) {
	f := newFixture(t)
	f.commit("v1.0.0")
	want := f.git("rev-parse", "HEAD")
	for _, rev := range []string{"v1.0.0", "HEAD", "main"} {
		got, err := f.repo().CommitHash(context.Background(), rev)
		if err != nil || got != want {
			t.Errorf("CommitHash(%q) = %q, %v; want %q", rev, got, err, want)
		}
	}
	var gitErr *GitError
	if _, err := f.repo().CommitHash(context.Background(), "nope"); !errors.As(err, &gitErr) {
		t.Errorf("CommitHash(nope) err = %v, want a *GitError", err)
	}
}

func TestCommonDirIsSharedByWorktrees(t *testing.T) {
	f := newFixture(t)
	f.commit()
	want, err := filepath.EvalSymlinks(filepath.Join(f.dir, ".git"))
	if err != nil {
		t.Fatal(err)
	}
	wt := filepath.Join(t.TempDir(), "wt")
	f.git("worktree", "add", "-q", "--detach", wt)
	for _, dir := range []string{f.dir, wt} {
		got, err := Repo{Dir: dir}.CommonDir(context.Background())
		if err != nil {
			t.Fatalf("CommonDir(%s): %v", dir, err)
		}
		if got, _ = filepath.EvalSymlinks(got); got != want {
			t.Errorf("CommonDir(%s) = %q, want %q", dir, got, want)
		}
	}
	if _, err := os.Stat(want); err != nil {
		t.Fatal(err)
	}
}

func TestRemoteURL(t *testing.T) {
	f := newFixture(t)
	if _, found, err := f.repo().RemoteURL(context.Background(), "origin"); found || err != nil {
		t.Errorf("no remote: found %v, err %v; want false, nil", found, err)
	}
	f.git("remote", "add", "origin", "git@github.com:o/r.git")
	url, found, err := f.repo().RemoteURL(context.Background(), "origin")
	if url != "git@github.com:o/r.git" || !found || err != nil {
		t.Errorf("RemoteURL = %q, %v, %v", url, found, err)
	}
}
