// Package producertest is the suite every changelog producer runs: the
// guarantees of the Changelog Producer component that hold for any evidence
// bundle, whoever authors the changelog.
package producertest

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/schemacheck"
)

const (
	base = "0000000000000000000000000000000000000000"
	head = "3333333333333333333333333333333333333333"
)

// Commit builds the evidence of one commit.
func Commit(hash, subject, body string, pr int64) releasemodel.CommitEvidence {
	r := releasemodel.CommitRecord{Hash: hash, Subject: subject, Body: body, Trailers: map[string]string{}, Files: []releasemodel.FileChange{}}
	if pr > 0 {
		r.PullRequest = &releasemodel.PullRequestInfo{Number: pr, Origin: releasemodel.PullRequestOriginSquash}
	}
	return releasemodel.CommitEvidence{Record: r}
}

// Bundle builds an evidence bundle over commits, oldest first.
func Bundle(commits ...releasemodel.CommitEvidence) []byte {
	from := base
	if commits == nil {
		commits = []releasemodel.CommitEvidence{}
	}
	ev := releasemodel.Evidence{
		SchemaVersion: "1.0.0",
		Purpose:       releasemodel.EvidencePurposeRelease,
		Range:         releasemodel.Range{From: &from, To: head},
		Commits:       commits,
		Sources:       []releasemodel.AdapterRun{{Adapter: releasemodel.AdapterCommits, From: &from, To: head}},
		Budget:        releasemodel.BudgetReport{Omitted: []releasemodel.OmittedDiff{}},
	}
	data, err := json.Marshal(ev)
	if err != nil {
		panic(err)
	}
	return data
}

// Run runs the suite against p. Every case builds a bundle that itself
// validates, so a failure is the producer's.
func Run(t *testing.T, p authoring.Producer) {
	t.Helper()
	produce := func(t *testing.T, bundle []byte) (releasemodel.Changelog, []byte) {
		t.Helper()
		if err := schemacheck.Evidence(bundle); err != nil {
			t.Fatalf("the conformance bundle does not validate: %v\n%s", err, bundle)
		}
		out, err := p.Produce(context.Background(), bundle)
		if err != nil {
			t.Fatalf("Produce: %v", err)
		}
		if err := schemacheck.Changelog(out); err != nil {
			t.Fatalf("the changelog does not validate: %v\n%s", err, out)
		}
		var c releasemodel.Changelog
		if err := json.Unmarshal(out, &c); err != nil {
			t.Fatalf("the changelog does not decode: %v", err)
		}
		if c.Version != nil {
			t.Errorf("version = %q, want it unset", *c.Version)
		}
		return c, out
	}

	t.Run("empty bundle gives no entries and nothing omitted", func(t *testing.T) {
		c, out := produce(t, Bundle())
		if len(c.Entries) != 0 {
			t.Errorf("entries = %d, want 0\n%s", len(c.Entries), out)
		}
		if c.Omitted != nil && len(*c.Omitted) != 0 {
			t.Errorf("omitted = %v, want none", *c.Omitted)
		}
	})

	t.Run("every commit is cited or omitted", func(t *testing.T) {
		hashes := []string{"a1", "b2", "c3", "d4"}
		c, out := produce(t, Bundle(
			Commit(hashes[0], "feat(cli): add --dry-run (#4)", "", 4),
			Commit(hashes[1], "fix!: reject empty names", "Body.\n\nBREAKING CHANGE: names are required", 0),
			Commit(hashes[2], "tidy up the readme", "", 0),
			Commit(hashes[3], "chore: bump deps", "", 0),
		))
		accounted := map[string]bool{}
		for _, e := range c.Entries {
			for _, s := range e.Sources {
				if cs, ok := s.Value.(releasemodel.CommitSource); ok {
					accounted[cs.Hash] = true
				}
			}
		}
		if c.Omitted != nil {
			for _, o := range *c.Omitted {
				accounted[o.Hash] = true
			}
		}
		for _, h := range hashes {
			if !accounted[h] {
				t.Errorf("commit %s is neither cited nor omitted\n%s", h, out)
			}
		}
	})

	t.Run("every breaking entry carries a migration", func(t *testing.T) {
		c, out := produce(t, Bundle(Commit("a1", "feat!: drop the flag", "", 0)))
		for _, e := range c.Entries {
			if e.Breaking && (e.Migration == nil || *e.Migration == "") {
				t.Errorf("breaking entry %q has no migration\n%s", e.Summary, out)
			}
		}
	})
}
