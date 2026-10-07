// Package version is the Version Calculator: the next version for a release
// line, and every version-policy rule the tooling enforces — the leap guard,
// pre-1.0 semantics, explicit graduation to 1.0.0, and prerelease
// derivation.
//
// Every function here is pure. No rule reads the environment, the repository
// or a branch name (ADR-001): each is a function of its declared inputs. A
// policy violation is a result, never a warning.
package version

import (
	"fmt"

	"github.com/brettcallaghan/release/internal/semver"
)

// Impact is the release impact computed for a set of changes.
type Impact int

const (
	ImpactNone Impact = iota
	ImpactPatch
	ImpactMinor
	ImpactMajor
)

func (i Impact) String() string {
	switch i {
	case ImpactNone:
		return "none"
	case ImpactPatch:
		return "patch"
	case ImpactMinor:
		return "minor"
	case ImpactMajor:
		return "major"
	}
	return fmt.Sprintf("Impact(%d)", int(i))
}

// Policy is the version policy a project configures.
type Policy struct {
	// Leaps accepts a proposed version that increments a component by more
	// than one. It never changes a computed next version.
	Leaps bool
	// Pre1Remap maps an impact of major to a minor increment while the base
	// is below 1.0.0.
	Pre1Remap bool
}

// Kind is what a calculation concluded.
type Kind int

const (
	// Release: Result.Version is the version to release.
	Release Kind = iota
	// NoRelease: nothing releasable; no version is returned.
	NoRelease
	// Violation: the request breaks a policy rule; Result.Rule and
	// Result.Reason say which and why. No version is returned.
	Violation
)

func (k Kind) String() string {
	switch k {
	case Release:
		return "release"
	case NoRelease:
		return "no-release"
	case Violation:
		return "violation"
	}
	return fmt.Sprintf("Kind(%d)", int(k))
}

// Result is the outcome of Next or CheckProposed. Version is meaningful only
// when Kind is Release.
type Result struct {
	Kind    Kind
	Version semver.Version
	// Rule names the release tooling standard a violation breaks, when one
	// applies (for example "RTS-019").
	Rule   string
	Reason string
}

func release(v semver.Version) Result { return Result{Kind: Release, Version: v} }

func violation(rule, format string, args ...any) Result {
	return Result{Kind: Violation, Rule: rule, Reason: fmt.Sprintf(format, args...)}
}

// Request is the input to Next.
type Request struct {
	// Base is the latest released version on the line; nil means no release
	// yet, and the calculation starts from 0.0.0.
	Base     *semver.Version
	Impact   Impact
	Graduate bool
	Policy   Policy
}

var one = semver.Version{Major: 1}

// base resolves an optional base version, refusing one that is not a
// release version.
func base(b *semver.Version) (semver.Version, *Result) {
	if b == nil {
		return semver.Version{}, nil
	}
	if b.IsPrerelease() || len(b.Build) > 0 {
		r := violation("", "base version %s is not a release version", b)
		return semver.Version{}, &r
	}
	return *b, nil
}

// Next computes the next version for a release line. The computed version
// always increments exactly one component by one; Policy.Leaps does not
// affect it.
//
//   - Graduation below 1.0.0 yields 1.0.0 whatever the impact; graduation at
//     or above 1.0.0 is a violation.
//   - Impact none without graduation yields NoRelease.
//   - Below 1.0.0 an impact of major is a minor increment WHILE Pre1Remap is
//     on, and a violation (RTS-019: 1.0.0 needs an explicit graduation)
//     while it is off.
func Next(req Request) Result {
	b, bad := base(req.Base)
	if bad != nil {
		return *bad
	}
	pre1 := b.Major == 0

	if req.Graduate {
		if !pre1 {
			return violation("RTS-019", "graduation requested but base version %s is already at or above 1.0.0", b)
		}
		return release(one)
	}

	switch req.Impact {
	case ImpactNone:
		return Result{Kind: NoRelease, Reason: "the computed impact is none"}
	case ImpactPatch:
		return release(semver.Version{Major: b.Major, Minor: b.Minor, Patch: b.Patch + 1})
	case ImpactMinor:
		return release(semver.Version{Major: b.Major, Minor: b.Minor + 1})
	case ImpactMajor:
		if !pre1 {
			return release(semver.Version{Major: b.Major + 1})
		}
		if req.Policy.Pre1Remap {
			return release(semver.Version{Major: 0, Minor: b.Minor + 1})
		}
		return violation("RTS-019", "an impact of major from %s would produce 1.0.0, which requires an explicit graduation", b)
	}
	return violation("", "unknown impact %s", req.Impact)
}

// CheckProposed checks a proposed next version against a base (nil means
// 0.0.0) under the policy, and returns it as a Release when accepted.
//
// A proposal is refused unless it is a release version that follows the
// base, increases exactly one component with every lower component reset to
// zero, and — while Policy.Leaps is off — increases that component by
// exactly one. A proposal at or above 1.0.0 from a base below it is refused
// unless graduate is set, and graduation accepts only 1.0.0.
func CheckProposed(baseVersion *semver.Version, proposed semver.Version, graduate bool, policy Policy) Result {
	b, bad := base(baseVersion)
	if bad != nil {
		return *bad
	}
	if proposed.IsPrerelease() || len(proposed.Build) > 0 {
		return violation("", "proposed version %s is not a release version", proposed)
	}

	if graduate {
		if b.Major != 0 {
			return violation("RTS-019", "graduation requested but base version %s is already at or above 1.0.0", b)
		}
		if semver.Compare(proposed, one) != 0 {
			return violation("RTS-019", "graduation produces 1.0.0, not %s", proposed)
		}
		return release(proposed)
	}
	if b.Major == 0 && proposed.Major != 0 {
		return violation("RTS-019", "proposed version %s from %s requires an explicit graduation", proposed, b)
	}

	from := [3]uint64{b.Major, b.Minor, b.Patch}
	to := [3]uint64{proposed.Major, proposed.Minor, proposed.Patch}
	for i := range from {
		if to[i] == from[i] {
			continue
		}
		if to[i] < from[i] {
			return violation("", "proposed version %s does not follow base version %s", proposed, b)
		}
		for j := i + 1; j < len(to); j++ {
			if to[j] != 0 {
				return violation("", "proposed version %s increments one component of %s without resetting the components after it", proposed, b)
			}
		}
		if to[i]-from[i] > 1 && !policy.Leaps {
			return violation("RTS-015", "proposed version %s leaps from %s by more than one; leaps are not enabled", proposed, b)
		}
		return release(proposed)
	}
	return violation("", "proposed version %s does not follow base version %s", proposed, b)
}

// Prerelease derives the prerelease version previewing planned with ordinal
// n (<planned>-rc.<n>) through Semver Core. It orders before planned.
func Prerelease(planned semver.Version, n int) (semver.Version, error) {
	return semver.PrereleaseFor(planned, n)
}
