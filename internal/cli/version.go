package cli

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/spf13/cobra"

	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/resolve"
	"github.com/brettcallaghan/release/internal/version"
)

// namedBumps are the values the bump argument accepts.
var namedBumps = map[string]releasemodel.Bump{
	"major": releasemodel.BumpMajor,
	"minor": releasemodel.BumpMinor,
	"patch": releasemodel.BumpPatch,
}

// newVersionCmd is `release version [bump]`: the version a release would
// carry, resolved by the release core with the loaded configuration's
// policy. It writes nothing.
func newVersionCmd(s *streams) *cobra.Command {
	var (
		target string
		asJSON bool
	)
	cmd := &cobra.Command{
		Use:   "version [bump]",
		Short: "Resolve the version a release would carry, without writing anything",
		Long: "version answers with the base version, the resolved impact, the commits that\n" +
			"set the signal floor, the commits that carry no signal, and the next\n" +
			"version. It is read-only and invokes no producer.\n\n" +
			"bump is major, minor or patch, and names the impact instead of using the\n" +
			"signal floor: the highest impact the commits' Release-Type trailers,\n" +
			"conventional types and breaking markers indicate. A named bump over commits\n" +
			"that indicate none prints a warning. A named bump below the signal floor is\n" +
			"refused: exit 1, no answer, and standard error names the floor and the\n" +
			"commits that set it.\n" +
			"Without --target, the release line is the configured target, or else the\n" +
			"branch origin/HEAD points to. Exits 1, after printing the answer, when no\n" +
			"release is warranted or the version policy refuses the version.",
		Args:      cobra.MaximumNArgs(1),
		ValidArgs: []string{"major", "minor", "patch"},
		RunE: func(cmd *cobra.Command, args []string) error {
			req := resolve.Request{Dir: s.root, Target: target, Policy: s.cfg.Policy()}
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

			res, err := resolve.Resolve(ctx, req)
			if err != nil {
				return err
			}
			if res.BelowFloor {
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release version: %s\n", belowFloor(res))
				return nil
			}
			answer := versionAnswer(res)
			if asJSON {
				err = writeJSON(s.out, answer)
			} else {
				err = writeYAML(s.out, answer)
			}
			if err != nil {
				return err
			}
			if res.Named && res.Floor == releasemodel.BumpNone {
				fmt.Fprintf(s.err, "release version: warning: nothing releasable %s; release cut stages the named bump only with --force\n", res.NothingReleasable())
			}
			if res.Next.Kind != version.Release {
				s.exitCode = StatusRefused
				fmt.Fprintf(s.err, "release version: %s\n", refusal(res.Next))
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&target, "target", "", "The release line to compute for: a `branch` (default: the configured target, else the branch origin/HEAD points to)")
	cmd.Flags().BoolVar(&asJSON, "json", false, "Render the answer as JSON instead of YAML")
	return cmd
}

// versionAnswer is the command's answer, keys in the contract's order.
func versionAnswer(res resolve.Resolution) yMap {
	base := yamlNode(yNull())
	if res.Base != nil {
		base = yStr(res.Base.String())
	}
	next := yamlNode(yNull())
	if res.Next.Kind == version.Release {
		next = yStr(res.Next.Version.String())
	}
	drivers := ySeq{}
	for _, d := range res.DeterminedBy {
		drivers = append(drivers, yMap{
			{"hash", yStr(d.Hash)},
			{"subject", yStr(d.Subject)},
			{"signal", yStr(string(d.Signal))},
		})
	}
	unclassified := ySeq{}
	for _, c := range res.Unclassified {
		unclassified = append(unclassified, yMap{
			{"hash", yStr(c.Hash)},
			{"subject", yStr(c.Subject)},
		})
	}
	return yMap{
		{"base", base},
		{"impact", yStr(strings.ToLower(string(res.Impact)))},
		{"named", yBool(res.Named)},
		{"determined_by", drivers},
		{"unclassified", unclassified},
		{"next", next},
	}
}

// refusal explains, on standard error, why no version was answered.
func refusal(r version.Result) string {
	if r.Kind == version.NoRelease {
		return "no release is warranted: " + r.Reason
	}
	if r.Rule != "" {
		return fmt.Sprintf("the version policy refuses this release (%s): %s", r.Rule, r.Reason)
	}
	return "the version policy refuses this release: " + r.Reason
}

// belowFloor explains, on standard error, a named bump below the signal
// floor: the floor and the commits that set it.
func belowFloor(res resolve.Resolution) string {
	var b strings.Builder
	fmt.Fprintf(&b, "the named bump %s is below the signal floor %s, which these commits set:",
		strings.ToLower(string(res.Impact)), strings.ToLower(string(res.Floor)))
	for _, d := range res.DeterminedBy {
		fmt.Fprintf(&b, "\n  %s %s (%s)", d.Hash, d.Subject, d.Signal)
	}
	return b.String()
}
