package cli

import (
	"bufio"
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/authoring/producers"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/cut"
	"github.com/brettcallaghan/release/internal/gitread"
)

// newCutCmd is `release cut [bump]`: the release core's Stage Orchestrator
// cuts the release as a stage branch and opens its pull request. It is the
// only command that writes to the repository or the hosting platform.
func newCutCmd(s *streams) *cobra.Command {
	var (
		target    string
		skipNotes bool
		force     bool
		redoNotes bool
		noPR      bool
		dryRun    bool
		yes       bool
		asJSON    bool
	)
	cmd := &cobra.Command{
		Use:   "cut [bump]",
		Short: "Cut a release as a stage branch and open its pull request",
		Long: "cut fetches the target branch from origin and prepares the release from its\n" +
			"tip in a scratch checkout: it resolves the version, authors and renders the\n" +
			"changelog, writes it into CHANGELOG.md, runs commands.propagate to write the\n" +
			"version into the manifests (with RELEASE_VERSION set), refuses a change to\n" +
			"any path that is neither CHANGELOG.md nor in release_paths, and runs\n" +
			"commands.validate. Only then does it write: it creates the stage branch at\n" +
			"the fetched tip, commits the release as \"Release <version>\", pushes the\n" +
			"branch to origin, and opens a pull request into the target with gh. Your\n" +
			"working tree, index and current branch are never read or changed.\n\n" +
			"bump is major, minor or patch, and names the impact instead of inferring\n" +
			"it. A named bump is refused when the commits since the base version indicate\n" +
			"no bump (there are none, or none is releasable), unless --force is given.\n" +
			"When standard input is a terminal, cut shows the writes it will make and\n" +
			"asks once before the first. It never tags, publishes, merges, or changes the\n" +
			"target branch. Exits 1 when it refuses (nothing to release, an impact\n" +
			"disagreement, an existing stage branch, a failing configured command, a\n" +
			"path outside release_paths), having written nothing, and 3 when a write\n" +
			"failed after another succeeded, having reported every write it made. If the\n" +
			"push fails, cut deletes the local stage branch it created.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"major", "minor", "patch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			if skipNotes && redoNotes {
				return errors.New("--skip-notes and --redo-notes cannot be combined: one skips authoring, the other repeats it")
			}
			req := cut.Request{
				Dir:    s.root,
				Target: target,
				Config: s.cfg,
				Producers: func(dir string, env []string) map[string]authoring.Producer {
					return producers.Registry(s.cfg.Changelog, dir, env...)
				},
				Redo:          redoNotes,
				Skip:          skipNotes,
				Force:         force,
				NoPullRequest: noPR,
				DryRun:        dryRun,
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
				branch, err := s.cfg.TargetBranch(ctx, gitread.Repo{Dir: req.Dir})
				if errors.Is(err, config.ErrNoDefaultBranch) {
					return fmt.Errorf("no --target given and no target configured: %w", err)
				}
				if err != nil {
					return err
				}
				req.Target = branch
			}
			if s.interactive && !yes && !dryRun {
				req.Confirm = s.confirmCut
			}

			res, err := cut.Cut(ctx, req)
			if err != nil {
				return s.cutFailed(asJSON, res, err)
			}
			if dryRun {
				return s.answer(asJSON, yMap{
					{"version", yStr(res.Version)},
					{"branch", yStr(res.Branch)},
					{"writes", writesNode(res.Writes)},
				})
			}
			return s.answer(asJSON, cutAnswer(res))
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "The release line to cut from: a `branch` on origin (default: the configured target, else the branch origin/HEAD points to)")
	cmd.Flags().BoolVar(&skipNotes, "skip-notes", false, "Stage without an authoring exchange. Requires bump")
	cmd.Flags().BoolVar(&force, "force", false, "Cut the named bump even when nothing releasable exists since the base version. Requires bump")
	cmd.Flags().BoolVar(&redoNotes, "redo-notes", false, "Discard any cached corpus and author again")
	cmd.Flags().BoolVar(&noPR, "no-pull-request", false, "Create and push the stage branch and stop; open no pull request")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report every intended action and change nothing")
	cmd.Flags().BoolVar(&yes, "yes", false, "Do not prompt before writing. Only meaningful interactively")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Render the answer as JSON instead of YAML")
	return cmd
}

// cutAnswer is the command's answer, keys in the contract's order.
func cutAnswer(res cut.Result) yMap {
	commit, pr := yamlNode(yNull()), yamlNode(yNull())
	if res.Commit != "" {
		commit = yStr(res.Commit)
	}
	if res.PullRequest != "" {
		pr = yStr(res.PullRequest)
	}
	return yMap{
		{"version", yStr(res.Version)},
		{"branch", yStr(res.Branch)},
		{"commit", commit},
		{"pull_request", pr},
		{"writes", writesNode(res.Writes)},
	}
}

func writesNode(writes []cut.Write) ySeq {
	out := ySeq{}
	for _, w := range writes {
		out = append(out, yStr(w.String()))
	}
	return out
}

// cutFailed maps a cut's error to its exit status and report. A refusal
// exits 1; a write that failed after another exits 3, answering with every
// write made and printing the commands that finish or discard the release.
func (s *streams) cutFailed(asJSON bool, res cut.Result, err error) error {
	var (
		refused  *cut.Refusal
		none     *cut.NoRelease
		disagree *cut.Disagreement
		rejected *authoring.Rejection
		failed   *producers.Failure
		partial  *cut.WriteFailure
	)
	switch {
	case errors.As(err, &partial):
		if len(partial.Writes) == 0 {
			return partial.Err
		}
		fmt.Fprintf(s.err, "release cut: %v\n", partial.Err)
		s.exitCode = StatusFailedAfterChange
		fmt.Fprintln(s.err, "release cut: these writes were made:")
		for _, w := range partial.Writes {
			fmt.Fprintf(s.err, "  %s\n", w)
		}
		if len(partial.Remedy) > 0 {
			fmt.Fprintln(s.err, "release cut: to finish or discard the release:")
			for _, r := range partial.Remedy {
				fmt.Fprintf(s.err, "  %s\n", r)
			}
		}
		return s.answer(asJSON, cutAnswer(res))
	case errors.As(err, &refused):
		s.exitCode = StatusRefused
		fmt.Fprintf(s.err, "release cut: %s\n", refused.Reason)
		if len(refused.Output) > 0 {
			out := string(refused.Output)
			if !strings.HasSuffix(out, "\n") {
				out += "\n"
			}
			fmt.Fprint(s.err, out)
		}
		return nil
	case errors.As(err, &none), errors.As(err, &disagree), errors.As(err, &rejected), errors.As(err, &failed):
		s.exitCode = StatusRefused
		fmt.Fprintf(s.err, "release cut: %v\n", err)
		return nil
	}
	return err
}

// confirmCut shows the version and the writes a cut will make on standard
// error, and asks once.
func (s *streams) confirmCut(plan cut.Result) (bool, error) {
	fmt.Fprintf(s.err, "release cut: releasing %s from %s (%s). This will:\n", plan.Version, plan.Target, plan.Base[:min(7, len(plan.Base))])
	for _, w := range plan.Writes {
		fmt.Fprintf(s.err, "  %s\n", w)
	}
	fmt.Fprint(s.err, "Proceed? [y/N] ")
	line, err := bufio.NewReader(s.in).ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(s.err)
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}
