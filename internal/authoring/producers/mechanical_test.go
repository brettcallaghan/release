package producers

import (
	"context"
	"encoding/json"
	"reflect"
	"testing"

	"github.com/brettcallaghan/release/internal/authoring/producers/producertest"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/schemacheck"
)

// withConv attaches the parsed conventional subject the tooling would.
func withConv(c releasemodel.CommitEvidence, typ string, scope *string, breaking bool) releasemodel.CommitEvidence {
	c.Conventional = &releasemodel.Conventional{Type: typ, Scope: scope, Breaking: breaking}
	return c
}

func mechanical(t *testing.T, commits ...releasemodel.CommitEvidence) releasemodel.Changelog {
	t.Helper()
	out, err := Mechanical{}.Produce(context.Background(), producertest.Bundle(commits...))
	if err != nil {
		t.Fatalf("Produce: %v", err)
	}
	if err := schemacheck.Changelog(out); err != nil {
		t.Fatalf("does not validate: %v\n%s", err, out)
	}
	var c releasemodel.Changelog
	if err := json.Unmarshal(out, &c); err != nil {
		t.Fatal(err)
	}
	return c
}

func scope(s string) *string { return &s }

// AC: AC-CLOG-004.10
func TestMechanicalPassesTheConformanceSuite(t *testing.T) {
	producertest.Run(t, Mechanical{})
}

// AC: AC-CLOG-004.10
func TestMechanicalMapsConventionalTypesToKinds(t *testing.T) {
	want := map[string]releasemodel.ChangeKind{
		"feat": releasemodel.ChangeKindFeature, "fix": releasemodel.ChangeKindFix,
		"perf": releasemodel.ChangeKindPerformance, "revert": releasemodel.ChangeKindRevert,
		"refactor": releasemodel.ChangeKindRefactor, "docs": releasemodel.ChangeKindDocumentation,
		"build": releasemodel.ChangeKindBuild, "ci": releasemodel.ChangeKindCi,
		"test": releasemodel.ChangeKindTest, "style": releasemodel.ChangeKindStyle,
		"chore": releasemodel.ChangeKindChore, "wibble": releasemodel.ChangeKindChore,
		"FEAT": releasemodel.ChangeKindFeature, "Fix": releasemodel.ChangeKindFix,
	}
	for typ, kind := range want {
		c := mechanical(t, withConv(producertest.Commit("a1", typ+": do it", "", 0), typ, nil, false))
		if len(c.Entries) != 1 || c.Entries[0].Kind == nil || *c.Entries[0].Kind != kind {
			t.Errorf("type %q: entries = %+v, want kind %s", typ, c.Entries, kind)
		}
		if got := c.Entries[0].Summary; got != "do it" {
			t.Errorf("type %q: summary = %q, want %q", typ, got, "do it")
		}
	}
}

// AC: AC-CLOG-004.14
func TestMechanicalGivesANonConventionalCommitNoKind(t *testing.T) {
	c := mechanical(t, producertest.Commit("a1", "Update the readme (#9)", "", 9))
	e := c.Entries[0]
	if e.Kind != nil {
		t.Errorf("kind = %v, want none", *e.Kind)
	}
	if e.Summary != "Update the readme" || !e.Derived || e.Breaking || e.Scope != nil {
		t.Errorf("entry = %+v", e)
	}
}

// AC: AC-CLOG-004.10
func TestMechanicalStripsPrefixAndPullRequestSuffix(t *testing.T) {
	c := mechanical(t,
		withConv(producertest.Commit("a1", "feat(cli): add --dry-run (#12)", "", 12), "feat", scope("cli"), false),
		withConv(producertest.Commit("b2", "fix: a (#3) mention stays", "", 0), "fix", nil, false),
	)
	if got := c.Entries[0]; got.Summary != "add --dry-run" || got.Scope == nil || *got.Scope != "cli" {
		t.Errorf("entry 0 = %+v", got)
	}
	if got := c.Entries[1].Summary; got != "a (#3) mention stays" {
		t.Errorf("entry 1 summary = %q", got)
	}
	want := []releasemodel.ChangeSource{
		releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: "a1"}),
		releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: 12}),
	}
	if !reflect.DeepEqual(c.Entries[0].Sources, want) {
		t.Errorf("sources = %+v, want the commit then the pull request", c.Entries[0].Sources)
	}
	if len(c.Entries[1].Sources) != 1 {
		t.Errorf("a commit with no pull request cites only itself: %+v", c.Entries[1].Sources)
	}
}

// AC: AC-CLOG-004.10
func TestMechanicalBreakingMarkers(t *testing.T) {
	c := mechanical(t,
		withConv(producertest.Commit("a1", "feat!: drop it", "", 0), "feat", nil, true),
		withConv(producertest.Commit("b2", "feat: drop that", "Why.\n\nBREAKING CHANGE: use the new flag\n  and rerun setup\nReviewed-by: x", 0), "feat", nil, true),
		withConv(producertest.Commit("c3", "feat: calm", "", 0), "feat", nil, false),
	)
	if !c.Entries[0].Breaking || c.Entries[0].Migration == nil || *c.Entries[0].Migration != noMigration {
		t.Errorf("marker without footer = %+v", c.Entries[0])
	}
	if !c.Entries[1].Breaking || c.Entries[1].Migration == nil || *c.Entries[1].Migration != "use the new flag\n  and rerun setup" {
		t.Errorf("marker with footer: migration = %v", c.Entries[1].Migration)
	}
	if c.Entries[2].Breaking || c.Entries[2].Migration != nil {
		t.Errorf("non-breaking = %+v", c.Entries[2])
	}
}

// AC: AC-CLOG-004.10
func TestMechanicalKeepsOrderSourcesAndLeavesVersionUnset(t *testing.T) {
	c := mechanical(t, producertest.Commit("a1", "x", "", 0), producertest.Commit("b2", "y", "", 0))
	if c.Entries[0].Summary != "x" || c.Entries[1].Summary != "y" {
		t.Errorf("order = %+v", c.Entries)
	}
	if c.Version != nil || c.ProposedBump != nil || c.SchemaVersion != "1.0.0" || c.Omitted != nil {
		t.Errorf("changelog = %+v", c)
	}
	if len(c.Sources) != 1 || c.Sources[0].Adapter != releasemodel.AdapterCommits {
		t.Errorf("sources = %+v, want the bundle's", c.Sources)
	}
}

func TestMechanicalRejectsANonBundle(t *testing.T) {
	if _, err := (Mechanical{}).Produce(context.Background(), []byte(`{"commits":1}`)); err == nil {
		t.Error("want an error for input that is not an evidence bundle")
	}
}
