// Package gitread reads release facts out of a git repository by running the
// installed git binary (ADR-006). It is the only part of the release core
// that touches a repository; everything it returns is plain data for the
// pure components.
package gitread

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

// ErrGitUnavailable is wrapped when the git binary cannot be run at all, so a
// caller can report an invalid environment rather than a failed query.
var ErrGitUnavailable = errors.New("git is not available")

// GitError is a git invocation that ran and exited unsuccessfully.
type GitError struct {
	Args     []string
	ExitCode int
	Stderr   string
}

func (e *GitError) Error() string {
	msg := strings.TrimSpace(e.Stderr)
	if msg == "" {
		msg = "no output"
	}
	return fmt.Sprintf("git %s: exit status %d: %s", strings.Join(e.Args, " "), e.ExitCode, msg)
}

// Repo is a git repository reached through the git binary.
type Repo struct {
	// Dir is the repository's working directory (or any directory inside it).
	Dir string
	// Git is the git executable to run; empty means "git" from PATH.
	Git string
}

// gitEnv makes every invocation non-interactive and its output
// locale-independent, while keeping the user's own git configuration.
var gitEnv = []string{
	"GIT_TERMINAL_PROMPT=0",
	"GIT_PAGER=cat",
	"GIT_OPTIONAL_LOCKS=0",
	"LC_ALL=C",
	"LANG=C",
}

// run executes git with args in the repository and returns its stdout.
func (r Repo) run(ctx context.Context, args ...string) ([]byte, error) {
	bin := r.Git
	if bin == "" {
		bin = "git"
	}
	full := append([]string{"-C", r.Dir}, args...)
	cmd := exec.CommandContext(ctx, bin, full...)
	cmd.Env = append(os.Environ(), gitEnv...)
	cmd.Stdin = nil
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr
	err := cmd.Run()
	if err == nil {
		return stdout.Bytes(), nil
	}
	var exitErr *exec.ExitError
	if errors.As(err, &exitErr) {
		return nil, &GitError{Args: args, ExitCode: exitErr.ExitCode(), Stderr: stderr.String()}
	}
	if ctxErr := ctx.Err(); ctxErr != nil {
		return nil, ctxErr
	}
	return nil, fmt.Errorf("%w: %v", ErrGitUnavailable, err)
}

// DefaultBranch returns the ref origin/HEAD points to, read locally (for
// example refs/remotes/origin/main). found is false, with a nil error, WHEN
// origin/HEAD is unset; `git remote set-head origin --auto` sets it.
func (r Repo) DefaultBranch(ctx context.Context) (ref string, found bool, err error) {
	out, err := r.run(ctx, "symbolic-ref", "--quiet", "refs/remotes/origin/HEAD")
	var gitErr *GitError
	if errors.As(err, &gitErr) && gitErr.ExitCode == 1 {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return strings.TrimSpace(string(out)), true, nil
}
