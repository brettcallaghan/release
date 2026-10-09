package gitread

import (
	"context"
	"fmt"
	"regexp"
	"strconv"
	"strings"

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
// subject, body, and the trailers git itself parses from the message's final
// paragraph, unfolded, as key/value pairs.
var commitFormat = strings.Join([]string{
	"%H", "%an", "%ae", "%s", "%b",
	"%(trailers:only,unfold,separator=%x1d,key_value_separator=%x1c)",
}, "%x1f") + "%x1e"

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
		records = append(records, rec)
	}
	return records, nil
}

func parseCommit(raw string) (releasemodel.CommitRecord, error) {
	f := strings.Split(raw, fieldSep)
	if len(f) != 6 {
		return releasemodel.CommitRecord{}, fmt.Errorf("git log: unexpected record with %d fields", len(f))
	}
	rec := releasemodel.CommitRecord{
		Hash:        f[0],
		AuthorName:  f[1],
		AuthorEmail: f[2],
		Subject:     f[3],
		Body:        f[4],
		PullRequest: pullRequestInfo(f[3]),
		Files:       []releasemodel.FileChange{},
		Trailers:    parseTrailers(f[5]),
	}
	return rec, nil
}

// pullRequestInfo returns the pull request a squash subject carries: its
// number and the SQUASH origin. Title and description are left unset. It
// returns nil WHEN the subject carries no number.
func pullRequestInfo(subject string) *releasemodel.PullRequestInfo {
	n := pullRequestNumber(subject)
	if n == nil {
		return nil
	}
	return &releasemodel.PullRequestInfo{Number: *n, Origin: releasemodel.PullRequestOriginSquash}
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
