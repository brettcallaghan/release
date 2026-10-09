package authoring

import (
	"bytes"
	"context"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

// stub is a producer that answers with a fixed document and records each
// bundle it received.
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

func TestRedoKeepsTheCacheWhenTheProducerFails(t *testing.T) {
	p := &stub{}
	req := evRequest(t, p)
	p.out = encode(t, changelogFor(req.Evidence))
	if _, err := ExchangeEvidence(context.Background(), req); err != nil {
		t.Fatal(err)
	}
	cached := p.out
	p.out, p.err = nil, errors.New("producer exploded")
	req.Redo = true
	if _, err := ExchangeEvidence(context.Background(), req); err == nil || !strings.Contains(err.Error(), "producer exploded") {
		t.Errorf("err = %v, want the producer's failure", err)
	}
	if now, _ := os.ReadFile(req.CachePath); !bytes.Equal(now, cached) {
		t.Error("a failed redo replaced the cached changelog")
	}
}

func TestAnInvalidCachedChangelogIsRejected(t *testing.T) {
	p := &stub{err: errors.New("must not be called")}
	req := evRequest(t, p)
	if err := os.MkdirAll(filepath.Dir(req.CachePath), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(req.CachePath, []byte(`{"schema_version":"1.0.0"}`), 0o644); err != nil {
		t.Fatal(err)
	}
	_, err := ExchangeEvidence(context.Background(), req)
	var rej *Rejection
	if !errors.As(err, &rej) || rej.Source != req.CachePath || len(p.received) != 0 {
		t.Errorf("err = %v after %d invocations, want a rejection naming the cache file", err, len(p.received))
	}
}

func TestCacheWriteLeavesNoTemporaryFiles(t *testing.T) {
	p := &stub{}
	req := evRequest(t, p)
	p.out = encode(t, changelogFor(req.Evidence))
	for i := 0; i < 2; i++ {
		req.Redo = i == 1
		if _, err := ExchangeEvidence(context.Background(), req); err != nil {
			t.Fatal(err)
		}
	}
	entries, err := os.ReadDir(filepath.Dir(req.CachePath))
	if err != nil {
		t.Fatal(err)
	}
	var names []string
	for _, e := range entries {
		names = append(names, e.Name())
	}
	want := []string{filepath.Base(EvidencePath(req.CachePath)), filepath.Base(req.CachePath)}
	if strings.Join(names, ",") != strings.Join(want, ",") {
		t.Errorf("cache directory holds %q, want only %q", names, want)
	}
}

// kp returns a pointer to an entry kind.
func kp(k releasemodel.ChangeKind) *releasemodel.ChangeKind { return &k }
