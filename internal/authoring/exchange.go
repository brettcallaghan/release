package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"unicode"

	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/schemacheck"
	"github.com/brettcallaghan/release/internal/semver"
)

// Obtained says how an accepted changelog came to be.
type Obtained int

const (
	// Mechanical means --skip-notes: the mechanical producer composed it, and
	// no cache was read or written.
	Mechanical Obtained = iota
	// FromCache means it was read from the cache; no producer ran.
	FromCache
	// ByProducer means the configured producer authored it and it was cached.
	ByProducer
)

// EvidenceRequest is one exchange of an evidence bundle for a changelog.
type EvidenceRequest struct {
	// Evidence is the bundle the producer is given.
	Evidence releasemodel.Evidence
	// Producer is the configured producer name; Producers maps every name a
	// configuration may select to its producer.
	Producer  string
	Producers map[string]Producer
	// CachePath is where the authored changelog for this range is cached.
	CachePath string
	// Redo ignores a cached changelog and replaces it once the producer's
	// answer is accepted.
	Redo bool
	// Skip exchanges the bundle with the mechanical producer in place of the
	// configured one, and reads and writes no cache.
	Skip bool
}

// EvidenceResult is an accepted changelog and where it came from.
type EvidenceResult struct {
	Changelog releasemodel.Changelog
	// Producer names the producer that authored it.
	Producer string
	Obtained Obtained
}

// ProducerFailure is the configured producer failing or being unavailable.
// The exchange stops: no other producer is tried.
type ProducerFailure struct {
	Producer string
	Err      error
	// Skippable is false WHEN Err already names --skip-notes.
	Skippable bool
}

func (f *ProducerFailure) Error() string {
	msg := fmt.Sprintf("the %s producer failed: %v", f.Producer, f.Err)
	if f.Skippable {
		msg += "; pass --skip-notes to author the changelog with the mechanical producer"
	}
	return msg
}

func (f *ProducerFailure) Unwrap() error { return f.Err }

// ExchangeEvidence exchanges the bundle for a changelog and validates it
// against the bundle. A producer that fails or is unavailable stops the
// exchange; there is no fallback.
func ExchangeEvidence(ctx context.Context, req EvidenceRequest) (EvidenceResult, error) {
	name := req.Producer
	if req.Skip {
		name = config.ProducerMechanicalName
	}
	if !req.Skip && !req.Redo {
		data, err := os.ReadFile(req.CachePath)
		switch {
		case err == nil:
			c, err := acceptChangelog(data, req.CachePath, req.Evidence, true)
			if err != nil {
				return EvidenceResult{}, err
			}
			return EvidenceResult{Changelog: c, Producer: name, Obtained: FromCache}, nil
		case !errors.Is(err, os.ErrNotExist):
			return EvidenceResult{}, fmt.Errorf("reading the cached changelog: %w", err)
		}
	}
	p, ok := req.Producers[name]
	if !ok {
		return EvidenceResult{}, fmt.Errorf("%w: %q (want one of %s)", ErrUnknownProducer, name, names(req.Producers))
	}
	in, err := MarshalEvidence(req.Evidence)
	if err != nil {
		return EvidenceResult{}, err
	}
	out, err := p.Produce(ctx, in)
	if err != nil {
		if req.Skip {
			return EvidenceResult{}, fmt.Errorf("the mechanical producer failed: %w", err)
		}
		return EvidenceResult{}, &ProducerFailure{Producer: name, Err: err, Skippable: !strings.Contains(err.Error(), "--skip-notes")}
	}
	source := "the " + name + " producer"
	c, err := AcceptChangelog(out, source, req.Evidence)
	if err != nil {
		return EvidenceResult{}, err
	}
	if req.Skip {
		return EvidenceResult{Changelog: c, Producer: name, Obtained: Mechanical}, nil
	}
	if err := writeAtomically(EvidencePath(req.CachePath), in); err != nil {
		return EvidenceResult{}, fmt.Errorf("keeping the evidence bundle: %w", err)
	}
	if err := writeAtomically(req.CachePath, out); err != nil {
		return EvidenceResult{}, fmt.Errorf("caching the authored changelog: %w", err)
	}
	return EvidenceResult{Changelog: c, Producer: name, Obtained: ByProducer}, nil
}

// MarshalEvidence renders a bundle as the JSON a producer receives:
// indented, and without escaping HTML characters, so prose reads as written.
func MarshalEvidence(ev releasemodel.Evidence) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(ev); err != nil {
		return nil, fmt.Errorf("encoding the evidence bundle: %w", err)
	}
	return buf.Bytes(), nil
}

// ChangelogCachePath is the cache file for a commit range in the repository
// at repo: <git common dir>/release/changelog/<from>..<to>.json, where from
// is "root" for a line with no release. The common directory is shared by
// every worktree.
func ChangelogCachePath(ctx context.Context, repo gitread.Repo, from, to string) (string, error) {
	dir, err := repo.CommonDir(ctx)
	if err != nil {
		return "", err
	}
	if from == "" {
		from = "root"
	}
	return filepath.Join(dir, "release", "changelog", from+".."+to+".json"), nil
}

// EvidencePath is where the bundle is kept beside the changelog cached at
// cachePath: the same name with ".evidence.json" in place of ".json".
func EvidencePath(cachePath string) string {
	return strings.TrimSuffix(cachePath, ".json") + ".evidence.json"
}

// AcceptChangelog validates a changelog against the schema and against the
// bundle it was authored from, and decodes it. A shape failure is returned
// at once, as a *Rejection; otherwise the *Rejection names every rule the
// changelog breaks. source names where the document came from.
//
// The corpus-level sources must equal the bundle's byte for byte, which
// enforces a producer's copy-through. See acceptChangelog for the file-read
// case.
func AcceptChangelog(data []byte, source string, ev releasemodel.Evidence) (releasemodel.Changelog, error) {
	return acceptChangelog(data, source, ev, false)
}

// acceptChangelog is AcceptChangelog. rangeOnly is set for a changelog read
// from a file (the cache, or a named corpus): its sources record the evidence
// it was authored from, so only the commit range is compared with the bundle's.
func acceptChangelog(data []byte, source string, ev releasemodel.Evidence, rangeOnly bool) (releasemodel.Changelog, error) {
	reject := func(format string, args ...any) (releasemodel.Changelog, error) {
		return releasemodel.Changelog{}, &Rejection{Source: source, Reason: fmt.Sprintf(format, args...)}
	}
	var probe struct {
		SchemaVersion *string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return reject("it is not a JSON object (%v); it begins %q", err, excerpt(data))
	}
	if probe.SchemaVersion != nil {
		v, err := semver.Parse(*probe.SchemaVersion)
		if err != nil {
			return reject("its schema_version %q is not a version", *probe.SchemaVersion)
		}
		if v.Major != SchemaMajor {
			return reject("it declares schema_version %s; this release implements major %d", *probe.SchemaVersion, SchemaMajor)
		}
	}
	if err := schemacheck.Changelog(data); err != nil {
		return reject("%v", err)
	}
	var c releasemodel.Changelog
	if err := json.Unmarshal(data, &c); err != nil {
		return reject("%v", err)
	}
	if problems := check(c, ev, rangeOnly); len(problems) > 0 {
		return reject("%s", strings.Join(problems, "; "))
	}
	return c, nil
}

// check applies the validation rules that need the bundle and returns every
// rule the changelog breaks.
func check(c releasemodel.Changelog, ev releasemodel.Evidence, rangeOnly bool) []string {
	var problems []string
	got, _ := json.Marshal(c.Sources)
	want, _ := json.Marshal(ev.Sources)
	switch {
	case rangeOnly:
		if !sameRange(c.Sources, ev.Sources) {
			problems = append(problems, fmt.Sprintf("its sources %s record a range other than the bundle's %s; "+
				"a changelog for another range does not apply", got, want))
		}
	case !bytes.Equal(got, want):
		problems = append(problems, fmt.Sprintf("its sources %s are not the bundle's %s; "+
			"a producer carries them through unchanged, and a changelog for another range does not apply", got, want))
	}

	commits := map[string]bool{}
	pulls := map[int64]bool{}
	for _, ce := range ev.Commits {
		commits[ce.Record.Hash] = true
		if pr := ce.Record.PullRequest; pr != nil {
			pulls[pr.Number] = true
		}
		if pr := ce.PullRequest; pr != nil {
			pulls[pr.Number] = true
		}
	}

	cited := map[string]bool{}
	for i, e := range c.Entries {
		label := fmt.Sprintf("entry %d (%q)", i+1, excerpt([]byte(e.Summary)))
		entryCommits := false
		for _, s := range e.Sources {
			switch v := s.Value.(type) {
			case releasemodel.CommitSource:
				entryCommits = true
				cited[v.Hash] = true
				if !commits[v.Hash] {
					problems = append(problems, fmt.Sprintf("%s cites commit %s, which is not in the evidence bundle", label, v.Hash))
				}
			case releasemodel.PullRequestSource:
				if !pulls[v.Number] {
					problems = append(problems, fmt.Sprintf("%s cites pull request #%d, which no commit in the evidence bundle carries", label, v.Number))
				}
			}
		}
		if !entryCommits {
			problems = append(problems, label+" cites no commit")
		}
		switch summary := strings.TrimSpace(e.Summary); {
		case summary == "":
			problems = append(problems, label+" has an empty summary")
		case blankLine.MatchString(summary):
			problems = append(problems, label+" has a summary holding a blank line")
		case sentences(summary) > 2:
			problems = append(problems, fmt.Sprintf("%s has a summary of %d sentences; it must be one or two", label, sentences(summary)))
		}
		if e.Breaking && (e.Migration == nil || strings.TrimSpace(*e.Migration) == "") {
			problems = append(problems, label+" is breaking and carries no migration")
		}
	}

	omitted := map[string]bool{}
	if c.Omitted != nil {
		for _, o := range *c.Omitted {
			omitted[o.Hash] = true
			switch reason := strings.TrimSpace(o.Reason); {
			case !commits[o.Hash]:
				problems = append(problems, fmt.Sprintf("omitted commit %s is not in the evidence bundle", o.Hash))
			case reason == "":
				problems = append(problems, fmt.Sprintf("omitted commit %s has no reason", o.Hash))
			case strings.ContainsAny(reason, "\r\n"):
				problems = append(problems, fmt.Sprintf("omitted commit %s has a reason of more than one line", o.Hash))
			}
			if cited[o.Hash] {
				problems = append(problems, fmt.Sprintf("commit %s is both cited by an entry and omitted", o.Hash))
			}
		}
	}
	for _, ce := range ev.Commits {
		if h := ce.Record.Hash; !cited[h] && !omitted[h] {
			problems = append(problems, fmt.Sprintf("commit %s is neither cited by an entry nor omitted", h))
		}
	}
	return problems
}

// sameRange reports whether every run in a and b records one from and one to.
func sameRange(a, b []releasemodel.AdapterRun) bool {
	type span struct {
		from string
		set  bool
		to   string
	}
	seen := map[span]bool{}
	for _, runs := range [][]releasemodel.AdapterRun{a, b} {
		for _, r := range runs {
			sp := span{to: r.To}
			if r.From != nil {
				sp.from, sp.set = *r.From, true
			}
			seen[sp] = true
		}
	}
	return len(seen) <= 1
}

var blankLine = regexp.MustCompile(`\n[ \t\r]*\n`)

// sentences counts the sentence ends in s: each ".", "!" or "?" followed by
// whitespace and an uppercase letter, or by the end of s.
func sentences(s string) int {
	rs := []rune(strings.TrimSpace(s))
	n := 0
	for i, r := range rs {
		if r != '.' && r != '!' && r != '?' {
			continue
		}
		if i == len(rs)-1 {
			n++
			continue
		}
		j := i + 1
		for j < len(rs) && unicode.IsSpace(rs[j]) {
			j++
		}
		if j > i+1 && j < len(rs) && unicode.IsUpper(rs[j]) {
			n++
		}
	}
	return n
}
