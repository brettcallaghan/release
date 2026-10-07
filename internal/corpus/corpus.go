// Package corpus is the Changelog Corpus Builder: commit records in, an
// assembled changelog corpus out. It is a pure function of its inputs. It
// reads no git, no file and no network, and it never proposes an impact or
// sets a version, both of which belong to others (ADR-003).
package corpus

import (
	"errors"
	"regexp"
	"strings"

	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
)

// SchemaVersion is the changelog schema version an assembled corpus declares.
const SchemaVersion = "1.0"

// Range is the commit range the records were read from.
type Range struct {
	// From is the base commit's full hash: the commit the previous release
	// was tagged on. Empty for a release line with no release yet.
	From string
	// To is the head commit's full hash.
	To string
}

// Input is everything the builder reads.
type Input struct {
	// Records are the commit records in the range, in the order entries are
	// to appear.
	Records []releasemodel.CommitRecord
	// Range is the range the records were read from.
	Range Range
	// Adapters is the enabled optional adapter set, from configuration. The
	// commit adapter is always enabled.
	Adapters config.Adapters
}

// ErrAdapterUnavailable is the refusal for an enabled adapter that is not
// implemented. The builder never skips an adapter configuration enabled.
var ErrAdapterUnavailable = errors.New("an enabled changelog adapter is not implemented")

// Build assembles a corpus: one entry per record, in record order, with the
// commit adapter recorded in sources together with its range. proposed_bump
// and version are left unset.
func Build(in Input) (releasemodel.Changelog, error) {
	if in.Adapters.PullRequests {
		return releasemodel.Changelog{}, errors.Join(ErrAdapterUnavailable, errors.New("adapters.pull_requests: the pull request adapter is not implemented"))
	}
	if in.Adapters.Diffs {
		return releasemodel.Changelog{}, errors.Join(ErrAdapterUnavailable, errors.New("adapters.diffs: no diff adapter is implemented"))
	}
	run := releasemodel.AdapterRun{Adapter: releasemodel.AdapterCommits, To: in.Range.To}
	if in.Range.From != "" {
		from := in.Range.From
		run.From = &from
	}
	entries := make([]releasemodel.ChangelogEntry, 0, len(in.Records))
	for _, r := range in.Records {
		entries = append(entries, Entry(r))
	}
	return releasemodel.Changelog{
		SchemaVersion: SchemaVersion,
		Entries:       entries,
		Sources:       []releasemodel.AdapterRun{run},
	}, nil
}

// conventionalSubject is "type(scope)!: description"; the scope and the
// breaking "!" are optional. The bump classifier reads the same shape.
var conventionalSubject = regexp.MustCompile(`^([A-Za-z]+)(?:\(([^()]*)\))?!?: (.*)$`)

// pullRequestSuffix is the reference a hosting platform appends to a squash
// commit subject.
var pullRequestSuffix = regexp.MustCompile(` \(#[0-9]+\)$`)

// kinds maps a conventional-commit type, lower-cased, to its change kind.
// Any other type, and a subject that is not conventional, is a CHORE.
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

// Entry is the entry one commit record yields. Its kind and scope come from
// the subject's conventional-commit type and scope. Its prose comes from an
// author-written release-notes block WHEN the body carries one, verbatim and
// with derived false, and otherwise from the subject, with derived true.
func Entry(r releasemodel.CommitRecord) releasemodel.ChangelogEntry {
	subject := pullRequestSuffix.ReplaceAllString(r.Subject, "")
	e := releasemodel.ChangelogEntry{
		Kind:     releasemodel.ChangeKindChore,
		Summary:  subject,
		Breaking: bump.Breaking(r),
		Derived:  true,
	}
	if m := conventionalSubject.FindStringSubmatch(subject); m != nil {
		if k, ok := kinds[strings.ToLower(m[1])]; ok {
			e.Kind = k
		}
		if scope := strings.TrimSpace(m[2]); scope != "" {
			e.Scope = &scope
		}
		if desc := strings.TrimSpace(m[3]); desc != "" {
			e.Summary = desc
		}
	}
	if summary, detail, ok := ReleaseNotes(r); ok {
		e.Summary, e.Derived = summary, false
		if detail != "" {
			e.Detail = &detail
		}
	}
	if e.Breaking {
		if note, ok := bump.BreakingNote(r); ok {
			e.Migration = &note
		}
	}
	if r.PullRequest != nil {
		e.Sources = []releasemodel.ChangeSource{releasemodel.NewChangeSourcePullRequest(releasemodel.PullRequestSource{Number: *r.PullRequest})}
	} else {
		e.Sources = []releasemodel.ChangeSource{releasemodel.NewChangeSourceCommit(releasemodel.CommitSource{Hash: r.Hash})}
	}
	return e
}

// notesHeading opens an author-written block: a level-two heading reading
// "Release notes", in any letter case.
var notesHeading = regexp.MustCompile(`(?i)^##[ \t]+release notes[ \t]*$`)

// ReleaseNotes returns the author-written release-notes block in a record's
// body: the lines after a `## Release notes` heading, up to the next `## `
// heading or the end of the body. Its first paragraph is the summary and the
// rest, from the next paragraph on, the detail. Both are verbatim: only the
// blank lines around them are dropped. WHEN the block runs to the end of the
// body, a final paragraph made only of trailers and breaking-change footers
// is the commit's metadata, not prose, and is left out. ok is false WHEN the
// body has no such heading or the block holds no prose.
func ReleaseNotes(r releasemodel.CommitRecord) (summary, detail string, ok bool) {
	lines := strings.Split(strings.ReplaceAll(r.Body, "\r\n", "\n"), "\n")
	start := -1
	for i, line := range lines {
		if notesHeading.MatchString(line) {
			start = i + 1
			break
		}
	}
	if start < 0 {
		return "", "", false
	}
	end, toEnd := len(lines), true
	for i := start; i < len(lines); i++ {
		if strings.HasPrefix(lines[i], "## ") {
			end, toEnd = i, false
			break
		}
	}
	block := trimBlank(lines[start:end])
	if toEnd {
		last := lastParagraph(block)
		if last < len(block) && metadataOnly(block[last:], r.Trailers) {
			block = trimBlank(block[:last])
		}
	}
	if len(block) == 0 {
		return "", "", false
	}
	first := len(block)
	for i, l := range block {
		if blank(l) {
			first = i
			break
		}
	}
	return strings.Join(block[:first], "\n"), strings.Join(trimBlank(block[first:]), "\n"), true
}

func blank(line string) bool { return strings.TrimSpace(line) == "" }

// trimBlank drops the blank lines at either end.
func trimBlank(lines []string) []string {
	for len(lines) > 0 && blank(lines[0]) {
		lines = lines[1:]
	}
	for len(lines) > 0 && blank(lines[len(lines)-1]) {
		lines = lines[:len(lines)-1]
	}
	return lines
}

// lastParagraph is the index of the first line of the final paragraph of
// lines, which carry no blank line at either end.
func lastParagraph(lines []string) int {
	for i := len(lines) - 1; i >= 0; i-- {
		if blank(lines[i]) {
			return i + 1
		}
	}
	return 0
}

// metadataOnly reports a paragraph every line of which is a trailer git
// parsed from the commit, or a breaking-change footer.
func metadataOnly(para []string, trailers map[string]string) bool {
	for _, l := range para {
		if strings.HasPrefix(l, "BREAKING CHANGE:") || strings.HasPrefix(l, "BREAKING-CHANGE:") {
			continue
		}
		key, _, found := strings.Cut(l, ":")
		if !found {
			return false
		}
		if _, ok := trailers[key]; !ok {
			return false
		}
	}
	return true
}
