package cli

import (
	"context"
	"errors"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/authoring/producers"
	"github.com/brettcallaghan/release/internal/changelog"
	"github.com/brettcallaghan/release/internal/changelogfile"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/suggest"
)

// newChangelogCmd is `release changelog [bump]`: the changelog section a
// release would carry, authored from the range's evidence and rendered by the
// release core.
// It writes the changelog file only under --write.
func newChangelogCmd(s *streams) *cobra.Command {
	var (
		target     string
		skipNotes  bool
		redoNotes  bool
		corpusPath string
		force      bool
		write      bool
		asJSON     bool
	)
	cmd := &cobra.Command{
		Use:   "changelog [bump]",
		Short: "Author and render the changelog section a release would carry",
		Long: "changelog resolves the base version and the signal floor, collects the\n" +
			"evidence of the commits since the base version (each commit's record and,\n" +
			"by default, its diff), has the configured producer author a structured\n" +
			"changelog from it, suggests the bump from the entries' kinds raised to the\n" +
			"signal floor, and answers with the version, the bump and the suggestion\n" +
			"with its reasons, where the authored changelog is kept, and the rendered\n" +
			"changelog section. It never prompts and confirms nothing: without a named\n" +
			"bump the section is a preview under the suggested bump. It writes\n" +
			"CHANGELOG.md only with --write, which requires a bump.\n\n" +
			"bump is major, minor or patch. A named bump below the suggestion proceeds\n" +
			"with a warning naming the entries that drove the suggestion; one below\n" +
			"the signal floor is refused unless --force is given. The authored\n" +
			"changelog is cached for the commit range, and reused until --redo-notes.\n" +
			"Exits 1 when no release is warranted.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"major", "minor", "patch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipNotes && redoNotes {
				return errors.New("--skip-notes and --redo-notes cannot be combined: one skips authoring, the other repeats it")
			}
			if skipNotes && corpusPath != "" {
				return errors.New("--skip-notes and --corpus cannot be combined: no changelog is cached to read or write")
			}
			if write && len(args) == 0 {
				return errors.New("--write needs a named bump: this command confirms no suggestion; name major, minor or patch")
			}
			if force && len(args) == 0 {
				s.exitCode = StatusRefused
				fmt.Fprintln(s.err, "release changelog: --force needs a named bump: it accepts that bump below the signal floor; name major, minor or patch")
				return nil
			}
			req := changelog.Request{
				Dir:        s.root,
				Target:     target,
				Config:     s.cfg,
				Producers:  producers.Registry(s.cfg.Changelog, s.root),
				CorpusPath: corpusPath,
				Redo:       redoNotes,
				Skip:       skipNotes,
				Forced:     force,
				// A preview: without a named bump the section renders under
				// the suggestion, and nothing is written on its strength.
				Accepted: true,
			}
			if corpusPath != "" && !filepath.IsAbs(corpusPath) {
				abs, err := filepath.Abs(corpusPath)
				if err != nil {
					return err
				}
				req.CorpusPath = abs
			}
			if len(args) == 1 {
				b, ok := namedBumps[args[0]]
				if !ok {
					return fmt.Errorf("invalid bump %q: want major, minor or patch", args[0])
				}
				req.Named = &b
			}
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			if req.Target == "" {
				ref, err := s.cfg.TargetRef(ctx, gitread.Repo{Dir: req.Dir})
				if errors.Is(err, config.ErrNoDefaultBranch) {
					return fmt.Errorf("no --target given and no target configured: %w", err)
				}
				if err != nil {
					return err
				}
				req.Target = ref
			}

			res, err := changelog.Prepare(ctx, req)
			var rejected *authoring.Rejection
			var failed *producers.Failure
			if errors.As(err, &rejected) || errors.As(err, &failed) {
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release changelog: %v\n", err)
				return nil
			}
			if err != nil {
				return err
			}

			switch res.Outcome {
			case changelog.NoRelease:
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release changelog: %s\n", refusal(res.Next))
				return nil
			case changelog.Refused:
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release changelog: %s\n", belowFloorRefusal(res.Refusal))
				return nil
			case changelog.Halted:
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release changelog: %s\n", unkindedHalt(res.Suggestion))
				return nil
			}

			if skipNotes {
				fmt.Fprintln(s.err, "release changelog: no producer read the diffs: --skip-notes takes each entry's kind from its conventional subject, so the suggestion is the signal floor")
			}
			if w := res.Suggestion.Decision.Warning; w != nil {
				fmt.Fprintf(s.err, "release changelog: warning: %s\n", warning(*w, res.Suggestion))
			}
			if write {
				if err := changelogfile.Write(s.root, res.Version, res.Rendering.Section); err != nil {
					return err
				}
			}
			return s.answer(asJSON, s.changelogAnswer(res))
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "The release line to collect from: a `branch` (default: the configured target, else the branch origin/HEAD points to)")
	cmd.Flags().BoolVar(&skipNotes, "skip-notes", false, "Author with the mechanical producer instead of the configured one, and read no cache")
	cmd.Flags().BoolVar(&redoNotes, "redo-notes", false, "Discard any cached corpus and author again")
	cmd.Flags().StringVar(&corpusPath, "corpus", "", "Read the corpus at this `path` instead of the cache when it exists, and write it there otherwise")
	cmd.Flags().BoolVar(&force, "force", false, "Accept a named bump below the signal floor. Requires bump")
	cmd.Flags().BoolVar(&write, "write", false, "Write the rendered section into CHANGELOG.md. Requires bump")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Render the answer as JSON instead of YAML")
	return cmd
}

// changelogAnswer is the command's answer, keys in the contract's order.
func (s *streams) changelogAnswer(res changelog.Result) yMap {
	reasons, unclassified := suggestionNodes(res)
	path := yamlNode(yNull())
	if res.CorpusPath != "" {
		path = yStr(s.display(res.CorpusPath))
	}
	return yMap{
		{"version", yStr(res.Version)},
		{"bump", yStr(lower(res.Bump))},
		{"named", yBool(res.Suggestion.Decision.DecidedBy == suggest.ByNamed)},
		{"suggested", yStr(lower(res.Suggestion.Suggestion))},
		{"floor", yStr(lower(res.Suggestion.Floor))},
		{"reasons", reasons},
		{"raised_by_floor", yBool(res.Suggestion.Reasons.FloorRaised)},
		{"unclassified", unclassified},
		{"corpus_path", path},
		// Trimmed as the YAML style trims every multi-line value, so both
		// renderings carry the same value.
		{"section", yStr(strings.TrimSpace(res.Rendering.Section))},
	}
}

// suggestionNodes are the suggestion's reasons and the unclassified
// commits, as release changelog and a halted release cut answer them.
func suggestionNodes(res changelog.Result) (reasons, unclassified ySeq) {
	reasons, unclassified = ySeq{}, ySeq{}
	for _, d := range res.Suggestion.Reasons.Entries {
		reasons = append(reasons, driverNode(d))
	}
	for _, c := range res.Suggestion.Unclassified {
		unclassified = append(unclassified, yMap{
			{"hash", yStr(c.Hash)},
			{"subject", yStr(c.Subject)},
		})
	}
	return reasons, unclassified
}

// driverNode is an entry that drove the suggestion: its kind, lower-cased,
// or null; whether it is breaking; its scope or null; its summary; and its
// sources as references: "#<n>" for a pull request, the full hash for a
// commit, and the id for an issue.
func driverNode(d suggest.Driver) yMap {
	kind, scope := yamlNode(yNull()), yamlNode(yNull())
	if d.Kind != nil {
		kind = yStr(strings.ToLower(string(*d.Kind)))
	}
	if d.Scope != nil {
		scope = yStr(*d.Scope)
	}
	sources := ySeq{}
	for _, src := range d.Sources {
		sources = append(sources, yStr(sourceRef(src)))
	}
	return yMap{
		{"kind", kind},
		{"breaking", yBool(d.Breaking)},
		{"scope", scope},
		{"summary", yStr(d.Summary)},
		{"sources", sources},
	}
}

func sourceRef(s releasemodel.ChangeSource) string {
	switch v := s.Value.(type) {
	case releasemodel.PullRequestSource:
		return fmt.Sprintf("#%d", v.Number)
	case releasemodel.CommitSource:
		return v.Hash
	case releasemodel.IssueSource:
		return v.Id
	}
	return ""
}

func lower(b releasemodel.Bump) string { return strings.ToLower(string(b)) }

// belowFloorRefusal names the signal floor, the commits that set it, and how
// to force the named bump.
func belowFloorRefusal(r *suggest.Refusal) string {
	var b strings.Builder
	fmt.Fprintf(&b, "the named bump %s is below the signal floor %s; pass --force to accept it anyway. These commits set the floor:",
		lower(r.Named), lower(r.Floor))
	for _, d := range r.FloorDrivers {
		fmt.Fprintf(&b, "\n  %s %s (%s)", d.Commit.Hash, d.Commit.Subject, d.Signal)
	}
	return b.String()
}

// unkindedHalt is the halt a preview cannot answer: the suggestion is none
// and entries carry no kind, so only a named bump releases them.
func unkindedHalt(s suggest.Result) string {
	var b strings.Builder
	b.WriteString("the authored changelog suggests no bump, but these entries carry no kind, so only a named bump can release them; name it (release changelog <major|minor|patch>):")
	if s.Halt != nil {
		for _, d := range s.Halt.Unkinded {
			fmt.Fprintf(&b, "\n  - %s", d.Summary)
		}
	}
	return b.String()
}

// warning explains a named bump the evidence argues against: below the
// suggestion, naming the entries that drove it, or forced below the floor.
func warning(w suggest.Warning, s suggest.Result) string {
	named := lower(s.Decision.Bump)
	if w.Kind == suggest.BelowFloor {
		return fmt.Sprintf("the named bump %s is below the signal floor %s, accepted under --force", named, lower(s.Floor))
	}
	var b strings.Builder
	fmt.Fprintf(&b, "the named bump %s is below the suggested bump %s", named, lower(s.Suggestion))
	if s.Reasons.FloorRaised {
		fmt.Fprintf(&b, ", which the signal floor %s sets", lower(s.Floor))
	}
	if len(s.Reasons.Entries) > 0 {
		b.WriteString("; these entries drove the suggestion:")
		for _, d := range s.Reasons.Entries {
			fmt.Fprintf(&b, "\n  - %s", d.Summary)
		}
	}
	return b.String()
}

// answer writes a command's answer in the requested rendering.
func (s *streams) answer(asJSON bool, a yMap) error {
	if asJSON {
		return writeJSON(s.out, a)
	}
	return writeYAML(s.out, a)
}

// display is a path as an answer reports it: relative to the repository
// root, with forward slashes, WHEN it lies inside the root, and otherwise as
// given. Both sides are compared with symbolic links resolved, because git
// reports the root resolved (macOS's /var is /private/var) while a path the
// user named is not.
func (s *streams) display(path string) string {
	if rel, ok := within(s.root, path); ok {
		return rel
	}
	root, err := filepath.EvalSymlinks(s.root)
	if err != nil {
		return path
	}
	resolved, err := filepath.EvalSymlinks(path)
	if err != nil {
		return path
	}
	if rel, ok := within(root, resolved); ok {
		return rel
	}
	return path
}

// within is path relative to root, with forward slashes, and whether path
// lies inside root.
func within(root, path string) (string, bool) {
	rel, err := filepath.Rel(root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return "", false
	}
	return filepath.ToSlash(rel), true
}
