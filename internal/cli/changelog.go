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
// release would carry, authored from the range's evidence and rendered by the
// release core.
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
		Short: "Author and render the changelog section a release would carry",
		Long: "changelog resolves the next version, collects the evidence of the commits\n" +
			"since the base version (each commit's record and, by default, its diff),\n" +
			"has the configured producer author a structured changelog from it, and\n" +
			"answers with the version, where the authored changelog is kept, and the\n" +
			"rendered changelog section. It writes CHANGELOG.md only with --write.\n\n" +
			"bump is major, minor or patch, and names the impact instead of inferring\n" +
			"it. The authored changelog is cached for the commit range, and reused\n" +
			"until --redo-notes. Exits 1 when no release is warranted.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"major", "minor", "patch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipNotes && redoNotes {
				return errors.New("--skip-notes and --redo-notes cannot be combined: one skips authoring, the other repeats it")
			}
			if skipNotes && corpusPath != "" {
				return errors.New("--skip-notes and --corpus cannot be combined: no changelog is cached to read or write")
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

			if res.Outcome == changelog.NoRelease {
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release changelog: %s\n", refusal(res.Resolution.Next))
				return nil
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
	cmd.Flags().BoolVar(&skipNotes, "skip-notes", false, "Author with the mechanical producer instead of the configured one, and read no cache")
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
