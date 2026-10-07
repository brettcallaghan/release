// Package config is the release core's configuration model: every
// repository-specific value the tooling reads, its documented default, and
// the validation that refuses a configuration that cannot work.
//
// It extends the version resolution default policy (resolve.DefaultPolicy)
// rather than defining a second one, and it finds and reads no file: a front
// end decodes one into Config and calls Validate. No setting disables a check
// the tooling performs, and nothing is derived from the name in Target
// (ADR-001).
package config

import (
	"context"
	"errors"
	"fmt"
	"path"
	"strings"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/resolve"
	"github.com/brettcallaghan/release/internal/semver"
)

// Commands are the repository's delegated commands. Each is a string run
// through `sh -c` by the Delegated Command Runner.
type Commands struct {
	// Validate is the validation command; nil means unset.
	Validate *string `toml:"validate"`
	// Propagate writes a version into the repository's manifests; nil means
	// unset.
	Propagate *string `toml:"propagate"`
	// Preflight runs in order before a cut.
	Preflight []string `toml:"preflight"`
}

// Config is the tooling's configuration. The toml tags are the setting
// names a configuration file declares.
type Config struct {
	// Target is the release line's branch. Empty means the repository's
	// default branch: the one origin/HEAD points to.
	Target          string   `toml:"target"`
	TagPrefix       string   `toml:"tag_prefix"`
	StagePrefix     string   `toml:"stage_prefix"`
	ReleaseBranches []string `toml:"release_branches"`
	ReleasePaths    []string `toml:"release_paths"`
	Leaps           bool     `toml:"leaps"`
	Pre1Remap       bool     `toml:"pre1_remap"`
	// Signals are the commit signal names the classifier reads.
	Signals  []string `toml:"signals"`
	Commands Commands `toml:"commands"`
}

// signalOrder is every commit signal, in the order the default lists them.
var signalOrder = []bump.Signal{bump.SignalReleaseType, bump.SignalConventionalType, bump.SignalBreakingMarker}

// Default is the configuration that applies to every setting a repository
// leaves undeclared. Its version settings are resolve.DefaultPolicy's.
func Default() Config {
	p := resolve.DefaultPolicy()
	signals := make([]string, 0, len(signalOrder))
	for _, s := range signalOrder {
		if p.Signals[s] {
			signals = append(signals, string(s))
		}
	}
	return Config{
		TagPrefix:       p.TagPrefix,
		StagePrefix:     p.StagePrefix,
		ReleaseBranches: []string{},
		ReleasePaths:    []string{},
		Leaps:           p.Version.Leaps,
		Pre1Remap:       p.Version.Pre1Remap,
		Signals:         signals,
		Commands:        Commands{Preflight: []string{}},
	}
}

// Refusal is a configuration the tooling cannot accept, with every reason
// found.
type Refusal struct {
	Reasons []string
}

func (r *Refusal) Error() string {
	return "configuration refused:\n  - " + strings.Join(r.Reasons, "\n  - ")
}

// collisionProbes are stage branch versions a refusal names as an example
// when the glob matches one: a release and a prerelease.
var collisionProbes = []semver.Version{
	{Major: 1, Minor: 2, Patch: 3},
	{Major: 1, Minor: 2, Patch: 3, Prerelease: []string{"rc", "1"}},
}

// collisionExample names a probe stage branch the glob matches, or nothing
// when the overlap lies elsewhere in the version space.
func collisionExample(glob, stagePrefix string) string {
	for _, v := range collisionProbes {
		stage := semver.StageBranchName(stagePrefix, v)
		if ok, _ := path.Match(glob, stage); ok {
			return fmt.Sprintf(", such as %q", stage)
		}
	}
	return ""
}

// Validate returns nil for a configuration the tooling can work with, and
// otherwise a *Refusal carrying every reason, not only the first.
func (c Config) Validate() error {
	var reasons []string
	if c.StagePrefix == "" {
		reasons = append(reasons, "stage_prefix is empty; a stage branch must be distinguishable by its prefix")
	}
	known := bump.AllSignals()
	for _, s := range c.Signals {
		if !known[bump.Signal(s)] {
			reasons = append(reasons, fmt.Sprintf("signals names %q, which is not a commit signal (want one of %s)", s, signalList()))
		}
	}
	for _, glob := range c.ReleaseBranches {
		if _, err := path.Match(glob, ""); err != nil {
			reasons = append(reasons, fmt.Sprintf("release_branches glob %q is malformed: %v", glob, err))
			continue
		}
		if c.StagePrefix != "" && matchesStageName(glob, c.StagePrefix) {
			reasons = append(reasons, fmt.Sprintf("release_branches glob %q can match a stage branch under stage_prefix %q%s (RTS-009)", glob, c.StagePrefix, collisionExample(glob, c.StagePrefix)))
		}
	}
	if len(reasons) > 0 {
		return &Refusal{Reasons: reasons}
	}
	return nil
}

func signalList() string {
	names := make([]string, len(signalOrder))
	for i, s := range signalOrder {
		names[i] = string(s)
	}
	return strings.Join(names, ", ")
}

// Policy is the version resolution policy the configuration declares.
func (c Config) Policy() resolve.Policy {
	p := resolve.DefaultPolicy()
	p.TagPrefix = c.TagPrefix
	p.StagePrefix = c.StagePrefix
	p.Version.Leaps = c.Leaps
	p.Version.Pre1Remap = c.Pre1Remap
	p.Signals = bump.Signals{}
	for _, s := range c.Signals {
		p.Signals[bump.Signal(s)] = true
	}
	return p
}

// ErrNoDefaultBranch is the refusal when a command needs the repository's
// default branch and origin/HEAD is unset.
var ErrNoDefaultBranch = errors.New("origin/HEAD is unset, so the repository has no default branch; " +
	"set it with: git remote set-head origin --auto")

// remotePrefix is where origin/HEAD's branch lives, read locally.
const remotePrefix = "refs/remotes/origin/"

// TargetRef is the release line a command acts on, as a ref: the configured
// target when one is declared, otherwise the branch origin/HEAD points to,
// read locally (refs/remotes/origin/<branch>). It returns ErrNoDefaultBranch
// WHEN no target is declared and origin/HEAD is unset.
func (c Config) TargetRef(ctx context.Context, repo gitread.Repo) (string, error) {
	if c.Target != "" {
		return c.Target, nil
	}
	ref, found, err := repo.DefaultBranch(ctx)
	if err != nil {
		return "", err
	}
	if !found {
		return "", ErrNoDefaultBranch
	}
	return ref, nil
}

// TargetBranch is TargetRef as a branch name: the default branch is named
// without its refs/remotes/origin/ prefix.
func (c Config) TargetBranch(ctx context.Context, repo gitread.Repo) (string, error) {
	ref, err := c.TargetRef(ctx, repo)
	if err != nil {
		return "", err
	}
	if c.Target == "" {
		return strings.TrimPrefix(ref, remotePrefix), nil
	}
	return ref, nil
}
