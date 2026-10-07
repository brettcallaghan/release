package delegate

import (
	"context"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func ptr(s string) *string { return &s }

// AC: AC-CFG-001.1
func TestAConfiguredCommandRunsInTheRepositoryRoot(t *testing.T) {
	for _, step := range []string{"validate", "propagate", "preflight"} {
		t.Run(step, func(t *testing.T) {
			dir := t.TempDir()
			t.Setenv("RELEASE_DELEGATE_PROBE", step)
			res, err := Shell{}.Run(context.Background(), step, ptr(`printf %s "$RELEASE_DELEGATE_PROBE" > ran`), dir)
			if err != nil {
				t.Fatal(err)
			}
			if res.Outcome != Succeeded || res.Step != step {
				t.Fatalf("result = %+v, want %s succeeded", res, step)
			}
			got, err := os.ReadFile(filepath.Join(dir, "ran"))
			if err != nil {
				t.Fatalf("the command did not run in the repository root: %v", err)
			}
			if string(got) != step {
				t.Errorf("the command saw %q, want the inherited environment's %q", got, step)
			}
		})
	}
}

// AC: AC-CFG-001.2
func TestAFailingCommandStopsAndReportsItsOutput(t *testing.T) {
	cmd := `echo "out line"; echo "err line" >&2; exit 3`
	res, err := Shell{}.Run(context.Background(), "validate", ptr(cmd), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Failed {
		t.Fatalf("outcome = %v, want failed", res.Outcome)
	}
	if res.ExitCode != 3 || res.Command != cmd || res.Step != "validate" {
		t.Errorf("result = %+v, want exit 3 for the command", res)
	}
	if string(res.Stdout) != "out line\n" || string(res.Stderr) != "err line\n" {
		t.Errorf("stdout %q, stderr %q", res.Stdout, res.Stderr)
	}
}

func TestOutputIsCarriedInFull(t *testing.T) {
	// Well past any pipe buffer, so truncation would show.
	res, err := Shell{}.Run(context.Background(), "validate", ptr(`yes x | head -n 100000; yes y | head -n 100000 >&2; false`), t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res.ExitCode != 1 {
		t.Errorf("exit = %d, want 1", res.ExitCode)
	}
	if len(res.Stdout) != 200000 || len(res.Stderr) != 200000 {
		t.Errorf("captured %d bytes of stdout and %d of stderr, want 200000 each", len(res.Stdout), len(res.Stderr))
	}
}

// AC: AC-CFG-001.3
func TestAStepWithNoCommandIsUnconfiguredNotSucceeded(t *testing.T) {
	res, err := Shell{}.Run(context.Background(), "propagate", nil, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Unconfigured || res.Step != "propagate" {
		t.Errorf("result = %+v, want propagate unconfigured", res)
	}
	res, err = Shell{}.RunAll(context.Background(), "preflight", []string{}, t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Unconfigured || res.Step != "preflight" {
		t.Errorf("result = %+v, want preflight unconfigured", res)
	}
}

func TestPreflightStopsAtTheFirstFailure(t *testing.T) {
	dir := t.TempDir()
	res, err := Shell{}.RunAll(context.Background(), "preflight", []string{"true", "false", "touch x"}, dir)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Failed || res.Command != "false" || res.ExitCode != 1 {
		t.Errorf("result = %+v, want the second command failing", res)
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); !os.IsNotExist(err) {
		t.Errorf("the third command ran: stat x = %v", err)
	}
}

func TestPreflightSucceedsWhenEveryCommandDoes(t *testing.T) {
	dir := t.TempDir()
	res, err := Shell{}.RunAll(context.Background(), "preflight", []string{"true", "touch x"}, dir)
	if err != nil || res.Outcome != Succeeded {
		t.Fatalf("result = %+v, %v; want succeeded", res, err)
	}
	if _, err := os.Stat(filepath.Join(dir, "x")); err != nil {
		t.Errorf("the second command did not run: %v", err)
	}
}

func TestStandardInputIsEmpty(t *testing.T) {
	// cat exits at once on an empty stdin; on an inherited terminal it would
	// wait.
	res, err := Shell{}.Run(context.Background(), "validate", ptr(`test -z "$(cat)"`), t.TempDir())
	if err != nil || res.Outcome != Succeeded {
		t.Errorf("result = %+v, %v; want succeeded on an empty stdin", res, err)
	}
}

func TestAShellThatCannotStartIsAnError(t *testing.T) {
	_, err := Shell{Sh: filepath.Join(t.TempDir(), "no-such-sh")}.Run(context.Background(), "validate", ptr("true"), t.TempDir())
	if err == nil || !strings.Contains(err.Error(), "validate") {
		t.Errorf("err = %v, want an error naming the step", err)
	}
}

func TestOutcomeNames(t *testing.T) {
	for o, want := range map[Outcome]string{Succeeded: "succeeded", Failed: "failed", Unconfigured: "unconfigured", Outcome(9): "Outcome(9)"} {
		if o.String() != want {
			t.Errorf("%d.String() = %q, want %q", int(o), o, want)
		}
	}
}
