// Package authoring is the Changelog Authoring Bridge: it hands an assembled
// corpus to the producer configuration names, validates what comes back
// before anything reads it, compares its proposed impact with the computed
// one, and caches it against the commit range it describes.
//
// It holds no language-model client, credential or network dependency. A
// producer is known here only by its name, and returned prose is carried
// byte for byte: the cache holds the producer's own output, unedited.
package authoring

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/schemacheck"
	"github.com/brettcallaghan/release/internal/semver"
)

// SchemaMajor is the major schema_version this bridge implements.
const SchemaMajor = 1

// Producer authors a corpus: it receives the assembled corpus as JSON and
// returns the authored corpus as JSON.
type Producer interface {
	Produce(ctx context.Context, assembled []byte) ([]byte, error)
}

// Request is one authoring exchange.
type Request struct {
	// Assembled is the corpus the builder assembled.
	Assembled releasemodel.Changelog
	// Computed is the impact the tooling computed for the range.
	Computed releasemodel.Bump
	// Producer is the configured producer name; Producers maps every name a
	// configuration may select to its producer.
	Producer  string
	Producers map[string]Producer
	// CachePath is where the authored corpus for this range is cached:
	// CachePath's answer, or a path the maintainer named.
	CachePath string
	// Redo ignores and replaces a cached corpus.
	Redo bool
	// Skip invokes no producer and reads no cache: the assembled corpus
	// proceeds unchanged.
	Skip bool
}

// Outcome is how the exchange concluded.
type Outcome int

const (
	// Skipped means no exchange ran; Corpus is the assembled corpus.
	Skipped Outcome = iota
	// Cached means the corpus was read from CachePath; no producer ran.
	Cached
	// Authored means the producer ran and its corpus was cached.
	Authored
)

// Result is an accepted corpus and how its proposal compares.
type Result struct {
	Outcome Outcome
	// Corpus is the accepted corpus.
	Corpus releasemodel.Changelog
	// Proposed is the corpus's proposed impact; nil WHEN Skipped.
	Proposed *releasemodel.Bump
	// Computed is the request's computed impact.
	Computed releasemodel.Bump
	// Disagrees is true WHEN Proposed differs from Computed. The bridge
	// reports a disagreement and never resolves it.
	Disagrees bool
}

// Rejection is a corpus the bridge refused before anything read it.
type Rejection struct {
	// Source says where the corpus came from: the producer, or a file.
	Source string
	Reason string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("the corpus from %s is rejected: %s", r.Source, r.Reason)
}

// ErrUnknownProducer is the refusal for a producer name nothing registers.
var ErrUnknownProducer = errors.New("no changelog producer has that name")

// Exchange performs one authoring exchange.
func Exchange(ctx context.Context, req Request) (Result, error) {
	if req.Skip {
		return Result{Outcome: Skipped, Corpus: req.Assembled, Computed: req.Computed}, nil
	}
	if !req.Redo {
		data, err := os.ReadFile(req.CachePath)
		switch {
		case err == nil:
			c, err := Accept(data, req.CachePath)
			if err != nil {
				return Result{}, err
			}
			return compare(Cached, c, req.Computed), nil
		case !errors.Is(err, os.ErrNotExist):
			return Result{}, fmt.Errorf("reading the cached corpus: %w", err)
		}
	}

	p, ok := req.Producers[req.Producer]
	if !ok {
		return Result{}, fmt.Errorf("%w: %q (want one of %s)", ErrUnknownProducer, req.Producer, names(req.Producers))
	}
	in, err := Marshal(req.Assembled)
	if err != nil {
		return Result{}, err
	}
	out, err := p.Produce(ctx, in)
	if err != nil {
		return Result{}, fmt.Errorf("the %s producer failed: %w", req.Producer, err)
	}
	c, err := Accept(out, "the "+req.Producer+" producer")
	if err != nil {
		return Result{}, err
	}
	if err := os.MkdirAll(filepath.Dir(req.CachePath), 0o755); err != nil {
		return Result{}, fmt.Errorf("caching the authored corpus: %w", err)
	}
	if err := os.WriteFile(req.CachePath, out, 0o644); err != nil {
		return Result{}, fmt.Errorf("caching the authored corpus: %w", err)
	}
	return compare(Authored, c, req.Computed), nil
}

func compare(o Outcome, c releasemodel.Changelog, computed releasemodel.Bump) Result {
	return Result{
		Outcome:   o,
		Corpus:    c,
		Proposed:  c.ProposedBump,
		Computed:  computed,
		Disagrees: *c.ProposedBump != computed,
	}
}

// Accept validates an authored corpus and decodes it. It rejects, as a
// *Rejection, a document declaring a schema_version whose major this bridge
// does not implement, one that does not validate against the changelog
// schema, and one carrying no proposed_bump. source names where the
// document came from.
func Accept(data []byte, source string) (releasemodel.Changelog, error) {
	reject := func(format string, args ...any) (releasemodel.Changelog, error) {
		return releasemodel.Changelog{}, &Rejection{Source: source, Reason: fmt.Sprintf(format, args...)}
	}
	var probe struct {
		SchemaVersion *string `json:"schema_version"`
	}
	if err := json.Unmarshal(data, &probe); err != nil {
		return reject("it is not a JSON object: %v", err)
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
	if c.ProposedBump == nil {
		return reject("it carries no proposed_bump; a producer must propose an impact")
	}
	return c, nil
}

// Marshal renders a corpus as the JSON a producer receives: indented, and
// without escaping HTML characters, so prose reads as written.
func Marshal(c releasemodel.Changelog) ([]byte, error) {
	var buf bytes.Buffer
	enc := json.NewEncoder(&buf)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	if err := enc.Encode(c); err != nil {
		return nil, fmt.Errorf("encoding the corpus: %w", err)
	}
	return buf.Bytes(), nil
}

// CachePath is the cache file for a commit range in the repository at repo:
// <git common dir>/release/corpus/<from>..<to>.json, where from is "root"
// for a line with no release. The common directory is shared by every
// worktree, so a corpus authored in one is found from another.
func CachePath(ctx context.Context, repo gitread.Repo, from, to string) (string, error) {
	dir, err := repo.CommonDir(ctx)
	if err != nil {
		return "", err
	}
	if from == "" {
		from = "root"
	}
	return filepath.Join(dir, "release", "corpus", from+".."+to+".json"), nil
}

func names(m map[string]Producer) string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}
