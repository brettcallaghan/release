package producers

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"strings"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// MechanicalSchemaVersion is the changelog schema version Mechanical writes.
const MechanicalSchemaVersion = "1.0.0"

// noMigration is the migration of a breaking commit whose body carries no
// BREAKING CHANGE: footer.
const noMigration = "This commit is marked breaking and names no migration."

// Mechanical is the producer of last resort: a pure, in-process function of
// the evidence bundle that derives one entry per commit from its subject. It
// holds no model, reads no diff and does no I/O.
type Mechanical struct{}

// Produce reads an evidence bundle and returns the changelog as JSON.
func (Mechanical) Produce(_ context.Context, evidence []byte) ([]byte, error) {
	var ev releasemodel.Evidence
	if err := json.Unmarshal(evidence, &ev); err != nil {
		return nil, fmt.Errorf("the mechanical producer cannot read the evidence bundle: %w", err)
	}
	entries := make([]releasemodel.ChangelogEntry, 0, len(ev.Commits))
	for _, c := range ev.Commits {
		entries = append(entries, mechanicalEntry(c))
	}
	sources := ev.Sources
	if sources == nil {
		sources = []releasemodel.AdapterRun{}
	}
	return json.Marshal(releasemodel.Changelog{
		SchemaVersion: MechanicalSchemaVersion,
		Entries:       entries,
		Sources:       sources,
	})
}

// subjectPrefix is "type(scope)!: " at the head of a conventional subject.
var subjectPrefix = regexp.MustCompile(`^[A-Za-z]+(?:\([^()]*\))?!?: (.*)$`)

// pullRequestSuffix is the reference a hosting platform appends to a squash
// commit subject.
var pullRequestSuffix = regexp.MustCompile(` \(#[0-9]+\)$`)

// kinds maps a conventional-commit type, lower-cased, to its change kind.
// Any other type is a CHORE; no type means no kind.
var kinds = map[string]releasemodel.ChangeKind{
	"feat":     releasemodel.ChangeKindFeature,
	"fix":      releasemodel.ChangeKindFix,
	"perf":     releasemodel.ChangeKindPerformance,
	"revert":   releasemodel.ChangeKindRevert,
	"refactor": releasemodel.ChangeKindRefactor,
	"docs":     releasemodel.ChangeKindDocumentation,
	"build":    releasemodel.ChangeKindBuild,
	"ci":       releasemodel.ChangeKindCi,
	"test":     releasemodel.ChangeKindTest,
	"style":    releasemodel.ChangeKindStyle,
}

func mechanicalEntry(c releasemodel.CommitEvidence) releasemodel.ChangelogEntry {
	r := c.Record
	conv := c.Conventional
	if conv == nil {
		conv = r.Conventional
	}
	subject := pullRequestSuffix.ReplaceAllString(r.Subject, "")
	e := releasemodel.ChangelogEntry{
		Summary:  subject,
		Breaking: bump.Breaking(r) || (conv != nil && conv.Breaking),
		Derived:  true,
	}
	if conv != nil {
		kind, ok := kinds[strings.ToLower(conv.Type)]
		if !ok {
			kind = releasemodel.ChangeKindChore
		}
		e.Kind = &kind
		if conv.Scope != nil {
			if scope := strings.TrimSpace(*conv.Scope); scope != "" {
				e.Scope = &scope
			}
		}
		if m := subjectPrefix.FindStringSubmatch(subject); m != nil {
			if desc := strings.TrimSpace(m[1]); desc != "" {
				e.Summary = desc
			}
		}
	}
	if e.Breaking {
		note, ok := bump.BreakingNote(r)
		if !ok {
			note = noMigration
		}
		e.Migration = &note
	}
	e.Sources = []releasemodel.ChangeSource{releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: r.Hash})}
	pr := r.PullRequest
	if pr == nil {
		pr = c.PullRequest
	}
	if pr != nil {
		e.Sources = append(e.Sources, releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: pr.Number}))
	}
	return e
}
