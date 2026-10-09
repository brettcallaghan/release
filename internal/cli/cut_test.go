package cli

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring/producers"
	"github.com/brettcallaghan/release/internal/changelog"
	"github.com/brettcallaghan/release/internal/config"
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

// authoredCutRepo is cutRepo configured with a command producer that records
// each run in .git/runs.log and answers with a changelog of one feature
// citing every commit, carrying the bundle's sources through.
func authoredCutRepo(t *testing.T) (work, bare string, runs func() int) {
	t.Helper()
	work, bare = cutRepo(t)
	cfg := config.Default()
	prepared, err := changelog.Prepare(context.Background(), changelog.Request{
		Dir: work, Target: "main", Config: cfg, Skip: true, Producers: producers.Registry(cfg.Changelog, work),
	})
	if err != nil {
		t.Fatal(err)
	}
	sources := []any{}
	for _, ce := range prepared.Evidence.Commits {
		sources = append(sources, map[string]any{"__type": "Commit", "hash": ce.Record.Hash})
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": "1.0.0",
		"entries": []any{map[string]any{"kind": "FEATURE", "summary": "Add a flag.", "breaking": false, "derived": false,
			"sources": sources}},
		"sources": prepared.Evidence.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	common := filepath.Join(work, ".git")
	writeFile(t, common, "authored.json", string(data))
	writeFile(t, work, "release.toml", "release_paths = [\"VERSION\"]\n[commands]\npropagate = \"echo $RELEASE_VERSION > VERSION\"\n"+
		"[changelog]\nproducer = \"command\"\nproducer_command = '"+
		`d="$(git rev-parse --git-common-dir)"; echo run >> "$d/runs.log"; cat "$d/authored.json"`+"'\n")
	return work, bare, func() int {
		data, err := os.ReadFile(filepath.Join(common, "runs.log"))
		if err != nil {
			return 0
		}
		return strings.Count(string(data), "run\n")
	}
}

func cutWithTerminal(t *testing.T, work, answers string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	var out, errb bytes.Buffer
	s := &streams{out: &out, err: &errb, in: strings.NewReader(answers), interactive: true}
	code = s.execute(append([]string{"-C", work, "cut", "--no-pull-request"}, args...))
	return out.String(), errb.String(), code
}

// AC: AC-STG-001.19
// AC: AC-VER-005.5
func TestATerminalIsAskedForTheBumpWithTheSuggestionAndItsReasons(t *testing.T) {
	work, bare, runs := authoredCutRepo(t)
	stdout, stderr, code := cutWithTerminal(t, work, "\ny\n")
	if code != 0 || !branchExists(bare, "cut/1.9.0") {
		t.Fatalf("accepted: exit = %d\n%s%s", code, stdout, stderr)
	}
	prompt, _, _ := strings.Cut(stderr, "Proceed?")
	for _, want := range []string{"suggested bump: minor", "- Add a flag.", "signal floor: minor", "feat: add a flag (conventional-type)", "Bump? [minor]"} {
		if !strings.Contains(prompt, want) {
			t.Errorf("before the first write, stderr lacks %q:\n%s", want, prompt)
		}
	}
	if n := runs(); n != 1 {
		t.Errorf("the producer ran %d times; the confirming call must read the cached changelog", n)
	}
	if msg := gitOutput(t, bare, "log", "-1", "--format=%B", "cut/1.9.0"); !strings.Contains(msg, "Release-Type: minor") {
		t.Errorf("release commit:\n%s\nwant the accepted minor as its trailer", msg)
	}
}

func TestATerminalAnswerNamingABumpIsTheConfirmedBump(t *testing.T) {
	work, bare, runs := authoredCutRepo(t)
	_, stderr, code := cutWithTerminal(t, work, "major\ny\n")
	if code != 0 || !branchExists(bare, "cut/2.0.0") || branchExists(bare, "cut/1.9.0") {
		t.Fatalf("exit = %d\n%s\nwant cut/2.0.0 from the named major", code, stderr)
	}
	if n := runs(); n != 1 {
		t.Errorf("the producer ran %d times; the confirming call must read the cached changelog", n)
	}
	if msg := gitOutput(t, bare, "log", "-1", "--format=%B", "cut/2.0.0"); !strings.Contains(msg, "Release-Type: major") {
		t.Errorf("release commit:\n%s\nwant the named major as its trailer", msg)
	}
}

func TestATerminalAnswerDecliningWritesNothing(t *testing.T) {
	for _, answer := range []string{"n\n", "huge\n", ""} {
		work, bare, _ := authoredCutRepo(t)
		_, stderr, code := cutWithTerminal(t, work, answer)
		if code != int(StatusRefused) || !strings.Contains(stderr, "declined") || strings.Contains(stderr, "Proceed?") {
			t.Errorf("answer %q: exit = %d, stderr:\n%s\nwant a refusal before the write prompt", answer, code, stderr)
		}
		if branchExists(work, "cut/1.9.0") || branchExists(bare, "cut/1.9.0") {
			t.Errorf("answer %q: a declined cut wrote", answer)
		}
	}
}

func gitOutput(t *testing.T, dir string, args ...string) string {
	t.Helper()
	out, err := exec.Command("git", append([]string{"-C", dir}, args...)...).Output()
	if err != nil {
		t.Fatalf("git %v: %v", args, err)
	}
	return string(out)
}
