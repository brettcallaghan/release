// Package buildinfo reports the tool's own version, for `release --version`.
package buildinfo

import "runtime/debug"

// version is stamped at release time with
// -ldflags "-X github.com/brettcallaghan/release/internal/buildinfo.version=vX.Y.Z".
var version string

// Version returns the stamped version; else the module version recorded by
// `go install …@vX.Y.Z`; else "devel" for a build from a working tree.
func Version() string {
	if version != "" {
		return version
	}
	if info, ok := debug.ReadBuildInfo(); ok {
		return normalize(info.Main.Version)
	}
	return "devel"
}

// normalize maps the toolchain's placeholder for an unversioned build to
// "devel", so tests and working-tree builds print one stable value.
func normalize(v string) string {
	if v == "" || v == "(devel)" {
		return "devel"
	}
	return v
}
