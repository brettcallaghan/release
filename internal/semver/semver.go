// Package semver is the version type the release core speaks: parsing and
// formatting per Semantic Versioning 2.0.0, precedence ordering, and every
// name a release derives from a version (tag, stage branch, prerelease).
//
// The package is pure. It reads no git, filesystem or network, and holds no
// default for any prefix: callers pass the tag prefix and the stage prefix.
package semver

import (
	"errors"
	"fmt"
	"strconv"
	"strings"
)

// ErrInvalid is wrapped by every error that refuses a string as a version or
// as a name derived from one.
var ErrInvalid = errors.New("invalid version")

// Version is a parsed Semantic Versioning 2.0.0 version.
//
// Prerelease and Build hold the dot-separated identifiers in order; both are
// nil when the version has none.
type Version struct {
	Major, Minor, Patch uint64
	Prerelease          []string
	Build               []string
}

// Parse parses MAJOR.MINOR.PATCH[-PRERELEASE][+BUILD]. It refuses any other
// string, including one carrying a tag prefix such as a leading "v": callers
// strip the prefix first, or use ParseTagName.
func Parse(s string) (Version, error) {
	refuse := func(why string) (Version, error) {
		return Version{}, fmt.Errorf("%w %q: %s", ErrInvalid, s, why)
	}
	if s == "" {
		return refuse("empty string")
	}

	rest := s
	var v Version
	if i := strings.IndexByte(rest, '+'); i >= 0 {
		build, err := identifiers(rest[i+1:], false)
		if err != nil {
			return refuse("build metadata: " + err.Error())
		}
		v.Build = build
		rest = rest[:i]
	}
	if i := strings.IndexByte(rest, '-'); i >= 0 {
		pre, err := identifiers(rest[i+1:], true)
		if err != nil {
			return refuse("prerelease: " + err.Error())
		}
		v.Prerelease = pre
		rest = rest[:i]
	}

	core := strings.Split(rest, ".")
	if len(core) != 3 {
		return refuse("want MAJOR.MINOR.PATCH")
	}
	nums := [3]*uint64{&v.Major, &v.Minor, &v.Patch}
	for i, part := range core {
		if !isNumeric(part) {
			return refuse(fmt.Sprintf("%q is not a numeric identifier", part))
		}
		if len(part) > 1 && part[0] == '0' {
			return refuse(fmt.Sprintf("%q has a leading zero", part))
		}
		n, err := strconv.ParseUint(part, 10, 64)
		if err != nil {
			return refuse(fmt.Sprintf("%q is out of range", part))
		}
		*nums[i] = n
	}
	return v, nil
}

// identifiers splits a dot-separated identifier list and validates each.
// Prerelease numeric identifiers may not carry a leading zero; build
// identifiers may.
func identifiers(s string, prerelease bool) ([]string, error) {
	if s == "" {
		return nil, errors.New("empty")
	}
	ids := strings.Split(s, ".")
	for _, id := range ids {
		if id == "" {
			return nil, errors.New("empty identifier")
		}
		for i := 0; i < len(id); i++ {
			if !isIdentChar(id[i]) {
				return nil, fmt.Errorf("%q contains a character outside [0-9A-Za-z-]", id)
			}
		}
		if prerelease && isNumeric(id) && len(id) > 1 && id[0] == '0' {
			return nil, fmt.Errorf("numeric identifier %q has a leading zero", id)
		}
	}
	return ids, nil
}

func isIdentChar(c byte) bool {
	return c == '-' || ('0' <= c && c <= '9') || ('a' <= c && c <= 'z') || ('A' <= c && c <= 'Z')
}

func isNumeric(s string) bool {
	if s == "" {
		return false
	}
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}

// String formats the version. For any version Parse accepted, it returns the
// exact string that was parsed.
func (v Version) String() string {
	var b strings.Builder
	b.WriteString(strconv.FormatUint(v.Major, 10))
	b.WriteByte('.')
	b.WriteString(strconv.FormatUint(v.Minor, 10))
	b.WriteByte('.')
	b.WriteString(strconv.FormatUint(v.Patch, 10))
	if len(v.Prerelease) > 0 {
		b.WriteByte('-')
		b.WriteString(strings.Join(v.Prerelease, "."))
	}
	if len(v.Build) > 0 {
		b.WriteByte('+')
		b.WriteString(strings.Join(v.Build, "."))
	}
	return b.String()
}

// IsPrerelease reports whether the version carries prerelease identifiers.
func (v Version) IsPrerelease() bool { return len(v.Prerelease) > 0 }

// Compare orders a and b by Semantic Versioning precedence: -1 when a
// precedes b, +1 when it follows, 0 when they have equal precedence. Build
// metadata is ignored.
func Compare(a, b Version) int {
	if c := cmpUint(a.Major, b.Major); c != 0 {
		return c
	}
	if c := cmpUint(a.Minor, b.Minor); c != 0 {
		return c
	}
	if c := cmpUint(a.Patch, b.Patch); c != 0 {
		return c
	}
	// A version without a prerelease has higher precedence than one with.
	switch {
	case len(a.Prerelease) == 0 && len(b.Prerelease) == 0:
		return 0
	case len(a.Prerelease) == 0:
		return 1
	case len(b.Prerelease) == 0:
		return -1
	}
	for i := 0; i < len(a.Prerelease) && i < len(b.Prerelease); i++ {
		if c := cmpIdent(a.Prerelease[i], b.Prerelease[i]); c != 0 {
			return c
		}
	}
	return cmpInt(len(a.Prerelease), len(b.Prerelease))
}

// Compare is the method form of Compare(v, o).
func (v Version) Compare(o Version) int { return Compare(v, o) }

// Less reports whether v precedes o.
func (v Version) Less(o Version) bool { return Compare(v, o) < 0 }

// cmpIdent compares two prerelease identifiers: numeric ones numerically,
// alphanumeric ones in ASCII order, and numeric before alphanumeric.
func cmpIdent(a, b string) int {
	an, bn := isNumeric(a), isNumeric(b)
	switch {
	case an && bn:
		// No leading zeros, so the longer digit string is the larger number;
		// comparing this way never overflows.
		if c := cmpInt(len(a), len(b)); c != 0 {
			return c
		}
		return strings.Compare(a, b)
	case an:
		return -1
	case bn:
		return 1
	}
	return strings.Compare(a, b)
}

func cmpUint(a, b uint64) int {
	switch {
	case a < b:
		return -1
	case a > b:
		return 1
	}
	return 0
}

func cmpInt(a, b int) int { return cmpUint(uint64(a), uint64(b)) }

// TagName derives the tag name for v: the tag prefix followed by the
// formatted version. ParseTagName with the same prefix is its inverse.
func TagName(tagPrefix string, v Version) string {
	return tagPrefix + v.String()
}

// ParseTagName parses a tag name derived by TagName with the same prefix
// back to its version.
func ParseTagName(tagPrefix, name string) (Version, error) {
	return parsePrefixed("tag name", tagPrefix, name)
}

// StageBranchName derives the stage branch name for v: the stage prefix
// followed by the formatted version. No tag prefix appears in it.
// ParseStageBranchName with the same prefix is its inverse.
func StageBranchName(stagePrefix string, v Version) string {
	return stagePrefix + v.String()
}

// ParseStageBranchName parses a stage branch name derived by StageBranchName
// with the same prefix back to its version.
func ParseStageBranchName(stagePrefix, name string) (Version, error) {
	return parsePrefixed("stage branch name", stagePrefix, name)
}

func parsePrefixed(form, prefix, name string) (Version, error) {
	rest, ok := strings.CutPrefix(name, prefix)
	if !ok {
		return Version{}, fmt.Errorf("%w: %s %q does not start with prefix %q", ErrInvalid, form, name, prefix)
	}
	v, err := Parse(rest)
	if err != nil {
		return Version{}, fmt.Errorf("%s %q: %w", form, name, err)
	}
	return v, nil
}

// PrereleaseFor derives the prerelease version previewing planned with
// ordinal n: <planned>-rc.<n>. planned must be a release version (no
// prerelease, no build metadata) and n must be at least 1.
func PrereleaseFor(planned Version, n int) (Version, error) {
	if planned.IsPrerelease() || len(planned.Build) > 0 {
		return Version{}, fmt.Errorf("%w: planned version %q must carry no prerelease or build metadata", ErrInvalid, planned)
	}
	if n < 1 {
		return Version{}, fmt.Errorf("%w: prerelease ordinal %d must be at least 1", ErrInvalid, n)
	}
	return Version{
		Major:      planned.Major,
		Minor:      planned.Minor,
		Patch:      planned.Patch,
		Prerelease: []string{"rc", strconv.Itoa(n)},
	}, nil
}
