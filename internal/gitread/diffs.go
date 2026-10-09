package gitread

import (
	"context"
	"fmt"
	"path"
	"sort"
	"strings"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

// filePatch is one file of a commit's diff against its first parent.
type filePatch struct {
	path   string
	binary bool
	header string   // the diff --git block up to the first hunk
	hunks  []string // each hunk, from its @@ line to the next
}

// size is the byte length of the whole patch text.
func (p filePatch) size() int64 {
	n := int64(len(p.header))
	for _, h := range p.hunks {
		n += int64(len(h))
	}
	return n
}

// commitDiff is a commit's carried files in path order, with excluded ones
// already set aside.
type commitDiff struct {
	hash     string
	files    []filePatch
	excluded []string
	demand   int64
}

// Diffs returns, per commit given and in that order, its diff against its
// first parent (the empty tree for a root commit), and a report of the patch
// text carried against budget bytes. The diffs of paths matched by exclude
// are left out and listed in the report as EXCLUDED; binary files are kept
// with an empty patch and listed as BINARY.
//
// The budget is shared equally among the commits: one whose patch text is
// smaller than its share leaves the rest to those still over theirs, in
// commit order, until none is over or the budget is spent. Within a commit,
// files are taken in path order. A file that does not fit what remains is cut
// at the last whole hunk that does and marked truncated; one of which no hunk
// fits keeps an empty patch, untruncated. Both are listed as OVER_BUDGET. The
// patch text returned never exceeds budget, and Total counts the patch text
// of every file that is neither excluded nor binary before the budget.
//
// A diff is never interpreted, and nothing here reads configuration.
func (r Repo) Diffs(ctx context.Context, commits []string, exclude []string, budget int64) ([]releasemodel.DiffRecord, releasemodel.BudgetReport, error) {
	if budget < 0 {
		budget = 0
	}
	diffs := make([]commitDiff, len(commits))
	var total int64
	for i, hash := range commits {
		d, err := r.commitPatches(ctx, hash, exclude)
		if err != nil {
			return nil, releasemodel.BudgetReport{}, err
		}
		diffs[i] = d
		total += d.demand
	}

	alloc := allocate(diffs, budget)

	report := releasemodel.BudgetReport{Limit: budget, Total: total, Omitted: []releasemodel.OmittedDiff{}}
	records := make([]releasemodel.DiffRecord, len(diffs))
	for i, d := range diffs {
		rec := releasemodel.DiffRecord{Commit: d.hash, Files: []releasemodel.FileDiff{}}
		remaining := alloc[i]
		// Excluded paths and carried files interleave in path order in the
		// report.
		var entries []releasemodel.OmittedDiff
		for _, p := range d.excluded {
			entries = append(entries, releasemodel.OmittedDiff{Commit: d.hash, Path: p, Reason: releasemodel.OmissionReasonExcluded})
		}
		for _, f := range d.files {
			fd := releasemodel.FileDiff{Path: f.path}
			switch {
			case f.binary:
				entries = append(entries, releasemodel.OmittedDiff{Commit: d.hash, Path: f.path, Reason: releasemodel.OmissionReasonBinary})
			case f.size() <= remaining:
				fd.Patch = f.header + strings.Join(f.hunks, "")
			default:
				fd.Patch, fd.Truncated = cut(f, remaining)
				rec.Truncated = true
				entries = append(entries, releasemodel.OmittedDiff{Commit: d.hash, Path: f.path, Reason: releasemodel.OmissionReasonOverBudget, Truncated: fd.Truncated})
			}
			remaining -= int64(len(fd.Patch))
			report.Used += int64(len(fd.Patch))
			rec.Files = append(rec.Files, fd)
		}
		sort.SliceStable(entries, func(a, b int) bool { return entries[a].Path < entries[b].Path })
		report.Omitted = append(report.Omitted, entries...)
		records[i] = rec
	}
	return records, report, nil
}

// cut returns the longest patch of f within limit made of its header and
// whole hunks. truncated is false, with an empty patch, WHEN no hunk fits.
func cut(f filePatch, limit int64) (patch string, truncated bool) {
	used := int64(len(f.header))
	if used > limit || len(f.hunks) == 0 {
		return "", false
	}
	var b strings.Builder
	b.WriteString(f.header)
	for _, h := range f.hunks {
		if used+int64(len(h)) > limit {
			break
		}
		used += int64(len(h))
		b.WriteString(h)
		truncated = true
	}
	if !truncated {
		return "", false
	}
	return b.String(), true
}

// allocate shares budget among the commits: equally, with what a commit
// under its share does not use passed on to those still over theirs. The
// budget's remainder after integer division goes to the earliest of them.
func allocate(diffs []commitDiff, budget int64) []int64 {
	alloc := make([]int64, len(diffs))
	var active []int
	for i, d := range diffs {
		if d.demand > 0 {
			active = append(active, i)
		}
	}
	remaining := budget
	for len(active) > 0 {
		share := remaining / int64(len(active))
		var over []int
		for _, i := range active {
			if diffs[i].demand <= share {
				alloc[i] = diffs[i].demand
				remaining -= diffs[i].demand
			} else {
				over = append(over, i)
			}
		}
		if len(over) == len(active) {
			extra := remaining % int64(len(over))
			for n, i := range over {
				alloc[i] = share
				if int64(n) < extra {
					alloc[i]++
				}
			}
			break
		}
		active = over
	}
	return alloc
}

// commitPatches reads one commit's diff against its first parent and splits
// it per file.
func (r Repo) commitPatches(ctx context.Context, hash string, exclude []string) (commitDiff, error) {
	d := commitDiff{hash: hash}
	base, err := r.firstParent(ctx, hash)
	if err != nil {
		return d, err
	}
	common := []string{"diff", "--no-color", "--no-ext-diff", "-M"}
	names, err := r.run(ctx, append(append(common[:4:4], "--raw", "-z"), base, hash, "--")...)
	if err != nil {
		return d, err
	}
	text, err := r.run(ctx, append(common[:4:4], base, hash, "--")...)
	if err != nil {
		return d, err
	}
	paths := rawPaths(string(names))
	sections := splitSections(string(text))
	if len(sections) != len(paths) {
		return d, fmt.Errorf("git diff: %s: patch has %d files but the file list has %d", hash, len(sections), len(paths))
	}
	for i, s := range sections {
		p := paths[i]
		if excluded(p, exclude) {
			d.excluded = append(d.excluded, p)
			continue
		}
		fp := parseSection(p, s)
		d.files = append(d.files, fp)
		if !fp.binary {
			d.demand += fp.size()
		}
	}
	sort.Strings(d.excluded)
	sort.SliceStable(d.files, func(a, b int) bool { return d.files[a].path < d.files[b].path })
	return d, nil
}

// firstParent returns commit's first parent, or the empty tree WHEN it has
// none.
func (r Repo) firstParent(ctx context.Context, hash string) (string, error) {
	out, err := r.run(ctx, "rev-list", "--parents", "-n", "1", hash)
	if err != nil {
		return "", err
	}
	fields := strings.Fields(string(out))
	if len(fields) > 1 {
		return fields[1], nil
	}
	tree, err := r.run(ctx, "hash-object", "-t", "tree", "/dev/null")
	if err != nil {
		return "", err
	}
	return strings.TrimSpace(string(tree)), nil
}

// rawPaths returns the post-change path of each entry of `git diff --raw -z`,
// in git's order. A rename or copy entry carries the old path then the new.
func rawPaths(raw string) []string {
	toks := strings.Split(strings.TrimSuffix(raw, "\x00"), "\x00")
	var paths []string
	for i := 0; i < len(toks); {
		meta := toks[i]
		if !strings.HasPrefix(meta, ":") {
			i++
			continue
		}
		fields := strings.Fields(meta)
		status := fields[len(fields)-1]
		if status != "" && (status[0] == 'R' || status[0] == 'C') {
			if i+2 < len(toks) {
				paths = append(paths, toks[i+2])
			}
			i += 3
			continue
		}
		if i+1 < len(toks) {
			paths = append(paths, toks[i+1])
		}
		i += 2
	}
	return paths
}

// splitSections splits patch text into one block per file, each starting at
// a "diff --git " line. Such a line cannot occur inside a block: patch lines
// begin with a space, "+", "-", "@" or "\".
func splitSections(text string) []string {
	var sections []string
	for text != "" {
		next := strings.Index(text[1:], "\ndiff --git ")
		if next < 0 {
			sections = append(sections, text)
			break
		}
		sections = append(sections, text[:next+2])
		text = text[next+2:]
	}
	return sections
}

// parseSection splits a file's block into its header and hunks.
func parseSection(p, s string) filePatch {
	fp := filePatch{path: p}
	for _, line := range strings.Split(s, "\n") {
		if strings.HasPrefix(line, "Binary files ") || line == "GIT binary patch" {
			fp.binary = true
			break
		}
	}
	if fp.binary {
		return fp
	}
	first := -1
	if strings.HasPrefix(s, "@@ ") {
		first = 0
	} else if i := strings.Index(s, "\n@@ "); i >= 0 {
		first = i + 1
	}
	if first < 0 {
		fp.header = s
		return fp
	}
	fp.header = s[:first]
	rest := s[first:]
	for rest != "" {
		next := strings.Index(rest[1:], "\n@@ ")
		if next < 0 {
			fp.hunks = append(fp.hunks, rest)
			break
		}
		fp.hunks = append(fp.hunks, rest[:next+2])
		rest = rest[next+2:]
	}
	return fp
}

// excluded reports whether p matches an exclusion entry: equal to it, under
// an entry ending in "/", or a glob match in which "**" spans any number of
// path segments and every other segment is matched by path.Match.
func excluded(p string, entries []string) bool {
	for _, e := range entries {
		e = strings.TrimPrefix(e, "./")
		if p == e || strings.HasSuffix(e, "/") && strings.HasPrefix(p, e) {
			return true
		}
		if matchSegments(strings.Split(e, "/"), strings.Split(p, "/")) {
			return true
		}
	}
	return false
}

func matchSegments(pat, segs []string) bool {
	for len(pat) > 0 {
		if pat[0] == "**" {
			for len(pat) > 1 && pat[1] == "**" {
				pat = pat[1:]
			}
			if len(pat) == 1 {
				return true
			}
			for i := 0; i <= len(segs); i++ {
				if matchSegments(pat[1:], segs[i:]) {
					return true
				}
			}
			return false
		}
		if len(segs) == 0 {
			return false
		}
		if ok, err := path.Match(pat[0], segs[0]); err != nil || !ok {
			return false
		}
		pat, segs = pat[1:], segs[1:]
	}
	return len(segs) == 0
}
