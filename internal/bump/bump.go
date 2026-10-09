// Package bump is the Bump Classifier: the release impact of a range, from a
// named bump, an authored changelog's entries, or signals in the commit
// records, together with everything that drove it. It is a pure function of
// its inputs and reads nothing outside them.
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
	// Named is the bump the maintainer named, or nil to infer one.
	Named *releasemodel.Bump
	// Changelog is the authored changelog's entries, or nil WHEN none was
	// supplied. Its proposed_bump is deliberately not an input (ADR-003).
	Changelog []releasemodel.ChangelogEntry
	// Commits are the records in the range.
	Commits []releasemodel.CommitRecord
	// Enabled is the set of commit signals to read.
	Enabled Signals
}

// CommitDriver is a commit that indicated the resolved impact, and the
// signal through which it did.
type CommitDriver struct {
	Commit releasemodel.CommitRecord
	Signal Signal
}

// Result is the resolved impact and what determined it.
type Result struct {
	Impact releasemodel.Bump
	// Named is true WHEN the impact is the maintainer's named bump.
	Named bool
	// Entries are the changelog entries that indicated Impact.
	Entries []releasemodel.ChangelogEntry
	// Commits are the commit records that indicated Impact, in input order.
	Commits []CommitDriver
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

// Classify resolves the release impact.
//
// A named bump is returned as-is. Otherwise the highest impact any enabled
// signal indicates wins, across the changelog's entries and the commit
// records, and every entry and record indicating it is reported. A
// Release-Type: none trailer indicates none, so it never lowers what another
// signal indicates. With no signal anywhere the impact is none.
func Classify(in Input) Result {
	if in.Named != nil {
		return Result{Impact: *in.Named, Named: true}
	}

	res := Result{Impact: releasemodel.BumpNone}
	raise := func(b releasemodel.Bump) {
		if Rank(b) > Rank(res.Impact) {
			res.Impact, res.Entries, res.Commits = b, nil, nil
		}
	}

	type commitSignal struct {
		bump   releasemodel.Bump
		signal Signal
	}
	perCommit := make([]*commitSignal, len(in.Commits))
	for i, c := range in.Commits {
		if b, sig, ok := commitImpact(c, in.Enabled); ok {
			perCommit[i] = &commitSignal{b, sig}
			raise(b)
		}
	}
	for _, e := range in.Changelog {
		raise(entryImpact(e))
	}

	for _, e := range in.Changelog {
		if entryImpact(e) == res.Impact {
			res.Entries = append(res.Entries, e)
		}
	}
	for i, c := range in.Commits {
		if s := perCommit[i]; s != nil && s.bump == res.Impact {
			res.Commits = append(res.Commits, CommitDriver{Commit: c, Signal: s.signal})
		}
	}
	return res
}

// entryImpact maps an authored changelog entry: breaking is major, a
// FEATURE is minor, anything else is patch.
func entryImpact(e releasemodel.ChangelogEntry) releasemodel.Bump {
	switch {
	case e.Breaking:
		return releasemodel.BumpMajor
	case e.Kind != nil && *e.Kind == releasemodel.ChangeKindFeature:
		return releasemodel.BumpMinor
	}
	return releasemodel.BumpPatch
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
// minor; fix, perf and refactor are patch; any other type is no signal.
func conventionalType(subject string) (releasemodel.Bump, bool) {
	m := conventionalSubject.FindStringSubmatch(subject)
	if m == nil {
		return "", false
	}
	switch strings.ToLower(m[1]) {
	case "feat":
		return releasemodel.BumpMinor, true
	case "fix", "perf", "refactor":
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
