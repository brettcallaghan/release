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
)

// newChangelogCmd is `release changelog [bump]`: the changelog section a
// release would carry, assembled, authored and rendered by the release core.
// It writes the changelog file only under --write.
func newChangelogCmd(s *streams) *cobra.Command {
	var (
		target     string
		skipNotes  bool
		redoNotes  bool
		corpusPath string
		write      bool
		asJSON     bool
	)
	cmd := &cobra.Command{
		Use:   "changelog [bump]",
		Short: "Assemble, author and render the changelog section a release would carry",
		Long: "changelog resolves the next version, assembles a changelog corpus from the\n" +
			"commits since the base version, has the configured producer author it, and\n" +
			"answers with the version, where the authored corpus is kept, and the\n" +
			"rendered changelog section. It writes CHANGELOG.md only with --write.\n\n" +
			"bump is major, minor or patch, and names the impact instead of inferring\n" +
			"it. An authored corpus proposes an impact; when the proposal disagrees with\n" +
			"the computed impact and no bump is named, changelog prints both and exits 1\n" +
			"without rendering. Naming the bump confirms it, and the proposal is then\n" +
			"advisory. The authored corpus is cached for the commit range, and reused\n" +
			"until --redo-notes. Exits 1 when no release is warranted.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"major", "minor", "patch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipNotes && redoNotes {
				return errors.New("--skip-notes and --redo-notes cannot be combined: one skips authoring, the other repeats it")
			}
			if skipNotes && corpusPath != "" {
				return errors.New("--skip-notes and --corpus cannot be combined: no corpus is authored to read or write")
			}
			req := changelog.Request{
				Dir:        s.root,
				Target:     target,
				Config:     s.cfg,
				Producers:  producers.Registry(s.cfg.Changelog, s.root),
				CorpusPath: corpusPath,
				Redo:       redoNotes,
				Skip:       skipNotes,
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
				fmt.Fprintf(s.err, "release changelog: %s\n", refusal(res.Resolution.Next))
				return nil
			case changelog.Disagreement:
				s.exitCode = StatusRefused
				computed := strings.ToLower(string(res.Authoring.Computed))
				proposed := strings.ToLower(string(*res.Authoring.Proposed))
				fmt.Fprintf(s.err, "release changelog: the authored corpus proposes %s, but the commits indicate %s; "+
					"confirm the impact by naming it (release changelog %s), or correct the commits\n", proposed, computed, computed)
				return s.answer(asJSON, yMap{
					{"computed", yStr(computed)},
					{"proposed", yStr(proposed)},
					{"corpus_path", yStr(s.display(res.CorpusPath))},
				})
			}

			if write {
				if err := changelogfile.Write(s.root, res.Version, res.Rendering.Section); err != nil {
					return err
				}
			}
			path := yamlNode(yNull())
			if res.CorpusPath != "" {
				path = yStr(s.display(res.CorpusPath))
			}
			return s.answer(asJSON, yMap{
				{"version", yStr(res.Version)},
				{"corpus_path", path},
				// Trimmed as the YAML style trims every multi-line value,
				// so both renderings carry the same value.
				{"section", yStr(strings.TrimSpace(res.Rendering.Section))},
			})
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "The release line to collect from: a `branch` (default: the configured target, else the branch origin/HEAD points to)")
	cmd.Flags().BoolVar(&skipNotes, "skip-notes", false, "Assemble mechanically and invoke no producer")
	cmd.Flags().BoolVar(&redoNotes, "redo-notes", false, "Discard any cached corpus and author again")
	cmd.Flags().StringVar(&corpusPath, "corpus", "", "Read the corpus at this `path` instead of the cache when it exists, and write it there otherwise")
	cmd.Flags().BoolVar(&write, "write", false, "Write the rendered section into CHANGELOG.md")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Render the answer as JSON instead of YAML")
	return cmd
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
// given.
func (s *streams) display(path string) string {
	rel, err := filepath.Rel(s.root, path)
	if err != nil || rel == ".." || strings.HasPrefix(rel, ".."+string(filepath.Separator)) {
		return path
	}
	return filepath.ToSlash(rel)
}
