// Package delegate is the Delegated Command Runner: it runs a repository's
// configured command for a named release step and reports the outcome, so
// validation, propagation and preflight stop, report and refuse identically.
//
// A configured command is a string run through `sh -c`, in the repository
// root, with the environment inherited and standard input empty. The
// tooling's own git and gh invocations are not configured commands and stay
// argument lists (ADR-006). Deciding when a step runs is the caller's.
package delegate

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
)

// Outcome is what running a step concluded.
type Outcome int

const (
	// Succeeded: every configured command exited 0.
	Succeeded Outcome = iota
	// Failed: a command exited non-zero; Result carries it and its output.
	Failed
	// Unconfigured: the step has no command. It is never a success.
	Unconfigured
)

func (o Outcome) String() string {
	switch o {
	case Succeeded:
		return "succeeded"
	case Failed:
		return "failed"
	case Unconfigured:
		return "unconfigured"
	}
	return fmt.Sprintf("Outcome(%d)", int(o))
}

// Result is the outcome of a step.
type Result struct {
	Step    string
	Outcome Outcome
	// Command is the command that failed; empty unless Outcome is Failed.
	Command string
	// ExitCode is the failed command's exit status; -1 WHEN a signal ended
	// it.
	ExitCode int
	// Stdout and Stderr are the failed command's output, in full.
	Stdout, Stderr []byte
}

// Shell runs configured commands.
type Shell struct {
	// Sh is the shell executable; empty means "sh" from PATH.
	Sh string
}

// Run runs the step's single configured command in dir. A nil command is
// the step left unconfigured. An error means the command could not be
// started; a command that ran and failed is a Result.
func (s Shell) Run(ctx context.Context, step string, command *string, dir string) (Result, error) {
	if command == nil {
		return Result{Step: step, Outcome: Unconfigured}, nil
	}
	return s.RunAll(ctx, step, []string{*command}, dir)
}

// RunAll runs the step's commands in dir, in order, and stops at the first
// that fails. No commands is the step left unconfigured.
func (s Shell) RunAll(ctx context.Context, step string, commands []string, dir string) (Result, error) {
	if len(commands) == 0 {
		return Result{Step: step, Outcome: Unconfigured}, nil
	}
	for _, c := range commands {
		res, err := s.run(ctx, step, c, dir)
		if err != nil || res.Outcome != Succeeded {
			return res, err
		}
	}
	return Result{Step: step, Outcome: Succeeded}, nil
}

func (s Shell) run(ctx context.Context, step, command, dir string) (Result, error) {
	sh := s.Sh
	if sh == "" {
		sh = "sh"
	}
	cmd := exec.CommandContext(ctx, sh, "-c", command)
	cmd.Dir = dir
	cmd.Stdin = nil // the null device: a command cannot wait on a terminal
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr

	err := cmd.Run()
	if err == nil {
		return Result{Step: step, Outcome: Succeeded}, nil
	}
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		return Result{}, fmt.Errorf("%s: running %q: %w", step, command, err)
	}
	return Result{
		Step:     step,
		Outcome:  Failed,
		Command:  command,
		ExitCode: exitErr.ExitCode(),
		Stdout:   stdout.Bytes(),
		Stderr:   stderr.Bytes(),
	}, nil
}
