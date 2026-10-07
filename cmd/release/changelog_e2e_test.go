package main_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// changelogFixture is a repository released at v1.8.0 with a feature and a
// fix since, configured with the command producer, whose command answers
// with authored.json and records each run in runs.log. The configuration
// and producer files are committed, so the tree is clean.
func changelogFixture(t *testing.T, proposed string) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	fixtureGit(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "release.toml", "[changelog]\nproducer = \"command\"\nproducer_command = \"echo run >> .git/runs.log; cat authored.json\"\n")
	writeAuthored(t, dir, proposed, "Compute for any release line.")
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "commit", "-q", "-m", "Initial commit")
	fixtureGit(t, dir, "tag", "-a", "-m", "v1.8.0", "v1.8.0")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "feat: add a --target flag (#12)")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "fix(cli): handle empty input (#11)")
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeAuthored writes the corpus the producer answers with: proposing
// proposed, with summary as the feature's prose.
func writeAuthored(t *testing.T, dir, proposed, summary string) {
	t.Helper()
	c := map[string]any{
		"schema_version": "1.0.0",
		"proposed_bump":  proposed,
		"entries": []any{
			map[string]any{"kind": "FEATURE", "summary": summary, "breaking": false, "derived": false,
				"sources": []any{map[string]any{"__type": "PullRequest", "number": 12}}},
			map[string]any{"kind": "FIX", "scope": "cli", "summary": "handle empty input", "breaking": false, "derived": true,
				"sources": []any{map[string]any{"__type": "PullRequest", "number": 11}}},
		},
		"sources": []any{map[string]any{"adapter": "COMMITS", "to": strings.Repeat("0", 40)}},
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, "authored.json", string(data))
}

// producerRuns is how many times the producer command has run.
func producerRuns(t *testing.T, dir string) int {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(dir, ".git", "runs.log"))
	if os.IsNotExist(err) {
		return 0
	}
	if err != nil {
		t.Fatal(err)
	}
	return strings.Count(string(data), "run\n")
}

type changelogAnswer struct {
	Version    string  `json:"version"`
	CorpusPath *string `json:"corpus_path"`
	Section    string  `json:"section"`
	Computed   string  `json:"computed"`
	Proposed   string  `json:"proposed"`
}

func changelogJSON(t *testing.T, dir string, args ...string) (changelogAnswer, string, int) {
	t.Helper()
	stdout, stderr, code := runIn(t, dir, append([]string{"changelog", "--target", "main", "--json"}, args...)...)
	var a changelogAnswer
	if stdout != "" {
		if err := json.Unmarshal([]byte(stdout), &a); err != nil {
			t.Fatalf("stdout is not one JSON object: %v\n%s", err, stdout)
		}
	}
	return a, stderr, code
}

// AC: AC-CLOG-003.4
func TestADisagreementHaltsForConfirmation(t *testing.T) {
	dir := changelogFixture(t, "MAJOR")
	a, stderr, code := changelogJSON(t, dir, "--write")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, stderr)
	}
	if a.Computed != "minor" || a.Proposed != "major" || a.Section != "" || a.Version != "" {
		t.Errorf("answer = %+v, want computed minor and proposed major, and no section", a)
	}
	if !strings.Contains(stderr, "release changelog minor") {
		t.Errorf("stderr does not say how to confirm:\n%s", stderr)
	}
	if status := gitOut(t, dir, "status", "--porcelain"); status != "" {
		t.Errorf("a halted run changed the tree, --write notwithstanding:\n%s", status)
	}
}

// AC: AC-CLOG-003.5
func TestAProposalNeverDeterminesTheVersion(t *testing.T) {
	dir := changelogFixture(t, "MAJOR")
	a, stderr, code := changelogJSON(t, dir, "minor", "--write")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if a.Version != "1.9.0" || !strings.HasPrefix(a.Section, "## 1.9.0\n") {
		t.Errorf("version = %q, section heading %q; want 1.9.0 from the confirmed minor, not 2.0.0 from the proposal", a.Version, firstLine(a.Section))
	}
	written, err := os.ReadFile(filepath.Join(dir, "CHANGELOG.md"))
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(string(written), "## 1.9.0\n") || strings.Contains(string(written), "2.0.0") {
		t.Errorf("CHANGELOG.md =\n%s\nwant a 1.9.0 section only", written)
	}
	if status := gitOut(t, dir, "status", "--porcelain"); status != "?? CHANGELOG.md\n" {
		t.Errorf("--write changed more than CHANGELOG.md:\n%s", status)
	}
}

func firstLine(s string) string {
	line, _, _ := strings.Cut(s, "\n")
	return line
}

// AC: AC-CLOG-003.10
func TestACachedCorpusCanBeDiscarded(t *testing.T) {
	dir := changelogFixture(t, "MINOR")
	first, stderr, code := changelogJSON(t, dir)
	if code != 0 {
		t.Fatalf("first run: exit = %d\n%s", code, stderr)
	}
	if first.CorpusPath == nil || !strings.HasPrefix(*first.CorpusPath, ".git/release/corpus/") {
		t.Fatalf("corpus_path = %v, want the cache under .git/release/corpus/", first.CorpusPath)
	}

	writeAuthored(t, dir, "MINOR", "Freshly authored prose.")
	again, _, _ := changelogJSON(t, dir)
	if producerRuns(t, dir) != 1 || strings.Contains(again.Section, "Freshly") {
		t.Errorf("without --redo-notes: %d producer runs, section\n%s\nwant 1 run and the cached prose", producerRuns(t, dir), again.Section)
	}

	redone, stderr, code := changelogJSON(t, dir, "--redo-notes")
	if code != 0 {
		t.Fatalf("--redo-notes: exit = %d\n%s", code, stderr)
	}
	if producerRuns(t, dir) != 2 || !strings.Contains(redone.Section, "Freshly authored prose.") {
		t.Errorf("--redo-notes: %d producer runs, section\n%s\nwant 2 runs and the fresh prose", producerRuns(t, dir), redone.Section)
	}
	cached, err := os.ReadFile(filepath.Join(dir, filepath.FromSlash(*redone.CorpusPath)))
	if err != nil {
		t.Fatal(err)
	}
	authored, _ := os.ReadFile(filepath.Join(dir, "authored.json"))
	if string(cached) != string(authored) {
		t.Errorf("the cache was not replaced with the producer's new answer:\n%s", cached)
	}
}

// AC: AC-CLOG-003.11
func TestAuthoringCanBeSkipped(t *testing.T) {
	dir := changelogFixture(t, "MINOR")
	writeFile(t, dir, "release.toml", "[changelog]\nproducer = \"command\"\nproducer_command = \"echo run >> .git/runs.log; exit 7\"\n")
	a, stderr, code := changelogJSON(t, dir, "minor", "--skip-notes")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if producerRuns(t, dir) != 0 {
		t.Error("--skip-notes invoked the producer")
	}
	if a.CorpusPath != nil {
		t.Errorf("corpus_path = %q, want null: nothing was authored", *a.CorpusPath)
	}
	if !strings.Contains(a.Section, "- add a --target flag (#12)") || !strings.Contains(a.Section, "- **cli:** handle empty input (#11)") {
		t.Errorf("section =\n%s\nwant the mechanically assembled entries", a.Section)
	}
}
