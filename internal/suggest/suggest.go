// Package suggest is the Bump Suggester: the bump suggested by the structured
// changelog's entry kinds, raised to the signal floor, with its reasons, and
// the confirmed bump or a halt naming what a person must confirm (ADR-003).
// It is a pure function of its inputs: it never prompts, reads the repository
// or invokes a producer.
package suggest

import (
	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// Input is everything Suggest reads.
type Input struct {
	// Entries are the structured changelog's entries.
	Entries []releasemodel.ChangelogEntry
	// Omitted are the commits no entry cites.
	Omitted []releasemodel.OmittedCommit
	// Floor is the signal floor and FloorDrivers the commits that set it.
	Floor        releasemodel.Bump
	FloorDrivers []bump.CommitDriver
	// Unclassified are the commits that carry no enabled signal.
	Unclassified []releasemodel.CommitRecord
	// Commits is the number of commits in the range.
	Commits int
	// Named is the bump a person named, or nil.
	Named *releasemodel.Bump
	// Forced allows a named bump below the signal floor.
	Forced bool
	// Accepted confirms the suggestion when no bump is named.
	Accepted bool
}

// Outcome says which result Suggest returned.
type Outcome int

const (
	// Decided: Result.Decision is set.
	Decided Outcome = iota
	// Halted: Result.Halt is set; the caller supplies an acceptance or a
	// named bump in its next call.
	Halted
	// Refused: a named bump below the signal floor was not forced;
	// Result.Refusal is set.
	Refused
	// NotReleasable: no bump is named and the range is not releasable; the
	// caller refuses. Result.Suggestion and Result.Reasons are set.
	NotReleasable
)

// DecidedBy says who decided the bump.
type DecidedBy string

const (
	// ByNamed: a person named the bump.
	ByNamed DecidedBy = "named"
	// ByConfirmed: a person accepted the suggestion.
	ByConfirmed DecidedBy = "confirmed"
)

// Driver is an entry that drove the suggestion from the kinds.
type Driver struct {
	Kind     *releasemodel.ChangeKind
	Breaking bool
	Scope    *string
	Summary  string
	Sources  []releasemodel.ChangeSource
}

// Reasons say why the suggestion is what it is.
type Reasons struct {
	// Entries are the entries that drove the suggestion from the kinds, in
	// entry order. Empty WHEN the kinds suggest none.
	Entries []Driver
	// FromKinds is the suggestion before the floor.
	FromKinds releasemodel.Bump
	// FloorRaised is true WHEN the floor is higher than FromKinds.
	FloorRaised bool
	// Floor and FloorDrivers are the signal floor and the commits, with
	// their signals, that set it.
	Floor        releasemodel.Bump
	FloorDrivers []bump.CommitDriver
}

// WarningKind classifies a warning.
type WarningKind string

const (
	// BelowSuggestion: the named bump is below the suggestion.
	BelowSuggestion WarningKind = "below-suggestion"
	// BelowFloor: the named bump is below the signal floor and was forced.
	BelowFloor WarningKind = "below-floor"
)

// Warning accompanies a decision a person made against the evidence.
type Warning struct {
	Kind    WarningKind
	Message string
}

// Decision is a confirmed bump.
type Decision struct {
	Bump      releasemodel.Bump
	DecidedBy DecidedBy
	// Warning is set WHEN the named bump is below the suggestion or, forced,
	// below the signal floor.
	Warning *Warning
}

// Halt asks a person to confirm.
type Halt struct {
	// OnlyNamed is true WHEN only a named bump answers the halt: accepting
	// the suggestion does not.
	OnlyNamed bool
	// Unkinded are the entries no producer classified, set WHEN OnlyNamed.
	Unkinded []Driver
}

// Refusal is a named bump below the signal floor, not forced.
type Refusal struct {
	Named        releasemodel.Bump
	Floor        releasemodel.Bump
	FloorDrivers []bump.CommitDriver
}

// Result is what Suggest concluded.
type Result struct {
	Outcome Outcome
	// Suggestion, Reasons and the context a person confirms from are set for
	// every outcome.
	Suggestion   releasemodel.Bump
	Reasons      Reasons
	Floor        releasemodel.Bump
	FloorDrivers []bump.CommitDriver
	Unclassified []releasemodel.CommitRecord
	// Releasable is true WHEN the suggestion, the floor included, is patch or
	// above, or any entry has no kind; an empty range is never releasable.
	Releasable bool

	Decision *Decision
	Halt     *Halt
	Refusal  *Refusal
}

// Suggest derives the suggestion and decides.
func Suggest(in Input) Result {
	fromKinds, drivers := fromKinds(in.Entries)
	suggestion, raised := fromKinds, bump.Rank(in.Floor) > bump.Rank(fromKinds)
	if raised {
		suggestion = in.Floor
	}
	unkinded := unkindedEntries(in.Entries)
	res := Result{
		Suggestion: suggestion,
		Reasons: Reasons{
			Entries: drivers, FromKinds: fromKinds, FloorRaised: raised,
			Floor: in.Floor, FloorDrivers: in.FloorDrivers,
		},
		Floor: in.Floor, FloorDrivers: in.FloorDrivers, Unclassified: in.Unclassified,
		Releasable: in.Commits > 0 && (bump.Rank(suggestion) >= bump.Rank(releasemodel.BumpPatch) || len(unkinded) > 0),
	}

	if in.Named != nil {
		named := *in.Named
		if bump.Rank(named) < bump.Rank(in.Floor) {
			if !in.Forced {
				res.Outcome = Refused
				res.Refusal = &Refusal{Named: named, Floor: in.Floor, FloorDrivers: in.FloorDrivers}
				return res
			}
			res.Outcome = Decided
			res.Decision = &Decision{Bump: named, DecidedBy: ByNamed, Warning: &Warning{
				Kind: BelowFloor, Message: "the named bump " + string(named) + " is below the signal floor " + string(in.Floor),
			}}
			return res
		}
		res.Outcome = Decided
		res.Decision = &Decision{Bump: named, DecidedBy: ByNamed}
		if bump.Rank(named) < bump.Rank(suggestion) {
			res.Decision.Warning = &Warning{
				Kind: BelowSuggestion, Message: "the named bump " + string(named) + " is below the suggestion " + string(suggestion),
			}
		}
		return res
	}

	if in.Commits == 0 || (suggestion == releasemodel.BumpNone && len(unkinded) == 0) {
		res.Outcome = NotReleasable
		return res
	}
	if suggestion == releasemodel.BumpNone {
		res.Outcome = Halted
		res.Halt = &Halt{OnlyNamed: true, Unkinded: unkinded}
		return res
	}
	if in.Accepted {
		res.Outcome = Decided
		res.Decision = &Decision{Bump: suggestion, DecidedBy: ByConfirmed}
		return res
	}
	res.Outcome = Halted
	res.Halt = &Halt{}
	return res
}

// fromKinds is the suggestion from the entries alone and the entries that
// drove it.
func fromKinds(entries []releasemodel.ChangelogEntry) (releasemodel.Bump, []Driver) {
	pick := func(match func(releasemodel.ChangelogEntry) bool) []Driver {
		var out []Driver
		for _, e := range entries {
			if match(e) {
				out = append(out, driver(e))
			}
		}
		return out
	}
	if d := pick(func(e releasemodel.ChangelogEntry) bool { return e.Breaking }); len(d) > 0 {
		return releasemodel.BumpMajor, d
	}
	if d := pick(func(e releasemodel.ChangelogEntry) bool {
		return e.Kind != nil && *e.Kind == releasemodel.ChangeKindFeature
	}); len(d) > 0 {
		return releasemodel.BumpMinor, d
	}
	if d := pick(func(e releasemodel.ChangelogEntry) bool {
		if e.Kind == nil {
			return false
		}
		switch *e.Kind {
		case releasemodel.ChangeKindFix, releasemodel.ChangeKindPerformance,
			releasemodel.ChangeKindRefactor, releasemodel.ChangeKindRevert:
			return true
		}
		return false
	}); len(d) > 0 {
		return releasemodel.BumpPatch, d
	}
	return releasemodel.BumpNone, nil
}

func unkindedEntries(entries []releasemodel.ChangelogEntry) []Driver {
	var out []Driver
	for _, e := range entries {
		if e.Kind == nil {
			out = append(out, driver(e))
		}
	}
	return out
}

func driver(e releasemodel.ChangelogEntry) Driver {
	return Driver{Kind: e.Kind, Breaking: e.Breaking, Scope: e.Scope, Summary: e.Summary, Sources: e.Sources}
}
