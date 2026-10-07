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
	if r.Impact != releasemodel.BumpNone || len(r.Commits) != 0 || len(r.Entries) != 0 {
		t.Fatalf("got %+v, want NONE with nothing reported", r)
	}
	if r := Classify(Input{Enabled: AllSignals()}); r.Impact != releasemodel.BumpNone {
		t.Errorf("empty range impact = %s, want NONE", r.Impact)
	}
}

// AC: AC-VER-002.5
func TestAuthoredProposalIsIgnored(t *testing.T) {
	// The changelog's proposed_bump (major) is not an input at all: only its
	// entries are, and a FIX entry indicates patch.
	major := releasemodel.BumpMajor
	_ = releasemodel.Changelog{ProposedBump: &major}
	r := Classify(Input{
		Changelog: []releasemodel.ChangelogEntry{{Kind: releasemodel.ChangeKindFix, Summary: "Fix a crash"}},
		Commits:   []releasemodel.CommitRecord{commit("a", "fix: crash", "", nil)},
		Enabled:   AllSignals(),
	})
	if r.Impact != releasemodel.BumpPatch {
		t.Fatalf("impact = %s, want PATCH", r.Impact)
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

func TestNamedBumpOverridesEverySignal(t *testing.T) {
	r := Classify(Input{
		Named:     bumpPtr(releasemodel.BumpPatch),
		Changelog: []releasemodel.ChangelogEntry{{Kind: releasemodel.ChangeKindFeature, Breaking: true}},
		Commits:   []releasemodel.CommitRecord{commit("a", "feat!: big", "", map[string]string{"Release-Type": "major"})},
		Enabled:   AllSignals(),
	})
	if r.Impact != releasemodel.BumpPatch || !r.Named || len(r.Commits) != 0 || len(r.Entries) != 0 {
		t.Fatalf("got %+v, want named PATCH with nothing inferred", r)
	}
}

func TestChangelogEntriesMap(t *testing.T) {
	feature := releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFeature, Summary: "f"}
	breaking := releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFix, Summary: "b", Breaking: true}
	chore := releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindChore, Summary: "c"}
	for _, tc := range []struct {
		entries []releasemodel.ChangelogEntry
		want    releasemodel.Bump
		drivers int
	}{
		{[]releasemodel.ChangelogEntry{chore}, releasemodel.BumpPatch, 1},
		{[]releasemodel.ChangelogEntry{chore, feature}, releasemodel.BumpMinor, 1},
		{[]releasemodel.ChangelogEntry{feature, breaking, chore}, releasemodel.BumpMajor, 1},
	} {
		r := Classify(Input{Changelog: tc.entries, Enabled: AllSignals()})
		if r.Impact != tc.want || len(r.Entries) != tc.drivers {
			t.Errorf("entries %v: got %s with %d drivers, want %s with %d", tc.entries, r.Impact, len(r.Entries), tc.want, tc.drivers)
		}
	}
	// Entries and commits indicating the same impact are both reported.
	r := Classify(Input{
		Changelog: []releasemodel.ChangelogEntry{feature},
		Commits:   []releasemodel.CommitRecord{commit("a", "feat: x", "", nil)},
		Enabled:   AllSignals(),
	})
	if len(r.Entries) != 1 || len(r.Commits) != 1 {
		t.Errorf("got %d entries and %d commits, want 1 and 1", len(r.Entries), len(r.Commits))
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
