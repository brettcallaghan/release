package gitread

import (
	"context"
	"sort"
	"strings"

	"github.com/brettcallaghan/release/internal/semver"
)

// ReleaseTag is a release tag: the tag's name and the version it carries.
type ReleaseTag struct {
	Name    string
	Version semver.Version
}

// ParseReleaseTag reports whether a tag name is a release tag under the tag
// prefix: the prefix followed by a version, or a bare version, where that
// version carries no prerelease and no build metadata.
func ParseReleaseTag(tagPrefix, name string) (ReleaseTag, bool) {
	v, err := semver.ParseTagName(tagPrefix, name)
	if err != nil && tagPrefix != "" {
		v, err = semver.Parse(name)
	}
	if err != nil || v.IsPrerelease() || len(v.Build) > 0 {
		return ReleaseTag{}, false
	}
	return ReleaseTag{Name: name, Version: v}, true
}

// ReleaseTags returns the release tags reachable from ref, in ascending
// version order, one per version. Tags that are not release tags are ignored.
// WHEN two tags carry the same version, the one whose name carries the tag
// prefix is kept.
func (r Repo) ReleaseTags(ctx context.Context, ref, tagPrefix string) ([]ReleaseTag, error) {
	out, err := r.run(ctx, "for-each-ref", "--merged="+ref, "--format=%(refname)", "refs/tags/")
	if err != nil {
		return nil, err
	}
	byVersion := map[string]ReleaseTag{}
	for _, line := range strings.Split(string(out), "\n") {
		name, ok := strings.CutPrefix(line, "refs/tags/")
		if !ok {
			continue
		}
		tag, ok := ParseReleaseTag(tagPrefix, name)
		if !ok {
			continue
		}
		key := tag.Version.String()
		if prev, seen := byVersion[key]; !seen || preferred(tagPrefix, tag.Name, prev.Name) {
			byVersion[key] = tag
		}
	}
	tags := make([]ReleaseTag, 0, len(byVersion))
	for _, t := range byVersion {
		tags = append(tags, t)
	}
	sort.Slice(tags, func(i, j int) bool { return tags[i].Version.Less(tags[j].Version) })
	return tags, nil
}

// preferred reports whether candidate should replace current as the tag for
// a version both carry: a prefixed name wins over a bare one.
func preferred(tagPrefix, candidate, current string) bool {
	return tagPrefix != "" && strings.HasPrefix(candidate, tagPrefix) && !strings.HasPrefix(current, tagPrefix)
}

// LatestReleaseTag returns the highest release tag reachable from ref. found
// is false, with a nil error, WHEN no release tag is reachable: the absence
// is a result, not an error and not a version.
func (r Repo) LatestReleaseTag(ctx context.Context, ref, tagPrefix string) (tag ReleaseTag, found bool, err error) {
	tags, err := r.ReleaseTags(ctx, ref, tagPrefix)
	if err != nil || len(tags) == 0 {
		return ReleaseTag{}, false, err
	}
	return tags[len(tags)-1], true, nil
}
