package producers

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// stubEnv turns this test binary into a stub producer: TestMain sees it set
// and, instead of running tests, reads a corpus on standard input and
// answers as the mode it names.
const (
	stubEnv     = "RELEASE_TEST_PRODUCER_STUB"
	stubArgsEnv = "RELEASE_TEST_PRODUCER_ARGS"
)

func TestMain(m *testing.M) {
	if mode := os.Getenv(stubEnv); mode != "" {
		os.Exit(stub(mode))
	}
	os.Exit(m.Run())
}

// stub authors a corpus the way a conforming producer would: it carries every
// entry and source through, writes prose, proposes an impact, and leaves the
// version unset.
func stub(mode string) int {
	if f := os.Getenv(stubArgsEnv); f != "" {
		_ = os.WriteFile(f, []byte(strings.Join(os.Args[1:], "\x00")), 0o644)
	}
	in, err := io.ReadAll(os.Stdin)
	if err != nil {
		return 9
	}
	if mode == "fail" {
		os.Stderr.WriteString("the model is unavailable\n")
		return 3
	}
	if mode == "silent" {
		return 0
	}
	var c map[string]any
	if err := json.Unmarshal(in, &c); err != nil {
		os.Stderr.WriteString("stub: input is not JSON\n")
		return 4
	}
	c["proposed_bump"] = "PATCH"
	if entries, _ := c["entries"].([]any); len(entries) > 0 {
		c["summary"] = "Authored by the stub — ünïcode kept."
	}
	out, _ := json.Marshal(c)
	if mode == "fenced" {
		out = append(append([]byte("```json\n"), out...), "\n```\n"...)
	}
	os.Stdout.Write(out)
	return 0
}

func self(t *testing.T) string {
	t.Helper()
	exe, err := os.Executable()
	if err != nil {
		t.Fatal(err)
	}
	return exe
}

// stubClaude puts a `claude` on PATH that is this test binary in stub mode,
// and returns the file its arguments are recorded in.
func stubClaude(t *testing.T, mode string) string {
	t.Helper()
	dir := t.TempDir()
	name := "claude"
	if runtime.GOOS == "windows" {
		name += ".exe"
	}
	data, err := os.ReadFile(self(t))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, name), data, 0o755); err != nil {
		t.Fatal(err)
	}
	t.Setenv("PATH", dir+string(os.PathListSeparator)+os.Getenv("PATH"))
	t.Setenv(stubEnv, mode)
	args := filepath.Join(t.TempDir(), "args")
	t.Setenv(stubArgsEnv, args)
	return args
}

// stubCommand is a producer_command that runs this test binary in stub mode.
func stubCommand(t *testing.T, mode string) Command {
	t.Helper()
	t.Setenv(stubEnv, mode)
	return Command{Command: "'" + filepath.ToSlash(self(t)) + "'", Dir: t.TempDir()}
}

func assembled(entries ...releasemodel.ChangelogEntry) []byte {
	if entries == nil {
		entries = []releasemodel.ChangelogEntry{}
	}
	c := releasemodel.Changelog{
		SchemaVersion: "1.0.0",
		Entries:       entries,
		Sources:       []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, To: "1111111111111111111111111111111111111111"}},
	}
	data, err := authoring.Marshal(c)
	if err != nil {
		panic(err)
	}
	return data
}

var someEntries = []releasemodel.ChangelogEntry{
	{Kind: kp(releasemodel.ChangeKindFeature), Summary: "add --target", Derived: true,
		Sources: []releasemodel.ChangeSource{releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 12})}},
	{Kind: kp(releasemodel.ChangeKindFix), Summary: "crash", Derived: true,
		Sources: []releasemodel.ChangeSource{releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: "abc"})}},
}

// conformance is the suite every producer runs (the Changelog Producer
// component's verification gate): an empty corpus yields an empty authored
// corpus, every result validates and proposes an impact, every entry and its
// sources are carried, and the version is left unset.
func conformance(t *testing.T, p authoring.Producer) {
	t.Helper()
	t.Run("empty corpus", func(t *testing.T) {
		out, err := p.Produce(context.Background(), assembled())
		if err != nil {
			t.Fatal(err)
		}
		c, err := authoring.Accept(out, "the producer")
		if err != nil {
			t.Fatal(err)
		}
		if len(c.Entries) != 0 {
			t.Errorf("an empty corpus came back with %d entries", len(c.Entries))
		}
		if c.Version != nil {
			t.Errorf("version = %q, want unset", *c.Version)
		}
	})
	t.Run("entries", func(t *testing.T) {
		out, err := p.Produce(context.Background(), assembled(someEntries...))
		if err != nil {
			t.Fatal(err)
		}
		c, err := authoring.Accept(out, "the producer")
		if err != nil {
			t.Fatal(err)
		}
		if c.Version != nil {
			t.Errorf("version = %q, want unset", *c.Version)
		}
		if len(c.Entries) != len(someEntries) {
			t.Fatalf("entries = %d, want %d", len(c.Entries), len(someEntries))
		}
		for i, e := range c.Entries {
			got, _ := json.Marshal(e.Sources)
			want, _ := json.Marshal(someEntries[i].Sources)
			if !bytes.Equal(got, want) {
				t.Errorf("entry %d sources = %s, want %s", i, got, want)
			}
		}
		if len(c.Sources) != 1 || c.Sources[0].Adapter != releasemodel.AdapterCommits {
			t.Errorf("corpus sources = %+v, want the commit adapter carried through", c.Sources)
		}
	})
}

func TestCommandProducerConforms(t *testing.T) {
	conformance(t, stubCommand(t, "author"))
}

func TestClaudeProducerConforms(t *testing.T) {
	stubClaude(t, "author")
	conformance(t, Claude{Dir: t.TempDir()})
}

func TestClaudeProducerInvokesTheChangelogSkill(t *testing.T) {
	argsFile := stubClaude(t, "author")
	if _, err := (Claude{Dir: t.TempDir()}).Produce(context.Background(), assembled()); err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile(argsFile)
	if err != nil {
		t.Fatal(err)
	}
	args := strings.Split(string(data), "\x00")
	if len(args) != 2 || args[0] != "-p" || !strings.Contains(args[1], "release:changelog") {
		t.Errorf("claude was run with %q, want -p and a prompt invoking release:changelog", args)
	}
}

func TestClaudeProducerStripsAWrappingFence(t *testing.T) {
	stubClaude(t, "fenced")
	out, err := (Claude{Dir: t.TempDir()}).Produce(context.Background(), assembled(someEntries...))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := authoring.Accept(out, "claude"); err != nil {
		t.Errorf("fenced output was not unwrapped: %v\n%s", err, out)
	}
}

func TestUnfenceLeavesOtherOutputAlone(t *testing.T) {
	for _, s := range []string{`{"a":1}`, "```json\n{}", "```\n{}\n```\nmore\n```", "```"} {
		if got := unfence([]byte(s)); string(got) != s {
			t.Errorf("unfence(%q) = %q, want it unchanged", s, got)
		}
	}
}

func TestClaudeAbsentFromPathIsRefusedByName(t *testing.T) {
	t.Setenv("PATH", t.TempDir())
	_, err := (Claude{}).Produce(context.Background(), assembled())
	var u *Unavailable
	if !errors.As(err, &u) || u.Producer != "claude" || !strings.Contains(err.Error(), "claude command is not on PATH") {
		t.Errorf("err = %v, want a refusal naming the claude command", err)
	}
}

func TestCommandProducerRefusals(t *testing.T) {
	var u *Unavailable
	_, err := (Command{}).Produce(context.Background(), assembled())
	if !errors.As(err, &u) || !strings.Contains(err.Error(), "changelog.producer_command") {
		t.Errorf("no command: err = %v, want a refusal naming changelog.producer_command", err)
	}
	_, err = (Command{Command: "true", Sh: "no-such-shell-here"}).Produce(context.Background(), assembled())
	if !errors.As(err, &u) || !strings.Contains(err.Error(), "no-such-shell-here") {
		t.Errorf("no shell: err = %v, want a refusal naming the shell", err)
	}
}

func TestANonZeroExitFails(t *testing.T) {
	_, err := stubCommand(t, "fail").Produce(context.Background(), assembled())
	var f *Failure
	if !errors.As(err, &f) || f.ExitCode != 3 || !strings.Contains(err.Error(), "the model is unavailable") {
		t.Errorf("command: err = %v, want exit status 3 with the producer's stderr", err)
	}
	stubClaude(t, "fail")
	if _, err := (Claude{Dir: t.TempDir()}).Produce(context.Background(), assembled()); !errors.As(err, &f) {
		t.Errorf("claude: err = %v, want a *Failure", err)
	}
}

func TestNoOutputFails(t *testing.T) {
	_, err := stubCommand(t, "silent").Produce(context.Background(), assembled())
	var f *Failure
	if !errors.As(err, &f) || !strings.Contains(err.Error(), "no corpus") {
		t.Errorf("err = %v, want a *Failure saying nothing came back", err)
	}
}

// AC-CLOG-003.8 participates here: configuration names a producer, and the
// registry resolves that name to the producer the bridge invokes.
func TestRegistrySelectsByConfiguredName(t *testing.T) {
	cmd := "./author.sh"
	reg := Registry(config.Changelog{Producer: "command", ProducerCommand: &cmd}, "/repo")
	if got, ok := reg["command"].(Command); !ok || got.Command != cmd || got.Dir != "/repo" {
		t.Errorf("command producer = %+v", reg["command"])
	}
	if got, ok := reg["claude"].(Claude); !ok || got.Dir != "/repo" {
		t.Errorf("claude producer = %+v", reg["claude"])
	}
	if len(reg) != len(config.Producers) {
		t.Errorf("registry has %d producers, config.Producers names %d", len(reg), len(config.Producers))
	}
}

// AC-CLOG-003.7 participates here: the default producer reaches a model only
// through the claude command, a separate process using the maintainer's own
// session, with nothing but the corpus and a prompt.
func TestProducersExchangeThroughAProcessOnly(t *testing.T) {
	stubClaude(t, "author")
	out, err := (Claude{Dir: t.TempDir()}).Produce(context.Background(), assembled(someEntries...))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Contains(out, []byte("ünïcode kept")) {
		t.Errorf("the producer's prose did not come back verbatim: %s", out)
	}
}

// kp returns a pointer to an entry kind.
func kp(k releasemodel.ChangeKind) *releasemodel.ChangeKind { return &k }
