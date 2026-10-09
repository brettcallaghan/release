package bump

import (
	"go/build"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

func commit(hash, subject, body string, trailers map[string]string) releasemodel.CommitRecord {
	if trailers == nil {
		trailers = map[string]string{}
	}
	return releasemodel.CommitRecord{Hash: hash, Subject: subject, Body: body, Trailers: trailers}
}

func bumpPtr(b releasemodel.Bump) *releasemodel.Bump { return &b }

func classify(commits ...releasemodel.CommitRecord) Result {
	return Classify(Input{Commits: commits, Enabled: AllSignals()})
}

func hashes(r Result) []string {
	var out []string
	for _, d := range r.Commits {
		out = append(out, d.Commit.Hash+":"+string(d.Signal))
	}
	return out
}

// AC: AC-VER-002.1
func TestEnabledSignalMapsToImpact(t *testing.T) {
	for _, tc := range []struct {
		name   string
		commit releasemodel.CommitRecord
		want   releasemodel.Bump
		signal Signal
	}{
		{"Release-Type: minor trailer", commit("a", "Change things", "", map[string]string{"Release-Type": "minor"}), releasemodel.BumpMinor, SignalReleaseType},
		{"feat type", commit("a", "feat: add a flag", "", nil), releasemodel.BumpMinor, SignalConventionalType},
		{"fix type", commit("a", "fix: handle empty input", "", nil), releasemodel.BumpPatch, SignalConventionalType},
		{"revert type", commit("a", "revert: undo the flag", "", nil), releasemodel.BumpPatch, SignalConventionalType},
		{"refactor type", commit("a", "refactor: split", "", nil), releasemodel.BumpPatch, SignalConventionalType},
		{"breaking marker", commit("a", "feat(cli)!: drop --old", "", nil), releasemodel.BumpMajor, SignalBreakingMarker},
	} {
		t.Run(tc.name, func(t *testing.T) {
			r := classify(tc.commit)
			if r.Impact != tc.want || r.Named {
				t.Fatalf("impact = %s (named %v), want %s", r.Impact, r.Named, tc.want)
			}
			if len(r.Commits) != 1 || r.Commits[0].Signal != tc.signal {
				t.Errorf("determined by %v, want commit a via %s", hashes(r), tc.signal)
			}
		})
	}
}

// AC: AC-VER-002.2
func TestHighestImpactAcrossRangeWins(t *testing.T) {
	r := classify(
		commit("p", "fix: one", "", nil),
		commit("m", "feat: two", "", nil),
	)
	if r.Impact != releasemodel.BumpMinor {
		t.Fatalf("impact = %s, want MINOR", r.Impact)
	}
	if got := hashes(r); len(got) != 1 || got[0] != "m:conventional-type" {
		t.Errorf("determined by %v, want only m", got)
	}
}

// AC: AC-VER-002.3
func TestCommitThatDroveResultIsReported(t *testing.T) {
	r := classify(
		commit("a", "fix: one", "", nil),
		commit("b", "feat: two", "", nil),
		commit("c", "chore: three", "Prose.\n\nBREAKING CHANGE: --target is now required\n", nil),
		commit("d", "docs: four", "", nil),
	)
	if r.Impact != releasemodel.BumpMajor {
		t.Fatalf("impact = %s, want MAJOR", r.Impact)
	}
	if got := hashes(r); len(got) != 1 || got[0] != "c:breaking-marker" {
		t.Errorf("determined by %v, want exactly c via breaking-marker", got)
	}
}

// AC: AC-VER-002.4
func TestNoSignalYieldsNone(t *testing.T) {
	r := classify(
		commit("a", "docs: readme", "", nil),
		commit("b", "Update dependencies", "", nil),
	)
	if r.Impact != releasemodel.BumpNone || r.Floor != releasemodel.BumpNone || len(r.Commits) != 0 {
		t.Fatalf("got %+v, want NONE with nothing reported", r)
	}
	if r := Classify(Input{Enabled: AllSignals()}); r.Impact != releasemodel.BumpNone {
		t.Errorf("empty range impact = %s, want NONE", r.Impact)
	}
}

// AC: AC-VER-002.5
func TestTheFloorIgnoresTheStructuredChangelog(t *testing.T) {
	// Input has no changelog field: the floor is computed from the commits
	// alone, so a breaking entry cannot raise it.
	r := classify(commit("a", "fix: crash", "", nil))
	if r.Floor != releasemodel.BumpPatch {
		t.Fatalf("floor = %s, want PATCH", r.Floor)
	}
}

// AC: AC-VER-002.6
func TestCommitsWithoutASignalAreUnclassified(t *testing.T) {
	r := classify(
		commit("a", "fix: one", "", nil),
		commit("b", "Update dependencies", "", nil),
		commit("c", "docs: three", "", nil),
	)
	if len(r.Unclassified) != 2 || r.Unclassified[0].Hash != "b" || r.Unclassified[1].Hash != "c" {
		t.Errorf("unclassified = %v, want b and c", r.Unclassified)
	}
	if len(r.Signals) != 3 || !r.Signals[0].Classified || r.Signals[1].Classified || r.Signals[2].Classified {
		t.Errorf("signals = %+v, want only a classified", r.Signals)
	}
	// With every signal disabled, every commit is unclassified.
	if r := Classify(Input{Commits: []releasemodel.CommitRecord{commit("a", "feat: x", "", nil)}, Enabled: Signals{}}); len(r.Unclassified) != 1 {
		t.Errorf("unclassified with no signal enabled = %v, want the one commit", r.Unclassified)
	}
}

// AC: AC-VER-002.7
func TestReleaseTypeNoneClassifiesACommit(t *testing.T) {
	r := classify(commit("a", "Skip this", "", map[string]string{"Release-Type": "none"}))
	if len(r.Unclassified) != 0 {
		t.Errorf("unclassified = %v, want none", r.Unclassified)
	}
	if len(r.Signals) != 1 || !r.Signals[0].Classified || r.Signals[0].Impact != releasemodel.BumpNone || r.Signals[0].Signal != SignalReleaseType {
		t.Errorf("signals = %+v, want a classified none via release-type", r.Signals)
	}
}

// AC: AC-VER-002.8
func TestTheFloorNeedsNoProducer(t *testing.T) {
	// The classifier is a pure function of commits and the enabled set; the
	// signature admits no producer or changelog.
	a := classify(commit("a", "feat: x", "", nil))
	b := classify(commit("a", "feat: x", "", nil))
	if a.Floor != b.Floor || a.Floor != releasemodel.BumpMinor {
		t.Errorf("floors = %s, %s; want MINOR twice", a.Floor, b.Floor)
	}
}

// AC: AC-CFG-004.7
func TestAConventionalTypeIsNoSignalWhenItsSignalIsNotEnabled(t *testing.T) {
	r := Classify(Input{
		Commits: []releasemodel.CommitRecord{commit("a", "feat: add", "", nil)},
		Enabled: Signals{SignalReleaseType: true, SignalBreakingMarker: true},
	})
	if len(r.Unclassified) != 1 || r.Floor != releasemodel.BumpNone {
		t.Errorf("got %+v, want the commit unclassified at floor NONE", r)
	}
}

func TestEachCommitIsReportedOnceWithItsPrecedingSignal(t *testing.T) {
	c := commit("a", "feat!: big", "", map[string]string{"Release-Type": "major"})
	r := classify(c)
	if got := hashes(r); len(got) != 1 || got[0] != "a:breaking-marker" {
		t.Errorf("determined by %v, want a once via breaking-marker", got)
	}
	d := commit("b", "feat: big", "", map[string]string{"Release-Type": "minor"})
	if got := hashes(classify(d)); len(got) != 1 || got[0] != "b:release-type" {
		t.Errorf("determined by %v, want b once via release-type", got)
	}
}

func TestReleaseTypeNoneNeverLowers(t *testing.T) {
	same := classify(commit("a", "feat: add", "", map[string]string{"Release-Type": "none"}))
	if same.Impact != releasemodel.BumpMinor {
		t.Errorf("feat + Release-Type: none on one commit = %s, want MINOR", same.Impact)
	}
	other := classify(
		commit("a", "fix: x", "", nil),
		commit("b", "Skip", "", map[string]string{"Release-Type": "none"}),
	)
	if other.Impact != releasemodel.BumpPatch {
		t.Errorf("Release-Type: none on another commit = %s, want PATCH", other.Impact)
	}
	alone := classify(commit("b", "Skip", "", map[string]string{"Release-Type": "NONE"}))
	if alone.Impact != releasemodel.BumpNone || len(alone.Commits) != 1 {
		t.Errorf("Release-Type: none alone = %+v, want NONE reported by b", alone)
	}
}

func TestNamedBumpContradictsEverySignalButTheFloorIsStillReported(t *testing.T) {
	r := Classify(Input{
		Named:   bumpPtr(releasemodel.BumpPatch),
		Commits: []releasemodel.CommitRecord{commit("a", "feat!: big", "", map[string]string{"Release-Type": "major"}), commit("b", "Skip", "", nil)},
		Enabled: AllSignals(),
	})
	if r.Impact != releasemodel.BumpPatch || !r.Named {
		t.Fatalf("got %+v, want named PATCH", r)
	}
	if r.Floor != releasemodel.BumpMajor {
		t.Errorf("floor = %s, want MAJOR", r.Floor)
	}
	if got := hashes(r); len(got) != 1 || got[0] != "a:breaking-marker" {
		t.Errorf("determined by %v, want a via breaking-marker", got)
	}
	if len(r.Unclassified) != 1 || r.Unclassified[0].Hash != "b" {
		t.Errorf("unclassified = %v, want b", r.Unclassified)
	}
}

func TestDisabledSignalsAreNotRead(t *testing.T) {
	c := commit("a", "feat!: x", "", map[string]string{"Release-Type": "patch"})
	for _, tc := range []struct {
		enabled Signals
		want    releasemodel.Bump
	}{
		{Signals{}, releasemodel.BumpNone},
		{Signals{SignalReleaseType: true}, releasemodel.BumpPatch},
		{Signals{SignalConventionalType: true}, releasemodel.BumpMinor},
		{Signals{SignalBreakingMarker: true}, releasemodel.BumpMajor},
	} {
		if r := Classify(Input{Commits: []releasemodel.CommitRecord{c}, Enabled: tc.enabled}); r.Impact != tc.want {
			t.Errorf("enabled %v: impact %s, want %s", tc.enabled, r.Impact, tc.want)
		}
	}
}

func TestConventionalTypes(t *testing.T) {
	for subject, want := range map[string]releasemodel.Bump{
		"feat: a":              releasemodel.BumpMinor,
		"Feat(scope): a":       releasemodel.BumpMinor,
		"fix(parser): a":       releasemodel.BumpPatch,
		"perf: a":              releasemodel.BumpPatch,
		"refactor: a":          releasemodel.BumpPatch,
		"revert: a":            releasemodel.BumpPatch,
		"chore: a":             "",
		"docs: a":              "",
		"feat:no space":        "",
		"Fix the parser (#87)": "",
	} {
		got, ok := conventionalType(subject)
		if (want == "") == ok || got != want {
			t.Errorf("conventionalType(%q) = %q, %v; want %q", subject, got, ok, want)
		}
	}
}

func TestBreakingMarker(t *testing.T) {
	for _, tc := range []struct {
		subject, body string
		want          bool
	}{
		{"feat!: x", "", true},
		{"fix(api)!: x", "", true},
		{"feat: x!", "", false},
		{"x", "BREAKING-CHANGE: y", true},
		{"x", "Prose.\n\nBREAKING CHANGE: y\nRelease-Type: major\n", true},
		{"x", "BREAKING CHANGE: only in the first paragraph\n\nMore prose.", false},
		{"x", "Prose mentioning BREAKING CHANGE: inline", false},
		{"x", "breaking change: lowercase is not the footer", false},
	} {
		if got := Breaking(commit("a", tc.subject, tc.body, nil)); got != tc.want {
			t.Errorf("Breaking(%q, %q) = %v, want %v", tc.subject, tc.body, got, tc.want)
		}
	}
}

func TestReleaseTypeTrailer(t *testing.T) {
	for _, tc := range []struct {
		trailers map[string]string
		want     releasemodel.Bump
		found    bool
	}{
		{map[string]string{"Release-Type": "Major"}, releasemodel.BumpMajor, true},
		{map[string]string{"release-type": "patch"}, releasemodel.BumpPatch, true},
		{map[string]string{"Release-Type": "huge"}, "", false},
		{map[string]string{"Other": "minor"}, "", false},
		{map[string]string{"Release-Type": "patch", "RELEASE-TYPE": "minor"}, releasemodel.BumpMinor, true},
	} {
		got, found := releaseType(tc.trailers)
		if got != tc.want || found != tc.found {
			t.Errorf("releaseType(%v) = %q, %v; want %q, %v", tc.trailers, got, found, tc.want, tc.found)
		}
	}
}

func TestRank(t *testing.T) {
	order := []releasemodel.Bump{releasemodel.BumpNone, releasemodel.BumpPatch, releasemodel.BumpMinor, releasemodel.BumpMajor}
	for i, b := range order {
		if Rank(b) != i {
			t.Errorf("Rank(%s) = %d, want %d", b, Rank(b), i)
		}
	}
	if Rank("BOGUS") != 0 {
		t.Errorf("an unknown bump must rank as none")
	}
}

// The classifier is pure: no git, no I/O, nothing from this module but the
// generated types.
func TestImportsOnlyReleasemodel(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp, "github.com/brettcallaghan/release/") && imp != "github.com/brettcallaghan/release/internal/releasemodel" {
			t.Errorf("internal/bump imports %s; it is pure and may import only releasemodel", imp)
		}
	}
}

func TestBreakingNote(t *testing.T) {
	cases := []struct {
		body string
		want string
		ok   bool
	}{
		{"", "", false},
		{"Some prose.", "", false},
		{"Prose.\n\nBREAKING CHANGE: the --foo flag is gone; use --bar", "the --foo flag is gone; use --bar", true},
		{"Prose.\n\nBREAKING-CHANGE: renamed\ncontinued here\nRefs: #12", "renamed\ncontinued here", true},
		{"BREAKING CHANGE: early\n\nNot the final paragraph.", "", false},
		{"Prose.\n\nBREAKING CHANGE: ", "", false},
	}
	for _, tc := range cases {
		got, ok := BreakingNote(commit("a", "feat: x", tc.body, nil))
		if got != tc.want || ok != tc.ok {
			t.Errorf("BreakingNote(%q) = %q, %v; want %q, %v", tc.body, got, ok, tc.want, tc.ok)
		}
	}
}
