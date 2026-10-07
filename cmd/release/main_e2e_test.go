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
