// Package changelog is the release core's changelog operation: resolve the
// version, collect the range's evidence, exchange it with the configured
// producer for a structured changelog, and render it. It composes version
// resolution, the Diff Collector, the Evidence Builder, the Changelog
// Authoring Bridge and the Changelog Renderer so that a front end holds no
// release logic. It writes only the authoring cache; placing the section in
// CHANGELOG.md is the Changelog Writer's, at the caller's request.
package changelog

import (
	"context"
	"regexp"
	"strings"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/evidence"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/render"
	"github.com/brettcallaghan/release/internal/resolve"
	"github.com/brettcallaghan/release/internal/version"
)

// Request is the input to Prepare.
type Request struct {
	// Dir is the repository's working directory.
	Dir string
	// Git is the git executable; empty means "git" from PATH.
	Git string
	// Target is the release line's ref.
	Target string
	// Named is the bump the maintainer named, or nil to classify one.
	Named *releasemodel.Bump
	// Config is the loaded configuration: its policy, changelog settings and
	// adapters.
	Config config.Config
	// Producers maps every producer name to its producer.
	Producers map[string]authoring.Producer
	// CorpusPath, WHEN set, replaces the cache path: the changelog is read
	// from it WHEN it exists and written to it otherwise.
	CorpusPath string
	// Redo discards a cached changelog and authors again.
	Redo bool
	// Skip invokes no configured producer: the mechanical producer composes
	// the changelog, and no cache is read or written.
	Skip bool
}

// Outcome is what Prepare concluded.
type Outcome int

const (
	// Prepared: Version, Changelog and Rendering are set.
	Prepared Outcome = iota
	// NoRelease: the version calculation answered no release, or a policy
	// violation; Resolution.Next says which.
	NoRelease
)

// Result is what Prepare concluded.
type Result struct {
	Outcome    Outcome
	Resolution resolve.Resolution
	// Evidence is the bundle the producer was given. Unset for NoRelease.
	Evidence releasemodel.Evidence
	// Authoring is the exchange's result: the accepted changelog, the
	// producer that authored it and how it was obtained. Unset for
	// NoRelease.
	Authoring authoring.EvidenceResult
	// CorpusPath is where the authored changelog is cached, or the path the
	// request named. Empty WHEN Skip.
	CorpusPath string
	// Version is the resolved version, without a tag prefix.
	Version string
	// Changelog is the accepted changelog with its version set.
	Changelog releasemodel.Changelog
	// Rendering is the rendered changelog section and release body.
	Rendering render.Rendering
}

// Prepare resolves, collects evidence, authors and renders. An error means
// the repository, the producer or the returned changelog failed; an
// unwarranted release is a Result.
func Prepare(ctx context.Context, req Request) (Result, error) {
	repo := gitread.Repo{Dir: req.Dir, Git: req.Git}
	// Pin the target first: the commits read, the range recorded and the
	// cache key must all describe one head, even if the branch moves.
	head, err := repo.CommitHash(ctx, req.Target)
	if err != nil {
		return Result{}, err
	}
	res, err := resolve.Resolve(ctx, resolve.Request{
		Dir: req.Dir, Git: req.Git, Target: head, Named: req.Named, Policy: req.Config.Policy(),
	})
	if err != nil {
		return Result{}, err
	}
	out := Result{Resolution: res}
	if res.Next.Kind != version.Release {
		out.Outcome = NoRelease
		return out, nil
	}

	in := evidence.Input{
		Range:   evidence.Range{To: head},
		Commits: res.Commits,
		Budget:  releasemodel.BudgetReport{Limit: req.Config.Changelog.DiffBudget.Release},
		Signals: map[string]releasemodel.Bump{},
	}
	if res.BaseTag != "" {
		if in.Range.From, err = repo.CommitHash(ctx, res.BaseTag); err != nil {
			return Result{}, err
		}
		in.BaseVersion = res.Base.String()
	}
	for _, s := range res.Signals {
		if s.Classified {
			in.Signals[s.Commit.Hash] = s.Impact
		}
	}
	if req.Config.Adapters.Diffs {
		// Oldest first, the order the bundle lists them in, so the budget
		// is shared out in the order a reader meets the commits.
		hashes := make([]string, len(res.Commits))
		for i, c := range res.Commits {
			hashes[len(hashes)-1-i] = c.Hash
		}
		in.DiffsCollected = true
		if in.Diffs, in.Budget, err = repo.Diffs(ctx, hashes, req.Config.Changelog.DiffExclude, req.Config.Changelog.DiffBudget.Release); err != nil {
			return Result{}, err
		}
	}
	if out.Evidence, err = evidence.Build(in); err != nil {
		return Result{}, err
	}

	if !req.Skip {
		out.CorpusPath = req.CorpusPath
		if out.CorpusPath == "" {
			if out.CorpusPath, err = authoring.ChangelogCachePath(ctx, repo, in.Range.From, in.Range.To); err != nil {
				return Result{}, err
			}
		}
	}
	out.Authoring, err = authoring.ExchangeEvidence(ctx, authoring.EvidenceRequest{
		Evidence:  out.Evidence,
		Producer:  req.Config.Changelog.Producer,
		Producers: req.Producers,
		CachePath: out.CorpusPath,
		Redo:      req.Redo,
		Skip:      req.Skip,
	})
	if err != nil {
		return Result{}, err
	}

	out.Version = res.Next.Version.String()
	out.Changelog = out.Authoring.Changelog
	v := out.Version
	out.Changelog.Version = &v
	repoURL, err := webURL(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	if out.Rendering, err = render.Render(out.Changelog, req.Config.Changelog, repoURL); err != nil {
		return Result{}, err
	}
	out.Outcome = Prepared
	return out, nil
}

// remotePatterns recognise a GitHub remote over HTTPS or SSH.
var remotePatterns = []*regexp.Regexp{
	regexp.MustCompile(`^https://github\.com/([^/]+)/([^/]+?)(?:\.git)?/?$`),
	regexp.MustCompile(`^(?:ssh://)?git@github\.com[:/]([^/]+)/([^/]+?)(?:\.git)?/?$`),
}

// webURL is the repository's web URL, derived from its origin remote, so
// rendered sources link to it. It is empty, and sources render as plain
// text, WHEN there is no origin or it is not a GitHub remote.
func webURL(ctx context.Context, repo gitread.Repo) (string, error) {
	remote, found, err := repo.RemoteURL(ctx, "origin")
	if err != nil || !found {
		return "", err
	}
	return WebURL(remote), nil
}

// WebURL maps a GitHub remote URL to the repository's web URL, and any
// other remote to "".
func WebURL(remote string) string {
	remote = strings.TrimSpace(remote)
	for _, p := range remotePatterns {
		if m := p.FindStringSubmatch(remote); m != nil {
			return "https://github.com/" + m[1] + "/" + m[2]
		}
	}
	return ""
}
