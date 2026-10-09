// Package resolve is the release core's version resolution operation: the
// base version reachable from a target ref, the release impact of the
// commits since it, and the next version under policy. It composes the Tag
// Reader, Commit Collector, Bump Classifier and Version Calculator so that a
// front end holds no release logic. It reads the repository and never writes
// to it.
package resolve

import (
	"context"
	"fmt"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/semver"
	"github.com/brettcallaghan/release/internal/version"
)

// Policy is everything a project configures that version resolution reads.
type Policy struct {
	// TagPrefix precedes the version in a release tag name.
	TagPrefix string
	// StagePrefix precedes the version in a stage branch name.
	StagePrefix string
	// Version is the version policy: leaps and pre1_remap.
	Version version.Policy
	// Signals are the commit signals the classifier reads.
	Signals bump.Signals
}

// DefaultPolicy is the policy that applies when a project configures none:
// tag prefix "v", stage prefix "cut/", leaps off, pre1_remap on, and every
// commit signal enabled.
func DefaultPolicy() Policy {
	return Policy{
		TagPrefix:   "v",
		StagePrefix: "cut/",
		Version:     version.Policy{Leaps: false, Pre1Remap: true},
		Signals:     bump.AllSignals(),
	}
}

// Request is the input to Resolve.
type Request struct {
	// Dir is the repository's working directory.
	Dir string
	// Git is the git executable; empty means "git" from PATH.
	Git string
	// Target is the ref whose next version is resolved.
	Target string
	// Named is the bump the maintainer named, or nil to classify one.
	Named *releasemodel.Bump
	// Graduate requests 1.0.0 from a base below it.
	Graduate bool
	Policy   Policy
}

// Driver is a commit that determined the impact, and the signal through
// which it did.
type Driver struct {
	Hash    string
	Subject string
	Signal  bump.Signal
}

// Resolution is what Resolve concluded.
type Resolution struct {
	// Base is the highest release version reachable from the target, or nil
	// WHEN no release tag is reachable.
	Base *semver.Version
	// BaseTag is the tag carrying Base; empty WHEN Base is nil.
	BaseTag string
	Impact  releasemodel.Bump
	// Named is true WHEN Impact is the maintainer's named bump.
	Named bool
	// Floor is the signal floor: the highest impact the commits' explicit
	// signals indicate, whether or not a bump was named. none means nothing
	// releasable exists since the base version.
	Floor releasemodel.Bump
	// BelowFloor is true WHEN a bump was named and ranks below Floor. Next
	// is still computed from the named bump; a front end that refuses a
	// named bump below the floor reads this to do so.
	BelowFloor bool
	// DeterminedBy lists the commits that set Floor, newest first, whether or
	// not a bump was named.
	DeterminedBy []Driver
	// Unclassified lists the commits that carry no enabled signal, newest
	// first.
	Unclassified []releasemodel.CommitRecord
	// Signals is every commit's signal, newest first.
	Signals []bump.CommitSignal
	// Commits are every commit record in the range, newest first.
	Commits []releasemodel.CommitRecord
	// Next is the calculator's answer: a version, no release, or a policy
	// violation.
	Next version.Result
}

// NothingReleasable describes a range whose commits indicate no impact:
// "since <tag>: no commits", or how many commits it holds that indicate none.
func (r Resolution) NothingReleasable() string {
	since := "since " + r.BaseTag
	if r.BaseTag == "" {
		since = "with no release tag reachable"
	}
	switch n := len(r.Commits); n {
	case 0:
		return since + ": no commits"
	case 1:
		return since + ": 1 commit, which indicates no bump"
	default:
		return fmt.Sprintf("%s: %d commits, none of which indicates a bump", since, n)
	}
}

// Resolve computes the next version for the target ref. An error means the
// repository could not be read; a refused or unwarranted release is a
// Resolution, not an error.
func Resolve(ctx context.Context, req Request) (Resolution, error) {
	repo := gitread.Repo{Dir: req.Dir, Git: req.Git}

	var res Resolution
	tag, found, err := repo.LatestReleaseTag(ctx, req.Target, req.Policy.TagPrefix)
	if err != nil {
		return Resolution{}, err
	}
	if found {
		v := tag.Version
		res.Base, res.BaseTag = &v, tag.Name
	}

	commits, err := repo.Commits(ctx, res.BaseTag, req.Target)
	if err != nil {
		return Resolution{}, err
	}

	classified := bump.Classify(bump.Input{
		Named:   req.Named,
		Commits: commits,
		Enabled: req.Policy.Signals,
	})
	res.Commits = commits
	res.Impact, res.Named, res.Floor = classified.Impact, classified.Named, classified.Floor
	res.BelowFloor = classified.Named && bump.Rank(classified.Impact) < bump.Rank(classified.Floor)
	res.Unclassified, res.Signals = classified.Unclassified, classified.Signals
	for _, d := range classified.Commits {
		res.DeterminedBy = append(res.DeterminedBy, Driver{Hash: d.Commit.Hash, Subject: d.Commit.Subject, Signal: d.Signal})
	}

	if res.Next, err = Next(res.Base, res.Impact, req.Graduate, req.Policy); err != nil {
		return Resolution{}, err
	}
	return res, nil
}

// Next is the version calculator's answer for a bump from base, nil for a
// line with no release, under policy.
func Next(base *semver.Version, b releasemodel.Bump, graduate bool, policy Policy) (version.Result, error) {
	impact, err := toImpact(b)
	if err != nil {
		return version.Result{}, err
	}
	return version.Next(version.Request{
		Base:     base,
		Impact:   impact,
		Graduate: graduate,
		Policy:   policy.Version,
	}), nil
}

// toImpact maps the classifier's bump onto the calculator's own impact
// enum; the calculator does not import the generated schema types.
func toImpact(b releasemodel.Bump) (version.Impact, error) {
	switch b {
	case releasemodel.BumpNone:
		return version.ImpactNone, nil
	case releasemodel.BumpPatch:
		return version.ImpactPatch, nil
	case releasemodel.BumpMinor:
		return version.ImpactMinor, nil
	case releasemodel.BumpMajor:
		return version.ImpactMajor, nil
	}
	return 0, fmt.Errorf("unknown bump %q", b)
}
