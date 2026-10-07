package cli

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"reflect"
	"strings"
	"testing"

	"github.com/spf13/cobra"

	"github.com/brettcallaghan/release/internal/config"
)

// writeFile writes content to dir/rel, creating its parent.
func writeFile(t *testing.T, dir, rel, content string) {
	t.Helper()
	path := filepath.Join(dir, rel)
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func load(t *testing.T, dir, explicit string) (config.Config, error) {
	t.Helper()
	cfg, _, err := loadConfig(context.Background(), dir, explicit)
	return cfg, err
}

func refusalReasons(t *testing.T, err error) []string {
	t.Helper()
	var r *configRefusal
	if !errors.As(err, &r) {
		t.Fatalf("err = %T %v, want a configuration refusal", err, err)
	}
	return r.reasons
}

func TestDiscoveryFindsReleaseTomlAtTheRoot(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", `tag_prefix = "rel-"`)
	cfg, err := load(t, dir, "")
	if err != nil || cfg.TagPrefix != "rel-" {
		t.Errorf("tag_prefix = %q, %v; want rel- from release.toml", cfg.TagPrefix, err)
	}
}

func TestDiscoveryFindsConfigReleaseToml(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "config/release.toml", `stage_prefix = "stage/"`)
	cfg, err := load(t, dir, "")
	if err != nil || cfg.StagePrefix != "stage/" {
		t.Errorf("stage_prefix = %q, %v; want stage/ from config/release.toml", cfg.StagePrefix, err)
	}
}

func TestDiscoveryRefusesBothFilesNamingBoth(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "")
	writeFile(t, dir, "config/release.toml", "")
	_, err := load(t, dir, "")
	got := refusalReasons(t, err)
	if len(got) != 1 || !strings.Contains(got[0], "release.toml and config/release.toml") {
		t.Errorf("reasons = %v, want one naming both paths", got)
	}

	// --config settles it.
	writeFile(t, dir, "release.toml", `tag_prefix = "chosen-"`)
	cfg, err := load(t, dir, filepath.Join(dir, "release.toml"))
	if err != nil || cfg.TagPrefix != "chosen-" {
		t.Errorf("with --config: tag_prefix = %q, %v; want chosen-", cfg.TagPrefix, err)
	}
}

func TestNoFileMeansTheDefaults(t *testing.T) {
	cfg, err := load(t, t.TempDir(), "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg, config.Default()) {
		t.Errorf("cfg = %+v, want the defaults", cfg)
	}
}

func TestDiscoveryIsRelativeToTheRepositoryRootNotTheDirectory(t *testing.T) {
	dir := gitFixture(t)
	writeFile(t, dir, "release.toml", `tag_prefix = "rel-"`)
	sub := filepath.Join(dir, "nested", "deeper")
	if err := os.MkdirAll(sub, 0o755); err != nil {
		t.Fatal(err)
	}
	cfg, root, err := loadConfig(context.Background(), sub, "")
	if err != nil || cfg.TagPrefix != "rel-" {
		t.Errorf("tag_prefix = %q, %v; want rel- from the root's release.toml", cfg.TagPrefix, err)
	}
	want, _ := filepath.EvalSymlinks(dir)
	if got, _ := filepath.EvalSymlinks(root); got != want {
		t.Errorf("root = %q, want %q", got, want)
	}
}

func TestAMissingConfigFileIsRefused(t *testing.T) {
	missing := filepath.Join(t.TempDir(), "nope.toml")
	got := refusalReasons(t, func() error { _, err := load(t, t.TempDir(), missing); return err }())
	if len(got) != 1 || !strings.Contains(got[0], "--config names "+missing) {
		t.Errorf("reasons = %v, want one naming the missing file", got)
	}
}

func TestAnUnknownKeyIsRefusedByName(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "tag_prefix = \"v\"\ntag_prefx = \"v\"\n[commands]\nbuild = \"make\"\n")
	got := refusalReasons(t, func() error { _, err := load(t, dir, ""); return err }())
	want := []string{`"tag_prefx" is not a configuration setting`, `"commands.build" is not a configuration setting`}
	if !reflect.DeepEqual(got, want) {
		t.Errorf("reasons = %v, want %v", got, want)
	}
}

// AC: AC-CFG-002.4
func TestAVerificationCheckCannotBeDisabledByConfiguration(t *testing.T) {
	for _, attempt := range []string{
		"skip_checks = true",
		"verify = false",
		"[checks]\nrts_009 = false",
		"[commands]\nskip_validate = true",
	} {
		dir := t.TempDir()
		writeFile(t, dir, "release.toml", attempt)
		if _, err := load(t, dir, ""); err == nil {
			t.Errorf("%q was accepted", attempt)
		} else {
			refusalReasons(t, err)
		}
	}
}

func TestEveryReasonIsReportedTogether(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "stage_prefix = \"\"\nsignals = [\"vibes\"]\nunknown = 1\n")
	got := refusalReasons(t, func() error { _, err := load(t, dir, ""); return err }())
	if len(got) != 3 {
		t.Errorf("reasons = %v, want the unknown key, the empty stage_prefix and the unknown signal", got)
	}
}

func TestAMalformedFileIsRefused(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "leaps = \"sometimes\"\n")
	got := refusalReasons(t, func() error { _, err := load(t, dir, ""); return err }())
	if len(got) != 1 || !strings.Contains(got[0], "leaps") {
		t.Errorf("reasons = %v, want one naming leaps", got)
	}
}

func TestDeclaredListsReplaceTheDefaults(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "signals = [\"release-type\"]\n[commands]\npreflight = [\"make lint\"]\nvalidate = \"make test\"\n")
	cfg, err := load(t, dir, "")
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(cfg.Signals, []string{"release-type"}) {
		t.Errorf("signals = %v, want only release-type", cfg.Signals)
	}
	if !reflect.DeepEqual(cfg.Commands.Preflight, []string{"make lint"}) || cfg.Commands.Validate == nil || *cfg.Commands.Validate != "make test" || cfg.Commands.Propagate != nil {
		t.Errorf("commands = %+v", cfg.Commands)
	}
}

// A refused configuration ends the run before any subcommand's own work: a
// probe command that would write never runs.
func TestARefusedConfigurationRunsNothingElse(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "stage_prefix = \"\"\n")
	marker := filepath.Join(dir, "written")

	var stdout, stderr strings.Builder
	s := &streams{out: &stdout, err: &stderr}
	root := newRootCmd(s)
	root.AddCommand(&cobra.Command{
		Use: "probe",
		RunE: func(*cobra.Command, []string) error {
			return os.WriteFile(marker, nil, 0o644)
		},
	})
	root.SetArgs([]string{"-C", dir, "probe"})
	root.SetOut(&stdout)
	root.SetErr(&stderr)
	if err := root.Execute(); err == nil {
		t.Fatal("the refused configuration was accepted")
	}
	if _, err := os.Stat(marker); !os.IsNotExist(err) {
		t.Errorf("the command ran after the refusal: stat = %v", err)
	}
	if !strings.Contains(stderr.String(), "stage_prefix is empty") {
		t.Errorf("stderr lacks the reason:\n%s", stderr.String())
	}
}

func TestHelpAndTheRootLoadNothing(t *testing.T) {
	dir := t.TempDir()
	writeFile(t, dir, "release.toml", "unknown = 1\n")
	for _, args := range [][]string{{"-C", dir, "--version"}, {"-C", dir, "help"}, {"-C", dir, "config", "--help"}} {
		if _, stderr, code := run(args...); code != 0 {
			t.Errorf("%v: exit %d\n%s", args, code, stderr)
		}
	}
}

// gitFixture is an empty git repository with one commit.
func gitFixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	runFixtureGit(t, dir, []string{"init", "-q", "-b", "main"})
	runFixtureGit(t, dir, []string{"commit", "-q", "--allow-empty", "-m", "Initial commit"})
	return dir
}
