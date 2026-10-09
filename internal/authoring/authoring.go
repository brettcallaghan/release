// Package authoring is the Changelog Authoring Bridge: it hands an evidence
// bundle to the producer configuration names, validates the structured
// changelog that comes back against the schema and the bundle before
// anything reads it, and caches it against the commit range it describes.
//
// It holds no language-model client, credential or network dependency. A
// producer is known here only by its name, and returned prose is carried
// byte for byte: the cache holds the producer's own output, unedited.
package authoring

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"
)

// SchemaMajor is the major schema_version this bridge implements.
const SchemaMajor = 1

// Producer authors a changelog: it receives the evidence bundle as JSON and
// returns the structured changelog as JSON.
type Producer interface {
	Produce(ctx context.Context, evidence []byte) ([]byte, error)
}

// Rejection is a changelog the bridge refused before anything read it.
type Rejection struct {
	// Source says where the changelog came from: the producer, or a file.
	Source string
	Reason string
}

func (r *Rejection) Error() string {
	return fmt.Sprintf("the changelog from %s is rejected: %s", r.Source, r.Reason)
}

// ErrUnknownProducer is the refusal for a producer name nothing registers.
var ErrUnknownProducer = errors.New("no changelog producer has that name")

// writeAtomically replaces path with data only once data is fully written,
// so a failed write never destroys the file already there.
func writeAtomically(path string, data []byte) error {
	dir := filepath.Dir(path)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return err
	}
	tmp, err := os.CreateTemp(dir, filepath.Base(path)+".*.tmp")
	if err != nil {
		return err
	}
	defer os.Remove(tmp.Name())
	if _, err := tmp.Write(data); err != nil {
		tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	if err := os.Chmod(tmp.Name(), 0o644); err != nil {
		return err
	}
	return os.Rename(tmp.Name(), path)
}

func names(m map[string]Producer) string {
	out := make([]string, 0, len(m))
	for n := range m {
		out = append(out, n)
	}
	sort.Strings(out)
	return strings.Join(out, ", ")
}

// excerpt is the start of a document, for a rejection to quote.
func excerpt(data []byte) string {
	s := strings.TrimSpace(string(data))
	if r := []rune(s); len(r) > 60 {
		return string(r[:60]) + "…"
	}
	return s
}
