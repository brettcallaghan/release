package cli

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/spf13/cobra"
)

// helpDir holds the golden help output, one file per command, which the
// release CLI contract makes normative for the command surface's shape.
const helpDir = "../../testdata/help"

// TestGoldenHelp asserts `--help` for every command in the tree against
// testdata/help/<command path>.txt, and fails on a golden file whose command
// no longer exists. Regenerate with scripts/regenerate-snapshots.sh.
func TestGoldenHelp(t *testing.T) {
	want := map[string]bool{}
	var walk func(c *cobra.Command)
	walk = func(c *cobra.Command) {
		if c.Hidden || c.Name() == "help" {
			return
		}
		name := strings.ReplaceAll(c.CommandPath(), " ", "-") + ".txt"
		want[name] = true
		args := append(strings.Fields(c.CommandPath())[1:], "--help")

		t.Run(name, func(t *testing.T) {
			var stdout, stderr bytes.Buffer
			if code := Execute(args, &stdout, &stderr); code != int(StatusSuccess) {
				t.Fatalf("%v: exit %d\n%s", args, code, stderr.String())
			}
			path := filepath.Join(helpDir, name)
			if *update {
				if err := os.WriteFile(path, stdout.Bytes(), 0o644); err != nil {
					t.Fatal(err)
				}
			}
			golden, err := os.ReadFile(path)
			if err != nil {
				t.Fatalf("no golden help for %q: %v (run scripts/regenerate-snapshots.sh)", c.CommandPath(), err)
			}
			if !bytes.Equal(stdout.Bytes(), golden) {
				t.Errorf("--help for %q differs from %s\n--- got ---\n%s\n--- want ---\n%s",
					c.CommandPath(), path, stdout.String(), golden)
			}
		})
		for _, sub := range c.Commands() {
			walk(sub)
		}
	}
	walk(newRootCmd(&streams{}))

	entries, err := os.ReadDir(helpDir)
	if err != nil {
		t.Fatal(err)
	}
	for _, e := range entries {
		if !want[e.Name()] {
			t.Errorf("testdata/help/%s has no command; delete it", e.Name())
		}
	}
}
