package suggest

import (
	"go/parser"
	"go/token"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

const (
	none  = releasemodel.BumpNone
	patch = releasemodel.BumpPatch
	minor = releasemodel.BumpMinor
	major = releasemodel.BumpMajor
)

func entry(kind string, breaking bool, summary string) releasemodel.ChangelogEntry {
	e := releasemodel.ChangelogEntry{Summary: summary, Breaking: breaking}
	if kind != "" {
		k := releasemodel.ChangeKind(kind)
		e.Kind = &k
	}
	return e
}

func bp(b releasemodel.Bump) *releasemodel.Bump { return &b }

var driverCommit = bump.CommitDriver{
	Commit: releasemodel.CommitRecord{Hash: "abc", Subject: "feat: x"},
	Signal: bump.SignalConventionalType,
}

// AC: AC-VER-005.11
func TestSuggestionFromKinds(t *testing.T) {
	cases := []struct {
		name    string
		entries []releasemodel.ChangelogEntry
		want    releasemodel.Bump
	}{
		{"breaking and feature", []releasemodel.ChangelogEntry{entry("FEATURE", false, "f"), entry("FIX", true, "b")}, major},
		{"breaking alone, no kind needed", []releasemodel.ChangelogEntry{entry("", true, "b")}, major},
		{"feature and fix", []releasemodel.ChangelogEntry{entry("FIX", false, "x"), entry("FEATURE", false, "f")}, minor},
		{"feature", []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, minor},
		{"fix", []releasemodel.ChangelogEntry{entry("FIX", false, "x")}, patch},
		{"performance", []releasemodel.ChangelogEntry{entry("PERFORMANCE", false, "x")}, patch},
		{"refactor", []releasemodel.ChangelogEntry{entry("REFACTOR", false, "x")}, patch},
		{"revert", []releasemodel.ChangelogEntry{entry("REVERT", false, "x")}, patch},
		{"documentation and chore", []releasemodel.ChangelogEntry{entry("DOCUMENTATION", false, "d"), entry("CHORE", false, "c")}, none},
		{"build ci test style", []releasemodel.ChangelogEntry{entry("BUILD", false, ""), entry("CI", false, ""), entry("TEST", false, ""), entry("STYLE", false, "")}, none},
		{"no entries", nil, none},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Suggest(Input{Entries: c.entries, Floor: none, Commits: 3, Named: bp(patch), Forced: true})
			if got.Suggestion != c.want {
				t.Fatalf("suggestion = %s, want %s", got.Suggestion, c.want)
			}
			if got.Reasons.FloorRaised {
				t.Fatalf("floor none must not raise")
			}
		})
	}
}

// AC: AC-VER-005.12
func TestReasonsNameDrivingEntries(t *testing.T) {
	got := Suggest(Input{
		Entries: []releasemodel.ChangelogEntry{entry("FIX", false, "fix one"), entry("FEATURE", false, "the feature"), entry("FIX", false, "fix two")},
		Floor:   patch, Commits: 3, Accepted: true,
	})
	if got.Suggestion != minor {
		t.Fatalf("suggestion = %s", got.Suggestion)
	}
	r := got.Reasons
	if len(r.Entries) != 1 || r.Entries[0].Summary != "the feature" {
		t.Fatalf("drivers = %+v", r.Entries)
	}
	if r.FloorRaised {
		t.Fatalf("floor patch must not raise minor")
	}
}

// AC: AC-VER-005.2
func TestFloorRaisesSuggestion(t *testing.T) {
	got := Suggest(Input{
		Entries: []releasemodel.ChangelogEntry{entry("FIX", false, "x")},
		Floor:   minor, FloorDrivers: []bump.CommitDriver{driverCommit}, Commits: 2, Accepted: true,
	})
	if got.Suggestion != minor || !got.Reasons.FloorRaised || got.Reasons.FromKinds != patch {
		t.Fatalf("got %+v", got)
	}
	if len(got.Reasons.FloorDrivers) != 1 || got.Reasons.FloorDrivers[0].Commit.Hash != "abc" {
		t.Fatalf("floor drivers = %+v", got.Reasons.FloorDrivers)
	}
	if got.Decision == nil || got.Decision.Bump != minor || got.Decision.DecidedBy != ByConfirmed {
		t.Fatalf("decision = %+v", got.Decision)
	}
}

// A floor below the kinds' suggestion leaves the suggestion alone.
func TestFloorBelowKinds(t *testing.T) {
	got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Floor: patch, Commits: 1})
	if got.Suggestion != minor || got.Reasons.FloorRaised {
		t.Fatalf("got %+v", got)
	}
}

// AC: AC-VER-005.19
func TestWithoutModelSuggestionIsFloor(t *testing.T) {
	got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("", false, "c1")}, Floor: patch, Commits: 1})
	if got.Suggestion != patch || !got.Reasons.FloorRaised {
		t.Fatalf("got %+v", got)
	}
}

// AC: AC-VER-005.6
func TestNamedAtOrAboveSuggestion(t *testing.T) {
	for _, named := range []releasemodel.Bump{minor, major} {
		got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Floor: patch, Commits: 1, Named: bp(named)})
		if got.Outcome != Decided || got.Decision.Bump != named || got.Decision.DecidedBy != ByNamed || got.Decision.Warning != nil {
			t.Fatalf("%s: %+v", named, got)
		}
	}
}

// AC: AC-VER-005.16
func TestNamedBelowSuggestionWarns(t *testing.T) {
	got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Floor: patch, Commits: 1, Named: bp(patch)})
	if got.Outcome != Decided || got.Decision.Bump != patch || got.Decision.Warning == nil || got.Decision.Warning.Kind != BelowSuggestion {
		t.Fatalf("got %+v", got)
	}
	if len(got.Reasons.Entries) != 1 {
		t.Fatalf("reasons missing: %+v", got.Reasons)
	}
}

// AC: AC-VER-005.14
func TestNamedBelowFloorRefused(t *testing.T) {
	got := Suggest(Input{Floor: minor, FloorDrivers: []bump.CommitDriver{driverCommit}, Commits: 1, Named: bp(patch)})
	if got.Outcome != Refused || got.Decision != nil {
		t.Fatalf("got %+v", got)
	}
	if got.Refusal.Floor != minor || got.Refusal.Named != patch || len(got.Refusal.FloorDrivers) != 1 {
		t.Fatalf("refusal = %+v", got.Refusal)
	}
}

// AC: AC-VER-005.15
func TestForcedNamedBelowFloor(t *testing.T) {
	got := Suggest(Input{Floor: minor, Commits: 1, Named: bp(patch), Forced: true})
	if got.Outcome != Decided || got.Decision.Bump != patch || got.Decision.Warning == nil || got.Decision.Warning.Kind != BelowFloor {
		t.Fatalf("got %+v", got)
	}
}

// A named bump at the floor is not forced and not below it.
func TestNamedAtFloorNeedsNoForce(t *testing.T) {
	got := Suggest(Input{Floor: minor, Commits: 1, Named: bp(minor)})
	if got.Outcome != Decided || got.Decision.Warning != nil {
		t.Fatalf("got %+v", got)
	}
}

// A named none is any named bump: refused below a floor, decided otherwise.
func TestNamedNone(t *testing.T) {
	if got := Suggest(Input{Floor: patch, Commits: 1, Named: bp(none)}); got.Outcome != Refused {
		t.Fatalf("below floor: %+v", got)
	}
	if got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("", false, "c")}, Floor: none, Commits: 1, Named: bp(none)}); got.Outcome != Decided || got.Decision.Bump != none {
		t.Fatalf("over floor none: %+v", got)
	}
}

// AC: AC-VER-005.17
func TestAcceptanceConfirmsSuggestion(t *testing.T) {
	got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Commits: 1, Accepted: true})
	if got.Outcome != Decided || got.Decision.Bump != minor || got.Decision.DecidedBy != ByConfirmed || got.Halt != nil {
		t.Fatalf("got %+v", got)
	}
}

// AC: AC-VER-005.18
func TestUnconfirmedSuggestionHalts(t *testing.T) {
	got := Suggest(Input{
		Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Floor: patch,
		FloorDrivers: []bump.CommitDriver{driverCommit},
		Unclassified: []releasemodel.CommitRecord{{Hash: "u1"}}, Commits: 2,
	})
	if got.Outcome != Halted || got.Decision != nil || got.Halt == nil || got.Halt.OnlyNamed {
		t.Fatalf("got %+v", got)
	}
	if got.Suggestion != minor || len(got.Reasons.Entries) != 1 || got.Floor != patch ||
		len(got.FloorDrivers) != 1 || len(got.Unclassified) != 1 {
		t.Fatalf("halt lacks what a person confirms from: %+v", got)
	}
	// The next call supplying the confirmation proceeds.
	in := Input{Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Floor: patch, Commits: 2, Accepted: true}
	if next := Suggest(in); next.Outcome != Decided {
		t.Fatalf("next = %+v", next)
	}
}

// AC: AC-VER-005.20
func TestNoneWithUnkindedEntryHaltsForNamedBump(t *testing.T) {
	entries := []releasemodel.ChangelogEntry{entry("DOCUMENTATION", false, "d"), entry("", false, "mystery")}
	for _, accepted := range []bool{false, true} {
		got := Suggest(Input{Entries: entries, Commits: 2, Accepted: accepted})
		if got.Outcome != Halted || !got.Halt.OnlyNamed || len(got.Halt.Unkinded) != 1 || got.Halt.Unkinded[0].Summary != "mystery" {
			t.Fatalf("accepted=%v: %+v", accepted, got)
		}
		if got.Decision != nil || !got.Releasable {
			t.Fatalf("accepted=%v: %+v", accepted, got)
		}
	}
	got := Suggest(Input{Entries: entries, Commits: 2, Named: bp(patch)})
	if got.Outcome != Decided || got.Decision.Bump != patch || got.Decision.Warning != nil {
		t.Fatalf("named: %+v", got)
	}
}

// AC: AC-VER-005.9
func TestNoneEveryEntryKindedNotReleasable(t *testing.T) {
	entries := []releasemodel.ChangelogEntry{entry("DOCUMENTATION", false, "d"), entry("CHORE", false, "c")}
	for _, accepted := range []bool{false, true} {
		got := Suggest(Input{Entries: entries, Commits: 2, Accepted: accepted})
		if got.Outcome != NotReleasable || got.Halt != nil || got.Decision != nil || got.Releasable {
			t.Fatalf("accepted=%v: %+v", accepted, got)
		}
	}
}

// AC: AC-VER-005.8
func TestChangelogCanMakeRangeReleasable(t *testing.T) {
	cases := []struct {
		name  string
		entry releasemodel.ChangelogEntry
	}{
		{"feature entry", entry("FEATURE", false, "f")},
		{"entry with no kind", entry("", false, "c")},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{c.entry}, Floor: none, Commits: 1})
			if !got.Releasable || got.Outcome == NotReleasable {
				t.Fatalf("got %+v", got)
			}
		})
	}
	if got := Suggest(Input{Floor: patch, Commits: 1}); !got.Releasable {
		t.Fatalf("floor patch alone must be releasable: %+v", got)
	}
}

// AC: AC-VER-005.10
func TestEmptyRangeNotReleasable(t *testing.T) {
	got := Suggest(Input{Entries: []releasemodel.ChangelogEntry{entry("FEATURE", false, "f")}, Commits: 0, Accepted: true})
	if got.Outcome != NotReleasable || got.Releasable || got.Decision != nil {
		t.Fatalf("got %+v", got)
	}
}

// The component reads nothing outside its inputs: it imports neither git
// access, prompting nor process start.
func TestImportsNothingImpure(t *testing.T) {
	fset := token.NewFileSet()
	files, err := parser.ParseFile(fset, "suggest.go", nil, parser.ImportsOnly)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range files.Imports {
		p := strings.Trim(imp.Path.Value, `"`)
		switch p {
		case "github.com/brettcallaghan/release/internal/bump", "github.com/brettcallaghan/release/internal/releasemodel":
		default:
			t.Errorf("unexpected import %s", p)
		}
	}
}
