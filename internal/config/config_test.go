package config

import (
	"context"
	"errors"
	"os"
	"os/exec"
	"path"
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
		Changelog: Changelog{
			Groups: []ChangelogGroup{
				{Kind: "FEATURE", Heading: "Features"},
				{Kind: "FIX", Heading: "Fixes"},
				{Kind: "PERFORMANCE", Heading: "Performance"},
				{Kind: "REVERT", Heading: "Reverts"},
			},
			OtherHeading:    "Other changes",
			BreakingHeading: "Breaking changes",
			Producer:        "claude",
		},
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

// AC: AC-CFG-002.2
func TestOverlapIsCheckedAcrossEveryVersionNotOnlyTheProbes(t *testing.T) {
	cases := []struct {
		glob    string
		refused bool
	}{
		{"cut/2.*", true},                // matches cut/2.0.0, which no probe is
		{"cut/[0-9]*", true},             // a class
		{"cu?/1.*", true},                // ? over the prefix
		{"[a-c]ut/*", true},              // a class over the prefix
		{`cut\/*`, true},                 // an escaped literal
		{"cut/*-rc.*", true},             // prerelease stage branches only
		{"cut/latest", true},             // under the stage prefix, in version characters
		{"cut/*/x", false},               // a version holds no slash
		{"cut", false},                   // the prefix alone is no stage branch
		{"cut/", false},                  // nor is the prefix with no version
		{"release/*", false},             // another namespace
		{"*", false},                     // * does not cross the prefix's slash
		{"cut/_*", false},                // _ is no version character
		{`cut/[^0-9a-zA-Z.+\-]*`, false}, // no version character
	}
	for _, tc := range cases {
		c := Default()
		c.ReleaseBranches = []string{tc.glob}
		got := reasons(t, c)
		if refused := len(got) > 0; refused != tc.refused {
			t.Errorf("glob %q: refused = %v (%v), want %v", tc.glob, refused, got, tc.refused)
		}
	}
}

// Every stage branch a glob actually matches must be caught: the check may
// over-refuse, never under-refuse.
func TestOverlapCheckNeverMissesAMatchingStageBranch(t *testing.T) {
	versions := []string{"0.1.0", "1.2.3", "2.0.0", "10.20.30", "1.2.3-rc.1", "3.0.0-beta.2", "1.0.0+build.7"}
	globs := []string{"cut/2.*", "cut/1*", "cut/*.0", "*/3.*", "cut/?.?.?", "cut/[13]*", "cut/*+*", "cut/1?.*", "cut/*rc*", "c*/0.1.0"}
	for _, g := range globs {
		for _, v := range versions {
			stage := "cut/" + v
			if ok, _ := path.Match(g, stage); ok && !matchesStageName(g, "cut/") {
				t.Errorf("glob %q matches %q but the check missed it", g, stage)
			}
		}
	}
}

func TestARefusalNamesAnExampleWhenAProbeMatches(t *testing.T) {
	c := Default()
	c.ReleaseBranches = []string{"cut/1.*", "cut/2.*"}
	got := reasons(t, c)
	if len(got) != 2 || !strings.Contains(got[0], `such as "cut/1.2.3"`) || strings.Contains(got[1], "such as") {
		t.Errorf("reasons = %v, want an example for cut/1.* and none for cut/2.*", got)
	}
}

func TestAnUnknownChangelogKindIsRefused(t *testing.T) {
	c := Default()
	c.Changelog.Groups = []ChangelogGroup{{Kind: "SECURITY", Heading: "Security"}}
	got := reasons(t, c)
	if len(got) != 1 || !strings.Contains(got[0], `"SECURITY"`) || !strings.Contains(got[0], "not a change kind") {
		t.Errorf("reasons = %q, want one naming SECURITY as not a change kind", got)
	}
}

func TestAChangelogKindListedTwiceIsRefused(t *testing.T) {
	c := Default()
	c.Changelog.Groups = []ChangelogGroup{
		{Kind: "FIX", Heading: "Fixes"},
		{Kind: "FEATURE", Heading: "Features"},
		{Kind: "FIX", Heading: "Bugs"},
	}
	got := reasons(t, c)
	if len(got) != 1 || !strings.Contains(got[0], `"FIX"`) || !strings.Contains(got[0], "more than once") {
		t.Errorf("reasons = %q, want one naming FIX as listed more than once", got)
	}
}

func TestEveryChangeKindIsKnown(t *testing.T) {
	for _, k := range ChangeKinds {
		if !KnownChangeKind(k) {
			t.Errorf("KnownChangeKind(%q) = false", k)
		}
		c := Default()
		c.Changelog.Groups = []ChangelogGroup{{Kind: k, Heading: "h"}}
		if err := c.Validate(); err != nil {
			t.Errorf("a group of kind %q is refused: %v", k, err)
		}
	}
	if KnownChangeKind("feature") {
		t.Error("kinds are case-sensitive; feature is not FEATURE")
	}
}

func TestAuthoringSettingsDefaults(t *testing.T) {
	d := Default()
	if d.Changelog.Producer != "claude" {
		t.Errorf("changelog.producer default = %q, want claude", d.Changelog.Producer)
	}
	if d.Changelog.ProducerCommand != nil {
		t.Errorf("changelog.producer_command default = %q, want unset", *d.Changelog.ProducerCommand)
	}
	if d.Adapters.PullRequests || d.Adapters.Diffs {
		t.Errorf("adapters default = %+v, want both off", d.Adapters)
	}
}

func TestAuthoringSettingsRefusals(t *testing.T) {
	blank := "  "
	cmd := "./author.sh"
	cases := []struct {
		name   string
		mutate func(*Config)
		want   string
	}{
		{"command producer without a command", func(c *Config) { c.Changelog.Producer = "command" }, "changelog.producer_command is unset"},
		{"command producer with a blank command", func(c *Config) {
			c.Changelog.Producer, c.Changelog.ProducerCommand = "command", &blank
		}, "changelog.producer_command is unset"},
		{"unknown producer", func(c *Config) { c.Changelog.Producer = "gpt" }, `changelog.producer names "gpt"`},
		{"pull request adapter", func(c *Config) { c.Adapters.PullRequests = true }, "adapters.pull_requests is on"},
		{"diff adapter", func(c *Config) { c.Adapters.Diffs = true }, "adapters.diffs is on"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c := Default()
			tc.mutate(&c)
			got := reasons(t, c)
			if len(got) != 1 || !strings.Contains(got[0], tc.want) {
				t.Errorf("reasons = %q, want one containing %q", got, tc.want)
			}
		})
	}

	c := Default()
	c.Changelog.Producer, c.Changelog.ProducerCommand = "command", &cmd
	if err := c.Validate(); err != nil {
		t.Errorf("command producer with a command is refused: %v", err)
	}
}
