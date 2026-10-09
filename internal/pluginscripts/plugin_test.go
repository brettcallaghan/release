package pluginscripts

import (
	"bytes"
	"encoding/json"
	"errors"
	"io/fs"
	"os"
	"path/filepath"
	"testing"
)

var pluginRoot = filepath.Join("..", "..", "plugin")

// A plugin hook would run in every session that enables the plugin, whether
// or not a release skill runs; a top-level bin/ would put a CLI on the PATH
// of every such session. Provisioning happens only inside a skill.
//
// AC: AC-PLG-001.7
func TestPluginProvisionsNothingOutsideASkill(t *testing.T) {
	for _, dir := range []string{"hooks", "bin"} {
		if _, err := os.Stat(filepath.Join(pluginRoot, dir)); !errors.Is(err, fs.ErrNotExist) {
			t.Errorf("plugin/%s exists (err %v); the plugin must provision only from a skill", dir, err)
		}
	}
	raw, err := os.ReadFile(filepath.Join(pluginRoot, ".claude-plugin", "plugin.json"))
	if err != nil {
		t.Fatal(err)
	}
	var manifest map[string]json.RawMessage
	if err := json.Unmarshal(raw, &manifest); err != nil {
		t.Fatalf("plugin.json: %v", err)
	}
	for _, key := range []string{"hooks", "mcpServers", "lspServers"} {
		if _, ok := manifest[key]; ok {
			t.Errorf("plugin.json declares %q; nothing may run in a session without a skill", key)
		}
	}
}

// The skill runs the script by the path the plugin installs it at.
func TestCutReleaseSkillRunsInstallCLIFromThePlugin(t *testing.T) {
	skill, err := os.ReadFile(filepath.Join(pluginRoot, "skills", "cut-release", "SKILL.md"))
	if err != nil {
		t.Fatal(err)
	}
	const run = `bash "${CLAUDE_PLUGIN_ROOT}/skills/cut-release/scripts/install-cli" "${CLAUDE_PLUGIN_DATA}"`
	if !bytes.Contains(skill, []byte(run)) {
		t.Errorf("SKILL.md does not run %s", run)
	}
	info, err := os.Stat(filepath.Join(pluginRoot, "skills", "cut-release", "scripts", "install-cli"))
	if err != nil {
		t.Fatal(err)
	}
	if info.Mode().Perm()&0o111 == 0 {
		t.Errorf("install-cli is not executable: %v", info.Mode())
	}
}
