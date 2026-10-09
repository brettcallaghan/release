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
	"github.com/brettcallaghan/release/internal/changelog"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/cut"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
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
			"tip in a scratch checkout: it resolves the base version and the signal floor,\n" +
			"authors the changelog from the commits' evidence, suggests the bump, has it\n" +
			"confirmed, writes the rendered section into CHANGELOG.md, runs\n" +
			"commands.propagate to write the version into the manifests (with\n" +
			"RELEASE_VERSION set), refuses a change to any path that is neither\n" +
			"CHANGELOG.md nor in release_paths, and runs commands.validate. Only then\n" +
			"does it write: it creates the stage branch at the fetched tip, commits the\n" +
			"release as \"Release <version>\" with a Release-Type trailer carrying the\n" +
			"confirmed bump, pushes the branch to origin, and opens a pull request into\n" +
			"the target with gh. Your working tree, index and current branch are never\n" +
			"read or changed.\n\n" +
			"bump is major, minor or patch, and is a decision that needs no confirmation;\n" +
			"below the suggestion it proceeds with a warning, and below the signal floor\n" +
			"or over a range with nothing releasable it is refused unless --force is\n" +
			"given. Without a bump the suggested bump must be confirmed: --yes accepts\n" +
			"it; on a terminal cut shows it with its reasons and asks; otherwise cut\n" +
			"halts, answering the suggestion, and exits 1. On a terminal cut also shows\n" +
			"the writes it will make and asks once before the first, unless --yes. It\n" +
			"never tags, publishes, merges, or changes the target branch. Exits 1 when it\n" +
			"refuses or halts, having written nothing, and 3 when a write failed after\n" +
			"another succeeded, having reported every write it made. If the push fails,\n" +
			"cut deletes the local stage branch it created.",
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
				Accepted:      yes,
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
			var halt *cut.Halt
			if errors.As(err, &halt) {
				if !s.interactive || halt.OnlyNamed {
					return s.cutHalted(asJSON, res, halt)
				}
				// The second call reads the authored changelog from the
				// cache and is pinned to the first call's base, so the
				// suggestion confirmed is the one shown: a target that
				// moved meanwhile is a refusal.
				req.Redo = false
				req.ExpectBase = res.Base
				named, accepted, perr := s.promptBump(res)
				if perr != nil {
					return perr
				}
				if !accepted && named == nil {
					s.exitCode = StatusRefused
					fmt.Fprintln(s.err, "release cut: declined; nothing was written")
					return nil
				}
				req.Named, req.Accepted = named, accepted
				res, err = cut.Cut(ctx, req)
			}
			if err != nil {
				return s.cutFailed(asJSON, res, err)
			}
			if d := res.Changelog.Suggestion.Decision; d != nil && d.Warning != nil {
				fmt.Fprintf(s.err, "release cut: warning: %s\n", warning(*d.Warning, res.Changelog.Suggestion))
			}
			if dryRun {
				return s.answer(asJSON, yMap{
					{"version", yStr(res.Version)},
					{"branch", yStr(res.Branch)},
					{"writes", writesNode(res.Writes)},
					{"section", yStr(strings.TrimSpace(res.Changelog.Rendering.Section))},
				})
			}
			return s.answer(asJSON, cutAnswer(res))
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "The release line to cut from: a `branch` on origin (default: the configured target, else the branch origin/HEAD points to)")
	cmd.Flags().BoolVar(&skipNotes, "skip-notes", false, "Author with the mechanical producer in place of an authoring exchange")
	cmd.Flags().BoolVar(&force, "force", false, "Stage the named bump even when it is below the signal floor or the range holds nothing releasable. Requires bump")
	cmd.Flags().BoolVar(&redoNotes, "redo-notes", false, "Discard any cached corpus and author again")
	cmd.Flags().BoolVar(&noPR, "no-pull-request", false, "Create and push the stage branch and stop; open no pull request")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Report every intended action and the changelog section, and change nothing")
	cmd.Flags().BoolVar(&yes, "yes", false, "Do not prompt: accept the suggested bump when none is named, and proceed to the writes")
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
	case errors.As(err, &none), errors.As(err, &rejected), errors.As(err, &failed):
		s.exitCode = StatusRefused
		fmt.Fprintf(s.err, "release cut: %v\n", err)
		return nil
	}
	return err
}

// cutHalted reports a bump awaiting confirmation: the suggestion on standard
// error with how to confirm it, and the answer the contract gives a halt.
func (s *streams) cutHalted(asJSON bool, res cut.Result, halt *cut.Halt) error {
	s.exitCode = StatusRefused
	fmt.Fprintf(s.err, "release cut: %v\n", halt)
	s.showSuggestion(res.Changelog)
	reasons, unclassified := suggestionNodes(res.Changelog)
	path := yamlNode(yNull())
	if res.Changelog.CorpusPath != "" {
		path = yStr(s.display(res.Changelog.CorpusPath))
	}
	return s.answer(asJSON, yMap{
		{"suggested", yStr(lower(halt.Suggestion.Suggestion))},
		{"floor", yStr(lower(halt.Suggestion.Floor))},
		{"reasons", reasons},
		{"raised_by_floor", yBool(halt.Suggestion.Reasons.FloorRaised)},
		{"unclassified", unclassified},
		{"corpus_path", path},
	})
}

// showSuggestion shows a person confirming the bump what it rests on: the
// suggested bump and the entries that drove it, whether the signal floor
// raised it, the floor and the commits that set it, the unclassified
// commits, and the entries that carry no kind.
func (s *streams) showSuggestion(c changelog.Result) {
	sg := c.Suggestion
	fmt.Fprintf(s.err, "  suggested bump: %s\n", lower(sg.Suggestion))
	for _, d := range sg.Reasons.Entries {
		fmt.Fprintf(s.err, "    - %s\n", d.Summary)
	}
	if sg.Reasons.FloorRaised {
		fmt.Fprintf(s.err, "  the signal floor raised it from %s\n", lower(sg.Reasons.FromKinds))
	}
	fmt.Fprintf(s.err, "  signal floor: %s\n", lower(sg.Floor))
	for _, d := range sg.FloorDrivers {
		fmt.Fprintf(s.err, "    - %s %s (%s)\n", d.Commit.Hash, d.Commit.Subject, d.Signal)
	}
	if len(sg.Unclassified) > 0 {
		fmt.Fprintln(s.err, "  unclassified commits:")
		for _, u := range sg.Unclassified {
			fmt.Fprintf(s.err, "    - %s %s\n", u.Hash, u.Subject)
		}
	}
	if sg.Halt != nil && len(sg.Halt.Unkinded) > 0 {
		fmt.Fprintln(s.err, "  entries with no kind:")
		for _, d := range sg.Halt.Unkinded {
			fmt.Fprintf(s.err, "    - %s\n", d.Summary)
		}
	}
}

// promptBump shows the suggestion and asks for the bump, offering the
// suggestion: an empty answer or y accepts it; major, minor or patch names
// a bump; anything else, or the end of input, declines.
func (s *streams) promptBump(res cut.Result) (named *releasemodel.Bump, accepted bool, err error) {
	suggestion := lower(res.Changelog.Suggestion.Suggestion)
	fmt.Fprintln(s.err, "release cut: the bump needs your confirmation.")
	s.showSuggestion(res.Changelog)
	fmt.Fprintf(s.err, "Bump? [%s] (y accepts it, or major, minor, patch) ", suggestion)
	line, err := s.reader().ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(s.err)
		return nil, false, nil
	}
	switch answer := strings.ToLower(strings.TrimSpace(line)); answer {
	case "", "y", "yes":
		return nil, true, nil
	default:
		if b, ok := namedBumps[answer]; ok {
			return &b, false, nil
		}
		return nil, false, nil
	}
}

// confirmCut shows the version and the writes a cut will make on standard
// error, and asks once.
func (s *streams) confirmCut(plan cut.Result) (bool, error) {
	fmt.Fprintf(s.err, "release cut: releasing %s from %s (%s). This will:\n", plan.Version, plan.Target, plan.Base[:min(7, len(plan.Base))])
	for _, w := range plan.Writes {
		fmt.Fprintf(s.err, "  %s\n", w)
	}
	fmt.Fprint(s.err, "Proceed? [y/N] ")
	line, err := s.reader().ReadString('\n')
	if err != nil && line == "" {
		fmt.Fprintln(s.err)
		return false, nil
	}
	answer := strings.ToLower(strings.TrimSpace(line))
	return answer == "y" || answer == "yes", nil
}

// reader is standard input, buffered once, so two prompts in one run share
// what was read ahead.
func (s *streams) reader() *bufio.Reader {
	if s.stdin == nil {
		s.stdin = bufio.NewReader(s.in)
	}
	return s.stdin
}
