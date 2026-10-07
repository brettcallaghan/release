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
		Long: "version answers with the base version, the resolved impact, what determined\n" +
			"that impact, and the next version. It is read-only.\n\n" +
			"bump is major, minor or patch, and names the impact instead of inferring\n" +
			"it. Without a bump, the impact is inferred from the commits since the base\n" +
			"version. Without --target, the release line is the configured target, or\n" +
			"else the branch origin/HEAD points to. Exits 1, after printing the answer,\n" +
			"when no release is warranted or the version policy refuses the version.",
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
			answer := versionAnswer(res)
			if asJSON {
				err = writeJSON(s.out, answer)
			} else {
				err = writeYAML(s.out, answer)
			}
			if err != nil {
				return err
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
	return yMap{
		{"base", base},
		{"impact", yStr(strings.ToLower(string(res.Impact)))},
		{"named", yBool(res.Named)},
		{"determined_by", drivers},
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
