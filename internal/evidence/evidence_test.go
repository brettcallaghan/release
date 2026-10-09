package evidence

import (
	"encoding/json"
	"reflect"
	"sort"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/schemacheck"
)

const (
	head = "3333333333333333333333333333333333333333"
	mid  = "2222222222222222222222222222222222222222"
	root = "1111111111111111111111111111111111111111"
	base = "0000000000000000000000000000000000000000"
)

// commit is a record with the fields the bundle carries, newest first in a
// range built by inputOf.
func commit(hash, subject, body string, trailers map[string]string) releasemodel.CommitRecord {
	return releasemodel.CommitRecord{
		Hash:        hash,
		Subject:     subject,
		Body:        body,
		AuthorDate:  "2026-10-01T10:00:00Z",
		AuthorName:  "A Author",
		AuthorEmail: "a@example.com",
		Files: []releasemodel.FileChange{{
			Path: "internal/x.go", Status: releasemodel.FileStatusModified, Additions: 2, Deletions: 1,
		}},
		Trailers: trailers,
	}
}

func withPR(r releasemodel.CommitRecord, number int64, title, description string, origin releasemodel.PullRequestOrigin) releasemodel.CommitRecord {
	r.PullRequest = &releasemodel.PullRequestInfo{Number: number, Title: str(title), Description: str(description), Origin: origin}
	return r
}

func str(s string) *string { return &s }

func diff(hash, path, patch string, truncated bool) releasemodel.DiffRecord {
	return releasemodel.DiffRecord{
		Commit:    hash,
		Files:     []releasemodel.FileDiff{{Path: path, Patch: patch, Truncated: truncated}},
		Truncated: truncated,
	}
}

// build runs Build and validates the bundle as it would cross the seam: as
// JSON, against the generated schema.
func build(t *testing.T, in Input) releasemodel.Evidence {
	t.Helper()
	ev, err := Build(in)
	if err != nil {
		t.Fatalf("Build: %v", err)
	}
	validates(t, ev)
	return ev
}

func validates(t *testing.T, ev releasemodel.Evidence) {
	t.Helper()
	data, err := json.Marshal(ev)
	if err != nil {
		t.Fatalf("marshalling the bundle: %v", err)
	}
	if err := schemacheck.Evidence(data); err != nil {
		t.Errorf("the bundle does not validate: %v\n%s", err, data)
	}
}

// jsonKeys returns the sorted keys of a JSON object.
func jsonKeys(t *testing.T, v any) []string {
	t.Helper()
	data, err := json.Marshal(v)
	if err != nil {
		t.Fatalf("marshalling: %v", err)
	}
	var m map[string]json.RawMessage
	if err := json.Unmarshal(data, &m); err != nil {
		t.Fatalf("unmarshalling: %v", err)
	}
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	return keys
}

// mixedInput is a range of four commits, newest first as the collector lists
// them: a squash commit with a pull request, a merge commit, a direct
// unclassified commit and a conventional commit with no pull request.
func mixedInput() Input {
	squash := withPR(
		commit(head, "feat(cli): add --dry-run (#42)", "Adds a dry run.\n\nBREAKING CHANGE: none", map[string]string{"Release-Type": "minor"}),
		42, "add --dry-run", "Adds a dry run.", releasemodel.PullRequestOriginSquash,
	)
	merge := withPR(
		commit(mid, "Merge pull request #7 from fork/branch", "Fix the parser\n\nDetails here.", nil),
		7, "Fix the parser", "Details here.", releasemodel.PullRequestOriginMerge,
	)
	direct := commit("4444444444444444444444444444444444444444", "wip tidy", "", map[string]string{})
	fix := commit(root, "fix(core): handle empty range", "", map[string]string{})
	return Input{
		Range:       Range{From: base, To: head},
		BaseVersion: "1.8.0",
		Commits: []releasemodel.CommitRecord{
			squash,
			direct,
			merge,
			fix,
		},
		Signals: map[string]releasemodel.Bump{
			head: releasemodel.BumpMinor,
			root: releasemodel.BumpPatch,
		},
	}
}

// AC: AC-CLOG-002.1
func TestBundleValidatesAgainstTheSchema(t *testing.T) {
	withDiffs := mixedInput()
	withDiffs.DiffsCollected = true
	withDiffs.Diffs = []releasemodel.DiffRecord{
		diff(head, "internal/cli/dry.go", "@@ -1 +1 @@\n-old\n+new\n", false),
		diff(root, "internal/core/range.go", "@@ -1 +1 @@\n-x\n+y\n", true),
	}
	withDiffs.Budget = releasemodel.BudgetReport{
		Limit: 1000, Total: 2000, Used: 900,
		Omitted: []releasemodel.OmittedDiff{{Commit: root, Path: "big.bin", Reason: releasemodel.OmissionReasonOverBudget, Truncated: false}},
	}

	cases := map[string]Input{
		"empty range, first release": {Range: Range{To: head}},
		"mixed commits, no diffs":    mixedInput(),
		"mixed commits, with diffs":  withDiffs,
	}
	for name, in := range cases {
		t.Run(name, func(t *testing.T) {
			build(t, in)
		})
	}
}

// AC: AC-CLOG-002.6
func TestEveryCommitInRangeIsCarriedOldestFirst(t *testing.T) {
	in := mixedInput()
	ev := build(t, in)

	if len(ev.Commits) != len(in.Commits) {
		t.Fatalf("got %d commits, want %d: every commit in the range must be carried", len(ev.Commits), len(in.Commits))
	}
	// The input is newest first; the bundle is oldest first.
	for i, want := range []string{root, mid, "4444444444444444444444444444444444444444", head} {
		if got := ev.Commits[i].Record.Hash; got != want {
			t.Errorf("commit %d: got %s, want %s", i, got, want)
		}
	}

	squash := ev.Commits[3]
	if squash.Record.Subject != "feat(cli): add --dry-run (#42)" || squash.Record.Body != "Adds a dry run.\n\nBREAKING CHANGE: none" {
		t.Errorf("the record's subject or body was altered: %q / %q", squash.Record.Subject, squash.Record.Body)
	}
	if !reflect.DeepEqual(squash.Record.Trailers, map[string]string{"Release-Type": "minor"}) {
		t.Errorf("trailers not carried: %v", squash.Record.Trailers)
	}
	if squash.PullRequest == nil || squash.PullRequest.Number != 42 || *squash.PullRequest.Title != "add --dry-run" || squash.PullRequest.Origin != releasemodel.PullRequestOriginSquash {
		t.Errorf("squash pull request information not carried: %+v", squash.PullRequest)
	}
	if merge := ev.Commits[1]; merge.PullRequest == nil || merge.PullRequest.Origin != releasemodel.PullRequestOriginMerge {
		t.Errorf("merge pull request information not carried: %+v", merge.PullRequest)
	}
	if direct := ev.Commits[2]; direct.PullRequest != nil {
		t.Errorf("a direct commit gained pull request information: %+v", direct.PullRequest)
	}
	for _, c := range ev.Commits {
		if len(c.Record.Files) != 1 || c.Record.Files[0].Path != "internal/x.go" {
			t.Errorf("commit %s: changed files not carried: %+v", c.Record.Hash, c.Record.Files)
		}
	}
}

// AC: AC-CLOG-002.7
func TestBundleStatesItsPurposeRangeAndBaseVersion(t *testing.T) {
	ev := build(t, mixedInput())

	if ev.Purpose != releasemodel.EvidencePurposeRelease {
		t.Errorf("purpose: got %q, want RELEASE", ev.Purpose)
	}
	if ev.Range.From == nil || *ev.Range.From != base || ev.Range.To != head {
		t.Errorf("range: got from=%v to=%s", ev.Range.From, ev.Range.To)
	}
	if ev.BaseVersion == nil || *ev.BaseVersion != "1.8.0" {
		t.Errorf("base version: got %v, want 1.8.0", ev.BaseVersion)
	}

	first := build(t, Input{Range: Range{To: head}})
	if first.BaseVersion != nil || first.Range.From != nil {
		t.Errorf("a line with no release must carry no base version or range start, got version=%v from=%v", first.BaseVersion, first.Range.From)
	}
}

// AC: AC-CLOG-002.4
func TestEveryContributingAdapterIsRecorded(t *testing.T) {
	in := mixedInput()
	ev := build(t, in)
	if len(ev.Sources) != 1 || ev.Sources[0].Adapter != releasemodel.AdapterCommits {
		t.Fatalf("with no diffs collected, sources must be the COMMITS adapter alone: %+v", ev.Sources)
	}
	if ev.Sources[0].From == nil || *ev.Sources[0].From != base || ev.Sources[0].To != head {
		t.Errorf("COMMITS range: got %+v", ev.Sources[0])
	}

	in.DiffsCollected = true
	in.Budget = releasemodel.BudgetReport{Limit: 500}
	ev = build(t, in)
	if len(ev.Sources) != 2 {
		t.Fatalf("with diffs collected, want COMMITS and DIFFS, got %+v", ev.Sources)
	}
	if ev.Sources[0].Adapter != releasemodel.AdapterCommits || ev.Sources[1].Adapter != releasemodel.AdapterDiffs {
		t.Errorf("adapters: got %s then %s", ev.Sources[0].Adapter, ev.Sources[1].Adapter)
	}
	for _, run := range ev.Sources {
		if run.From == nil || *run.From != base || run.To != head {
			t.Errorf("%s range: got from=%v to=%s", run.Adapter, run.From, run.To)
		}
	}
	if ev.Sources[1].Budget == nil || ev.Sources[1].Budget.Limit != 500 {
		t.Errorf("the DIFFS run must carry the budget report, got %+v", ev.Sources[1].Budget)
	}
}

// AC: AC-CLOG-002.8
func TestBudgetReportRecordsWhatWasLeftOut(t *testing.T) {
	in := mixedInput()
	in.DiffsCollected = true
	in.Diffs = []releasemodel.DiffRecord{
		diff(head, "internal/cli/dry.go", "@@ partial", true),
	}
	in.Budget = releasemodel.BudgetReport{
		Limit: 100, Total: 5000, Used: 100,
		Omitted: []releasemodel.OmittedDiff{
			{Commit: head, Path: "internal/cli/dry.go", Reason: releasemodel.OmissionReasonOverBudget, Truncated: true},
			{Commit: root, Path: "vendor/blob.bin", Reason: releasemodel.OmissionReasonBinary, Truncated: false},
		},
	}
	ev := build(t, in)

	if !reflect.DeepEqual(ev.Budget, in.Budget) {
		t.Errorf("the budget report was not carried unchanged:\n got %+v\nwant %+v", ev.Budget, in.Budget)
	}
	if ev.Sources[1].Budget == nil || !reflect.DeepEqual(*ev.Sources[1].Budget, in.Budget) {
		t.Errorf("the DIFFS run's budget differs from the bundle's: %+v", ev.Sources[1].Budget)
	}
	var headEv *releasemodel.CommitEvidence
	for i := range ev.Commits {
		if ev.Commits[i].Record.Hash == head {
			headEv = &ev.Commits[i]
		}
	}
	if headEv == nil || headEv.Diff == nil || !headEv.Diff.Truncated {
		t.Errorf("a commit whose patch the budget cut must carry its DiffRecord marked truncated: %+v", headEv)
	}
}

// AC: AC-CLOG-002.8
func TestNoDiffsCollectedReportsZeroTotalsAgainstTheLimit(t *testing.T) {
	in := mixedInput()
	in.Budget = releasemodel.BudgetReport{Limit: 2048, Total: 999, Used: 999}
	ev := build(t, in)

	want := releasemodel.BudgetReport{Limit: 2048, Total: 0, Used: 0, Omitted: []releasemodel.OmittedDiff{}}
	if !reflect.DeepEqual(ev.Budget, want) {
		t.Errorf("budget with no diffs collected: got %+v, want %+v", ev.Budget, want)
	}
	for _, c := range ev.Commits {
		if c.Diff != nil {
			t.Errorf("commit %s carries a diff though none were collected", c.Record.Hash)
		}
	}
}

// AC: AC-CLOG-002.11
func TestEachCommitCarriesTheSignalTheClassifierRead(t *testing.T) {
	in := mixedInput()
	ev := build(t, in)

	signals := map[string]*releasemodel.Bump{}
	for _, c := range ev.Commits {
		signals[c.Record.Hash] = c.Signal
	}
	if s := signals[head]; s == nil || *s != releasemodel.BumpMinor {
		t.Errorf("the Release-Type minor commit: got signal %v, want MINOR", s)
	}
	if s := signals[root]; s == nil || *s != releasemodel.BumpPatch {
		t.Errorf("the classified fix commit: got signal %v, want PATCH", s)
	}
	if s := signals[mid]; s != nil {
		t.Errorf("the unclassified merge commit states a signal %v; it must carry none", *s)
	}
	data, err := json.Marshal(ev.Commits[2])
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"signal"`) {
		t.Errorf("an unclassified commit must omit the signal key: %s", data)
	}
}

// AC: AC-CLOG-002.5
func TestBundleProposesNoImpactAndAuthorsNoEntry(t *testing.T) {
	in := mixedInput()
	in.DiffsCollected = true
	in.Budget = releasemodel.BudgetReport{Limit: 10}
	ev := build(t, in)

	wantTop := []string{"base_version", "budget", "commits", "purpose", "range", "schema_version", "sources"}
	if got := jsonKeys(t, ev); !reflect.DeepEqual(got, wantTop) {
		t.Errorf("top-level keys: got %v, want %v", got, wantTop)
	}
	for _, c := range ev.Commits {
		for _, key := range jsonKeys(t, c) {
			switch key {
			case "record", "conventional", "pull_request", "diff", "signal":
			default:
				t.Errorf("commit %s carries key %q, which is an authored or derived field", c.Record.Hash, key)
			}
		}
	}
}

// AC: AC-CLOG-002.9
func TestCommitWithoutConventionalTypeGetsNoKind(t *testing.T) {
	in := Input{
		Range: Range{From: base, To: head},
		Commits: []releasemodel.CommitRecord{
			commit(head, "Update the readme", "", nil),
		},
	}
	ev := build(t, in)
	c := ev.Commits[0]
	if c.Conventional != nil {
		t.Errorf("a non-conventional subject was given a conventional part: %+v", c.Conventional)
	}
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(data), `"conventional"`) {
		t.Errorf("the commit states a conventional part it does not have: %s", data)
	}
}

func TestBuildRefusesWhatItCannotJoin(t *testing.T) {
	cases := map[string]func(*Input){
		"no head commit": func(in *Input) { in.Range.To = "" },
		"commit listed twice": func(in *Input) {
			in.Commits = append(in.Commits, in.Commits[0])
		},
		"diff naming a commit outside the range": func(in *Input) {
			in.DiffsCollected = true
			in.Diffs = []releasemodel.DiffRecord{diff(base, "x", "", false)}
		},
		"two diffs for one commit": func(in *Input) {
			in.DiffsCollected = true
			in.Diffs = []releasemodel.DiffRecord{diff(head, "x", "", false), diff(head, "y", "", false)}
		},
		"diffs given with no diff collector": func(in *Input) {
			in.Diffs = []releasemodel.DiffRecord{diff(head, "x", "", false)}
		},
		"signal naming a commit outside the range": func(in *Input) {
			in.Signals[base] = releasemodel.BumpMajor
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			in := mixedInput()
			mutate(&in)
			if _, err := Build(in); err == nil {
				t.Fatal("Build accepted an input it cannot join")
			}
		})
	}
}
