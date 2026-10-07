package changelogfile

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const section190 = "## 1.9.0\n\n### Fixes\n\n- Tags with a `v` prefix are recognised (#87)\n"

func fixture(t *testing.T, name string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join("testdata", name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// write copies a fixture (none when name is empty) into a fresh repository
// root as CHANGELOG.md, writes section for version, and returns the result.
func write(t *testing.T, name, version, section string) string {
	t.Helper()
	dir := t.TempDir()
	if name != "" {
		if err := os.WriteFile(filepath.Join(dir, Name), []byte(fixture(t, name)), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	if err := Write(dir, version, section); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, Name))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

// AC: AC-REND-002.1
func TestANewSectionIsPlacedAboveExistingSections(t *testing.T) {
	got := write(t, "earlier.md", "1.9.0", section190)
	before, after, ok := strings.Cut(fixture(t, "earlier.md"), "## 1.8.0")
	if !ok {
		t.Fatal("fixture has no 1.8.0 section")
	}
	if want := before + section190 + "\n## 1.8.0" + after; got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
}

func TestBracketedHeadingsAreSections(t *testing.T) {
	got := write(t, "bracketed.md", "1.3.0", "## 1.3.0\n\n- third\n")
	if want := "# Changelog\n\n## 1.3.0\n\n- third\n\n## [v1.2.0] - 2026-01-01\n"; !strings.HasPrefix(got, want) {
		t.Errorf("got:\n%s\nwant it to start:\n%s", got, want)
	}
	got = write(t, "bracketed.md", "1.2.0", "## 1.2.0\n\n- first, corrected\n")
	if want := "# Changelog\n\n## 1.2.0\n\n- first, corrected\n\n## [v1.1.0] - 2025-12-01\n\n- zeroth\n"; got != want {
		t.Errorf("a ## [v1.2.0] section was not replaced:\n%s", got)
	}
}

// AC: AC-REND-002.2
func TestRestagingReplacesRatherThanDuplicates(t *testing.T) {
	got := write(t, "same-version-top.md", "1.9.0", section190)
	if n := strings.Count(got, "## 1.9.0"); n != 1 {
		t.Errorf("%d sections for 1.9.0:\n%s", n, got)
	}
	if want := "# Changelog\n\n" + section190 + "\n## 1.8.0\n\n- older feature (#80)\n"; got != want {
		t.Errorf("got:\n%s\nwant:\n%s", got, want)
	}
	dir := t.TempDir()
	for range 3 {
		if err := Write(dir, "1.9.0", section190); err != nil {
			t.Fatal(err)
		}
	}
	b, _ := os.ReadFile(filepath.Join(dir, Name))
	if string(b) != "# Changelog\n\n"+section190 {
		t.Errorf("writing three times is not idempotent:\n%s", b)
	}
}

func TestTheLastSectionIsReplacedToTheEndOfTheFile(t *testing.T) {
	got := write(t, "earlier.md", "1.7.2", "## 1.7.2\n\n- corrected\n")
	if !strings.HasSuffix(got, "## 1.8.0\n\n### Features\n\n- older feature (#80)\n\n## 1.7.2\n\n- corrected\n") {
		t.Errorf("got:\n%s", got)
	}
}

// AC: AC-REND-002.3
func TestAnAbsentChangelogIsCreated(t *testing.T) {
	if got := write(t, "", "1.9.0", section190); got != "# Changelog\n\n"+section190 {
		t.Errorf("got %q", got)
	}
}

func TestAnEmptyFileTakesTheSectionAlone(t *testing.T) {
	if got := write(t, "empty.md", "1.9.0", section190); got != section190 {
		t.Errorf("got %q", got)
	}
}

func TestWithNoSectionTheSectionFollowsThePreamble(t *testing.T) {
	got := write(t, "preamble-only.md", "0.1.0", "## 0.1.0\n\n- first\n")
	if want := "# Changelog\n\nNothing released yet.\n\n## 0.1.0\n\n- first\n"; got != want {
		t.Errorf("got %q, want %q", got, want)
	}
}

// AC: AC-REND-002.4
func TestExistingContentIsPreservedExactly(t *testing.T) {
	old := fixture(t, "hand-edited.md")
	at := strings.Index(old, "## 1.8.0")
	if at < 0 || !strings.Contains(old, "\r\n") {
		t.Fatal("the fixture lost its 1.8.0 section or its CRLF line endings")
	}
	got := write(t, "hand-edited.md", "1.9.0", section190)
	if want := old[:at] + section190 + "\n" + old[at:]; got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
	// A section other than the one written keeps its bytes when an earlier
	// one is replaced.
	got = write(t, "hand-edited.md", "1.8.0", "## 1.8.0\n\n- rewritten\n")
	tail := old[strings.Index(old, "## v1.7.2"):]
	if want := old[:at] + "## 1.8.0\n\n- rewritten\n\n" + tail; got != want {
		t.Errorf("got %q\nwant %q", got, want)
	}
}

func TestAnUnparseableVersionIsRefused(t *testing.T) {
	if err := Write(t.TempDir(), "next", section190); err == nil {
		t.Error("a section for version \"next\" was written")
	}
}

func TestHeadingsThatAreNotVersionsAreNotSections(t *testing.T) {
	for _, line := range []string{"## Unreleased\n", "### 1.0.0\n", "##1.0.0\n", "## \n", "## [next]\n", "# 1.0.0\n"} {
		if _, ok := headingVersion(line); ok {
			t.Errorf("%q read as a section heading", line)
		}
	}
	for _, line := range []string{"## 1.0.0", "## v1.0.0 (2026-01-01)\n", "## [1.0.0] - 2026\n", "## [v1.0.0-rc.1]\r\n"} {
		if _, ok := headingVersion(line); !ok {
			t.Errorf("%q not read as a section heading", line)
		}
	}
}

func TestAFileThatCannotBeReadIsAnError(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, Name), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := Write(dir, "1.9.0", section190); err == nil {
		t.Error("a CHANGELOG.md that is a directory was written over")
	}
}

// AC: AC-REND-002.5
func TestWritingTheChangelogDoesNotTouchGit(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	git := func(args ...string) string {
		t.Helper()
		cmd := exec.Command("git", append([]string{"-C", dir, "-c", "user.name=t", "-c", "user.email=t@example.com"}, args...)...)
		out, err := cmd.CombinedOutput()
		if err != nil {
			t.Fatalf("git %v: %v\n%s", args, err, out)
		}
		return string(out)
	}
	git("init", "-q", "-b", "main")
	if err := os.WriteFile(filepath.Join(dir, Name), []byte(fixture(t, "earlier.md")), 0o644); err != nil {
		t.Fatal(err)
	}
	git("add", Name)
	git("commit", "-q", "-m", "earlier")
	git("tag", "v1.8.0")
	state := func() string {
		return git("rev-parse", "HEAD") + git("tag", "--list") + git("ls-files", "--stage")
	}
	before := state()

	if err := Write(dir, "1.9.0", section190); err != nil {
		t.Fatal(err)
	}
	if after := state(); after != before {
		t.Errorf("git state changed:\nbefore %s\nafter  %s", before, after)
	}
	if got := git("status", "--porcelain"); got != " M "+Name+"\n" {
		t.Errorf("status = %q, want only an unstaged change to %s", got, Name)
	}
}
