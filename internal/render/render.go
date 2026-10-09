// Package render is the Changelog Renderer: one changelog corpus turned into
// the markdown of every target — the CHANGELOG.md section and the release
// body — in one pure, deterministic call.
//
// It reads no file, no git and no network. Both targets come from the same
// corpus value in the same invocation, so they cannot disagree (ADR-003).
package render

import (
	"errors"
	"fmt"
	"strings"

	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/semver"
)

// SchemaMajor is the major schema_version this renderer implements.
const SchemaMajor = 1

// ErrNoVersion is the refusal for a corpus whose version is unset: a section
// is headed by the version it describes.
var ErrNoVersion = errors.New("the changelog has no version; it cannot be rendered until the version is resolved")

// Rendering is every target, rendered from one corpus.
type Rendering struct {
	// Section is the CHANGELOG.md section, headed `## <version>`.
	Section string
	// Body is the release body: Section without its heading line and the
	// blank line after it.
	Body string
}

// Render renders the corpus into every target under the changelog settings.
// repoURL is the repository's web URL, such as https://github.com/o/r; when
// it is empty, sources render as plain text rather than links.
func Render(c releasemodel.Changelog, settings config.Changelog, repoURL string) (Rendering, error) {
	if err := checkSchemaVersion(c.SchemaVersion); err != nil {
		return Rendering{}, err
	}
	if c.Version == nil || strings.TrimSpace(*c.Version) == "" {
		return Rendering{}, ErrNoVersion
	}
	repoURL = strings.TrimRight(repoURL, "/")

	var blocks []string
	if c.Title != nil && text(*c.Title) != "" {
		blocks = append(blocks, "**"+text(*c.Title)+"**")
	}
	if c.Summary != nil && text(*c.Summary) != "" {
		blocks = append(blocks, text(*c.Summary))
	}

	groups, err := group(c.Entries, settings)
	if err != nil {
		return Rendering{}, err
	}
	for _, g := range groups {
		var list strings.Builder
		for i, e := range g.entries {
			item, loose := entry(e, repoURL)
			list.WriteString(item)
			if i < len(g.entries)-1 {
				list.WriteString("\n")
				// A blank line ends an item carrying paragraphs, so the
				// next item does not read as a continuation of them.
				if loose {
					list.WriteString("\n")
				}
			}
		}
		// A heading is configuration, rendered verbatim (not trimmed).
		blocks = append(blocks, "### "+g.heading, list.String())
	}

	heading := "## " + strings.TrimSpace(*c.Version) + "\n"
	if len(blocks) == 0 {
		return Rendering{Section: heading}, nil
	}
	body := strings.Join(blocks, "\n\n") + "\n"
	return Rendering{Section: heading + "\n" + body, Body: body}, nil
}

// checkSchemaVersion refuses a schema_version whose major this renderer does
// not implement, rather than rendering the instance partially.
func checkSchemaVersion(s string) error {
	v, err := semver.Parse(s)
	if err != nil {
		return fmt.Errorf("the changelog's schema_version %q is not a version: %w", s, err)
	}
	if v.Major != SchemaMajor {
		return fmt.Errorf("the changelog's schema_version %s has major %d; this renderer implements major %d", s, v.Major, SchemaMajor)
	}
	return nil
}

type rendered struct {
	heading string
	entries []releasemodel.ChangelogEntry
}

// group sorts entries into the groups that render, in render order: breaking
// changes, the configured groups in configured order, then other changes. An
// entry keeps its corpus order within its group, and an empty group is left
// out.
func group(entries []releasemodel.ChangelogEntry, settings config.Changelog) ([]rendered, error) {
	breaking := rendered{heading: settings.BreakingHeading}
	other := rendered{heading: settings.OtherHeading}
	configured := make([]rendered, len(settings.Groups))
	index := map[releasemodel.ChangeKind]int{}
	for i, g := range settings.Groups {
		configured[i].heading = g.Heading
		index[g.Kind] = i
	}
	for i, e := range entries {
		for _, s := range e.Sources {
			if s.Value == nil {
				return nil, fmt.Errorf("entry %d (%q) has a source with no kind", i, e.Summary)
			}
		}
		var i int
		var ok bool
		if e.Kind != nil {
			i, ok = index[*e.Kind]
		}
		switch {
		case e.Breaking:
			breaking.entries = append(breaking.entries, e)
		case ok:
			configured[i].entries = append(configured[i].entries, e)
		case e.Kind == nil || !config.KnownChangeKind(*e.Kind):
			other.entries = append(other.entries, e)
		}
	}
	var out []rendered
	for _, g := range append(append([]rendered{breaking}, configured...), other) {
		if len(g.entries) > 0 {
			out = append(out, g)
		}
	}
	return out, nil
}

// entry renders one list item: the scope, the summary and the sources on
// the first line, then detail and migration as indented paragraphs. loose
// reports whether it carries paragraphs.
func entry(e releasemodel.ChangelogEntry, repoURL string) (item string, loose bool) {
	var line strings.Builder
	line.WriteString("- ")
	if e.Scope != nil && text(*e.Scope) != "" {
		line.WriteString("**" + text(*e.Scope) + ":** ")
	}
	line.WriteString(text(e.Summary))
	if len(e.Sources) > 0 {
		refs := make([]string, len(e.Sources))
		for i, s := range e.Sources {
			refs[i] = source(s, repoURL)
		}
		line.WriteString(" (" + strings.Join(refs, ", ") + ")")
	}

	paragraphs := []string{line.String()}
	if e.Detail != nil && text(*e.Detail) != "" {
		paragraphs = append(paragraphs, indent(text(*e.Detail)))
	}
	if e.Migration != nil && text(*e.Migration) != "" {
		paragraphs = append(paragraphs, indent("Migration: "+text(*e.Migration)))
	}
	return strings.Join(paragraphs, "\n\n"), len(paragraphs) > 1
}

// source renders one source, as a link when the repository URL is known.
func source(s releasemodel.ChangeSource, repoURL string) string {
	switch v := s.Value.(type) {
	case releasemodel.PullRequestSource:
		ref := fmt.Sprintf("#%d", v.Number)
		if repoURL == "" {
			return ref
		}
		return fmt.Sprintf("[%s](%s/pull/%d)", ref, repoURL, v.Number)
	case releasemodel.CommitSource:
		short := v.Hash
		if len(short) > 7 {
			short = short[:7]
		}
		if repoURL == "" {
			return short
		}
		return fmt.Sprintf("[%s](%s/commit/%s)", short, repoURL, v.Hash)
	case releasemodel.IssueSource:
		return v.Id
	}
	panic(fmt.Sprintf("render: unhandled change source %T", s.Value))
}

// text is a corpus string with \n line endings and no surrounding white
// space, so the output's line endings and trailing newline are the
// renderer's alone.
func text(s string) string {
	s = strings.ReplaceAll(s, "\r\n", "\n")
	s = strings.ReplaceAll(s, "\r", "\n")
	return strings.TrimSpace(s)
}

// indent indents every non-empty line of a paragraph into a list item.
func indent(s string) string {
	lines := strings.Split(s, "\n")
	for i, l := range lines {
		if l != "" {
			lines[i] = "  " + l
		}
	}
	return strings.Join(lines, "\n")
}
