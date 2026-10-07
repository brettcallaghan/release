package cli

import (
	"context"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/BurntSushi/toml"
	"github.com/spf13/cobra"

	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/gitread"
)

// configCandidates are the paths, relative to the repository root, where the
// Configuration Loader looks for a file when --config is not given.
var configCandidates = []string{"release.toml", "config/release.toml"}

// configRefusal is a configuration the CLI refuses: a file it cannot find
// or decode, or one the release core's validation refuses. Every reason is
// reported.
type configRefusal struct {
	source  string
	reasons []string
}

func (e *configRefusal) Error() string {
	head := "configuration refused"
	if e.source != "" {
		head = "configuration " + e.source + " refused"
	}
	return head + ":\n  - " + strings.Join(e.reasons, "\n  - ")
}

// loadConfig is the Configuration Loader. It finds the configuration file
// for the repository at repo (explicit, when --config names one), decodes it
// over the release core's defaults, and validates it. It returns the
// configuration and the repository root.
func loadConfig(ctx context.Context, repo, explicit string) (config.Config, string, error) {
	if repo == "" {
		repo = "."
	}
	root := repo
	top, found, err := gitread.Repo{Dir: repo}.Toplevel(ctx)
	if err != nil {
		return config.Config{}, "", err
	}
	if found {
		root = top
	}

	cfg := config.Default()
	path, display := explicit, explicit
	if explicit == "" {
		var present []string
		for _, c := range configCandidates {
			if _, err := os.Stat(filepath.Join(root, filepath.FromSlash(c))); err == nil {
				present = append(present, c)
			}
		}
		switch len(present) {
		case 0:
			return cfg, root, nil
		case 1:
			path, display = filepath.Join(root, filepath.FromSlash(present[0])), present[0]
		default:
			return config.Config{}, "", &configRefusal{
				reasons: []string{fmt.Sprintf("both %s and %s exist; keep one, or name one with --config", present[0], present[1])},
			}
		}
	} else if _, err := os.Stat(explicit); err != nil {
		reason := fmt.Sprintf("--config names %s, which does not exist", explicit)
		if !os.IsNotExist(err) {
			reason = fmt.Sprintf("--config names %s, which cannot be read: %v", explicit, err)
		}
		return config.Config{}, "", &configRefusal{source: explicit, reasons: []string{reason}}
	}

	md, err := toml.DecodeFile(path, &cfg)
	if err != nil {
		return config.Config{}, "", &configRefusal{source: display, reasons: []string{err.Error()}}
	}
	var reasons []string
	for _, key := range md.Undecoded() {
		reasons = append(reasons, fmt.Sprintf("%q is not a configuration setting", key.String()))
	}
	var refused *config.Refusal
	if err := cfg.Validate(); errors.As(err, &refused) {
		reasons = append(reasons, refused.Reasons...)
	} else if err != nil {
		return config.Config{}, "", err
	}
	if len(reasons) > 0 {
		return config.Config{}, "", &configRefusal{source: display, reasons: reasons}
	}
	return cfg, root, nil
}

// newConfigCmd is `release config`: the effective configuration, declared
// values over defaults. A refused configuration never reaches it; the
// loader exits 2 first.
func newConfigCmd(s *streams) *cobra.Command {
	var asJSON bool
	cmd := &cobra.Command{
		Use:   "config",
		Short: "Answer with the effective configuration and validate it",
		Long: "config answers with the effective configuration: every setting, with the\n" +
			"values the configuration file declares over the defaults. The file is\n" +
			"release.toml or config/release.toml at the repository root, or the one\n" +
			"--config names. Exits 2 when the configuration is refused.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			ctx := cmd.Context()
			if ctx == nil {
				ctx = context.Background()
			}
			target, err := s.cfg.TargetBranch(ctx, gitread.Repo{Dir: s.root})
			if err != nil {
				return err
			}
			answer := configAnswer(s.cfg, target)
			if asJSON {
				return writeJSON(s.out, answer)
			}
			return writeYAML(s.out, answer)
		},
	}
	cmd.Flags().BoolVar(&asJSON, "json", false, "Render the answer as JSON instead of YAML")
	return cmd
}

// configAnswer is the command's answer, settings in the blueprint's order.
func configAnswer(c config.Config, target string) yMap {
	strs := func(v []string) ySeq {
		out := ySeq{}
		for _, s := range v {
			out = append(out, yStr(s))
		}
		return out
	}
	command := func(v *string) yamlNode {
		if v == nil {
			return yNull()
		}
		return yStr(*v)
	}
	return yMap{
		{"target", yStr(target)},
		{"tag_prefix", yStr(c.TagPrefix)},
		{"stage_prefix", yStr(c.StagePrefix)},
		{"release_branches", strs(c.ReleaseBranches)},
		{"release_paths", strs(c.ReleasePaths)},
		{"leaps", yBool(c.Leaps)},
		{"pre1_remap", yBool(c.Pre1Remap)},
		{"signals", strs(c.Signals)},
		{"commands", yMap{
			{"validate", command(c.Commands.Validate)},
			{"propagate", command(c.Commands.Propagate)},
			{"preflight", strs(c.Commands.Preflight)},
		}},
	}
}
