// Package producers holds the changelog producers this release ships, each
// conforming to the Changelog Producer component: the `claude` producer, the
// default, and the `command` producer. Both run outside the process and
// exchange the corpus as JSON over standard input and output, so neither
// holds a model client or a credential of its own.
package producers

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"os/exec"
	"strings"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/config"
)

// Unavailable is a producer's refusal to run: a prerequisite it names is
// missing.
type Unavailable struct {
	Producer     string
	Prerequisite string
	// Hint says what the maintainer can do instead.
	Hint string
}

func (u *Unavailable) Error() string {
	msg := fmt.Sprintf("the %s producer cannot run: %s", u.Producer, u.Prerequisite)
	if u.Hint != "" {
		msg += "; " + u.Hint
	}
	return msg
}

// alternatives is the hint a refusal gives.
const alternatives = `select another producer with changelog.producer, or name the bump and pass --skip-notes`

// Registry is every producer a configuration can select, by name, for the
// repository at dir.
func Registry(c config.Changelog, dir string) map[string]authoring.Producer {
	command := Command{Dir: dir}
	if c.ProducerCommand != nil {
		command.Command = *c.ProducerCommand
	}
	return map[string]authoring.Producer{
		config.ProducerClaudeName:  Claude{Dir: dir},
		config.ProducerCommandName: command,
	}
}

// Command is the `command` producer: it runs a configured command through
// `sh -c` with the assembled corpus on standard input, and reads the
// authored corpus from standard output.
type Command struct {
	// Command is changelog.producer_command.
	Command string
	// Dir is the directory the command runs in: the repository root.
	Dir string
	// Sh is the shell; empty means "sh" from PATH.
	Sh string
}

// Produce runs the command. A non-zero exit is a failure.
func (c Command) Produce(ctx context.Context, assembled []byte) ([]byte, error) {
	if strings.TrimSpace(c.Command) == "" {
		return nil, &Unavailable{Producer: config.ProducerCommandName, Prerequisite: "changelog.producer_command is unset", Hint: alternatives}
	}
	sh := c.Sh
	if sh == "" {
		sh = "sh"
	}
	bin, err := exec.LookPath(sh)
	if err != nil {
		return nil, &Unavailable{Producer: config.ProducerCommandName, Prerequisite: fmt.Sprintf("%s is not on PATH", sh), Hint: alternatives}
	}
	return run(ctx, c.Dir, assembled, bin, "-c", c.Command)
}

// ClaudePrompt is what the `claude` producer asks of `claude -p`: run the
// release plugin's changelog skill over the corpus on standard input.
const ClaudePrompt = "Use the release:changelog skill to author the changelog corpus on standard input. " +
	"Write only the authored corpus, as JSON, to standard output."

// Claude is the `claude` producer, the default: it runs `claude -p` with a
// prompt invoking the release:changelog skill, the assembled corpus on
// standard input, and reads the authored corpus from standard output. It
// uses the maintainer's own Claude Code session, so it needs no credential.
type Claude struct {
	// Bin is the claude executable; empty means "claude" from PATH.
	Bin string
	// Dir is the directory claude runs in: the repository root.
	Dir string
}

// Produce runs claude. A non-zero exit is a failure.
func (c Claude) Produce(ctx context.Context, assembled []byte) ([]byte, error) {
	name := c.Bin
	if name == "" {
		name = "claude"
	}
	bin, err := exec.LookPath(name)
	if err != nil {
		return nil, &Unavailable{
			Producer:     config.ProducerClaudeName,
			Prerequisite: fmt.Sprintf("the %s command is not on PATH (install Claude Code and the release plugin)", name),
			Hint:         alternatives,
		}
	}
	out, err := run(ctx, c.Dir, assembled, bin, "-p", ClaudePrompt)
	if err != nil {
		return nil, err
	}
	return unfence(out), nil
}

// unfence strips one markdown code fence wrapping the whole output, which a
// model may add despite the prompt. Anything else is returned as it came.
func unfence(out []byte) []byte {
	t := bytes.TrimSpace(out)
	if !bytes.HasPrefix(t, []byte("```")) || !bytes.HasSuffix(t, []byte("```")) {
		return out
	}
	nl := bytes.IndexByte(t, '\n')
	if nl < 0 {
		return out
	}
	inner := t[nl+1 : len(t)-3]
	if bytes.Contains(inner, []byte("```")) {
		return out
	}
	return inner
}

// Failure is a producer process that ran and failed: it exited
// unsuccessfully, or exited 0 and wrote no corpus.
type Failure struct {
	ExitCode int
	Stderr   string
	// Reason, WHEN set, says why a process that exited 0 failed.
	Reason string
}

func (f *Failure) Error() string {
	if f.Reason != "" {
		return f.Reason
	}
	msg := strings.TrimSpace(f.Stderr)
	if msg == "" {
		return fmt.Sprintf("exit status %d", f.ExitCode)
	}
	return fmt.Sprintf("exit status %d: %s", f.ExitCode, msg)
}

func run(ctx context.Context, dir string, stdin []byte, bin string, args ...string) ([]byte, error) {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Dir = dir
	cmd.Stdin = bytes.NewReader(stdin)
	var stdout, stderr bytes.Buffer
	cmd.Stdout, cmd.Stderr = &stdout, &stderr
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case errors.As(err, &exitErr):
		return nil, &Failure{ExitCode: exitErr.ExitCode(), Stderr: stderr.String()}
	case err != nil:
		return nil, err
	}
	if len(bytes.TrimSpace(stdout.Bytes())) == 0 {
		return nil, &Failure{Stderr: stderr.String(), Reason: "it wrote no corpus to standard output"}
	}
	return stdout.Bytes(), nil
}
