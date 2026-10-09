package pluginscripts

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"runtime"
	"testing"

	"github.com/BurntSushi/toml"

	"github.com/brettcallaghan/release/internal/config"
)

const (
	manifestPath    = "plugin/.claude-plugin/plugin.json"
	marketplacePath = ".claude-plugin/marketplace.json"
)

// propagateTree copies the two files the script writes into a temporary
// repository root, as the stage checkout `release cut` runs it in.
func propagateTree(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("propagate-version.sh runs under sh on unix-like systems")
	}
	for _, need := range []string{"sh", "python3"} {
		if _, err := exec.LookPath(need); err != nil {
			t.Skipf("%s is not on the PATH", need)
		}
	}
	root := t.TempDir()
	for _, p := range []string{manifestPath, marketplacePath} {
		data, err := os.ReadFile(filepath.Join("..", "..", p))
		if err != nil {
			t.Fatal(err)
		}
		mustMkdir(t, filepath.Dir(filepath.Join(root, p)))
		mustWrite(t, filepath.Join(root, p), data, 0o644)
	}
	return root
}

func runPropagate(t *testing.T, root, version string) (string, error) {
	t.Helper()
	script, err := filepath.Abs(filepath.Join("..", "..", "scripts", "propagate-version.sh"))
	if err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("sh", script)
	cmd.Dir = root
	cmd.Env = append(os.Environ(), "RELEASE_VERSION="+version)
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err = cmd.Run()
	return out.String(), err
}

func readJSON(t *testing.T, path string) map[string]any {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	var v map[string]any
	if err := json.Unmarshal(data, &v); err != nil {
		t.Fatalf("%s: %v", path, err)
	}
	return v
}

// requireReleasePath checks that the repository's release configuration
// lets the release commit carry path.
func requireReleasePath(t *testing.T, path string) {
	t.Helper()
	cfg := config.Default()
	if _, err := toml.DecodeFile(filepath.Join("..", "..", "config", "release.toml"), &cfg); err != nil {
		t.Fatalf("loading config/release.toml: %v", err)
	}
	for _, p := range cfg.ReleasePaths {
		if p == path {
			return
		}
	}
	t.Errorf("config/release.toml release_paths %v does not list %s", cfg.ReleasePaths, path)
}

// AC: AC-PLG-004.1
func TestCutWritesThePluginVersion(t *testing.T) {
	requireReleasePath(t, manifestPath)
	root := propagateTree(t)
	if out, err := runPropagate(t, root, "0.2.0"); err != nil {
		t.Fatalf("propagate-version.sh: %v\n%s", err, out)
	}
	if got := readJSON(t, filepath.Join(root, manifestPath))["version"]; got != "0.2.0" {
		t.Errorf("plugin.json version = %v, want 0.2.0", got)
	}
}

// AC: AC-PLG-004.3
func TestCutPinsTheMarketplaceToItsTag(t *testing.T) {
	requireReleasePath(t, marketplacePath)
	root := propagateTree(t)
	before := readJSON(t, filepath.Join(root, marketplacePath))
	if out, err := runPropagate(t, root, "0.2.0"); err != nil {
		t.Fatalf("propagate-version.sh: %v\n%s", err, out)
	}
	after := readJSON(t, filepath.Join(root, marketplacePath))

	want := map[string]any{"source": "git-subdir", "url": "brettcallaghan/release", "path": "plugin", "ref": "v0.2.0"}
	var entry map[string]any
	for _, p := range after["plugins"].([]any) {
		if m := p.(map[string]any); m["name"] == "release" {
			entry = m
		}
	}
	if entry == nil {
		t.Fatal("the release entry is gone")
	}
	if !reflect.DeepEqual(entry["source"], want) {
		t.Errorf("source = %v, want %v", entry["source"], want)
	}

	// Everything else is as it was.
	for _, p := range before["plugins"].([]any) {
		if m := p.(map[string]any); m["name"] == "release" {
			m["source"] = want
		}
	}
	if !reflect.DeepEqual(before, after) {
		t.Errorf("the marketplace changed beyond the release entry's source:\nbefore %v\nafter  %v", before, after)
	}
}

func TestPropagateFailsWithoutAReleaseEntryAndWritesNothing(t *testing.T) {
	root := propagateTree(t)
	mp := filepath.Join(root, marketplacePath)
	market := readJSON(t, mp)
	market["plugins"] = []any{}
	data, _ := json.Marshal(market)
	mustWrite(t, mp, data, 0o644)
	manifestBefore, _ := os.ReadFile(filepath.Join(root, manifestPath))

	out, err := runPropagate(t, root, "0.2.0")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a non-zero exit\n%s", err, out)
	}
	if !bytes.Contains([]byte(out), []byte("no plugin entry named release")) {
		t.Errorf("output = %q, want it to name the missing entry", out)
	}
	if after, _ := os.ReadFile(filepath.Join(root, manifestPath)); !bytes.Equal(after, manifestBefore) {
		t.Error("plugin.json was written although the script failed")
	}
	if after, _ := os.ReadFile(mp); !bytes.Equal(after, data) {
		t.Error("marketplace.json was written although the script failed")
	}
}
