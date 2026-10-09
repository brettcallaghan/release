// Package changelog is the release core's changelog operation: resolve the
// version, assemble the corpus from the range's commits, exchange it with
// the configured producer, and render it. It composes version resolution,
// the Changelog Corpus Builder, the Changelog Authoring Bridge and the
// Changelog Renderer so that a front end holds no release logic. It writes
// only the authoring cache; placing the section in CHANGELOG.md is the
// Changelog Writer's, at the caller's request.
package changelog

import (
	"context"
	"regexp"
	"strings"

	"github.com/brettcallaghan/release/internal/authoring"
	"github.com/brettcallaghan/release/internal/config"
	"github.com/brettcallaghan/release/internal/corpus"
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
	// Named is the bump the maintainer named, or nil to classify one. A
	// named bump governs, and a disagreeing proposal is advisory.
	Named *releasemodel.Bump
	// Config is the loaded configuration: its policy, changelog settings and
	// adapters.
	Config config.Config
	// Producers maps every producer name to its producer.
	Producers map[string]authoring.Producer
	// CorpusPath, WHEN set, replaces the cache path: the corpus is read from
	// it WHEN it exists and written to it otherwise.
	CorpusPath string
	// Redo discards a cached corpus and authors again.
	Redo bool
	// Skip invokes no producer: the assembled corpus is rendered.
	Skip bool
}

// Outcome is what Prepare concluded.
type Outcome int

const (
	// Prepared: Version, Corpus and Section are set.
	Prepared Outcome = iota
	// NoRelease: the version calculation answered no release, or a policy
	// violation; Resolution.Next says which.
	NoRelease
	// Disagreement: the proposal differs from the computed impact and no
	// bump was named. Nothing is rendered.
	Disagreement
)

// Result is what Prepare concluded.
type Result struct {
	Outcome    Outcome
	Resolution resolve.Resolution
	// Authoring is the exchange's result: how it concluded, the proposal and
	// the computed impact. Unset for NoRelease.
	Authoring authoring.Result
	// CorpusPath is where the authored corpus is cached, or the path the
	// request named. Empty WHEN Skip.
	CorpusPath string
	// Version is the resolved version, without a tag prefix.
	Version string
	// Corpus is the accepted corpus with its version set.
	Corpus releasemodel.Changelog
	// Rendering is the rendered changelog section and release body.
	Rendering render.Rendering
}

// Prepare resolves, assembles, authors and renders. An error means the
// repository, the producer or the returned corpus failed; an unwarranted
// release or a disagreement is a Result.
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

	rng := corpus.Range{To: head}
	if res.BaseTag != "" {
		if rng.From, err = repo.CommitHash(ctx, res.BaseTag); err != nil {
			return Result{}, err
		}
	}
	assembled, err := corpus.Build(corpus.Input{Records: res.Commits, Range: rng, Adapters: assemblyAdapters(req.Config.Adapters)})
	if err != nil {
		return Result{}, err
	}

	if !req.Skip {
		out.CorpusPath = req.CorpusPath
		if out.CorpusPath == "" {
			if out.CorpusPath, err = authoring.CorpusCachePath(ctx, repo, rng.From, rng.To); err != nil {
				return Result{}, err
			}
		}
	}
	out.Authoring, err = authoring.ExchangeCorpus(ctx, authoring.Request{
		Assembled: assembled,
		Computed:  res.Impact,
		Producer:  req.Config.Changelog.Producer,
		Producers: req.Producers,
		CachePath: out.CorpusPath,
		Redo:      req.Redo,
		Skip:      req.Skip,
	})
	if err != nil {
		return Result{}, err
	}
	if out.Authoring.Disagrees && !res.Named {
		out.Outcome = Disagreement
		return out, nil
	}

	out.Version = res.Next.Version.String()
	out.Corpus = out.Authoring.Corpus
	v := out.Version
	out.Corpus.Version = &v
	repoURL, err := webURL(ctx, repo)
	if err != nil {
		return Result{}, err
	}
	if out.Rendering, err = render.Render(out.Corpus, req.Config.Changelog, repoURL); err != nil {
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

// assemblyAdapters is the adapters the corpus assembler is asked for. It
// reads commits only; diffs default on in configuration, and assembly takes
// no diff adapter until the operation authors from evidence.
func assemblyAdapters(a config.Adapters) config.Adapters {
	a.Diffs = false
	return a
}
