// Package changelog is the release core's changelog operation: resolve the
// base version and the signal floor, collect the range's evidence, exchange
// it with the configured producer for a structured changelog, suggest and
// decide the bump, compute the version from it, and render. It composes
// version resolution, the Diff Collector, the Evidence Builder, the
// Changelog Authoring Bridge, the Bump Suggester and the Changelog Renderer
// so that a front end holds no release logic. It writes only the authoring cache; placing the section in
// CHANGELOG.md is the Changelog Writer's, at the caller's request.
package changelog

import (
	"context"
	"regexp"
	"strings"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/bump"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/evidence"
	"github.com/brettcallaghan/release/internal/gitread"
	"github.com/brettcallaghan/release/internal/releasemodel"
	"github.com/brettcallaghan/release/internal/render"
	"github.com/brettcallaghan/release/internal/resolve"
	"github.com/brettcallaghan/release/internal/suggest"
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
	// Named is the bump the maintainer named, or nil to have the suggestion
	// decide.
	Named *releasemodel.Bump
	// Forced accepts a named bump below the signal floor.
	Forced bool
	// Accepted confirms the suggested bump WHEN no bump is named.
	Accepted bool
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
	// NoRelease: nothing in the range is releasable, or the version policy
	// refuses the version; Next says which.
	NoRelease
	// Refused: the named bump is below the signal floor and was not forced.
	// Refusal is set, and no producer was invoked.
	Refused
	// Halted: the bump awaits a person: the suggestion awaits acceptance,
	// or, WHEN Suggestion.Halt.OnlyNamed, only a named bump answers it.
	// Nothing is rendered.
	Halted
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
	// Suggestion is the Bump Suggester's result: the suggested bump with its
	// reasons, and the decision, halt or refusal. Set WHEN a changelog was
	// authored.
	Suggestion suggest.Result
	// Refusal is set for Refused.
	Refusal *suggest.Refusal
	// Bump is the confirmed bump. Set for Prepared.
	Bump releasemodel.Bump
	// Next is the version calculator's answer for the confirmed bump, or,
	// WHEN none was confirmed, version resolution's answer.
	Next version.Result
	// Version is the version, without a tag prefix. Set for Prepared.
	Version string
	// Changelog is the accepted changelog with its version set.
	Changelog releasemodel.Changelog
	// Rendering is the rendered changelog section and release body.
	Rendering render.Rendering
}

// Prepare resolves, collects evidence, authors, decides the bump and
// renders. An error means the repository, the producer or the returned
// changelog failed; an unwarranted release, a refused named bump and a halt
// for confirmation are Results.
func Prepare(ctx context.Context, req Request) (Result, error) {
	repo := gitread.Repo{Dir: req.Dir, Git: req.Git}
	// Pin the target first: the commits read, the range recorded and the
	// cache key must all describe one head, even if the branch moves.
	head, err := repo.CommitHash(ctx, req.Target)
	if err != nil {
		return Result{}, err
	}
	policy := req.Config.Policy()
	res, err := resolve.Resolve(ctx, resolve.Request{
		Dir: req.Dir, Git: req.Git, Target: head, Named: req.Named, Policy: policy,
	})
	if err != nil {
		return Result{}, err
	}
	out := Result{Resolution: res, Next: res.Next}
	if len(res.Commits) == 0 && req.Named == nil {
		out.Outcome = NoRelease
		return out, nil
	}
	drivers := floorDrivers(res)
	// A named bump below the floor is refused before any producer runs: the
	// refusal reads only the floor, so authoring could not change it.
	if res.BelowFloor && !req.Forced {
		out.Outcome = Refused
		out.Refusal = &suggest.Refusal{Named: *req.Named, Floor: res.Floor, FloorDrivers: drivers}
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

	var omitted []releasemodel.OmittedCommit
	if c := out.Authoring.Changelog.Omitted; c != nil {
		omitted = *c
	}
	out.Suggestion = suggest.Suggest(suggest.Input{
		Entries:      out.Authoring.Changelog.Entries,
		Omitted:      omitted,
		Floor:        res.Floor,
		FloorDrivers: drivers,
		Unclassified: res.Unclassified,
		Commits:      len(res.Commits),
		Named:        req.Named,
		Forced:       req.Forced,
		Accepted:     req.Accepted,
	})
	switch out.Suggestion.Outcome {
	case suggest.NotReleasable:
		out.Outcome = NoRelease
		return out, nil
	case suggest.Refused:
		out.Outcome = Refused
		out.Refusal = out.Suggestion.Refusal
		return out, nil
	case suggest.Halted:
		out.Outcome = Halted
		return out, nil
	}

	out.Bump = out.Suggestion.Decision.Bump
	if out.Next, err = resolve.Next(res.Base, out.Bump, false, policy); err != nil {
		return Result{}, err
	}
	if out.Next.Kind != version.Release {
		out.Outcome = NoRelease
		return out, nil
	}
	out.Version = out.Next.Version.String()
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

// floorDrivers is the commits that set the signal floor, with their
// signals, as the Bump Suggester reads them.
func floorDrivers(res resolve.Resolution) []bump.CommitDriver {
	records := make(map[string]releasemodel.CommitRecord, len(res.Commits))
	for _, c := range res.Commits {
		records[c.Hash] = c
	}
	out := make([]bump.CommitDriver, 0, len(res.DeterminedBy))
	for _, d := range res.DeterminedBy {
		out = append(out, bump.CommitDriver{Commit: records[d.Hash], Signal: d.Signal})
	}
	return out
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
