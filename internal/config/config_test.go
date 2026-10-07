package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"reflect"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/resolve"
)

func reasons(t *testing.T, c Config) []string {
	t.Helper()
	err := c.Validate()
	if err == nil {
		return nil
	}
	var r *Refusal
	if !errors.As(err, &r) {
		t.Fatalf("Validate() = %T %v, want *Refusal", err, err)
	}
	return r.Reasons
}

// AC: AC-CFG-002.1
func TestUnconfiguredValuesTakeTheirDocumentedDefaults(t *testing.T) {
	d := Default()
	if d.StagePrefix != "cut/" {
		t.Errorf("stage_prefix default = %q, want cut/", d.StagePrefix)
	}
	if d.Target != "" {
		t.Errorf("target default = %q, want empty (the repository's default branch)", d.Target)
	}
	want := Config{
		TagPrefix:       "v",
		StagePrefix:     "cut/",
		ReleaseBranches: []string{},
		ReleasePaths:    []string{},
		Leaps:           false,
		Pre1Remap:       true,
		Signals:         []string{"release-type", "conventional-type", "breaking-marker"},
		Commands:        Commands{Preflight: []string{}},
	}
	if !reflect.DeepEqual(d, want) {
		t.Errorf("Default() = %+v\nwant %+v", d, want)
	}
	if err := d.Validate(); err != nil {
		t.Errorf("the defaults are refused: %v", err)
	}
	if got := d.Policy(); !reflect.DeepEqual(got, resolve.DefaultPolicy()) {
		t.Errorf("Default().Policy() = %+v, want resolve.DefaultPolicy() %+v", got, resolve.DefaultPolicy())
	}
}

// AC: AC-CFG-002.1
func TestTargetDefaultsToTheBranchOriginHEADPointsTo(t *testing.T) {
	dir := fixture(t)
	git(t, dir, "update-ref", "refs/remotes/origin/trunk", "HEAD")
	git(t, dir, "symbolic-ref", "refs/remotes/origin/HEAD", "refs/remotes/origin/trunk")

	repo := gitread.Repo{Dir: dir}
	ref, err := Default().TargetRef(context.Background(), repo)
	if err != nil || ref != "refs/remotes/origin/trunk" {
		t.Errorf("TargetRef = %q, %v; want refs/remotes/origin/trunk", ref, err)
	}
	branch, err := Default().TargetBranch(context.Background(), repo)
	if err != nil || branch != "trunk" {
		t.Errorf("TargetBranch = %q, %v; want trunk", branch, err)
	}
}

func TestOriginHEADUnsetRefusesNamingTheFix(t *testing.T) {
	dir := fixture(t)
	_, err := Default().TargetRef(context.Background(), gitread.Repo{Dir: dir})
	if !errors.Is(err, ErrNoDefaultBranch) {
		t.Fatalf("TargetRef err = %v, want ErrNoDefaultBranch", err)
	}
	if !strings.Contains(err.Error(), "git remote set-head origin --auto") {
		t.Errorf("refusal %q does not name git remote set-head origin --auto", err)
	}
	if _, err := Default().TargetBranch(context.Background(), gitread.Repo{Dir: dir}); !errors.Is(err, ErrNoDefaultBranch) {
		t.Errorf("TargetBranch err = %v, want ErrNoDefaultBranch", err)
	}
}

func TestAConfiguredTargetNeedsNoOriginHEAD(t *testing.T) {
	dir := fixture(t)
	c := Default()
	c.Target = "develop"
	ref, err := c.TargetRef(context.Background(), gitread.Repo{Dir: dir})
	if err != nil || ref != "develop" {
		t.Errorf("TargetRef = %q, %v; want develop", ref, err)
	}
	branch, err := c.TargetBranch(context.Background(), gitread.Repo{Dir: dir})
	if err != nil || branch != "develop" {
		t.Errorf("TargetBranch = %q, %v; want develop", branch, err)
	}
}

// AC: AC-CFG-002.2
func TestAReleaseBranchGlobMatchingAStageBranchIsRefused(t *testing.T) {
	cases := []struct {
		name        string
		stagePrefix string
		globs       []string
		refused     bool
	}{
		{"release glob over a release/ stage prefix", "release/", []string{"release/*"}, true},
		{"catch-all matches the default stage branch", "cut/", []string{"*/*"}, true},
		{"glob matching only the prerelease stage branch", "cut/", []string{"cut/*-rc.*"}, true},
		{"distinct namespaces", "cut/", []string{"release/*"}, false},
		{"star does not cross a slash", "stage/cut/", []string{"stage/*"}, false},
		{"no globs", "cut/", []string{}, false},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			c.StagePrefix, c.ReleaseBranches = tc.stagePrefix, tc.globs
			got := reasons(t, c)
			if refused := len(got) > 0; refused != tc.refused {
				t.Fatalf("refused = %v (%v), want %v", refused, got, tc.refused)
			}
			if tc.refused && !strings.Contains(got[0], "RTS-009") {
				t.Errorf("reason %q does not cite RTS-009", got[0])
			}
		})
	}
}

func TestAMalformedGlobIsRefused(t *testing.T) {
	c := Default()
	c.ReleaseBranches = []string{"release/["}
	got := reasons(t, c)
	if len(got) != 1 || !strings.Contains(got[0], "malformed") {
		t.Errorf("reasons = %v, want one naming the malformed glob", got)
	}
}

func TestEveryReasonIsReturnedNotOnlyTheFirst(t *testing.T) {
	c := Default()
	c.StagePrefix = ""
	c.Signals = []string{"release-type", "gut-feeling"}
	got := reasons(t, c)
	if len(got) != 2 {
		t.Fatalf("reasons = %v, want two", got)
	}
	if !strings.Contains(got[0], "stage_prefix") || !strings.Contains(got[1], `"gut-feeling"`) {
		t.Errorf("reasons = %v, want the empty stage_prefix and the unknown signal", got)
	}
	msg := c.Validate().Error()
	if !strings.Contains(msg, got[0]) || !strings.Contains(msg, got[1]) {
		t.Errorf("Error() = %q, want both reasons", msg)
	}
}

func TestPolicyCarriesTheDeclaredSettings(t *testing.T) {
	c := Default()
	c.TagPrefix, c.StagePrefix = "rel-", "stage/"
	c.Leaps, c.Pre1Remap = true, false
	c.Signals = []string{"release-type"}
	p := c.Policy()
	if p.TagPrefix != "rel-" || p.StagePrefix != "stage/" || !p.Version.Leaps || p.Version.Pre1Remap {
		t.Errorf("Policy() = %+v", p)
	}
	if !reflect.DeepEqual(p.Signals, bump.Signals{bump.SignalReleaseType: true}) {
		t.Errorf("Policy().Signals = %v, want only release-type", p.Signals)
	}
}

// AC: AC-CFG-002.5
func TestATargetNameSuggestingAMajorDerivesNoConstraint(t *testing.T) {
	for _, target := range []string{"release/v1", "v2.x", "maint-3"} {
		c := Default()
		c.Target = target
		if err := c.Validate(); err != nil {
			t.Errorf("target %q: refused: %v", target, err)
		}
		if p := c.Policy(); !reflect.DeepEqual(p, resolve.DefaultPolicy()) {
			t.Errorf("target %q changed the policy: %+v", target, p)
		}
		ref, err := c.TargetRef(context.Background(), gitread.Repo{Dir: t.TempDir()})
		if err != nil || ref != target {
			t.Errorf("target %q: TargetRef = %q, %v; want the name unchanged", target, ref, err)
		}
	}
}

func fixture(t *testing.T) string {
	t.Helper()
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	git(t, dir, "init", "-q", "-b", "main")
	git(t, dir, "commit", "-q", "--allow-empty", "-m", "Initial commit")
	return dir
}

func git(t *testing.T, dir string, args ...string) {
	t.Helper()
	cmd := exec.Command("git", append([]string{"-C", dir}, args...)...)
	cmd.Env = append(os.Environ(),
		"GIT_CONFIG_GLOBAL="+os.DevNull, "GIT_CONFIG_NOSYSTEM=1",
		"GIT_AUTHOR_NAME=Fixture", "GIT_AUTHOR_EMAIL=fixture@example.com",
		"GIT_COMMITTER_NAME=Fixture", "GIT_COMMITTER_EMAIL=fixture@example.com",
	)
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("git %s: %v\n%s", strings.Join(args, " "), err, out)
	}
}
