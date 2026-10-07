// End-to-end tests that build the real `release` binary and run it as a
// process. The in-process tests in internal/cli stop one layer short: only
// here is the status the command tree decided on checked against the status
// the process actually exits with.
//
// Fixture git repositories for the commands that read history belong in this
// tier too, as those commands land.
package main_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"sync"
	"testing"
)

// coverDirEnv names a directory that scripts/coverage.sh sets. When it is
// set, the binary is built with -cover and every run writes its counters
// there, so the end-to-end layer shows up in the coverage report.
const coverDirEnv = "RELEASE_E2E_COVERDIR"

var (
	buildOnce sync.Once
	binPath   string
	buildErr  error
	buildOut  []byte
)

func releaseBinary(t *testing.T) string {
	t.Helper()
	if testing.Short() {
		t.Skip("skipping end-to-end binary test in -short mode")
	}
	buildOnce.Do(func() {
		dir, err := os.MkdirTemp("", "release-e2e-*")
		if err != nil {
			buildErr = err
			return
		}
		name := "release"
		if runtime.GOOS == "windows" {
			// Windows will not execute a file without .exe, however
			// absolute the path.
			name += ".exe"
		}
		binPath = filepath.Join(dir, name)
		args := []string{"build", "-o", binPath}
		if os.Getenv(coverDirEnv) != "" {
			args = append(args, "-cover", "-coverpkg=github.com/brettcallaghan/release/...")
		}
		buildOut, buildErr = exec.Command("go", append(args, ".")...).CombinedOutput()
	})
	if buildErr != nil {
		t.Fatalf("building the release binary: %v\n%s", buildErr, buildOut)
	}
	return binPath
}

func run(t *testing.T, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	bin := releaseBinary(t)
	var out, errb strings.Builder
	cmd := exec.Command(bin, args...)
	cmd.Stdout, cmd.Stderr = &out, &errb
	if dir := os.Getenv(coverDirEnv); dir != "" {
		abs, err := filepath.Abs(dir)
		if err != nil {
			t.Fatal(err)
		}
		cmd.Env = append(os.Environ(), "GOCOVERDIR="+abs)
	}
	if err := cmd.Run(); err != nil {
		exit, ok := err.(*exec.ExitError)
		if !ok {
			t.Fatalf("running release %v: %v", args, err)
		}
		return out.String(), errb.String(), exit.ExitCode()
	}
	return out.String(), errb.String(), 0
}

func TestBareInvocationExitsInvalidAtTheProcessBoundary(t *testing.T) {
	stdout, stderr, code := run(t)
	if code != 2 {
		t.Errorf("exit = %d, want 2\n%s", code, stderr)
	}
	if stdout != "" {
		t.Errorf("stdout must stay empty, got:\n%s", stdout)
	}
	if !strings.Contains(stderr, "Usage:") {
		t.Errorf("stderr lacks usage:\n%s", stderr)
	}
}

// The negative control: a binary that always exited 2 would pass the test
// above.
func TestVersionExitsZeroAtTheProcessBoundary(t *testing.T) {
	stdout, stderr, code := run(t, "--version")
	if code != 0 {
		t.Errorf("exit = %d, want 0\n%s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "release ") {
		t.Errorf("stdout = %q", stdout)
	}
}

// runIn runs the binary with dir as its working directory.
func runIn(t *testing.T, dir string, args ...string) (stdout, stderr string, code int) {
	t.Helper()
	releaseBinary(t) // builds from this package's directory, so before the chdir
	wd, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	if cover := os.Getenv(coverDirEnv); cover != "" && !filepath.IsAbs(cover) {
		t.Setenv(coverDirEnv, filepath.Join(wd, cover))
	}
	t.Chdir(dir)
	return run(t, args...)
}

// fixtureGit runs git in dir, isolated from the user's configuration.
func fixtureGit(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}

// AC: AC-VER-001.5
func TestBranchNameDoesNotConstrainTheBaseMajor(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	fixtureGit(t, dir, "init", "-q", "-b", "release/v1")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "Initial commit")
	fixtureGit(t, dir, "tag", "v1.4.0")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "feat: on the 2.x line")
	fixtureGit(t, dir, "tag", "v2.3.0")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "fix: after 2.3.0")

	stdout, stderr, code := runIn(t, dir, "version", "--target", "release/v1")
	if code != 0 {
		t.Fatalf("exit = %d, want 0\n%s", code, stderr)
	}
	if !strings.Contains(stdout, "base: 2.3.0\n") || !strings.Contains(stdout, "next: 2.3.1\n") {
		t.Errorf("stdout =\n%s\nwant base 2.3.0 and next 2.3.1, unconstrained by the v1 in the branch name", stdout)
	}
}

// A refusal is an answer: printed in full, then exit 1 at the process
// boundary.
func TestNoReleaseExitsRefusedAtTheProcessBoundary(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	fixtureGit(t, dir, "init", "-q", "-b", "main")
	fixtureGit(t, dir, "commit", "-q", "--allow-empty", "-m", "docs: only docs")

	stdout, stderr, code := runIn(t, dir, "version", "--target", "main", "--json")
	if code != 1 {
		t.Fatalf("exit = %d, want 1\n%s", code, stderr)
	}
	if !strings.HasPrefix(stdout, "{") || !strings.Contains(stdout, `"next": null`) {
		t.Errorf("stdout =\n%s\nwant the JSON answer with next null", stdout)
	}
}
