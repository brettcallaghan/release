package main_test

import (
	"context"
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring/producers"
	"github.com/brettcallaghan/release/internal/changelog"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// changelogFixture is a repository released at v1.8.0 with a feature and a
// fix since, configured with the command producer, whose command answers
// with .git/authored.json and records each run in .git/runs.log. The
// configuration is committed and the producer's files live under .git, so
// the tree is clean.
func changelogFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	fixtureGit(t, dir, "init", "-q", "-b", "main")
	writeFile(t, dir, "release.toml", "[changelog]\nproducer = \"command\"\nproducer_command = \"echo run >> .git/runs.log; cat .git/authored.json\"\n")
	fixtureGit(t, dir, "add", ".")
	fixtureGit(t, dir, "commit", "-q", "-m", "Initial commit")
	fixtureGit(t, dir, "tag", "-a", "-m", "v1.8.0", "v1.8.0")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "feat: add a --target flag (#12)")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "fix(cli): handle empty input (#11)")
	writeAuthored(t, dir, "Compute for any release line.", false)
	return dir
}

func writeFile(t *testing.T, dir, name, content string) {
	t.Helper()
	if err := os.WriteFile(filepath.Join(dir, name), []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

// writeAuthored writes the changelog the producer answers with: an entry
// for each commit in the range, citing it and its pull request, with summary
// as the feature's prose. breaking marks the feature breaking, with a
// migration.
func writeAuthored(t *testing.T, dir, summary string, breaking bool) {
	t.Helper()
	ev := evidenceAt(t, dir)
	entries := []any{}
	for _, ce := range ev.Commits {
		e := map[string]any{"breaking": false, "derived": false, "sources": citing(ce)}
		switch {
		case strings.HasPrefix(ce.Record.Subject, "feat"):
			e["kind"], e["summary"] = "FEATURE", summary
			if breaking {
				e["breaking"], e["migration"] = true, "Pass --target explicitly."
			}
		case strings.HasPrefix(ce.Record.Subject, "fix"):
			e["kind"], e["scope"], e["summary"] = "FIX", "cli", "Handle empty input."
		default:
			t.Fatalf("no entry for %q", ce.Record.Subject)
		}
		entries = append(entries, e)
	}
	data, err := json.Marshal(map[string]any{
		"schema_version": "1.0.0",
		"entries":        entries,
		// The bundle's sources, which a producer carries through.
		"sources": ev.Sources,
	})
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, dir, filepath.Join(".git", "authored.json"), string(data))
}

// evidenceAt is the evidence bundle the release core builds for main in
// dir under the default configuration.
func evidenceAt(t *testing.T, dir string) releasemodel.Evidence {
	t.Helper()
	cfg := config.Default()
	res, err := changelog.Prepare(context.Background(), changelog.Request{
		Dir: dir, Target: "main", Config: cfg, Skip: true, Producers: producers.Registry(cfg.Changelog, dir),
	})
	if err != nil {
		t.Fatalf("building the evidence: %v", err)
	}
	return res.Evidence
}

// citing is the sources an entry for one commit cites: its pull request,
// WHEN it carries one, and the commit.
func citing(ce releasemodel.CommitEvidence) []any {
	sources := []any{}
	if pr := ce.Record.PullRequest; pr != nil {
		sources = append(sources, map[string]any{"__type": "PullRequest", "number": pr.Number})
	}
	return append(sources, map[string]any{"__type": "Commit", "hash": ce.Record.Hash})
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

// AC: AC-CLOG-003.5
func TestABreakingEntryNeverDeterminesTheVersion(t *testing.T) {
	dir := changelogFixture(t)
	writeAuthored(t, dir, "Compute for any release line.", true)
	a, stderr, code := changelogJSON(t, dir, "minor", "--write")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if a.Version != "1.9.0" || !strings.HasPrefix(a.Section, "## 1.9.0\n") {
		t.Errorf("version = %q, section heading %q; want 1.9.0 from the confirmed minor, not 2.0.0 from the breaking entry", a.Version, firstLine(a.Section))
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
	dir := changelogFixture(t)
	first, stderr, code := changelogJSON(t, dir)
	if code != 0 {
		t.Fatalf("first run: exit = %d\n%s", code, stderr)
	}
	if first.CorpusPath == nil || !strings.HasPrefix(*first.CorpusPath, ".git/release/changelog/") {
		t.Fatalf("corpus_path = %v, want the cache under .git/release/changelog/", first.CorpusPath)
	}

	writeAuthored(t, dir, "Freshly authored prose.", false)
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
	authored, _ := os.ReadFile(filepath.Join(dir, ".git", "authored.json"))
	if string(cached) != string(authored) {
		t.Errorf("the cache was not replaced with the producer's new answer:\n%s", cached)
	}
}

// AC: AC-CLOG-003.11
func TestAuthoringCanBeSkipped(t *testing.T) {
	dir := changelogFixture(t)
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
	feature := regexp.MustCompile(`(?m)^- add a --target flag \([0-9a-f]{7}, #12\)$`)
	fix := regexp.MustCompile(`(?m)^- \*\*cli:\*\* handle empty input \([0-9a-f]{7}, #11\)$`)
	if !feature.MatchString(a.Section) || !fix.MatchString(a.Section) {
		t.Errorf("section =\n%s\nwant the mechanical producer's entries", a.Section)
	}
}
