package corpus

import (
	"encoding/json"
	"errors"
	"reflect"
	"testing"

	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/schemacheck"
)

const (
	head = "1111111111111111111111111111111111111111"
	base = "0000000000000000000000000000000000000000"
)

func rec(hash, subject, body string, pr int64, trailers map[string]string) releasemodel.CommitRecord {
	r := releasemodel.CommitRecord{Hash: hash, Subject: subject, Body: body, Trailers: trailers}
	if r.Trailers == nil {
		r.Trailers = map[string]string{}
	}
	if pr > 0 {
		r.PullRequest = &pr
	}
	return r
}

func build(t *testing.T, in Input) releasemodel.Changelog {
	t.Helper()
	c, err := Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	validates(t, c)
	return c
}

// validates checks the corpus as it would cross the authoring seam: as JSON,
// against the generated schema.
func validates(t *testing.T, c releasemodel.Changelog) {
	t.Helper()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatalf("marshalling the corpus: %v", err)
	}
	if err := schemacheck.Changelog(data); err != nil {
		t.Errorf("the assembled corpus does not validate: %v\n%s", err, data)
	}
}

func str(s string) *string { return &s }

// AC: AC-CLOG-002.1
func TestAssembledCorpusValidatesAgainstTheSchema(t *testing.T) {
	cases := map[string]Input{
		"empty range, first release": {Range: Range{To: head}},
		"mixed records": {Range: Range{From: base, To: head}, Records: []releasemodel.CommitRecord{
			rec("a", "feat(cli)!: drop --legacy (#12)", "Removes it.\n\nBREAKING CHANGE: use --modern", 12, nil),
			rec("b", "fix: crash on empty input", "## Release notes\n\nNo more crash.\n\nDetails follow.", 0, nil),
			rec("c", "Merge branch 'x'", "", 0, nil),
		}},
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) { build(t, in) })
	}
}

// AC: AC-CLOG-002.2
func TestAuthorWrittenBlockBecomesTheEntryProse(t *testing.T) {
	body := "Some context for reviewers.\n\n" +
		"## Release notes\n\n" +
		"Resolution is **twice** as fast — naïvely cached.\n\n" +
		"It now caches per package.\n\n\n```\nrelease version\n\n# still works\n```\n\n" +
		"## Testing\n\nRan the suite."
	c := build(t, Input{Range: Range{To: head}, Records: []releasemodel.CommitRecord{
		rec("a", "perf(resolve): cache tag reads (#40)", body, 40, nil),
	}})
	e := c.Entries[0]
	if e.Summary != "Resolution is **twice** as fast — naïvely cached." {
		t.Errorf("summary = %q", e.Summary)
	}
	wantDetail := "It now caches per package.\n\n\n```\nrelease version\n\n# still works\n```"
	if e.Detail == nil || *e.Detail != wantDetail {
		t.Errorf("detail = %q, want %q", deref(e.Detail), wantDetail)
	}
	if e.Derived {
		t.Error("an author-written entry is marked derived")
	}
	if e.Kind != releasemodel.ChangeKindPerformance || deref(e.Scope) != "resolve" {
		t.Errorf("kind, scope = %s, %q; want PERFORMANCE, resolve", e.Kind, deref(e.Scope))
	}
}

func deref(s *string) string {
	if s == nil {
		return "<nil>"
	}
	return *s
}

func TestReleaseNotesBlock(t *testing.T) {
	cases := []struct {
		name            string
		body            string
		trailers        map[string]string
		summary, detail string
		ok              bool
	}{
		{"no heading", "Just prose.", nil, "", "", false},
		{"empty block", "## Release notes\n\n## Other\n\nx", nil, "", "", false},
		{"heading case", "## RELEASE NOTES\nOne line.", nil, "One line.", "", true},
		{"level three is not the heading", "### Release notes\n\nx", nil, "", "", false},
		{"multi-line summary", "## Release notes\nFirst line\nsecond line\n\nMore.", nil, "First line\nsecond line", "More.", true},
		{"trailing trailers dropped", "## Release notes\n\nShips it.\n\nCo-authored-by: A <a@x>\nRelease-Type: minor",
			map[string]string{"Co-authored-by": "A <a@x>", "Release-Type": "minor"}, "Ships it.", "", true},
		{"trailing prose kept", "## Release notes\n\nShips it.\n\nSee: the docs for more", nil, "Ships it.", "See: the docs for more", true},
		{"breaking footer dropped", "## Release notes\n\nRenames a flag.\n\nBREAKING CHANGE: use --new", nil, "Renames a flag.", "", true},
		{"folded trailer dropped", "## Release notes\n\nShips it.\n\nCo-authored-by: A\n  <a@x>",
			map[string]string{"Co-authored-by": "A <a@x>"}, "Ships it.", "", true},
		{"multi-line breaking footer dropped", "## Release notes\n\nRenames a flag.\n\nBREAKING CHANGE: use --new\ninstead of --old\nRelease-Type: major",
			map[string]string{"Release-Type": "major"}, "Renames a flag.", "", true},
		{"prose then trailer kept", "## Release notes\n\nShips it.\n\nThanks to\nRelease-Type: minor",
			map[string]string{"Release-Type": "minor"}, "Ships it.", "Thanks to\nRelease-Type: minor", true},
		{"only trailers", "## Release notes\n\nRelease-Type: patch", map[string]string{"Release-Type": "patch"}, "", "", false},
		{"crlf", "## Release notes\r\n\r\nWindows prose.\r\n", nil, "Windows prose.", "", true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			s, d, ok := ReleaseNotes(rec("a", "x", tc.body, 0, tc.trailers))
			if s != tc.summary || d != tc.detail || ok != tc.ok {
				t.Errorf("ReleaseNotes = %q, %q, %v; want %q, %q, %v", s, d, ok, tc.summary, tc.detail, tc.ok)
			}
		})
	}
}

// AC: AC-CLOG-002.3
func TestRecordWithoutABlockYieldsADerivedEntry(t *testing.T) {
	cases := []struct {
		subject string
		kind    releasemodel.ChangeKind
		scope   *string
		summary string
	}{
		{"feat: add a --target flag (#12)", releasemodel.ChangeKindFeature, nil, "add a --target flag"},
		{"fix(cli): handle empty input", releasemodel.ChangeKindFix, str("cli"), "handle empty input"},
		{"perf: faster", releasemodel.ChangeKindPerformance, nil, "faster"},
		{"revert: undo #3", releasemodel.ChangeKindRevert, nil, "undo #3"},
		{"refactor: split", releasemodel.ChangeKindRefactor, nil, "split"},
		{"docs: readme", releasemodel.ChangeKindDocumentation, nil, "readme"},
		{"build(deps): bump x", releasemodel.ChangeKindBuild, str("deps"), "bump x"},
		{"ci: cache", releasemodel.ChangeKindCi, nil, "cache"},
		{"test: more", releasemodel.ChangeKindTest, nil, "more"},
		{"style: gofmt", releasemodel.ChangeKindStyle, nil, "gofmt"},
		{"chore: tidy", releasemodel.ChangeKindChore, nil, "tidy"},
		{"wip(x): odd type", releasemodel.ChangeKindChore, str("x"), "odd type"},
		{"Feat: capitalised type", releasemodel.ChangeKindFeature, nil, "capitalised type"},
		{"Merge pull request #7 from a/b", releasemodel.ChangeKindChore, nil, "Merge pull request #7 from a/b"},
		{"Update the README (#9)", releasemodel.ChangeKindChore, nil, "Update the README"},
	}
	for _, tc := range cases {
		t.Run(tc.subject, func(t *testing.T) {
			e := Entry(rec("a", tc.subject, "A body with no block.", 0, nil))
			if e.Kind != tc.kind || e.Summary != tc.summary || deref(e.Scope) != deref(tc.scope) {
				t.Errorf("kind, scope, summary = %s, %q, %q; want %s, %q, %q", e.Kind, deref(e.Scope), e.Summary, tc.kind, deref(tc.scope), tc.summary)
			}
			if !e.Derived {
				t.Error("a derived entry is not marked derived")
			}
			if e.Detail != nil {
				t.Errorf("a derived entry carries detail %q", *e.Detail)
			}
		})
	}
}

func TestBreakingEntries(t *testing.T) {
	bang := Entry(rec("a", "feat!: drop it", "", 0, nil))
	if !bang.Breaking || bang.Migration != nil {
		t.Errorf("bang: breaking %v, migration %q; want true, unset", bang.Breaking, deref(bang.Migration))
	}
	footer := Entry(rec("a", "refactor: rename", "Prose.\n\nBREAKING CHANGE: rename Foo to Bar", 0, nil))
	if !footer.Breaking || deref(footer.Migration) != "rename Foo to Bar" {
		t.Errorf("footer: breaking %v, migration %q", footer.Breaking, deref(footer.Migration))
	}
	plain := Entry(rec("a", "fix: x", "BREAKING CHANGE: not in the final paragraph\n\nTrailing prose.", 0, nil))
	if plain.Breaking || plain.Migration != nil {
		t.Errorf("plain: breaking %v, migration %q", plain.Breaking, deref(plain.Migration))
	}
}

func TestEntrySources(t *testing.T) {
	withPR := Entry(rec("abc", "fix: x (#7)", "", 7, nil))
	if want := []releasemodel.ChangeSource{releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 7})}; !reflect.DeepEqual(withPR.Sources, want) {
		t.Errorf("sources = %+v, want the pull request", withPR.Sources)
	}
	direct := Entry(rec("abc", "fix: x", "", 0, nil))
	if want := []releasemodel.ChangeSource{releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: "abc"})}; !reflect.DeepEqual(direct.Sources, want) {
		t.Errorf("sources = %+v, want the commit", direct.Sources)
	}
}

// AC: AC-CLOG-002.4
func TestTheCommitAdapterIsRecordedWithItsRange(t *testing.T) {
	c := build(t, Input{Range: Range{From: base, To: head}, Records: []releasemodel.CommitRecord{rec("a", "fix: x", "", 0, nil)}})
	want := []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, From: str(base), To: head}}
	if !reflect.DeepEqual(c.Sources, want) {
		t.Errorf("sources = %+v, want %+v", c.Sources, want)
	}
	first := build(t, Input{Range: Range{To: head}})
	if len(first.Sources) != 1 || first.Sources[0].From != nil || first.Sources[0].To != head {
		t.Errorf("first release sources = %+v, want the commit adapter from the root to %s", first.Sources, head)
	}
}

// AC: AC-CLOG-002.5
func TestAssemblyProposesNoImpact(t *testing.T) {
	c := build(t, Input{Range: Range{To: head}, Records: []releasemodel.CommitRecord{
		rec("a", "feat!: big", "", 0, nil),
		rec("b", "fix: small", "", 0, nil),
	}})
	if c.ProposedBump != nil {
		t.Errorf("proposed_bump = %s, want unset", *c.ProposedBump)
	}
	if c.Version != nil {
		t.Errorf("version = %s, want unset", *c.Version)
	}
	data, _ := json.Marshal(c)
	var raw map[string]json.RawMessage
	if err := json.Unmarshal(data, &raw); err != nil {
		t.Fatal(err)
	}
	if _, ok := raw["proposed_bump"]; ok {
		t.Errorf("the corpus JSON carries proposed_bump: %s", data)
	}
}

// AC: AC-CLOG-001.7
func TestDiffsAreReadOnlyWhenEnabled(t *testing.T) {
	c := build(t, Input{Range: Range{To: head}, Records: []releasemodel.CommitRecord{rec("a", "fix: x", "", 0, nil)}})
	for _, run := range c.Sources {
		if run.Adapter == releasemodel.AdapterDiffs {
			t.Error("the diff adapter contributed although configuration left it off")
		}
	}
	for name, adapters := range map[string]config.Adapters{
		"diffs":         {Diffs: true},
		"pull requests": {PullRequests: true},
	} {
		if _, err := Build(Input{Range: Range{To: head}, Adapters: adapters}); !errors.Is(err, ErrAdapterUnavailable) {
			t.Errorf("%s enabled: err = %v, want ErrAdapterUnavailable", name, err)
		}
	}
}

func TestEntriesFollowRecordOrder(t *testing.T) {
	c := build(t, Input{Range: Range{To: head}, Records: []releasemodel.CommitRecord{
		rec("c", "docs: third", "", 0, nil),
		rec("b", "feat: second", "", 0, nil),
		rec("a", "fix: first", "", 0, nil),
	}})
	var got []string
	for _, e := range c.Entries {
		got = append(got, e.Summary)
	}
	if want := []string{"third", "second", "first"}; !reflect.DeepEqual(got, want) {
		t.Errorf("entry order = %q, want %q", got, want)
	}
}
