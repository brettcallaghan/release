package gitread

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/brettcallaghan/release/internal/releasemodel"
)

// Field and record separators for the log format. ASCII unit and record
// separators do not occur in commit messages a person writes; NUL cannot be
// used for both levels at once, and -z gives only one.
const (
	fieldSep   = "\x1f"
	recordSep  = "\x1e"
	trailerSep = "\x1d"
	keyValSep  = "\x1c"
)

// commitFormat is one record per commit: hash, author name, author email,
// subject, body, the trailers git itself parses from the message's final
// paragraph, unfolded, as key/value pairs, and the strict ISO 8601 author date.
var commitFormat = strings.Join([]string{
	"%H", "%an", "%ae", "%s", "%b", "%aI",
	"%(trailers:only,unfold,separator=%x1d,key_value_separator=%x1c)",
}, "%x1f") + "%x1e"

// filesFormat introduces each commit's file list: the record separator and
// the hash, then the NUL-separated entries -z appends.
const filesFormat = "%x1e%H%x1f"

// pullRequestSuffix is the reference a hosting platform appends to a squash
// commit subject: " (#<n>)" at the end of the line.
var pullRequestSuffix = regexp.MustCompile(` \(#([0-9]+)\)$`)

// Commits returns one record per commit reachable from ref and not from
// base, newest first as git log lists them. An empty base means every commit
// reachable from ref, which is the range WHEN no release tag exists yet.
//
// Merge commits are included. Trailers are git's own parse of the message's
// final paragraph (git interpret-trailers rules); a repeated key keeps its
// last value. Nothing here interprets what a trailer means.
func (r Repo) Commits(ctx context.Context, base, ref string) ([]releasemodel.CommitRecord, error) {
	rng := ref
	if base != "" {
		rng = base + ".." + ref
	}
	out, err := r.run(ctx, "log", "--format="+commitFormat, rng, "--")
	if err != nil {
		return nil, err
	}
	files, err := r.changedFiles(ctx, rng)
	if err != nil {
		return nil, err
	}
	var records []releasemodel.CommitRecord
	for _, raw := range strings.Split(string(out), recordSep) {
		raw = strings.TrimLeft(raw, "\n")
		if raw == "" {
			continue
		}
		rec, err := parseCommit(raw)
		if err != nil {
			return nil, err
		}
		if fs, ok := files[rec.Hash]; ok {
			rec.Files = fs
		}
		records = append(records, rec)
	}
	return records, nil
}

func parseCommit(raw string) (releasemodel.CommitRecord, error) {
	f := strings.Split(raw, fieldSep)
	if len(f) != 7 {
		return releasemodel.CommitRecord{}, fmt.Errorf("git log: unexpected record with %d fields", len(f))
	}
	trailers := parseTrailers(f[6])
	// git's strict ISO 8601 writes UTC as +00:00 in some versions and Z in
	// others; RFC 3339 output is the same under every git.
	date, err := time.Parse(time.RFC3339, f[5])
	if err != nil {
		return releasemodel.CommitRecord{}, fmt.Errorf("git log: author date %q: %w", f[5], err)
	}
	rec := releasemodel.CommitRecord{
		Hash:         f[0],
		AuthorName:   f[1],
		AuthorEmail:  f[2],
		Subject:      f[3],
		Body:         f[4],
		AuthorDate:   date.Format(time.RFC3339),
		Conventional: conventionalParts(f[3], f[4]),
		PullRequest:  pullRequestInfo(f[3], f[4], len(trailers) > 0),
		Files:        []releasemodel.FileChange{},
		Trailers:     trailers,
	}
	return rec, nil
}

// conventionalSubject is "type(scope)!: description"; the scope and the
// breaking "!" are optional.
var conventionalSubject = regexp.MustCompile(`^([A-Za-z]+)(?:\(([^()]*)\))?(!)?: `)

// conventionalParts returns the parts of a conventional-commit subject: the
// type as written, the scope, and whether the subject carries "!" or the
// body's final paragraph a BREAKING CHANGE: or BREAKING-CHANGE: line. It
// returns nil WHEN the subject is not of that form, whatever the body says.
func conventionalParts(subject, body string) *releasemodel.Conventional {
	m := conventionalSubject.FindStringSubmatch(subject)
	if m == nil {
		return nil
	}
	c := &releasemodel.Conventional{Type: m[1], Breaking: m[3] == "!"}
	if m[2] != "" {
		scope := m[2]
		c.Scope = &scope
	}
	if !c.Breaking {
		for _, line := range strings.Split(finalParagraph(body), "\n") {
			if strings.HasPrefix(line, "BREAKING CHANGE:") || strings.HasPrefix(line, "BREAKING-CHANGE:") {
				c.Breaking = true
				break
			}
		}
	}
	return c
}

// mergeSubject is the subject a hosting platform gives a merge commit.
var mergeSubject = regexp.MustCompile(`^Merge pull request #([0-9]+) from `)

// pullRequestInfo returns the pull request a commit carries, from the commit
// alone. A squash subject ends " (#<n>)": the title is the subject without
// that suffix and the description the body less its trailer paragraph. A
// merge subject reads "Merge pull request #<n> from <branch>": the title is
// the body's first paragraph and the description the rest of the body less
// its trailer paragraph. It returns nil for any other commit.
func pullRequestInfo(subject, body string, hasTrailers bool) *releasemodel.PullRequestInfo {
	body = strings.ReplaceAll(body, "\r\n", "\n")
	if hasTrailers {
		body = dropFinalParagraph(body)
	}
	if m := mergeSubject.FindStringSubmatch(subject); m != nil {
		n, err := strconv.ParseInt(m[1], 10, 64)
		if err != nil {
			return nil
		}
		first, rest, _ := strings.Cut(strings.TrimLeft(body, "\n"), "\n\n")
		return &releasemodel.PullRequestInfo{
			Number:      n,
			Title:       nonEmpty(first),
			Description: nonEmpty(rest),
			Origin:      releasemodel.PullRequestOriginMerge,
		}
	}
	n := pullRequestNumber(subject)
	if n == nil {
		return nil
	}
	return &releasemodel.PullRequestInfo{
		Number:      *n,
		Title:       nonEmpty(pullRequestSuffix.ReplaceAllString(subject, "")),
		Description: nonEmpty(body),
		Origin:      releasemodel.PullRequestOriginSquash,
	}
}

// nonEmpty trims surrounding blank space and returns nil WHEN nothing is
// left.
func nonEmpty(s string) *string {
	s = strings.Trim(s, " \t\n")
	if s == "" {
		return nil
	}
	return &s
}

// finalParagraph is the body's last blank-line-separated paragraph.
func finalParagraph(body string) string {
	body = strings.TrimRight(strings.ReplaceAll(body, "\r\n", "\n"), " \t\n")
	if i := strings.LastIndex(body, "\n\n"); i >= 0 {
		return body[i+2:]
	}
	return body
}

// dropFinalParagraph is the body without its last blank-line-separated
// paragraph.
func dropFinalParagraph(body string) string {
	body = strings.TrimRight(body, " \t\n")
	if i := strings.LastIndex(body, "\n\n"); i >= 0 {
		return body[:i]
	}
	return ""
}

// pullRequestNumber returns the number a hosting platform appended to a
// squash subject, or nil WHEN the subject carries none.
func pullRequestNumber(subject string) *int64 {
	m := pullRequestSuffix.FindStringSubmatch(subject)
	if m == nil {
		return nil
	}
	n, err := strconv.ParseInt(m[1], 10, 64)
	if err != nil {
		return nil
	}
	return &n
}

// changedFiles returns, per commit hash in rng, the files it changed against
// its first parent, from git's --name-status and --numstat with rename
// detection. A merge commit's files are taken against its first parent, as
// the Diff Collector diffs it.
func (r Repo) changedFiles(ctx context.Context, rng string) (map[string][]releasemodel.FileChange, error) {
	status, err := r.run(ctx, "log", "--format="+filesFormat, "-z", "-M", "--diff-merges=first-parent", "--name-status", rng, "--")
	if err != nil {
		return nil, err
	}
	counts, err := r.run(ctx, "log", "--format="+filesFormat, "-z", "-M", "--diff-merges=first-parent", "--numstat", rng, "--")
	if err != nil {
		return nil, err
	}
	numstat := map[string]map[string][2]int64{}
	for _, raw := range strings.Split(string(counts), recordSep) {
		hash, toks, ok := fileTokens(raw)
		if !ok {
			continue
		}
		m := map[string][2]int64{}
		for len(toks) > 0 {
			add, del, path, rest, err := nextNumstat(toks)
			if err != nil {
				return nil, err
			}
			m[path] = [2]int64{add, del}
			toks = rest
		}
		numstat[hash] = m
	}
	out := map[string][]releasemodel.FileChange{}
	for _, raw := range strings.Split(string(status), recordSep) {
		hash, toks, ok := fileTokens(raw)
		if !ok {
			continue
		}
		files := []releasemodel.FileChange{}
		for len(toks) > 0 {
			code := toks[0]
			if code == "" {
				return nil, fmt.Errorf("git log: empty status code")
			}
			var fc releasemodel.FileChange
			switch code[0] {
			case 'R', 'C':
				if len(toks) < 3 {
					return nil, fmt.Errorf("git log: truncated %s entry", code)
				}
				prev := toks[1]
				fc.PreviousPath = &prev
				fc.Path = toks[2]
				fc.Status = releasemodel.FileStatusRenamed
				if code[0] == 'C' {
					fc.Status = releasemodel.FileStatusCopied
				}
				toks = toks[3:]
			default:
				if len(toks) < 2 {
					return nil, fmt.Errorf("git log: truncated %s entry", code)
				}
				fc.Path = toks[1]
				switch code[0] {
				case 'A':
					fc.Status = releasemodel.FileStatusAdded
				case 'D':
					fc.Status = releasemodel.FileStatusDeleted
				default:
					fc.Status = releasemodel.FileStatusModified
				}
				toks = toks[2:]
			}
			c := numstat[hash][fc.Path]
			fc.Additions, fc.Deletions = c[0], c[1]
			files = append(files, fc)
		}
		out[hash] = files
	}
	return out, nil
}

// fileTokens splits one "<hash><US>" record of a -z log into its hash and
// NUL-separated tokens. ok is false for the empty text before the first
// record.
func fileTokens(raw string) (hash string, toks []string, ok bool) {
	hash, rest, found := strings.Cut(raw, fieldSep)
	if !found {
		return "", nil, false
	}
	rest = strings.TrimLeft(rest, "\x00\n")
	rest = strings.TrimSuffix(rest, "\x00")
	if rest == "" {
		return hash, nil, true
	}
	return hash, strings.Split(rest, "\x00"), true
}

// nextNumstat reads one numstat entry: "<add>\t<del>\t<path>", or for a
// rename or copy "<add>\t<del>\t" followed by the old and new paths as
// separate tokens. A binary file's "-" counts read as 0.
func nextNumstat(toks []string) (add, del int64, path string, rest []string, err error) {
	parts := strings.SplitN(toks[0], "\t", 3)
	if len(parts) != 3 {
		return 0, 0, "", nil, fmt.Errorf("git log: malformed numstat entry %q", toks[0])
	}
	add, del = count(parts[0]), count(parts[1])
	if parts[2] != "" {
		return add, del, parts[2], toks[1:], nil
	}
	if len(toks) < 3 {
		return 0, 0, "", nil, fmt.Errorf("git log: truncated numstat rename entry")
	}
	return add, del, toks[2], toks[3:], nil
}

// count parses a numstat count; "-" (binary) and anything unparsable is 0.
func count(s string) int64 {
	n, err := strconv.ParseInt(s, 10, 64)
	if err != nil {
		return 0
	}
	return n
}

// parseTrailers turns git's separated trailer output into a map. Later
// occurrences of a key overwrite earlier ones.
func parseTrailers(s string) map[string]string {
	trailers := map[string]string{}
	for _, t := range strings.Split(strings.TrimRight(s, "\n"), trailerSep) {
		key, value, ok := strings.Cut(t, keyValSep)
		if !ok || key == "" {
			continue
		}
		trailers[key] = value
	}
	return trailers
}
