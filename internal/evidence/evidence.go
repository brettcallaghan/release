// Package evidence is the Evidence Builder: commit records, diffs and
// classifier signals joined into the evidence bundle a changelog producer
// authors a release from. It is a pure function of its inputs. It reads no
// git, no file and no network, and it authors, summarises and classifies
// nothing: every value in the bundle is one the collectors or the classifier
// produced, copied through unchanged.
package evidence

import (
	"errors"
	"fmt"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

// SchemaVersion is the evidence schema version a bundle declares.
const SchemaVersion = "1.0.0"

// Range is the commit range the bundle covers.
type Range struct {
	// From is the base commit's full hash. Empty for a release line with no
	// release yet, whose range starts at its root.
	From string
	// To is the head commit's full hash.
	To string
}

// Input is everything the builder joins.
type Input struct {
	// Range is the commit range the records were read from.
	Range Range
	// BaseVersion is the version the range starts from, without a tag prefix.
	// Empty for a release line with no release yet.
	BaseVersion string
	// Commits are the commit records in the range, newest first, in the order
	// the Commit Collector lists them. The bundle lists them oldest first.
	Commits []releasemodel.CommitRecord
	// DiffsCollected reports whether the Diff Collector ran. When it did,
	// every commit carries its diff, and the DIFFS adapter is recorded.
	DiffsCollected bool
	// Diffs are the per-commit diffs within the diff budget. Read only when
	// DiffsCollected is set; each names a commit in the range.
	Diffs []releasemodel.DiffRecord
	// Budget is the diff budget report. When DiffsCollected is false only its
	// Limit is read, and the bundle reports the zero totals against it.
	Budget releasemodel.BudgetReport
	// Signals maps a commit hash to the bump the classifier read from it. A
	// commit absent from the map is unclassified. Each key names a commit in
	// the range.
	Signals map[string]releasemodel.Bump
}

// Build joins the inputs into an evidence bundle. Commits are listed oldest
// first, and every commit in the range is present whether or not it carries
// pull request information, a diff or a signal.
//
// It returns an error for a range with no head, a commit listed twice, and a
// diff or signal naming a commit outside the range.
func Build(in Input) (releasemodel.Evidence, error) {
	if in.Range.To == "" {
		return releasemodel.Evidence{}, errors.New("the evidence range has no head commit")
	}
	if !in.DiffsCollected && len(in.Diffs) > 0 {
		return releasemodel.Evidence{}, errors.New("diffs were given but the diff collector did not run")
	}

	inRange := make(map[string]bool, len(in.Commits))
	commits := make([]releasemodel.CommitEvidence, 0, len(in.Commits))
	for i := len(in.Commits) - 1; i >= 0; i-- {
		r := in.Commits[i]
		if inRange[r.Hash] {
			return releasemodel.Evidence{}, fmt.Errorf("commit %s is listed twice", r.Hash)
		}
		inRange[r.Hash] = true
		normalise(&r)
		commits = append(commits, releasemodel.CommitEvidence{
			Record:       r,
			Conventional: r.Conventional,
			PullRequest:  r.PullRequest,
		})
	}

	diffs := make(map[string]releasemodel.DiffRecord, len(in.Diffs))
	for _, d := range in.Diffs {
		if !inRange[d.Commit] {
			return releasemodel.Evidence{}, fmt.Errorf("the diff for commit %s names no commit in the range", d.Commit)
		}
		if _, dup := diffs[d.Commit]; dup {
			return releasemodel.Evidence{}, fmt.Errorf("commit %s has two diffs", d.Commit)
		}
		if d.Files == nil {
			d.Files = []releasemodel.FileDiff{}
		}
		diffs[d.Commit] = d
	}

	for hash := range in.Signals {
		if !inRange[hash] {
			return releasemodel.Evidence{}, fmt.Errorf("the signal for commit %s names no commit in the range", hash)
		}
	}

	for i := range commits {
		hash := commits[i].Record.Hash
		if d, ok := diffs[hash]; ok {
			commits[i].Diff = &d
		}
		if s, ok := in.Signals[hash]; ok {
			signal := s
			commits[i].Signal = &signal
		}
	}

	from := optional(in.Range.From)
	sources := []releasemodel.AdapterRun{{
		Adapter: releasemodel.AdapterCommits,
		From:    from,
		To:      in.Range.To,
	}}
	budget := releasemodel.BudgetReport{Limit: in.Budget.Limit, Omitted: []releasemodel.OmittedDiff{}}
	if in.DiffsCollected {
		budget = in.Budget
		if budget.Omitted == nil {
			budget.Omitted = []releasemodel.OmittedDiff{}
		}
		diffBudget := budget
		sources = append(sources, releasemodel.AdapterRun{
			Adapter: releasemodel.AdapterDiffs,
			From:    from,
			To:      in.Range.To,
			Budget:  &diffBudget,
		})
	}

	return releasemodel.Evidence{
		SchemaVersion: SchemaVersion,
		Purpose:       releasemodel.EvidencePurposeRelease,
		Range: releasemodel.Range{
			From: from,
			To:   in.Range.To,
		},
		BaseVersion: optional(in.BaseVersion),
		Commits:     commits,
		Sources:     sources,
		Budget:      budget,
	}, nil
}

// normalise replaces the nil collections a record may carry with empty ones,
// so each serialises as an array the schema requires rather than null. No
// value is changed.
func normalise(r *releasemodel.CommitRecord) {
	if r.Files == nil {
		r.Files = []releasemodel.FileChange{}
	}
	if r.Trailers == nil {
		r.Trailers = map[string]string{}
	}
}

// optional returns a pointer to s, or nil WHEN s is empty.
func optional(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
