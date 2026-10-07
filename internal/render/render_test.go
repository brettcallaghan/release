package render

import (
	"errors"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

var update = flag.Bool("update", false, "rewrite the golden files from the current implementation")

const repoURL = "https://github.com/o/r"

func str(s string) *string { return &s }

func pr(n int64) releasemodel.ChangeSource {
	return releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: n})
}

func commit(h string) releasemodel.ChangeSource {
	return releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: h})
}

func issue(id string) releasemodel.ChangeSource {
	return releasemodel.NewChangeSourceIssue(releasemodel.IssueSource{Id: id})
}

func corpus(version string, entries ...releasemodel.ChangelogEntry) releasemodel.Changelog {
	return releasemodel.Changelog{
		SchemaVersion: "1.0.0",
		ProposedBump:  releasemodel.BumpMinor,
		Version:       str(version),
		Entries:       entries,
	}
}

func defaults() config.Changelog { return config.Default().Changelog }

// The blueprint's approved example, byte for byte.
const example190 = "## 1.9.0\n" +
	"\n" +
	"**Faster resolution for large monorepos**\n" +
	"\n" +
	"Resolution now caches per package. Upgrade when convenient.\n" +
	"\n" +
	"### Breaking changes\n" +
	"\n" +
	"- **cli:** `--target` is required ([#412](https://github.com/o/r/pull/412))\n" +
	"\n" +
	"  Migration: pass `--target main` where you relied on the default.\n" +
	"\n" +
	"### Fixes\n" +
	"\n" +
	"- Tags with a `v` prefix are recognised ([#87](https://github.com/o/r/pull/87))\n"

func corpus190() releasemodel.Changelog {
	c := corpus("1.9.0",
		releasemodel.ChangelogEntry{
			Kind: releasemodel.ChangeKindFeature, Scope: str("cli"), Summary: "`--target` is required",
			Breaking: true, Migration: str("pass `--target main` where you relied on the default."),
			Sources: []releasemodel.ChangeSource{pr(412)},
		},
		releasemodel.ChangelogEntry{
			Kind: releasemodel.ChangeKindFix, Summary: "Tags with a `v` prefix are recognised",
			Sources: []releasemodel.ChangeSource{pr(87)},
		},
	)
	c.ProposedBump = releasemodel.BumpMajor
	c.Title = str("Faster resolution for large monorepos")
	c.Summary = str("Resolution now caches per package. Upgrade when convenient.")
	return c
}

func emojiHeadings() config.Changelog {
	return config.Changelog{
		Groups: []config.ChangelogGroup{
			{Kind: releasemodel.ChangeKindFeature, Heading: "🚀 Features"},
			{Kind: releasemodel.ChangeKindFix, Heading: "🪲 Bugs"},
			{Kind: releasemodel.ChangeKindPerformance, Heading: "Performance"},
			{Kind: releasemodel.ChangeKindRevert, Heading: "Reverts"},
		},
		OtherHeading:    "Other changes",
		BreakingHeading: "💥 Breaking changes",
	}
}

// mixed exercises every rule at once: order within groups, every source
// kind, scope, detail, an unrecognised kind, and a known kind no group
// renders.
func mixed() releasemodel.Changelog {
	return corpus("2.0.0",
		releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFix, Summary: "second fix in the corpus, first rendered", Sources: []releasemodel.ChangeSource{commit("0123456789abcdef0123456789abcdef01234567")}},
		releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindChore, Summary: "a chore no group renders", Sources: []releasemodel.ChangeSource{pr(1)}},
		releasemodel.ChangelogEntry{Kind: "SECURITY", Summary: "a kind the renderer does not know", Sources: []releasemodel.ChangeSource{issue("release-abc")}},
		releasemodel.ChangelogEntry{
			Kind: releasemodel.ChangeKindFeature, Scope: str("config"), Summary: "headings are configurable",
			Detail:  str("Each kind names its heading.\n\nGroups render in the configured order."),
			Sources: []releasemodel.ChangeSource{pr(10), pr(11), issue("42")},
		},
		releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFix, Summary: "a later fix", Sources: []releasemodel.ChangeSource{}},
		releasemodel.ChangelogEntry{
			Kind: releasemodel.ChangeKindRefactor, Summary: "an omitted kind still renders when breaking",
			Breaking: true, Migration: str("rebuild your plugins"), Sources: []releasemodel.ChangeSource{commit("fedcba9")},
		},
		releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFeature, Summary: "a second feature", Sources: []releasemodel.ChangeSource{pr(12)}},
	)
}

type goldenCase struct {
	corpus   releasemodel.Changelog
	settings config.Changelog
	url      string
}

var goldenCases = map[string]goldenCase{
	"example-1.9.0":     {corpus190(), defaults(), repoURL},
	"mixed":             {mixed(), defaults(), repoURL},
	"mixed-no-url":      {mixed(), defaults(), ""},
	"emoji-headings":    {mixed(), emojiHeadings(), repoURL},
	"unrecognised-kind": {corpus("1.0.1", releasemodel.ChangelogEntry{Kind: "SECURITY", Summary: "closes a hole", Sources: []releasemodel.ChangeSource{pr(5)}}), defaults(), repoURL},
	"no-entries":        {corpus("1.0.0"), defaults(), repoURL},
	"title-only":        {func() releasemodel.Changelog { c := corpus("0.1.0"); c.Title = str("First release"); return c }(), defaults(), repoURL},
}

func goldenNames() []string {
	names := make([]string, 0, len(goldenCases))
	for n := range goldenCases {
		names = append(names, n)
	}
	sort.Strings(names)
	return names
}

func checkGolden(t *testing.T, path, got string) {
	t.Helper()
	if *update {
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(got), 0o644); err != nil {
			t.Fatal(err)
		}
		return
	}
	want, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("%v (regenerate with scripts/regenerate-snapshots.sh)", err)
	}
	if got != string(want) {
		t.Errorf("%s differs:\n--- got ---\n%s\n--- want ---\n%s", path, got, want)
	}
}

// Every target of every case against its golden file.
func TestGoldenFiles(t *testing.T) {
	for _, name := range goldenNames() {
		t.Run(name, func(t *testing.T) {
			c := goldenCases[name]
			r, err := Render(c.corpus, c.settings, c.url)
			if err != nil {
				t.Fatal(err)
			}
			checkGolden(t, filepath.Join("testdata", name, "section.md"), r.Section)
			checkGolden(t, filepath.Join("testdata", name, "body.md"), r.Body)
		})
	}
}

func TestTheBlueprintExampleRendersByteForByte(t *testing.T) {
	r, err := Render(corpus190(), defaults(), repoURL+"/")
	if err != nil {
		t.Fatal(err)
	}
	if r.Section != example190 {
		t.Errorf("section:\n%s\nwant:\n%s", r.Section, example190)
	}
}

// AC: AC-REND-001.1
func TestIdenticalInputRendersIdenticalOutput(t *testing.T) {
	for _, name := range goldenNames() {
		c := goldenCases[name]
		a, errA := Render(c.corpus, c.settings, c.url)
		b, errB := Render(c.corpus, c.settings, c.url)
		if errA != nil || errB != nil {
			t.Fatalf("%s: %v, %v", name, errA, errB)
		}
		if a != b {
			t.Errorf("%s: two renderings differ:\n%q\n%q", name, a, b)
		}
		for _, out := range []string{a.Section, a.Body} {
			if strings.Contains(out, "\r") {
				t.Errorf("%s: output carries a carriage return", name)
			}
			if out != "" && (!strings.HasSuffix(out, "\n") || strings.HasSuffix(out, "\n\n")) {
				t.Errorf("%s: output does not end with exactly one newline: %q", name, out)
			}
		}
	}
}

// AC: AC-REND-001.2
func TestEveryTargetIsRenderedFromOneCorpusInstance(t *testing.T) {
	for _, name := range goldenNames() {
		c := goldenCases[name]
		r, err := Render(c.corpus, c.settings, c.url)
		if err != nil {
			t.Fatal(err)
		}
		heading := "## " + *c.corpus.Version + "\n"
		want := strings.TrimPrefix(r.Section, heading+"\n")
		if r.Section == heading {
			want = ""
		}
		if !strings.HasPrefix(r.Section, heading) || r.Body != want {
			t.Errorf("%s: body is not the section without its heading:\nsection %q\nbody    %q", name, r.Section, r.Body)
		}
	}
}

// AC: AC-REND-001.3
func TestEntryOrderWithinAGroupIsPreserved(t *testing.T) {
	r, err := Render(mixed(), defaults(), repoURL)
	if err != nil {
		t.Fatal(err)
	}
	inOrder(t, r.Section, "second fix in the corpus, first rendered", "a later fix")
	inOrder(t, r.Section, "headings are configurable", "a second feature")
	inOrder(t, r.Section, "### Breaking changes", "### Features", "### Fixes", "### Other changes")
}

func inOrder(t *testing.T, s string, parts ...string) {
	t.Helper()
	at := -1
	for _, p := range parts {
		i := strings.Index(s, p)
		if i < 0 || i <= at {
			t.Errorf("%q is missing or out of order in:\n%s", p, s)
			return
		}
		at = i
	}
}

// AC: AC-REND-001.4
func TestAnUnrecognisedKindRendersInTheOtherGroup(t *testing.T) {
	c := corpus("1.0.1", releasemodel.ChangelogEntry{Kind: "SECURITY", Summary: "closes a hole", Sources: []releasemodel.ChangeSource{pr(5)}})
	r, err := Render(c, defaults(), "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "## 1.0.1\n\n### Other changes\n\n- closes a hole (#5)\n"; r.Section != want {
		t.Errorf("section = %q, want %q", r.Section, want)
	}
}

func TestAKnownKindNoGroupNamesDoesNotRender(t *testing.T) {
	r, err := Render(mixed(), defaults(), repoURL)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(r.Section, "a chore no group renders") {
		t.Errorf("a CHORE entry rendered though no group names CHORE:\n%s", r.Section)
	}
}

// AC: AC-REND-001.5
func TestBreakingEntriesRenderAsTheirOwnSectionWithMigration(t *testing.T) {
	r, err := Render(corpus190(), defaults(), repoURL)
	if err != nil {
		t.Fatal(err)
	}
	want := "### Breaking changes\n\n- **cli:** `--target` is required ([#412](https://github.com/o/r/pull/412))\n\n  Migration: pass `--target main` where you relied on the default.\n"
	if !strings.Contains(r.Section, want) {
		t.Errorf("no breaking section carrying the entry and its migration:\n%s", r.Section)
	}
	if strings.Contains(r.Section, "### Features") || strings.Count(r.Section, "is required") != 1 {
		t.Errorf("the breaking entry rendered again in its kind's group:\n%s", r.Section)
	}
}

// AC: AC-REND-001.6
func TestACorpusWithNoVersionDoesNotRender(t *testing.T) {
	for _, v := range []*string{nil, str(""), str("  ")} {
		c := corpus190()
		c.Version = v
		if r, err := Render(c, defaults(), repoURL); !errors.Is(err, ErrNoVersion) || r != (Rendering{}) {
			t.Errorf("version %v: Render = %q, %v; want ErrNoVersion and nothing rendered", v, r, err)
		}
	}
}

func TestAnUnimplementedSchemaMajorIsRefused(t *testing.T) {
	for _, v := range []string{"2.0.0", "0.9.0", "one", ""} {
		c := corpus190()
		c.SchemaVersion = v
		if _, err := Render(c, defaults(), repoURL); err == nil || !strings.Contains(err.Error(), "schema_version") {
			t.Errorf("schema_version %q: err = %v, want a refusal naming schema_version", v, err)
		}
	}
	c := corpus190()
	c.SchemaVersion = "1.4.2"
	if _, err := Render(c, defaults(), repoURL); err != nil {
		t.Errorf("schema_version 1.4.2 refused: %v", err)
	}
}

// AC: AC-REND-001.7
func TestSectionHeadingsFollowTheConfiguration(t *testing.T) {
	r, err := Render(corpus190(), emojiHeadings(), repoURL)
	if err != nil {
		t.Fatal(err)
	}
	for _, h := range []string{"### 💥 Breaking changes\n", "### 🪲 Bugs\n\n- Tags with a `v` prefix"} {
		if !strings.Contains(r.Section, h) {
			t.Errorf("section lacks %q:\n%s", h, r.Section)
		}
	}
	if strings.Contains(r.Section, "### Fixes") {
		t.Errorf("the default heading rendered:\n%s", r.Section)
	}
}

func TestSourcesRenderAsLinksOnlyWithAURL(t *testing.T) {
	e := releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFix, Summary: "s", Sources: []releasemodel.ChangeSource{pr(3), commit("0123456789abcdef"), issue("release-x")}}
	cases := map[string]string{
		repoURL: "- s ([#3](https://github.com/o/r/pull/3), [0123456](https://github.com/o/r/commit/0123456789abcdef), release-x)\n",
		"":      "- s (#3, 0123456, release-x)\n",
	}
	for url, want := range cases {
		r, err := Render(corpus("1.0.0", e), defaults(), url)
		if err != nil {
			t.Fatal(err)
		}
		if !strings.HasSuffix(r.Section, want) {
			t.Errorf("url %q: section = %q, want it to end %q", url, r.Section, want)
		}
	}
}

func TestCarriageReturnsInTheCorpusAreNormalised(t *testing.T) {
	c := corpus("1.0.0", releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFix, Summary: "s", Detail: str("a\r\nb\r\n"), Sources: []releasemodel.ChangeSource{}})
	c.Summary = str("x\ry\r\n")
	r, err := Render(c, defaults(), "")
	if err != nil {
		t.Fatal(err)
	}
	if want := "## 1.0.0\n\nx\ny\n\n### Fixes\n\n- s\n\n  a\n  b\n"; r.Section != want {
		t.Errorf("section = %q, want %q", r.Section, want)
	}
}

func TestASourceWithNoKindIsRefused(t *testing.T) {
	c := corpus("1.0.0", releasemodel.ChangelogEntry{Kind: releasemodel.ChangeKindFix, Summary: "s", Sources: []releasemodel.ChangeSource{{}}})
	if _, err := Render(c, defaults(), ""); err == nil {
		t.Error("a source with no kind rendered")
	}
}

func TestConfiguredHeadingsRenderVerbatim(t *testing.T) {
	s := defaults()
	s.Groups[1].Heading = "  Fixes, spaced  "
	r, err := Render(corpus190(), s, repoURL)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(r.Section, "\n###   Fixes, spaced  \n") {
		t.Errorf("the heading was not rendered verbatim:\n%q", r.Section)
	}
}
