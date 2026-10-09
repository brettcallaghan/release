package gitread

import (
	"context"
	"fmt"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

func (f *fixture) diffs(commits []string, exclude []string, budget int64) ([]releasemodel.DiffRecord, releasemodel.BudgetReport) {
	f.t.Helper()
	recs, rep, err := f.repo().Diffs(context.Background(), commits, exclude, budget)
	if err != nil {
		f.t.Fatalf("Diffs: %v", err)
	}
	return recs, rep
}

func (f *fixture) head() string { return f.git("rev-parse", "HEAD") }

// lines returns n numbered lines, each starting with tag.
func lines(tag string, n int) string {
	var b strings.Builder
	for i := 0; i < n; i++ {
		fmt.Fprintf(&b, "%s line %03d\n", tag, i)
	}
	return b.String()
}

// edited changes every tenth line, so the diff has a hunk per change.
func edited(text string) string {
	ls := strings.Split(text, "\n")
	for i := 5; i < len(ls); i += 10 {
		ls[i] = "edited " + ls[i]
	}
	return strings.Join(ls, "\n")
}

func carried(recs []releasemodel.DiffRecord) int64 {
	var n int64
	for _, r := range recs {
		for _, fl := range r.Files {
			n += int64(len(fl.Patch))
		}
	}
	return n
}

func fileNamed(rec releasemodel.DiffRecord, path string) (releasemodel.FileDiff, bool) {
	for _, fl := range rec.Files {
		if fl.Path == path {
			return fl, true
		}
	}
	return releasemodel.FileDiff{}, false
}

func (f *fixture) twoFileCommit() string {
	f.write("go.sum", "sum\n")
	f.write("src/main.go", "package main\n")
	f.commitAll("base")
	f.write("go.sum", "sum\nmore\n")
	f.write("src/main.go", "package main\n\nfunc main() {}\n")
	f.commitAll("change both")
	return f.head()
}

// AC: AC-CLOG-001.10
func TestDiffsExcludedPathIsNotCarriedButIsReported(t *testing.T) {
	f := newFixture(t)
	c := f.twoFileCommit()
	recs, rep := f.diffs([]string{c}, []string{"go.sum"}, 1<<20)

	if _, ok := fileNamed(recs[0], "go.sum"); ok {
		t.Error("the excluded lockfile's patch was carried")
	}
	src, ok := fileNamed(recs[0], "src/main.go")
	if !ok || !strings.Contains(src.Patch, "+func main() {}") {
		t.Errorf("source patch = %+v", src)
	}
	want := releasemodel.OmittedDiff{Commit: c, Path: "go.sum", Reason: releasemodel.OmissionReasonExcluded}
	if len(rep.Omitted) != 1 || rep.Omitted[0] != want {
		t.Errorf("omitted = %+v, want [%+v]", rep.Omitted, want)
	}
	// The commit's file list still names the lockfile.
	var named bool
	for _, fc := range f.commits("", "HEAD")[0].Files {
		named = named || fc.Path == "go.sum"
	}
	if !named {
		t.Error("the commit's file list lost the lockfile")
	}
	if rep.Total != int64(len(src.Patch)) || rep.Used != rep.Total {
		t.Errorf("report = %+v, total must leave the excluded file out", rep)
	}
}

// AC: AC-CLOG-001.11
func TestDiffsConfiguredGlobExcludesGeneratedPaths(t *testing.T) {
	f := newFixture(t)
	f.write("gen/bindings.go", "package gen\n")
	f.write("gen/deep/more.go", "package deep\n")
	f.write("cmd/main.go", "package main\n")
	f.commitAll("base")
	f.write("gen/bindings.go", "package gen\n// x\n")
	f.write("gen/deep/more.go", "package deep\n// y\n")
	f.write("cmd/main.go", "package main\n// z\n")
	f.commitAll("change")
	recs, rep := f.diffs([]string{f.head()}, []string{"gen/**"}, 1<<20)
	if len(recs[0].Files) != 1 || recs[0].Files[0].Path != "cmd/main.go" {
		t.Errorf("files = %+v, want cmd/main.go only", recs[0].Files)
	}
	if len(rep.Omitted) != 2 || rep.Omitted[0].Path != "gen/bindings.go" || rep.Omitted[1].Path != "gen/deep/more.go" {
		t.Errorf("omitted = %+v", rep.Omitted)
	}
}

func TestExcludedMatching(t *testing.T) {
	for _, tc := range []struct {
		entry, path string
		want        bool
	}{
		{"go.sum", "go.sum", true},
		{"go.sum", "sub/go.sum", false},
		{"vendor/", "vendor/a/b.go", true},
		{"vendor/", "vendors/a.go", false},
		{"./vendor/", "vendor/a.go", true},
		{"**/*.lock", "yarn.lock", true},
		{"**/*.lock", "a/b/yarn.lock", true},
		{"**/*.lock", "a/b/yarn.txt", false},
		{"gen/**", "gen/x.go", true},
		{"gen/**", "gen", true},
		{"gen/**/out.go", "gen/out.go", true},
		{"gen/**/out.go", "gen/a/b/out.go", true},
		{"gen/**/out.go", "gen/a/b/in.go", false},
		{"*.golden", "a.golden", true},
		{"*.golden", "dir/a.golden", false},
		{"a/**/**/b", "a/b", true},
		{"[", "x", false},
		{"testdata/**/expected/", "testdata/cases/a/expected/stdout.txt", true},
		{"testdata/**/expected/", "testdata/expected/x", true},
		{"testdata/**/expected/", "testdata/cases/a/expected", false},
		{"testdata/**/expected/", "internal/x/testdata/c/expected/y", false},
		{"vendor/", "vendor/x.go", true},
		{"**/go.sum", "a/b/go.sum", true},
	} {
		if got := excluded(tc.path, []string{tc.entry}); got != tc.want {
			t.Errorf("excluded(%q, %q) = %v, want %v", tc.path, tc.entry, got, tc.want)
		}
	}
}

// AC: AC-CLOG-001.12
func TestDiffsTruncateAtWholeHunkAndReportIt(t *testing.T) {
	f := newFixture(t)
	f.write("big.txt", lines("orig", 60))
	f.commitAll("base")
	// Two changes far apart yield two hunks.
	content := strings.Replace(lines("orig", 60), "orig line 002\n", "changed line 002\n", 1)
	content = strings.Replace(content, "orig line 057\n", "changed line 057\n", 1)
	f.write("big.txt", content)
	f.commitAll("two hunks")
	c := f.head()

	_, full := f.diffs([]string{c}, nil, 1<<20)
	whole := full.Total
	if full.Used != whole || len(full.Omitted) != 0 {
		t.Fatalf("unbudgeted report = %+v", full)
	}

	budget := whole - 20
	recs, rep := f.diffs([]string{c}, nil, budget)
	fd := recs[0].Files[0]
	if !fd.Truncated || !recs[0].Truncated {
		t.Errorf("file truncated = %v, record truncated = %v, want both", fd.Truncated, recs[0].Truncated)
	}
	if int64(len(fd.Patch)) > budget || strings.Count(fd.Patch, "@@ -") != 1 {
		t.Errorf("patch is not header plus the first whole hunk:\n%s", fd.Patch)
	}
	if !strings.HasSuffix(fd.Patch, "\n") {
		t.Error("patch does not end on a hunk boundary")
	}
	if rep.Total != whole || rep.Used != int64(len(fd.Patch)) || rep.Limit != budget {
		t.Errorf("report = %+v", rep)
	}
	want := releasemodel.OmittedDiff{Commit: c, Path: "big.txt", Reason: releasemodel.OmissionReasonOverBudget, Truncated: true}
	if len(rep.Omitted) != 1 || rep.Omitted[0] != want {
		t.Errorf("omitted = %+v", rep.Omitted)
	}
}

// AC: AC-CLOG-001.12
func TestDiffsFileWithNoHunkThatFitsIsKeptEmpty(t *testing.T) {
	f := newFixture(t)
	f.write("a.txt", "a\n")
	f.write("b.txt", "b\n")
	f.commitAll("base")
	f.write("a.txt", "a\nsmall\n")
	f.write("b.txt", "b\n"+lines("big", 80))
	f.commitAll("change")
	c := f.head()

	_, full := f.diffs([]string{c}, nil, 1<<20)
	a, _ := fileNamed(func() releasemodel.DiffRecord { r, _ := f.diffs([]string{c}, nil, 1<<20); return r[0] }(), "a.txt")
	budget := int64(len(a.Patch)) + 50
	recs, rep := f.diffs([]string{c}, nil, budget)

	if got, _ := fileNamed(recs[0], "a.txt"); got.Patch != a.Patch || got.Truncated {
		t.Errorf("a.txt = %+v, want it whole", got)
	}
	b, ok := fileNamed(recs[0], "b.txt")
	if !ok || b.Patch != "" || b.Truncated {
		t.Errorf("b.txt = %+v, want present, empty, untruncated", b)
	}
	want := releasemodel.OmittedDiff{Commit: c, Path: "b.txt", Reason: releasemodel.OmissionReasonOverBudget}
	if len(rep.Omitted) != 1 || rep.Omitted[0] != want {
		t.Errorf("omitted = %+v, want [%+v]", rep.Omitted, want)
	}
	if !recs[0].Truncated {
		t.Error("record not marked truncated though the budget cut a file")
	}
	if rep.Total != full.Total {
		t.Errorf("total = %d, want %d reported before the budget", rep.Total, full.Total)
	}
}

// AC: AC-CLOG-001.12
func TestDiffsBinaryFileHasEmptyPatchAndIsListed(t *testing.T) {
	f := newFixture(t)
	f.write("img.bin", "\x00\x01\x02")
	f.write("t.txt", "x\n")
	f.commitAll("base")
	f.write("img.bin", "\x00\x03\x04\x05")
	f.write("t.txt", "x\ny\n")
	f.commitAll("change")
	c := f.head()
	recs, rep := f.diffs([]string{c}, nil, 1<<20)

	img, ok := fileNamed(recs[0], "img.bin")
	if !ok || img.Patch != "" || img.Truncated {
		t.Errorf("img.bin = %+v", img)
	}
	txt, _ := fileNamed(recs[0], "t.txt")
	want := releasemodel.OmittedDiff{Commit: c, Path: "img.bin", Reason: releasemodel.OmissionReasonBinary}
	if len(rep.Omitted) != 1 || rep.Omitted[0] != want || rep.Total != int64(len(txt.Patch)) {
		t.Errorf("report = %+v, want binary listed and left out of total", rep)
	}
	if recs[0].Truncated {
		t.Error("a binary file is not a truncation")
	}
}

// AC: AC-CLOG-001.12
func TestDiffsBudgetRedistributesFromSmallCommits(t *testing.T) {
	f := newFixture(t)
	f.write("small.txt", "s\n")
	f.write("big1.txt", lines("one", 100))
	f.write("big2.txt", lines("two", 100))
	f.commitAll("base")
	f.write("small.txt", "s\nt\n")
	f.commitAll("small")
	small := f.head()
	f.write("big1.txt", edited(lines("one", 100)))
	f.commitAll("big one")
	big1 := f.head()
	f.write("big2.txt", edited(lines("two", 100)))
	f.commitAll("big two")
	big2 := f.head()

	all, full := f.diffs([]string{small, big1, big2}, nil, 1<<20)
	smallSize := int64(len(all[0].Files[0].Patch))
	bigSize := int64(len(all[1].Files[0].Patch))

	// Equal shares would be budget/3; the small commit needs less, so the big
	// ones split the remainder, each getting more than a third.
	budget := smallSize + 2*bigSize - 120
	if budget/3 >= bigSize || budget/3 <= smallSize {
		t.Fatalf("fixture sizes do not exercise redistribution: small %d big %d budget %d", smallSize, bigSize, budget)
	}
	recs, rep := f.diffs([]string{small, big1, big2}, nil, budget)

	if recs[0].Truncated || int64(len(recs[0].Files[0].Patch)) != smallSize {
		t.Errorf("small commit cut: %+v", recs[0])
	}
	for i := 1; i <= 2; i++ {
		p := recs[i].Files[0]
		if len(p.Patch) <= int(budget/3) {
			t.Errorf("commit %d carried %d bytes, want more than an equal share %d", i, len(p.Patch), budget/3)
		}
		if int64(len(p.Patch)) > (budget-smallSize+1)/2 {
			t.Errorf("commit %d carried %d bytes, over its redistributed share", i, len(p.Patch))
		}
	}
	if got := carried(recs); got > budget || got != rep.Used {
		t.Errorf("carried %d, used %d, budget %d", got, rep.Used, budget)
	}
	if rep.Total != full.Total {
		t.Errorf("total = %d, want %d", rep.Total, full.Total)
	}
	if len(rep.Omitted) != 2 || rep.Omitted[0].Commit != big1 || rep.Omitted[1].Commit != big2 {
		t.Errorf("omitted = %+v, want both big commits in commit order", rep.Omitted)
	}
}

// AC: AC-CLOG-001.12
func TestDiffsNeverExceedBudget(t *testing.T) {
	f := newFixture(t)
	f.write("a.txt", lines("a", 30))
	f.write("b.txt", lines("b", 30))
	f.commitAll("base")
	var commits []string
	for i := 0; i < 4; i++ {
		f.write("a.txt", strings.Replace(lines("a", 30), fmt.Sprintf("a line %03d\n", i*7), fmt.Sprintf("edit %d\n", i), 1))
		f.write("b.txt", strings.Replace(lines("b", 30), fmt.Sprintf("b line %03d\n", i*5), fmt.Sprintf("edit %d\n", i), 1))
		f.commitAll(fmt.Sprintf("c%d", i))
		commits = append(commits, f.head())
	}
	for budget := int64(0); budget <= 4000; budget += 37 {
		recs, rep := f.diffs(commits, nil, budget)
		if got := carried(recs); got > budget || got != rep.Used {
			t.Fatalf("budget %d: carried %d, used %d", budget, got, rep.Used)
		}
		if len(recs) != len(commits) {
			t.Fatalf("budget %d: %d records", budget, len(recs))
		}
	}
	if _, rep := f.diffs(commits, nil, -5); rep.Limit != 0 || rep.Used != 0 {
		t.Errorf("negative budget report = %+v", rep)
	}
}

// AC: AC-CLOG-001.10
func TestDiffsRootCommitAgainstEmptyTree(t *testing.T) {
	f := newFixture(t)
	f.write("first.txt", "hello\n")
	f.commitAll("root")
	recs, rep := f.diffs([]string{f.head()}, nil, 1<<20)
	fd, ok := fileNamed(recs[0], "first.txt")
	if !ok || !strings.Contains(fd.Patch, "+hello") || !strings.Contains(fd.Patch, "new file mode") {
		t.Errorf("root diff = %+v", recs[0])
	}
	if rep.Total != int64(len(fd.Patch)) {
		t.Errorf("report = %+v", rep)
	}
}

func TestDiffsRenameAndMergeCommit(t *testing.T) {
	f := newFixture(t)
	f.write("old.txt", lines("r", 30))
	f.commitAll("base")
	f.git("mv", "old.txt", "new.txt")
	f.commitAll("rename only")
	rename := f.head()
	f.git("checkout", "-q", "-b", "topic")
	f.write("t.txt", "t\n")
	f.commitAll("topic")
	f.git("checkout", "-q", "main")
	f.write("m.txt", "m\n")
	f.commitAll("main")
	f.git("merge", "-q", "--no-ff", "-m", "merge", "topic")
	merge := f.head()

	recs, _ := f.diffs([]string{merge, rename}, nil, 1<<20)
	if fd, ok := fileNamed(recs[0], "t.txt"); !ok || !strings.Contains(fd.Patch, "+t") {
		t.Errorf("merge diff is not against the first parent: %+v", recs[0])
	}
	if len(recs[0].Files) != 1 {
		t.Errorf("merge files = %+v", recs[0].Files)
	}
	fd, ok := fileNamed(recs[1], "new.txt")
	if !ok || !strings.Contains(fd.Patch, "rename from old.txt") {
		t.Errorf("rename diff = %+v", recs[1])
	}
}

func TestDiffsErrors(t *testing.T) {
	f := newFixture(t)
	f.commitMsg("one")
	if _, _, err := f.repo().Diffs(context.Background(), []string{"nope"}, nil, 10); err == nil {
		t.Error("want an error for an unknown commit")
	}
	if got := rawPaths(":100644 100644 a b R100\x00old\x00new\x00:100644 100644 a b M\x00x\x00"); len(got) != 2 || got[0] != "new" || got[1] != "x" {
		t.Errorf("rawPaths = %v", got)
	}
	if got := splitSections(""); got != nil {
		t.Errorf("splitSections(empty) = %v", got)
	}
}

func TestDiffsDefaultExcludeSkipsGoldenSnapshots(t *testing.T) {
	f := newFixture(t)
	f.write("testdata/cases/a/expected/stdout.txt", "one\n")
	f.write("src/main.go", "package main\n")
	f.commitAll("base")
	f.write("testdata/cases/a/expected/stdout.txt", "one\ntwo\n")
	f.write("src/main.go", "package main\n// x\n")
	f.commitAll("change")
	c := f.head()
	recs, rep := f.diffs([]string{c}, []string{"testdata/**/expected/"}, 1<<20)
	if _, ok := fileNamed(recs[0], "testdata/cases/a/expected/stdout.txt"); ok {
		t.Error("the golden snapshot's patch was carried")
	}
	want := releasemodel.OmittedDiff{Commit: c, Path: "testdata/cases/a/expected/stdout.txt", Reason: releasemodel.OmissionReasonExcluded}
	if len(rep.Omitted) != 1 || rep.Omitted[0] != want {
		t.Errorf("omitted = %+v, want [%+v]", rep.Omitted, want)
	}
}

// submoduleFixture returns a repository holding a submodule bump and an edit
// to a regular file in one commit.
func submoduleFixture(t *testing.T) (*fixture, string) {
	t.Helper()
	sub := newFixture(t)
	sub.write("a.txt", "a\n")
	sub.commitAll("sub one")
	f := newFixture(t)
	f.write("f", "one\n")
	f.commitAll("base")
	f.git("-c", "protocol.file.allow=always", "submodule", "add", "-q", sub.dir, "sub")
	f.commitAll("add submodule")
	sub.write("a.txt", "a\nb\n")
	sub.commitAll("sub two")
	f.git("-C", "sub", "pull", "-q", "origin", "HEAD")
	f.write("f", "one\ntwo\n")
	f.commitAll("bump")
	return f, f.head()
}

func TestDiffsIgnoreUserDiffConfig(t *testing.T) {
	plain, pc := submoduleFixture(t)
	want, _ := plain.diffs([]string{pc}, nil, 1<<20)

	f, c := submoduleFixture(t)
	f.git("config", "diff.submodule", "log")
	f.git("config", "diff.noprefix", "true")
	got, _ := f.diffs([]string{c}, nil, 1<<20)

	file, ok := fileNamed(got[0], "f")
	if !ok || !strings.Contains(file.Patch, "+two") {
		t.Fatalf("regular file diff = %+v, ok = %v", file, ok)
	}
	if len(got[0].Files) != len(want[0].Files) {
		t.Fatalf("files = %+v, want %+v", got[0].Files, want[0].Files)
	}
	for i := range got[0].Files {
		if got[0].Files[i].Path != want[0].Files[i].Path || got[0].Files[i].Patch != want[0].Files[i].Patch {
			t.Errorf("file %d = %+v, want %+v", i, got[0].Files[i], want[0].Files[i])
		}
	}
}
