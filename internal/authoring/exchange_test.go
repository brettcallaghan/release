package authoring

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

const (
	hashA = "aaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaaa"
	hashB = "bbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbbb"
	hashC = "cccccccccccccccccccccccccccccccccccccccc"
)

func commitEv(hash, subject, body string, pr int64) releasemodel.CommitEvidence {
	r := releasemodel.CommitRecord{Hash: hash, Subject: subject, Body: body, Trailers: map[string]string{}, Files: []releasemodel.FileChange{}}
	if pr > 0 {
		r.PullRequest = &releasemodel.PullRequestInfo{Number: pr, Origin: releasemodel.PullRequestOriginSquash}
	}
	return releasemodel.CommitEvidence{Record: r}
}

func bundle(commits ...releasemodel.CommitEvidence) releasemodel.Evidence {
	from := "0000000000000000000000000000000000000000"
	return releasemodel.Evidence{
		SchemaVersion: "1.0.0",
		Purpose:       releasemodel.EvidencePurposeRelease,
		Range:         releasemodel.Range{From: &from, To: head},
		Commits:       commits,
		Sources:       []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, From: &from, To: head}},
		Budget:        releasemodel.BudgetReport{Omitted: []releasemodel.OmittedDiff{}},
	}
}

func threeCommits() releasemodel.Evidence {
	return bundle(
		commitEv(hashA, "feat: a (#7)", "", 7),
		commitEv(hashB, "fix: b", "", 0),
		commitEv(hashC, "test: c", "", 0),
	)
}

func cites(hashes ...string) []releasemodel.ChangeSource {
	out := []releasemodel.ChangeSource{}
	for _, h := range hashes {
		out = append(out, releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: h}))
	}
	return out
}

func entry(summary string, hashes ...string) releasemodel.ChangelogEntry {
	return releasemodel.ChangelogEntry{Kind: kp(releasemodel.ChangeKindFeature), Summary: summary, Sources: cites(hashes...)}
}

func omit(hash, reason string) releasemodel.OmittedCommit {
	return releasemodel.OmittedCommit{Hash: hash, Reason: reason}
}

// changelogFor is a conforming changelog for ev: one entry per commit.
func changelogFor(ev releasemodel.Evidence) releasemodel.Changelog {
	c := releasemodel.Changelog{SchemaVersion: "1.0.0", Entries: []releasemodel.ChangelogEntry{}, Sources: ev.Sources}
	for _, ce := range ev.Commits {
		c.Entries = append(c.Entries, entry("Does "+ce.Record.Subject+".", ce.Record.Hash))
	}
	return c
}

func encode(t *testing.T, c releasemodel.Changelog) []byte {
	t.Helper()
	data, err := json.Marshal(c)
	if err != nil {
		t.Fatal(err)
	}
	return data
}

// reject asserts c is rejected and that the rejection mentions every want.
func reject(t *testing.T, ev releasemodel.Evidence, c releasemodel.Changelog, want ...string) {
	t.Helper()
	_, err := AcceptChangelog(encode(t, c), "test", ev)
	var r *Rejection
	if !errors.As(err, &r) {
		t.Fatalf("err = %v, want a *Rejection", err)
	}
	for _, w := range want {
		if !strings.Contains(err.Error(), w) {
			t.Errorf("rejection %q does not mention %q", err, w)
		}
	}
}

func accept(t *testing.T, ev releasemodel.Evidence, c releasemodel.Changelog) {
	t.Helper()
	if _, err := AcceptChangelog(encode(t, c), "test", ev); err != nil {
		t.Fatalf("rejected: %v", err)
	}
}

func evRequest(t *testing.T, p Producer) EvidenceRequest {
	t.Helper()
	return EvidenceRequest{
		Evidence:  threeCommits(),
		Producer:  "stub",
		Producers: map[string]Producer{"stub": p, "mechanical": &stub{out: encode(t, changelogFor(threeCommits()))}},
		CachePath: filepath.Join(t.TempDir(), "release", "changelog", "x..y.json"),
	}
}

// AC: AC-CLOG-003.1
func TestAChangelogBreakingTheSchemaIsRejected(t *testing.T) {
	ev := threeCommits()
	for name, doc := range map[string]string{
		"not JSON":       "here are your notes!",
		"missing fields": `{"schema_version":"1.0.0","sources":[]}`,
		"unknown kind":   strings.Replace(string(encode(t, changelogFor(ev))), `"FEATURE"`, `"FEAT"`, 1),
	} {
		if _, err := AcceptChangelog([]byte(doc), "test", ev); err == nil {
			t.Errorf("%s: accepted", name)
		}
	}
}

// AC: AC-CLOG-003.2
func TestAnUnimplementedSchemaVersionIsRejectedAgainstTheBundle(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.SchemaVersion = "2.0.0"
	reject(t, ev, c, "schema_version 2.0.0")
}

// AC: AC-CLOG-003.1
func TestAChangelogForAnotherRangeIsRejected(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Sources = []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, To: "9999999999999999999999999999999999999999"}}
	reject(t, ev, c, "another range")
}

// AC: AC-CLOG-003.14
func TestAnEntryCitingACommitOutsideTheBundleIsRejected(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries[0].Sources = cites("dddddddddddddddddddddddddddddddddddddddd")
	c.Entries = append(c.Entries, entry("Cites a real commit again.", hashA))
	reject(t, ev, c, "dddddddddddddddddddddddddddddddddddddddd", "entry 1")
	c = changelogFor(ev)
	c.Entries[0].Sources = append(c.Entries[0].Sources, releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 99}))
	reject(t, ev, c, "pull request #99")
	c = changelogFor(ev)
	c.Entries[0].Sources = append(c.Entries[0].Sources, releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 7}))
	accept(t, ev, c)
}

// AC: AC-CLOG-003.15
func TestAnEntryWithNoSourceIsRejected(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries[1].Sources = []releasemodel.ChangeSource{}
	reject(t, ev, c, "entry 2", "cites no commit")
}

// AC: AC-CLOG-003.15
func TestAnEntryCitingOnlyAPullRequestIsRejected(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	extra := entry("Invented summary on a real pull request.")
	extra.Sources = []releasemodel.ChangeSource{releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 7})}
	c.Entries = append(c.Entries, extra)
	reject(t, ev, c, "entry 4", "cites no commit")
}

// AC: AC-CLOG-003.9
func TestACachedChangelogSurvivesAChangedDiffBudgetOrAdapterSet(t *testing.T) {
	from := "0000000000000000000000000000000000000000"
	diffs := func(limit int64) releasemodel.AdapterRun {
		return releasemodel.AdapterRun{Adapter: releasemodel.AdapterDiffs, From: &from, To: head, Budget: &releasemodel.BudgetReport{Limit: limit, Omitted: []releasemodel.OmittedDiff{}}}
	}
	authored := threeCommits()
	authored.Sources = append(authored.Sources, diffs(1000))
	p := &stub{out: encode(t, changelogFor(authored))}
	req := evRequest(t, p)
	req.Evidence = authored
	if _, err := ExchangeEvidence(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	variants := map[string][]releasemodel.AdapterRun{
		"budget":     {authored.Sources[0], diffs(2000)},
		"no diffs":   {authored.Sources[0]},
		"diffs only": {diffs(1000)},
	}
	for name, sources := range variants {
		req.Evidence.Sources = sources
		res, err := ExchangeEvidence(context.Background(), req)
		if err != nil || res.Obtained != FromCache || len(p.received) != 1 {
			t.Errorf("%s: res = %+v, err = %v, invocations = %d; want a cache hit", name, res, err, len(p.received))
		}
	}
	other := "9999999999999999999999999999999999999999"
	req.Evidence.Sources = []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, From: &from, To: other}}
	if _, err := ExchangeEvidence(context.Background(), req); err == nil || !strings.Contains(err.Error(), "another range") {
		t.Errorf("err = %v, want a rejection naming another range", err)
	}
	req.Evidence.Sources = []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, To: head}}
	if _, err := ExchangeEvidence(context.Background(), req); err == nil || !strings.Contains(err.Error(), "another range") {
		t.Errorf("a different from: err = %v, want a rejection naming another range", err)
	}
}

// AC: AC-CLOG-003.1
func TestAProducerAlteringTheBundlesSourcesIsRejected(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	from := "0000000000000000000000000000000000000000"
	c.Sources = []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, From: &from, To: head, Budget: &releasemodel.BudgetReport{Limit: 5, Omitted: []releasemodel.OmittedDiff{}}}}
	p := &stub{out: encode(t, c)}
	_, err := ExchangeEvidence(context.Background(), evRequest(t, p))
	var r *Rejection
	if !errors.As(err, &r) {
		t.Fatalf("err = %v, want a *Rejection", err)
	}
}

// AC: AC-CLOG-003.16
func TestACommitNeitherCitedNorOmittedIsRejectedAndNamed(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries = c.Entries[:2]
	reject(t, ev, c, hashC, "neither cited")
}

// AC: AC-CLOG-003.17
func TestAnOmissionWithoutAReasonIsRejectedAndNamed(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries = c.Entries[:2]
	c.Omitted = &[]releasemodel.OmittedCommit{omit(hashC, "  ")}
	reject(t, ev, c, hashC, "no reason")
	c.Omitted = &[]releasemodel.OmittedCommit{omit(hashC, "line one\nline two")}
	reject(t, ev, c, hashC, "more than one line")
	c.Omitted = &[]releasemodel.OmittedCommit{omit("eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "r"), omit(hashC, "tests only")}
	reject(t, ev, c, "eeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeeee", "not in the evidence bundle")
}

// AC: AC-CLOG-004.3
func TestInternalWorkOmittedWithAReasonIsAccepted(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries = c.Entries[:2]
	c.Omitted = &[]releasemodel.OmittedCommit{omit(hashC, "tests only")}
	accept(t, ev, c)
}

func TestACommitBothCitedAndOmittedIsRejected(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Omitted = &[]releasemodel.OmittedCommit{omit(hashA, "also here")}
	reject(t, ev, c, hashA, "both cited")
}

// AC: AC-CLOG-003.18
func TestABreakingEntryWithoutMigrationIsRejectedAndNamed(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries[2].Breaking = true
	reject(t, ev, c, "entry 3", "no migration")
	m := "run the tool again"
	c.Entries[2].Migration = &m
	accept(t, ev, c)
}

// AC: AC-CLOG-003.19
func TestASummaryThatIsEmptyBlankLinedOrLongIsRejected(t *testing.T) {
	ev := threeCommits()
	for name, summary := range map[string]string{
		"empty":       "  ",
		"blank line":  "One.\n\nTwo.",
		"three":       "It does one. It does two! Does it do three?",
		"three, ends": "It does one. It does two. Three",
	} {
		c := changelogFor(ev)
		c.Entries[1].Summary = summary
		if name == "three, ends" {
			c.Entries[1].Summary += "."
		}
		reject(t, ev, c, "entry 2")
	}
	for _, ok := range []string{"One.", "One. Two.", "Uses v1.2 and e.g. a thing", "Line\nwrapped across two lines."} {
		c := changelogFor(ev)
		c.Entries[1].Summary = ok
		accept(t, ev, c)
	}
}

func TestRejectionNamesEveryRuleBroken(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	c.Entries[0].Sources = []releasemodel.ChangeSource{}
	c.Entries[1].Breaking = true
	c.Entries = c.Entries[:2]
	c.Sources = []releasemodel.AdapterRun{}
	reject(t, ev, c, "entry 1", "entry 2", hashC, "another range")
}

// AC: AC-CLOG-004.1
func TestOneCommitMayYieldSeveralEntries(t *testing.T) {
	ev := bundle(commitEv(hashA, "feat: big squash", "four things", 0))
	c := changelogFor(ev)
	c.Entries = nil
	for _, s := range []string{"First.", "Second.", "Third.", "Fourth."} {
		c.Entries = append(c.Entries, entry(s, hashA))
	}
	accept(t, ev, c)
}

// AC: AC-CLOG-004.2
func TestSeveralCommitsMayYieldOneEntry(t *testing.T) {
	ev := bundle(commitEv(hashA, "feat: part", "", 0), commitEv(hashB, "feat: rest", "", 0))
	c := changelogFor(ev)
	c.Entries = []releasemodel.ChangelogEntry{entry("Both parts ship together.", hashA, hashB)}
	accept(t, ev, c)
}

func TestProposedBumpAndAnEmptyBundleAreAccepted(t *testing.T) {
	ev := bundle()
	c := changelogFor(ev)
	pb := releasemodel.BumpMajor
	c.ProposedBump = &pb
	accept(t, ev, c)
}

// AC: AC-CLOG-003.6
func TestAnAcceptedChangelogIsCachedByteForByte(t *testing.T) {
	ev := threeCommits()
	c := changelogFor(ev)
	t1 := "Résumé-safe <targets> & “quotes” 🚀"
	c.Title = &t1
	c.Entries[0].Summary = "Adds **bold** `code` — naïvely."
	raw := []byte("{ \"schema_version\":\"1.0.0\",\n" + strings.TrimPrefix(string(encodeNoEscape(t, c)), "{\"schema_version\":\"1.0.0\","))
	p := &stub{out: raw}
	req := evRequest(t, p)
	res, err := ExchangeEvidence(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Obtained != ByProducer || *res.Changelog.Title != t1 || res.Changelog.Entries[0].Summary != "Adds **bold** `code` — naïvely." {
		t.Errorf("result = %+v", res)
	}
	cached, err := os.ReadFile(req.CachePath)
	if err != nil || string(cached) != string(raw) {
		t.Errorf("cache = %q (%v), want the producer's bytes", cached, err)
	}
}

func encodeNoEscape(t *testing.T, c releasemodel.Changelog) []byte {
	t.Helper()
	return encode(t, c)
}

// AC: AC-CLOG-003.12
// AC: AC-CLOG-002.10
func TestTheProducerReceivesTheBundleAndItIsKeptBesideTheCache(t *testing.T) {
	ev := bundle(commitEv(hashA, "feat: x", "one\ntwo\nthree\nfour changes <&>", 0))
	p := &stub{out: encode(t, changelogFor(ev))}
	req := evRequest(t, p)
	req.Evidence = ev
	if _, err := ExchangeEvidence(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(p.received) != 1 || !strings.Contains(string(p.received[0]), "four changes <&>") {
		t.Fatalf("producer input = %q", p.received)
	}
	kept, err := os.ReadFile(filepath.Join(filepath.Dir(req.CachePath), "x..y.evidence.json"))
	if err != nil || string(kept) != string(p.received[0]) {
		t.Errorf("kept bundle = %q (%v), want what the producer was given", kept, err)
	}
	if got := EvidencePath("/a/named.json"); got != "/a/named.evidence.json" {
		t.Errorf("EvidencePath = %q", got)
	}
	if got := EvidencePath("/a/named"); got != "/a/named.evidence.json" {
		t.Errorf("EvidencePath = %q", got)
	}
}

// AC: AC-CLOG-003.7
func TestTheExchangeHasNoNetworkPath(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		if strings.Contains(dep, "anthropic") {
			t.Errorf("the bridge depends on %s, a model client", dep)
		}
		switch dep {
		case "net", "net/http", "crypto/tls", "net/rpc", "net/smtp":
			t.Errorf("the bridge depends on %s, a network package", dep)
		}
	}
}

// AC: AC-CLOG-003.8
func TestOnlyTheConfiguredProducerIsInvoked(t *testing.T) {
	chosen, other := &stub{}, &stub{}
	req := evRequest(t, chosen)
	chosen.out = encode(t, changelogFor(req.Evidence))
	req.Producers["other"] = other
	if _, err := ExchangeEvidence(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(chosen.received) != 1 || len(other.received) != 0 {
		t.Errorf("invocations: chosen %d, other %d", len(chosen.received), len(other.received))
	}
	req.Producer, req.Redo = "nope", true
	if _, err := ExchangeEvidence(context.Background(), req); !errors.Is(err, ErrUnknownProducer) {
		t.Errorf("err = %v, want ErrUnknownProducer", err)
	}
}

// AC: AC-CLOG-003.9
func TestACachedChangelogIsReusedWithoutAProducer(t *testing.T) {
	p := &stub{}
	req := evRequest(t, p)
	p.out = encode(t, changelogFor(req.Evidence))
	if _, err := ExchangeEvidence(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	res, err := ExchangeEvidence(context.Background(), req)
	if err != nil || res.Obtained != FromCache || len(p.received) != 1 {
		t.Errorf("res = %+v, err = %v, invocations = %d; want a cache hit", res, err, len(p.received))
	}
	if err := os.WriteFile(req.CachePath, []byte(`{"schema_version":"9.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := ExchangeEvidence(context.Background(), req); err == nil {
		t.Error("an invalid cached changelog was accepted")
	}
}

// AC: AC-CLOG-003.10
func TestRedoReplacesTheCacheOnlyAfterAcceptance(t *testing.T) {
	p := &stub{}
	req := evRequest(t, p)
	good := changelogFor(req.Evidence)
	p.out = encode(t, good)
	if _, err := ExchangeEvidence(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	first, _ := os.ReadFile(req.CachePath)
	req.Redo = true
	bad := changelogFor(req.Evidence)
	bad.Entries = bad.Entries[:1]
	p.out = encode(t, bad)
	if _, err := ExchangeEvidence(context.Background(), req); err == nil {
		t.Fatal("a rejected redo succeeded")
	}
	if now, _ := os.ReadFile(req.CachePath); string(now) != string(first) {
		t.Error("a rejected redo replaced the cache")
	}
	good.Entries[0].Summary = "Changed."
	p.out = encode(t, good)
	res, err := ExchangeEvidence(context.Background(), req)
	if err != nil || res.Obtained != ByProducer || len(p.received) != 3 {
		t.Fatalf("res = %+v, err = %v, invocations = %d", res, err, len(p.received))
	}
	if now, _ := os.ReadFile(req.CachePath); string(now) != string(p.out) {
		t.Error("an accepted redo did not replace the cache")
	}
}

// AC: AC-CLOG-003.11
func TestSkipUsesTheMechanicalProducerAndTouchesNoCache(t *testing.T) {
	configured := &stub{}
	req := evRequest(t, configured)
	req.Skip = true
	if err := os.MkdirAll(filepath.Dir(req.CachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(req.CachePath, []byte("not read"), 0o644); err != nil {
		t.Fatal(err)
	}
	res, err := ExchangeEvidence(context.Background(), req)
	if err != nil || res.Obtained != Mechanical || res.Producer != "mechanical" {
		t.Fatalf("res = %+v, err = %v", res, err)
	}
	if len(configured.received) != 0 {
		t.Error("the configured producer ran under skip")
	}
	if got, _ := os.ReadFile(req.CachePath); string(got) != "not read" {
		t.Error("skip wrote the cache")
	}
	if _, err := os.Stat(EvidencePath(req.CachePath)); err == nil {
		t.Error("skip kept an evidence bundle")
	}
}

func TestAFailingProducerStopsTheExchangeNamingSkipNotes(t *testing.T) {
	failing := &stub{err: errors.New("exit status 3: the model is unavailable")}
	mech := &stub{out: nil}
	req := evRequest(t, failing)
	req.Producers["mechanical"] = mech
	_, err := ExchangeEvidence(context.Background(), req)
	var pf *ProducerFailure
	if !errors.As(err, &pf) || !strings.Contains(err.Error(), "--skip-notes") || !strings.Contains(err.Error(), "the model is unavailable") {
		t.Fatalf("err = %v, want the failure naming --skip-notes", err)
	}
	if len(mech.received) != 0 {
		t.Error("another producer was tried")
	}
	if _, statErr := os.Stat(req.CachePath); statErr == nil {
		t.Error("a failure was cached")
	}
	// A refusal that already names --skip-notes is not told twice.
	failing.err = errors.New("claude is not on PATH; pass --skip-notes to author")
	_, err = ExchangeEvidence(context.Background(), req)
	if strings.Count(err.Error(), "--skip-notes") != 1 {
		t.Errorf("err = %v, want --skip-notes named once", err)
	}
}

// AC: AC-CLOG-003.9
func TestChangelogCachePathKeysOnTheRangeInTheCommonDir(t *testing.T) {
	if _, err := exec.LookPath("git"); err != nil {
		t.Skip("git not on PATH")
	}
	dir := t.TempDir()
	if out, err := exec.Command("git", "init", "-q", dir).CombinedOutput(); err != nil {
		t.Fatalf("git init: %v\n%s", err, out)
	}
	repo := gitread.Repo{Dir: dir}
	common, err := repo.CommonDir(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	got, err := ChangelogCachePath(context.Background(), repo, "abc", head)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(common, "release", "changelog", "abc.."+head+".json"); got != want {
		t.Errorf("path = %q, want %q", got, want)
	}
	if got, _ = ChangelogCachePath(context.Background(), repo, "", head); !strings.HasSuffix(got, "root.."+head+".json") {
		t.Errorf("path = %q, want root for a line with no release", got)
	}
}
