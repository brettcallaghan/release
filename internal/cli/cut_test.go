package cli

import (
	"bytes"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// cutRepo is a repository with a feature since v1.8.0, pushed to a bare
// origin, configured to propagate into VERSION.
func cutRepo(t *testing.T) (work, bare string) {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	for _, kv := range fixtureEnv {
		k, v, _ := strings.Cut(kv, "=")
		t.Setenv(k, v)
	}
	root := t.TempDir()
	work, bare = filepath.Join(root, "work"), filepath.Join(root, "origin.git")
	runFixtureGit(t, root, []string{"init", "-q", "--bare", "-b", "main", bare})
	runFixtureGit(t, root, []string{"init", "-q", "-b", "main", work})
	writeFile(t, work, "release.toml", "release_paths = [\"VERSION\"]\n[commands]\npropagate = \"echo $RELEASE_VERSION > VERSION\"\n")
	for _, args := range [][]string{
		{"add", "."}, {"commit", "-q", "-m", "Initial commit"}, {"tag", "-a", "-m", "v1.8.0", "v1.8.0"},
		{"commit", "-q", "--allow-empty", "-m", "feat: add a flag"},
		{"remote", "add", "origin", bare}, {"push", "-q", "origin", "main", "--tags"}, {"remote", "set-head", "origin", "main"},
	} {
		runFixtureGit(t, work, args)
	}
	return work, bare
}

func cutInteractively(t *testing.T, work, answer string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	s := &streams{out: &out, err: &errb, in: strings.NewReader(answer), interactive: true}
	code = s.execute(append([]string{"-C", work, "cut", "minor", "--skip-notes", "--no-pull-request"}, args...))
	return out.String(), errb.String(), code
}

func branchExists(dir, name string) bool {
	return exec.Command("git", "-C", dir, "show-ref", "--verify", "--quiet", "refs/heads/"+name).Run() == nil
}

func TestATerminalIsPromptedOnceBeforeTheFirstWrite(t *testing.T) {
	work, bare := cutRepo(t)
	_, stderr, code := cutInteractively(t, work, "n\n")
	if code != int(StatusRefused) || strings.Count(stderr, "Proceed? [y/N]") != 1 || !strings.Contains(stderr, "push cut/1.9.0 to origin") {
		t.Errorf("declined: exit = %d, stderr:\n%s\nwant one prompt naming the writes, then a refusal", code, stderr)
	}
	if branchExists(work, "cut/1.9.0") || branchExists(bare, "cut/1.9.0") {
		t.Error("a declined cut wrote")
	}

	stdout, stderr, code := cutInteractively(t, work, "y\n")
	if code != 0 || !strings.Contains(stdout, "commit: ") || !branchExists(bare, "cut/1.9.0") {
		t.Errorf("accepted: exit = %d\n%s%s", code, stdout, stderr)
	}
}

func TestYesSuppressesThePrompt(t *testing.T) {
	work, bare := cutRepo(t)
	_, stderr, code := cutInteractively(t, work, "", "--yes")
	if code != 0 || strings.Contains(stderr, "Proceed?") || !branchExists(bare, "cut/1.9.0") {
		t.Errorf("exit = %d, stderr:\n%s\nwant no prompt and a pushed branch", code, stderr)
	}
}

func TestAClosedTerminalDeclines(t *testing.T) {
	work, _ := cutRepo(t)
	if _, _, code := cutInteractively(t, work, ""); code != int(StatusRefused) || branchExists(work, "cut/1.9.0") {
		t.Errorf("exit = %d, want a refusal with nothing written on end of input", code)
	}
}

func TestSkipAndRedoNotesCannotBeCombined(t *testing.T) {
	work, _ := cutRepo(t)
	var out, errb bytes.Buffer
	s := &streams{out: &out, err: &errb, in: strings.NewReader(""), interactive: false}
	if code := s.execute([]string{"-C", work, "cut", "minor", "--skip-notes", "--redo-notes"}); code != int(StatusInvalid) {
		t.Errorf("exit = %d, want 2", code)
	}
	if code := s.execute([]string{"-C", work, "cut", "huge"}); code != int(StatusInvalid) {
		t.Errorf("invalid bump: exit = %d, want 2", code)
	}
}
