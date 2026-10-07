// Snapshot case harness: replays the data-driven cases under testdata/cases/
// against the real command tree.
//
// A case is pure data, with no Go code of its own:
//
//	testdata/cases/<name>/
//	  case.toml   description, cites (acceptance criteria), git, args, exit_code, artifacts
//	  input/      optional; copied to a scratch directory the run starts in
//
// `git` is a list of git invocations, each a list of arguments, run in the
// scratch directory before the command to build a fixture repository. They
// run isolated from the user's git configuration, with a fixed author,
// committer and date, so the hashes they produce are the same on every run
// and a case can snapshot them.
//
//	expected/   the artifacts case.toml declares, compared byte for byte
//
// Regenerate the expected side with scripts/regenerate-snapshots.sh, which
// runs this same test with -update, so the generator and the harness cannot
// disagree about what a case produces. Review the diff before committing it.
package cli

import (
	"bytes"
	"flag"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"slices"
	"testing"

	"github.com/BurntSushi/toml"
)

var update = flag.Bool("update", false, "rewrite the expected snapshot artifacts and golden help files from the current implementation")

const casesDir = "../../testdata/cases"

type caseManifest struct {
	Description string     `toml:"description"`
	Cites       []string   `toml:"cites"`
	Git         [][]string `toml:"git"`
	Args        []string   `toml:"args"`
	ExitCode    int        `toml:"exit_code"`

	// Artifacts names every file under expected/ the case asserts against.
	// checkAssertionCoverage reconciles it with expected/ in both
	// directions: a comparison skipped because its file went missing would
	// otherwise be indistinguishable from one that passed.
	Artifacts []string `toml:"artifacts"`
}

// knownArtifacts maps each artifact name to the stream it captures. A run
// has one standard output, so a case declares at most one stdout artifact:
// stdout.txt for the default YAML or a payload, stdout.json for --json.
var knownArtifacts = map[string]string{
	"stdout.txt":  "stdout",
	"stdout.json": "stdout",
	"stderr.txt":  "stderr",
}

func TestSnapshotCases(t *testing.T) {
	entries, err := os.ReadDir(casesDir)
	if err != nil {
		t.Fatalf("reading %s: %v", casesDir, err)
	}
	ran := 0
	for _, entry := range entries {
		if !entry.IsDir() {
			continue
		}
		ran++
		t.Run(entry.Name(), func(t *testing.T) {
			runSnapshotCase(t, filepath.Join(casesDir, entry.Name()))
		})
	}
	if ran == 0 {
		t.Fatalf("no snapshot cases under %s", casesDir)
	}
}

func runSnapshotCase(t *testing.T, dir string) {
	// Absolute before the chdir below, or every later read of expected/
	// resolves inside the scratch directory and silently finds nothing.
	dir, err := filepath.Abs(dir)
	if err != nil {
		t.Fatal(err)
	}

	var m caseManifest
	if _, err := toml.DecodeFile(filepath.Join(dir, "case.toml"), &m); err != nil {
		t.Fatalf("decoding case.toml: %v", err)
	}
	if m.Description == "" {
		t.Error("case.toml has no description")
	}
	checkCitations(t, m)

	work := t.TempDir()
	if input := filepath.Join(dir, "input"); exists(t, input) {
		if err := copyTree(input, work); err != nil {
			t.Fatalf("copying input/: %v", err)
		}
	}
	for _, args := range m.Git {
		runFixtureGit(t, work, args)
	}
	t.Chdir(work)

	var stdout, stderr bytes.Buffer
	code := Execute(m.Args, &stdout, &stderr)
	streams := map[string][]byte{"stdout": stdout.Bytes(), "stderr": stderr.Bytes()}

	if code != m.ExitCode {
		t.Errorf("exit code = %d, want %d\n--- stdout ---\n%s--- stderr ---\n%s",
			code, m.ExitCode, stdout.String(), stderr.String())
	}

	if *update {
		writeArtifacts(t, dir, m, streams)
	}
	checkAssertionCoverage(t, dir, m)
	for _, name := range m.Artifacts {
		stream, ok := knownArtifacts[name]
		if !ok {
			continue
		}
		want, err := os.ReadFile(filepath.Join(dir, "expected", name))
		if err != nil {
			continue // reported by checkAssertionCoverage
		}
		if got := streams[stream]; !bytes.Equal(got, want) {
			t.Errorf("%s does not match expected/%s\n--- got ---\n%s\n--- want ---\n%s", stream, name, got, want)
		}
	}
}

func writeArtifacts(t *testing.T, dir string, m caseManifest, streams map[string][]byte) {
	t.Helper()
	if err := os.MkdirAll(filepath.Join(dir, "expected"), 0o755); err != nil {
		t.Fatal(err)
	}
	for _, name := range m.Artifacts {
		stream, ok := knownArtifacts[name]
		if !ok {
			continue
		}
		if err := os.WriteFile(filepath.Join(dir, "expected", name), streams[stream], 0o644); err != nil {
			t.Fatal(err)
		}
	}
}

// checkAssertionCoverage fails a case whose declared artifacts and expected/
// directory disagree, or which asserts nothing but its exit status.
func checkAssertionCoverage(t *testing.T, dir string, m caseManifest) {
	t.Helper()

	if len(m.Artifacts) == 0 {
		t.Error("case.toml declares no artifacts, so the case asserts only its exit status")
	}
	declared := map[string]bool{}
	stdoutCount := 0
	for _, a := range m.Artifacts {
		stream, ok := knownArtifacts[a]
		if !ok {
			t.Errorf("case.toml declares %q, which this harness cannot compare", a)
			continue
		}
		if declared[a] {
			t.Errorf("case.toml declares %q twice", a)
		}
		declared[a] = true
		if stream == "stdout" {
			stdoutCount++
		}
	}
	if stdoutCount > 1 {
		t.Error("case.toml declares both stdout.txt and stdout.json; a run has one standard output")
	}

	present := map[string]bool{}
	entries, err := os.ReadDir(filepath.Join(dir, "expected"))
	if err != nil && !os.IsNotExist(err) {
		t.Fatal(err)
	}
	for _, e := range entries {
		present[e.Name()] = true
	}
	for name := range declared {
		if !present[name] {
			t.Errorf("case.toml declares %q but expected/%s does not exist", name, name)
		}
	}
	for name := range present {
		if !slices.Contains(m.Artifacts, name) {
			t.Errorf("expected/%s exists but case.toml does not declare it, so nothing reads it", name)
		}
	}
}

// citation is what a case may cite: an FRD acceptance criterion
// (AC-VER-001.2) or a section of a corpus document by slug and anchor
// (contract-release-cli#guarantees), for behaviour a contract specifies
// without numbered criteria.
var citation = regexp.MustCompile(`^(AC-[A-Z0-9]+-[0-9]+\.[0-9]+|[a-z0-9]+(-[a-z0-9]+)*#[a-z0-9]+(-[a-z0-9]+)*)$`)

// checkCitations requires every case to name what it verifies, so a
// criterion with no case is findable by searching testdata/cases.
func checkCitations(t *testing.T, m caseManifest) {
	t.Helper()
	if len(m.Cites) == 0 {
		t.Error("case.toml cites nothing; name the acceptance criteria or contract section the case verifies")
	}
	for _, c := range m.Cites {
		if !citation.MatchString(c) {
			t.Errorf("case.toml cites %q, which is neither an AC id (AC-VER-001.2) nor a slug#anchor (contract-release-cli#guarantees)", c)
		}
	}
}

// fixtureEnv isolates a case's git invocations from the user's and the
// system's configuration and pins every identity and date a hash covers.
var fixtureEnv = []string{
	"GIT_CONFIG_GLOBAL=" + os.DevNull,
	"GIT_CONFIG_NOSYSTEM=1",
	"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
	"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
	"GIT_AUTHOR_DATE=2026-01-01T00:00:00Z", "GIT_COMMITTER_DATE=2026-01-01T00:00:00Z",
}

func runFixtureGit(t *testing.T, dir string, args []string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(), fixtureEnv...)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("case.toml git %v: %v\n%s", args, err, out)
	}
}

func exists(t *testing.T, path string) bool {
	t.Helper()
	_, err := os.Stat(path)
	if err == nil {
		return true
	}
	if !os.IsNotExist(err) {
		t.Fatal(err)
	}
	return false
}

func copyTree(src, dst string) error {
	return filepath.WalkDir(src, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, err := filepath.Rel(src, path)
		if err != nil {
			return err
		}
		target := filepath.Join(dst, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
}
