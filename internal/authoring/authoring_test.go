package authoring

import (
	"bytes"
	"context"
	"errors"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// stub is a producer that answers with a fixed document and records each
// corpus it received.
type stub struct {
	out      []byte
	err      error
	received [][]byte
}

func (s *stub) Produce(_ context.Context, in []byte) ([]byte, error) {
	s.received = append(s.received, in)
	return s.out, s.err
}

const head = "1111111111111111111111111111111111111111"

func assembled() releasemodel.Changelog {
	return releasemodel.Changelog{
		SchemaVersion: "1.0.0",
		Entries: []releasemodel.ChangelogEntry{{
			Kind: releasemodel.ChangeKindFeature, Summary: "add --target", Derived: true,
			Sources: []releasemodel.ChangeSource{releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 12})},
		}},
		Sources: []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, To: head}},
	}
}

// authored is a producer's answer proposing bump, with prose that carries
// markdown, non-ASCII text, characters JSON encoders like to escape, and a
// key layout no Go encoder would produce.
func authored(bump string) []byte {
	return []byte(`{"schema_version":"1.0.0",
  "proposed_bump": "` + bump + `",
  "title": "Résumé-safe <targets> & “quotes” 🚀",
  "summary": "Line one\n\n- **bold** item\n- ` + "`code`" + ` item\t(tab)  ",
  "entries": [{"kind": "FEATURE", "summary": "Compute for any line — naïvely", "breaking": false,
    "derived": false, "detail": "Uses é and \\u escapes",
    "sources": [{"__type": "PullRequest", "number": 12}]}],
  "sources": [{"adapter": "COMMITS", "to": "` + head + `"}],
  "unknown_but_tolerated": 1
}
`)
}

func request(t *testing.T, p Producer) Request {
	t.Helper()
	return Request{
		Assembled: assembled(),
		Computed:  releasemodel.BumpMinor,
		Producer:  "stub",
		Producers: map[string]Producer{"stub": p},
		CachePath: filepath.Join(t.TempDir(), "release", "corpus", "root.."+head+".json"),
	}
}

// AC: AC-CLOG-003.1
func TestAnInvalidReturnedCorpusIsRejectedBeforeAnyConsumer(t *testing.T) {
	cases := map[string]string{
		"not JSON":            "here are your notes!",
		"fenced":              "```json\n" + string(authored("MINOR")) + "```",
		"missing entries":     `{"schema_version":"1.0.0","proposed_bump":"MINOR","sources":[]}`,
		"unknown kind":        strings.Replace(string(authored("MINOR")), `"FEATURE"`, `"FEAT"`, 1),
		"entry without flags": `{"schema_version":"1.0.0","proposed_bump":"MINOR","sources":[],"entries":[{"kind":"FIX","summary":"x","sources":[]}]}`,
		"no proposed_bump":    strings.Replace(string(authored("MINOR")), `"proposed_bump": "MINOR",`, ``, 1),
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			p := &stub{out: []byte(doc)}
			req := request(t, p)
			res, err := Exchange(context.Background(), req)
			var rej *Rejection
			if !errors.As(err, &rej) {
				t.Fatalf("err = %v, want a *Rejection", err)
			}
			if res.Corpus.Entries != nil || res.Proposed != nil {
				t.Errorf("a rejected exchange returned a corpus: %+v", res)
			}
			if _, err := os.Stat(req.CachePath); !errors.Is(err, os.ErrNotExist) {
				t.Errorf("a rejected corpus was cached (stat err %v)", err)
			}
		})
	}
}

// AC: AC-CLOG-003.2
func TestAnUnimplementedSchemaVersionIsRejected(t *testing.T) {
	for _, v := range []string{"2.0.0", "0.9.0", "one"} {
		doc := strings.Replace(string(authored("MINOR")), `"schema_version":"1.0.0"`, `"schema_version":"`+v+`"`, 1)
		_, err := Exchange(context.Background(), request(t, &stub{out: []byte(doc)}))
		var rej *Rejection
		if !errors.As(err, &rej) || !strings.Contains(rej.Reason, "schema_version") {
			t.Errorf("schema_version %s: err = %v, want a rejection naming schema_version", v, err)
		}
	}
	// A minor or patch revision of the implemented major is compatible.
	doc := strings.Replace(string(authored("MINOR")), `"schema_version":"1.0.0"`, `"schema_version":"1.3.1"`, 1)
	if _, err := Exchange(context.Background(), request(t, &stub{out: []byte(doc)})); err != nil {
		t.Errorf("schema_version 1.3.1 refused: %v", err)
	}
}

// AC: AC-CLOG-003.3
func TestAgreementProceeds(t *testing.T) {
	res, err := Exchange(context.Background(), request(t, &stub{out: authored("MINOR")}))
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Authored || res.Disagrees || *res.Proposed != releasemodel.BumpMinor || res.Computed != releasemodel.BumpMinor {
		t.Errorf("result = %+v, want an authored agreement on MINOR", res)
	}
}

func TestDisagreementIsReportedNotResolved(t *testing.T) {
	res, err := Exchange(context.Background(), request(t, &stub{out: authored("MAJOR")}))
	if err != nil {
		t.Fatal(err)
	}
	if !res.Disagrees || *res.Proposed != releasemodel.BumpMajor || res.Computed != releasemodel.BumpMinor {
		t.Errorf("result = %+v, want a disagreement between MAJOR proposed and MINOR computed", res)
	}
	if *res.Corpus.ProposedBump != releasemodel.BumpMajor {
		t.Error("the bridge changed the proposal")
	}
}

// AC: AC-CLOG-003.6
func TestReturnedProseIsCarriedByteForByte(t *testing.T) {
	out := authored("MINOR")
	req := request(t, &stub{out: out})
	res, err := Exchange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := *res.Corpus.Title, "Résumé-safe <targets> & “quotes” 🚀"; got != want {
		t.Errorf("title = %q, want %q", got, want)
	}
	if got, want := *res.Corpus.Summary, "Line one\n\n- **bold** item\n- `code` item\t(tab)  "; got != want {
		t.Errorf("summary = %q, want %q", got, want)
	}
	e := res.Corpus.Entries[0]
	if e.Summary != "Compute for any line — naïvely" || *e.Detail != "Uses é and \\u escapes" {
		t.Errorf("entry = %q / %q", e.Summary, *e.Detail)
	}
	cached, err := os.ReadFile(req.CachePath)
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(cached, out) {
		t.Errorf("the cache holds\n%s\nwant the producer's bytes\n%s", cached, out)
	}

	// Read back from the cache: the same prose again.
	again, err := Exchange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if *again.Corpus.Summary != *res.Corpus.Summary || *again.Corpus.Title != *res.Corpus.Title {
		t.Error("the cached corpus decodes to different prose")
	}
}

func TestAssembledCorpusReachesTheProducerAsJSON(t *testing.T) {
	p := &stub{out: authored("MINOR")}
	if _, err := Exchange(context.Background(), request(t, p)); err != nil {
		t.Fatal(err)
	}
	want, err := Marshal(assembled())
	if err != nil {
		t.Fatal(err)
	}
	if len(p.received) != 1 || !bytes.Equal(p.received[0], want) {
		t.Errorf("the producer received %q, want %q", p.received, want)
	}
	if bytes.Contains(want, []byte("proposed_bump")) {
		t.Error("the assembled corpus sent to the producer carries proposed_bump")
	}
}

// AC: AC-CLOG-003.7
func TestTheBridgeHasNoNetworkPath(t *testing.T) {
	if _, err := exec.LookPath("go"); err != nil {
		t.Skip("go not on PATH")
	}
	out, err := exec.Command("go", "list", "-deps", ".").Output()
	if err != nil {
		t.Fatalf("go list -deps: %v", err)
	}
	for _, dep := range strings.Fields(string(out)) {
		switch dep {
		case "net", "net/http", "crypto/tls", "net/rpc", "net/smtp":
			t.Errorf("the bridge depends on %s, a network package; it must not reach a model service itself", dep)
		}
	}
	// The exchange is the producer's: a stub producer is the only thing the
	// bridge calls, and it is called with the corpus alone.
	p := &stub{out: authored("MINOR")}
	if _, err := Exchange(context.Background(), request(t, p)); err != nil {
		t.Fatal(err)
	}
	if len(p.received) != 1 {
		t.Errorf("the producer was invoked %d times, want once", len(p.received))
	}
}

// AC: AC-CLOG-003.8
func TestTheConfiguredProducerIsTheOneInvoked(t *testing.T) {
	chosen, other := &stub{out: authored("MINOR")}, &stub{out: authored("MINOR")}
	req := request(t, chosen)
	req.Producers = map[string]Producer{"command": chosen, "claude": other}
	req.Producer = "command"
	if _, err := Exchange(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	if len(chosen.received) != 1 || len(other.received) != 0 {
		t.Errorf("invocations: configured %d, other %d; want 1, 0", len(chosen.received), len(other.received))
	}

	req = request(t, chosen)
	req.Producer = "manual"
	if _, err := Exchange(context.Background(), req); !errors.Is(err, ErrUnknownProducer) {
		t.Errorf("unknown producer: err = %v, want ErrUnknownProducer", err)
	}
}

// AC: AC-CLOG-003.9
func TestACachedCorpusIsReusedWithoutInvokingAProducer(t *testing.T) {
	p := &stub{out: authored("MINOR")}
	req := request(t, p)
	first, err := Exchange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if first.Outcome != Authored {
		t.Fatalf("first outcome = %v, want Authored", first.Outcome)
	}
	p.out, p.err = nil, errors.New("must not be called")
	second, err := Exchange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if second.Outcome != Cached || len(p.received) != 1 {
		t.Errorf("second outcome = %v after %d invocations; want Cached after 1", second.Outcome, len(p.received))
	}
}

func TestRedoReplacesTheCache(t *testing.T) {
	p := &stub{out: authored("MINOR")}
	req := request(t, p)
	if _, err := Exchange(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	p.out = authored("PATCH")
	req.Redo = true
	res, err := Exchange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Authored || len(p.received) != 2 || *res.Proposed != releasemodel.BumpPatch {
		t.Errorf("redo: outcome %v, %d invocations, proposed %v", res.Outcome, len(p.received), *res.Proposed)
	}
	cached, _ := os.ReadFile(req.CachePath)
	if !bytes.Equal(cached, authored("PATCH")) {
		t.Error("redo did not replace the cached corpus")
	}
}

func TestRedoKeepsTheCacheWhenTheProducerFails(t *testing.T) {
	p := &stub{out: authored("MINOR")}
	req := request(t, p)
	if _, err := Exchange(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	p.out, p.err = nil, errors.New("producer exploded")
	req.Redo = true
	if _, err := Exchange(context.Background(), req); err == nil || !strings.Contains(err.Error(), "producer exploded") {
		t.Errorf("err = %v, want the producer's failure", err)
	}
	cached, _ := os.ReadFile(req.CachePath)
	if !bytes.Equal(cached, authored("MINOR")) {
		t.Error("a failed redo replaced the cached corpus")
	}
}

func TestSkipInvokesNothingAndPassesTheAssembledCorpus(t *testing.T) {
	p := &stub{err: errors.New("must not be called")}
	req := request(t, p)
	if err := os.MkdirAll(filepath.Dir(req.CachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(req.CachePath, []byte("not even JSON"), 0o644); err != nil {
		t.Fatal(err)
	}
	req.Skip = true
	res, err := Exchange(context.Background(), req)
	if err != nil {
		t.Fatal(err)
	}
	if res.Outcome != Skipped || len(p.received) != 0 || res.Proposed != nil || res.Disagrees {
		t.Errorf("skip: %+v after %d invocations", res, len(p.received))
	}
	if res.Corpus.Entries[0].Summary != "add --target" {
		t.Errorf("skip changed the assembled corpus: %+v", res.Corpus)
	}
}

func TestAnInvalidCachedCorpusIsRejected(t *testing.T) {
	p := &stub{err: errors.New("must not be called")}
	req := request(t, p)
	if err := os.MkdirAll(filepath.Dir(req.CachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(req.CachePath, []byte(`{"schema_version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := Exchange(context.Background(), req)
	var rej *Rejection
	if !errors.As(err, &rej) || rej.Source != req.CachePath {
		t.Errorf("err = %v, want a rejection naming the cache file", err)
	}
}

func TestCachePathKeysOnTheRangeInTheCommonDir(t *testing.T) {
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
	got, err := CachePath(context.Background(), repo, "", head)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(common, "release", "corpus", "root.."+head+".json"); got != want {
		t.Errorf("CachePath = %q, want %q", got, want)
	}
	got, _ = CachePath(context.Background(), repo, "abc", head)
	if filepath.Base(got) != "abc.."+head+".json" {
		t.Errorf("CachePath with a base = %q", got)
	}
}

func TestAlteredProvenanceIsRejected(t *testing.T) {
	other := strings.Replace(string(authored("MINOR")), `"to": "`+head+`"`, `"to": "2222222222222222222222222222222222222222"`, 1)
	dropped := strings.Replace(string(authored("MINOR")), `[{"adapter": "COMMITS", "to": "`+head+`"}]`, `[]`, 1)
	for name, doc := range map[string]string{"another range": other, "dropped": dropped} {
		t.Run(name, func(t *testing.T) {
			if doc == string(authored("MINOR")) {
				t.Fatal("the replacement did not apply")
			}
			req := request(t, &stub{out: []byte(doc)})
			_, err := Exchange(context.Background(), req)
			var rej *Rejection
			if !errors.As(err, &rej) || !strings.Contains(rej.Reason, "sources") {
				t.Fatalf("producer: err = %v, want a rejection naming sources", err)
			}
			if _, err := os.Stat(req.CachePath); !errors.Is(err, os.ErrNotExist) {
				t.Error("a corpus with altered provenance was cached")
			}

			// The same document at the cache path is refused too.
			if err := os.MkdirAll(filepath.Dir(req.CachePath), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(req.CachePath, []byte(doc), 0o644); err != nil {
				t.Fatal(err)
			}
			req.Producers = map[string]Producer{"stub": &stub{err: errors.New("must not be called")}}
			if _, err := Exchange(context.Background(), req); !errors.As(err, &rej) {
				t.Errorf("cache: err = %v, want a rejection", err)
			}
		})
	}
}

func TestCacheWriteLeavesNoTemporaryFiles(t *testing.T) {
	req := request(t, &stub{out: authored("MINOR")})
	for i := 0; i < 2; i++ {
		req.Redo = i == 1
		if _, err := Exchange(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(req.CachePath))
	if err != nil {
		t.Fatal(err)
	}
	if len(entries) != 1 || entries[0].Name() != filepath.Base(req.CachePath) {
		var names []string
		for _, e := range entries {
			names = append(names, e.Name())
		}
		t.Errorf("cache directory holds %q, want only the corpus", names)
	}
}
