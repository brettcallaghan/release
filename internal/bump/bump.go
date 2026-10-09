// Package bump is the Bump Classifier: the signal floor of a range, from the
// explicit records commits carry in git, together with everything that
// drove it and the commits that carry no signal. It is a pure function of
// its inputs and reads nothing outside them; in particular it never reads an
// authored changelog or any other output of a producer (ADR-003).
package bump

import (
	"regexp"
	"sort"
	"strings"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

// Signal names a kind of commit metadata the classifier can read.
type Signal string

// The commit signals. Their names are what `release version` reports under
// determined_by.
const (
	// SignalReleaseType is a Release-Type: major|minor|patch|none trailer.
	SignalReleaseType Signal = "release-type"
	// SignalConventionalType is a conventional-commit subject type.
	SignalConventionalType Signal = "conventional-type"
	// SignalBreakingMarker is a "!" before the subject's colon, or a
	// BREAKING CHANGE: / BREAKING-CHANGE: line in the body's final paragraph.
	SignalBreakingMarker Signal = "breaking-marker"
)

// Signals is the set of enabled commit signals.
type Signals map[Signal]bool

// AllSignals returns a set with every commit signal enabled.
func AllSignals() Signals {
	return Signals{SignalReleaseType: true, SignalConventionalType: true, SignalBreakingMarker: true}
}

// Input is everything the classifier reads.
type Input struct {
	// Named is the bump the maintainer named, or nil to use the floor.
	Named *releasemodel.Bump
	// Commits are the records in the range.
	Commits []releasemodel.CommitRecord
	// Enabled is the set of commit signals to read.
	Enabled Signals
}

// CommitDriver is a commit that indicated the floor, and the signal through
// which it did.
type CommitDriver struct {
	Commit releasemodel.CommitRecord
	Signal Signal
}

// CommitSignal is what one commit indicates through the enabled signals.
type CommitSignal struct {
	Commit releasemodel.CommitRecord
	// Classified is false WHEN the commit carries no enabled signal; Impact
	// and Signal are then unset.
	Classified bool
	// Impact is the highest impact the commit indicates. A Release-Type:
	// none trailer classifies a commit at none.
	Impact releasemodel.Bump
	// Signal is the signal that indicated Impact: the breaking marker over
	// the release-type trailer over the conventional type, WHERE several
	// indicate it.
	Signal Signal
}

// Result is the resolved impact, the signal floor and what determined it.
type Result struct {
	// Impact is the named bump WHEN one was named, otherwise Floor.
	Impact releasemodel.Bump
	// Named is true WHEN Impact is the maintainer's named bump.
	Named bool
	// Floor is the highest impact the commits' enabled signals indicate,
	// none WHEN no commit carries one. It is computed whether or not a bump
	// was named.
	Floor releasemodel.Bump
	// Commits are the commit records that indicated Floor, in input order,
	// each reported once.
	Commits []CommitDriver
	// Signals is every commit's signal, in input order.
	Signals []CommitSignal
	// Unclassified are the commits that carry no enabled signal, in input
	// order.
	Unclassified []releasemodel.CommitRecord
}

// Rank orders bumps from none (0) to major (3). An unrecognized value ranks
// as none.
func Rank(b releasemodel.Bump) int {
	switch b {
	case releasemodel.BumpPatch:
		return 1
	case releasemodel.BumpMinor:
		return 2
	case releasemodel.BumpMajor:
		return 3
	}
	return 0
}

// Classify computes the signal floor and resolves the release impact.
//
// The floor is the highest impact any enabled signal indicates across the
// commits, and every commit indicating it is reported once. A Release-Type:
// none trailer indicates none, so it never lowers what another signal
// indicates. With no signal anywhere the floor is none. A named bump is the
// impact as-is, and the floor is still computed beside it.
func Classify(in Input) Result {
	res := Result{Floor: releasemodel.BumpNone}
	for _, c := range in.Commits {
		cs := CommitSignal{Commit: c}
		if b, sig, ok := commitImpact(c, in.Enabled); ok {
			cs.Classified, cs.Impact, cs.Signal = true, b, sig
			if Rank(b) > Rank(res.Floor) {
				res.Floor = b
			}
		} else {
			res.Unclassified = append(res.Unclassified, c)
		}
		res.Signals = append(res.Signals, cs)
	}
	for _, cs := range res.Signals {
		if cs.Classified && cs.Impact == res.Floor {
			res.Commits = append(res.Commits, CommitDriver{Commit: cs.Commit, Signal: cs.Signal})
		}
	}
	res.Impact = res.Floor
	if in.Named != nil {
		res.Impact, res.Named = *in.Named, true
	}
	return res
}

// commitImpact returns the highest impact a commit indicates through any
// enabled signal, and the signal that indicated it. ok is false WHEN the
// commit carries no enabled signal. Where signals tie, the breaking marker
// is reported over the trailer, and the trailer over the subject type.
func commitImpact(c releasemodel.CommitRecord, enabled Signals) (b releasemodel.Bump, sig Signal, ok bool) {
	consider := func(cand releasemodel.Bump, s Signal) {
		if !ok || Rank(cand) > Rank(b) {
			b, sig, ok = cand, s, true
		}
	}
	if enabled[SignalBreakingMarker] && Breaking(c) {
		consider(releasemodel.BumpMajor, SignalBreakingMarker)
	}
	if enabled[SignalReleaseType] {
		if rt, found := releaseType(c.Trailers); found {
			consider(rt, SignalReleaseType)
		}
	}
	if enabled[SignalConventionalType] {
		if ct, found := conventionalType(c.Subject); found {
			consider(ct, SignalConventionalType)
		}
	}
	return b, sig, ok
}

// conventionalSubject is "type(scope)!: description"; the scope and the
// breaking "!" are optional.
var conventionalSubject = regexp.MustCompile(`^([A-Za-z]+)(?:\([^()]*\))?(!)?: `)

// conventionalType maps a subject's conventional-commit type: feat is
// minor; fix, perf, refactor and revert are patch; any other type is no
// signal.
func conventionalType(subject string) (releasemodel.Bump, bool) {
	m := conventionalSubject.FindStringSubmatch(subject)
	if m == nil {
		return "", false
	}
	switch strings.ToLower(m[1]) {
	case "feat":
		return releasemodel.BumpMinor, true
	case "fix", "perf", "refactor", "revert":
		return releasemodel.BumpPatch, true
	}
	return "", false
}

// releaseType reads a Release-Type trailer, key and value matched
// case-insensitively. An unrecognized value is no signal. Where the key
// appears in more than one spelling, the highest value wins.
func releaseType(trailers map[string]string) (releasemodel.Bump, bool) {
	keys := make([]string, 0, len(trailers))
	for k := range trailers {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	var best releasemodel.Bump
	found := false
	for _, k := range keys {
		if !strings.EqualFold(k, "Release-Type") {
			continue
		}
		var b releasemodel.Bump
		switch strings.ToLower(strings.TrimSpace(trailers[k])) {
		case "major":
			b = releasemodel.BumpMajor
		case "minor":
			b = releasemodel.BumpMinor
		case "patch":
			b = releasemodel.BumpPatch
		case "none":
			b = releasemodel.BumpNone
		default:
			continue
		}
		if !found || Rank(b) > Rank(best) {
			best, found = b, true
		}
	}
	return best, found
}

// Breaking reports a breaking-change marker: a "!" immediately before the
// subject's colon, or a line in the body's final paragraph beginning
// BREAKING CHANGE: or BREAKING-CHANGE:. The footer is read from the body
// because git's trailer rules reject a key containing a space.
func Breaking(c releasemodel.CommitRecord) bool {
	if m := conventionalSubject.FindStringSubmatch(c.Subject); m != nil && m[2] == "!" {
		return true
	}
	for _, line := range strings.Split(finalParagraph(c.Body), "\n") {
		if strings.HasPrefix(line, "BREAKING CHANGE:") || strings.HasPrefix(line, "BREAKING-CHANGE:") {
			return true
		}
	}
	return false
}

// footerToken is the start of a conventional-commit footer or git trailer:
// a token, a colon and a space.
var footerToken = regexp.MustCompile(`^(?:[A-Za-z][A-Za-z0-9-]*|BREAKING CHANGE): `)

// BreakingNote returns the text of a BREAKING CHANGE: or BREAKING-CHANGE:
// footer in the body's final paragraph — the footer Breaking reads — with
// any continuation lines up to the next footer. ok is false WHEN no such
// footer exists, or its text is empty.
func BreakingNote(c releasemodel.CommitRecord) (note string, ok bool) {
	lines := strings.Split(finalParagraph(c.Body), "\n")
	for i, line := range lines {
		var rest string
		switch {
		case strings.HasPrefix(line, "BREAKING CHANGE:"):
			rest = strings.TrimPrefix(line, "BREAKING CHANGE:")
		case strings.HasPrefix(line, "BREAKING-CHANGE:"):
			rest = strings.TrimPrefix(line, "BREAKING-CHANGE:")
		default:
			continue
		}
		parts := []string{strings.TrimSpace(rest)}
		for _, more := range lines[i+1:] {
			if footerToken.MatchString(more) {
				break
			}
			parts = append(parts, strings.TrimRight(more, " \t"))
		}
		note = strings.TrimSpace(strings.Join(parts, "\n"))
		return note, note != ""
	}
	return "", false
}

// finalParagraph is the body's last blank-line-separated paragraph.
func finalParagraph(body string) string {
	body = strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), " \t\n")
	if i := strings.LastIndex(body, "\n\n"); i >= 0 {
		return body[i+2:]
	}
	return body
}
