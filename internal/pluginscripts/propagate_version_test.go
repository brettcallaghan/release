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

const manifestPath = "plugin/.claude-plugin/plugin.json"

// propagateTree copies the file the script writes into a temporary
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
	data, err := os.ReadFile(filepath.Join("..", "..", manifestPath))
	if err != nil {
		t.Fatal(err)
	}
	mustMkdir(t, filepath.Dir(filepath.Join(root, manifestPath)))
	mustWrite(t, filepath.Join(root, manifestPath), data, 0o644)
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
	requireNoPropagateTemps(t, root)
}

// The marketplace that vends the plugin lives in brettcallaghan/agent-plugins,
// so the release commit carries the plugin manifest and nothing else of the
// plugin's.
func TestReleasePathsCarryOnlyThePluginManifest(t *testing.T) {
	cfg := config.Default()
	if _, err := toml.DecodeFile(filepath.Join("..", "..", "config", "release.toml"), &cfg); err != nil {
		t.Fatalf("loading config/release.toml: %v", err)
	}
	if want := []string{manifestPath}; !reflect.DeepEqual(cfg.ReleasePaths, want) {
		t.Errorf("release_paths = %v, want %v", cfg.ReleasePaths, want)
	}
}

func TestPropagateFailsWithoutAVersionFieldAndWritesNothing(t *testing.T) {
	root := propagateTree(t)
	mp := filepath.Join(root, manifestPath)
	data := []byte(`{"name": "release"}` + "\n")
	mustWrite(t, mp, data, 0o644)

	out, err := runPropagate(t, root, "0.2.0")
	var exitErr *exec.ExitError
	if !errors.As(err, &exitErr) {
		t.Fatalf("err = %v, want a non-zero exit\n%s", err, out)
	}
	if !bytes.Contains([]byte(out), []byte("no version field")) {
		t.Errorf("output = %q, want it to name the missing field", out)
	}
	if after, _ := os.ReadFile(mp); !bytes.Equal(after, data) {
		t.Error("plugin.json was written although the script failed")
	}
	requireNoPropagateTemps(t, root)
}

func requireNoPropagateTemps(t *testing.T, root string) {
	t.Helper()
	left, _ := filepath.Glob(filepath.Join(filepath.Dir(filepath.Join(root, manifestPath)), ".propagate-*"))
	if len(left) != 0 {
		t.Errorf("temp files left beside %s: %v", manifestPath, left)
	}
}
