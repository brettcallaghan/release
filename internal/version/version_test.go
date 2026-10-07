package version

import (
	"go/build"
	"strings"
	"testing"

	"github.com/brettcallaghan/release/internal/semver"
)

func v(t *testing.T, s string) *semver.Version {
	t.Helper()
	parsed, err := semver.Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return &parsed
}

// describe renders a Result as "1.2.3", "no-release" or "violation", so the
// exhaustive table can state each expectation in one cell.
func describe(r Result) string {
	switch r.Kind {
	case Release:
		return r.Version.String()
	case NoRelease:
		return "no-release"
	}
	return "violation"
}

var impacts = []Impact{ImpactNone, ImpactPatch, ImpactMinor, ImpactMajor}

// TestNextExhaustive covers the cross product of impact × base regime
// (absent, pre-1.0, post-1.0) × graduation × pre1_remap × leaps. Each row
// gives the expectation per impact in the order none, patch, minor, major;
// leaps must not change any computed version, so both settings are checked
// against the same row.
func TestNextExhaustive(t *testing.T) {
	rows := []struct {
		base      string // "" means absent
		graduate  bool
		pre1Remap bool
		want      [4]string
	}{
		{"", false, true, [4]string{"no-release", "0.0.1", "0.1.0", "0.1.0"}},
		{"", false, false, [4]string{"no-release", "0.0.1", "0.1.0", "violation"}},
		{"", true, true, [4]string{"1.0.0", "1.0.0", "1.0.0", "1.0.0"}},
		{"", true, false, [4]string{"1.0.0", "1.0.0", "1.0.0", "1.0.0"}},

		{"0.6.4", false, true, [4]string{"no-release", "0.6.5", "0.7.0", "0.7.0"}},
		{"0.6.4", false, false, [4]string{"no-release", "0.6.5", "0.7.0", "violation"}},
		{"0.6.4", true, true, [4]string{"1.0.0", "1.0.0", "1.0.0", "1.0.0"}},
		{"0.6.4", true, false, [4]string{"1.0.0", "1.0.0", "1.0.0", "1.0.0"}},

		{"1.8.3", false, true, [4]string{"no-release", "1.8.4", "1.9.0", "2.0.0"}},
		{"1.8.3", false, false, [4]string{"no-release", "1.8.4", "1.9.0", "2.0.0"}},
		{"1.8.3", true, true, [4]string{"violation", "violation", "violation", "violation"}},
		{"1.8.3", true, false, [4]string{"violation", "violation", "violation", "violation"}},
	}
	for _, row := range rows {
		for i, impact := range impacts {
			for _, leaps := range []bool{false, true} {
				req := Request{Impact: impact, Graduate: row.graduate, Policy: Policy{Leaps: leaps, Pre1Remap: row.pre1Remap}}
				if row.base != "" {
					req.Base = v(t, row.base)
				}
				got := Next(req)
				if describe(got) != row.want[i] {
					t.Errorf("Next(base=%q impact=%s graduate=%v remap=%v leaps=%v) = %s %+v, want %s",
						row.base, impact, row.graduate, row.pre1Remap, leaps, describe(got), got, row.want[i])
				}
				if got.Kind == Violation && got.Reason == "" {
					t.Errorf("violation without a reason: %+v", got)
				}
			}
		}
	}
}

// AC: AC-VER-001.6
func TestFirstReleaseComputedFromZero(t *testing.T) {
	got := Next(Request{Impact: ImpactMinor, Policy: Policy{Pre1Remap: true}})
	if describe(got) != "0.1.0" {
		t.Errorf("Next(no base, minor) = %s, want 0.1.0", describe(got))
	}
}

// AC: AC-VER-003.1
func TestImpactIncrementsExactlyOneComponent(t *testing.T) {
	want := map[Impact]string{ImpactMajor: "2.0.0", ImpactMinor: "1.9.0", ImpactPatch: "1.8.4"}
	for impact, w := range want {
		if got := Next(Request{Base: v(t, "1.8.3"), Impact: impact}); describe(got) != w {
			t.Errorf("Next(1.8.3, %s) = %s, want %s", impact, describe(got), w)
		}
	}
}

// AC: AC-VER-003.2
func TestLeapRefusedWithoutPolicy(t *testing.T) {
	got := CheckProposed(v(t, "1.8.3"), *v(t, "1.10.0"), false, Policy{})
	if got.Kind != Violation || got.Rule != "RTS-015" {
		t.Errorf("CheckProposed(1.8.3, 1.10.0, no leaps) = %+v, want an RTS-015 violation", got)
	}
}

// AC: AC-VER-003.3
func TestLeapAcceptedWithPolicy(t *testing.T) {
	got := CheckProposed(v(t, "1.8.3"), *v(t, "1.10.0"), false, Policy{Leaps: true})
	if describe(got) != "1.10.0" {
		t.Errorf("CheckProposed(1.8.3, 1.10.0, leaps) = %+v, want accepted", got)
	}
}

// AC: AC-VER-003.4
func TestPre1ImpactMapping(t *testing.T) {
	want := map[Impact]string{ImpactMajor: "0.7.0", ImpactMinor: "0.7.0", ImpactPatch: "0.6.5"}
	for impact, w := range want {
		got := Next(Request{Base: v(t, "0.6.4"), Impact: impact, Policy: Policy{Pre1Remap: true}})
		if describe(got) != w {
			t.Errorf("Next(0.6.4, %s) = %s, want %s", impact, describe(got), w)
		}
	}
}

// AC: AC-VER-003.5
func TestGraduationProduces1(t *testing.T) {
	got := Next(Request{Base: v(t, "0.6.4"), Graduate: true, Policy: Policy{Pre1Remap: true}})
	if describe(got) != "1.0.0" {
		t.Errorf("Next(0.6.4, graduate) = %s, want 1.0.0", describe(got))
	}
}

// AC: AC-VER-003.6
func TestImpactAloneNeverProduces1(t *testing.T) {
	for _, remap := range []bool{true, false} {
		got := Next(Request{Base: v(t, "0.6.4"), Impact: ImpactMajor, Policy: Policy{Pre1Remap: remap}})
		if got.Kind == Release && got.Version.Major == 1 {
			t.Errorf("Next(0.6.4, major, remap=%v) = %s, an impact alone produced a major of 1", remap, got.Version)
		}
	}
}

// AC: AC-VER-003.7
func TestNoImpactNoRelease(t *testing.T) {
	for _, b := range []*semver.Version{nil, v(t, "0.6.4"), v(t, "1.8.3")} {
		got := Next(Request{Base: b, Impact: ImpactNone, Policy: Policy{Pre1Remap: true}})
		if got.Kind != NoRelease {
			t.Errorf("Next(%v, none) = %+v, want no-release", b, got)
		}
		if got.Version.String() != "0.0.0" {
			t.Errorf("no-release result carries a version: %s", got.Version)
		}
	}
}

func TestGraduationAbove1IsViolation(t *testing.T) {
	got := Next(Request{Base: v(t, "1.2.0"), Impact: ImpactMinor, Graduate: true})
	if got.Kind != Violation || got.Rule != "RTS-019" {
		t.Errorf("Next(1.2.0, graduate) = %+v, want an RTS-019 violation", got)
	}
}

func TestPre1MajorWithRemapOffIsViolation(t *testing.T) {
	got := Next(Request{Base: v(t, "0.4.0"), Impact: ImpactMajor, Policy: Policy{Pre1Remap: false}})
	if got.Kind != Violation || got.Rule != "RTS-019" {
		t.Errorf("Next(0.4.0, major, remap off) = %+v, want an RTS-019 violation", got)
	}
	if got := Next(Request{Base: v(t, "0.4.0"), Impact: ImpactMajor, Policy: Policy{Pre1Remap: true}}); describe(got) != "0.5.0" {
		t.Errorf("Next(0.4.0, major, remap on) = %s, want 0.5.0", describe(got))
	}
}

func TestNonReleaseBaseIsViolation(t *testing.T) {
	for _, b := range []string{"1.8.3-rc.1", "1.8.3+build"} {
		if got := Next(Request{Base: v(t, b), Impact: ImpactPatch}); got.Kind != Violation {
			t.Errorf("Next(%s) = %+v, want violation", b, got)
		}
		if got := CheckProposed(v(t, b), *v(t, "1.8.4"), false, Policy{}); got.Kind != Violation {
			t.Errorf("CheckProposed(%s) = %+v, want violation", b, got)
		}
	}
}

func TestUnknownImpactIsViolation(t *testing.T) {
	if got := Next(Request{Impact: Impact(9)}); got.Kind != Violation || !strings.Contains(got.Reason, "Impact(9)") {
		t.Errorf("Next(Impact(9)) = %+v", got)
	}
}

func TestCheckProposed(t *testing.T) {
	cases := []struct {
		base, proposed string
		graduate       bool
		leaps          bool
		want           string // accepted version, or "violation"
	}{
		// single steps, either leap setting
		{"1.8.3", "1.8.4", false, false, "1.8.4"},
		{"1.8.3", "1.9.0", false, false, "1.9.0"},
		{"1.8.3", "2.0.0", false, false, "2.0.0"},
		{"1.8.3", "2.0.0", false, true, "2.0.0"},
		{"0.6.4", "0.7.0", false, false, "0.7.0"},
		{"", "0.1.0", false, false, "0.1.0"},
		{"", "0.0.1", false, false, "0.0.1"},
		// leaps
		{"1.8.3", "1.8.5", false, false, "violation"},
		{"1.8.3", "1.8.5", false, true, "1.8.5"},
		{"1.8.3", "3.0.0", false, false, "violation"},
		{"1.8.3", "3.0.0", false, true, "3.0.0"},
		{"0.6.4", "0.9.0", false, true, "0.9.0"},
		// not following the base, or not resetting lower components
		{"1.8.3", "1.8.3", false, true, "violation"},
		{"1.8.3", "1.8.2", false, true, "violation"},
		{"1.8.3", "1.7.9", false, true, "violation"},
		{"1.8.3", "1.9.3", false, true, "violation"},
		{"1.8.3", "2.1.0", false, true, "violation"},
		{"", "0.0.0", false, true, "violation"},
		// graduation and RTS-019
		{"0.6.4", "1.0.0", false, false, "violation"},
		{"0.6.4", "1.0.0", false, true, "violation"},
		{"0.6.4", "2.0.0", false, true, "violation"},
		{"0.6.4", "1.0.0", true, false, "1.0.0"},
		{"", "1.0.0", true, false, "1.0.0"},
		{"0.6.4", "0.7.0", true, false, "violation"},
		{"0.6.4", "2.0.0", true, true, "violation"},
		{"1.8.3", "2.0.0", true, false, "violation"},
		// not a release version
		{"1.8.3", "1.9.0-rc.1", false, true, "violation"},
		{"1.8.3", "1.9.0+meta", false, true, "violation"},
	}
	for _, c := range cases {
		var b *semver.Version
		if c.base != "" {
			b = v(t, c.base)
		}
		got := CheckProposed(b, *v(t, c.proposed), c.graduate, Policy{Leaps: c.leaps})
		if describe(got) != c.want {
			t.Errorf("CheckProposed(base=%q, %s, graduate=%v, leaps=%v) = %+v, want %s",
				c.base, c.proposed, c.graduate, c.leaps, got, c.want)
		}
		if got.Kind == Violation && got.Reason == "" {
			t.Errorf("violation without a reason: %+v", got)
		}
	}
}

func TestPrereleaseThroughSemverCore(t *testing.T) {
	planned := v(t, "1.9.0")
	pre, err := Prerelease(*planned, 1)
	if err != nil {
		t.Fatal(err)
	}
	if pre.String() != "1.9.0-rc.1" || !pre.Less(*planned) {
		t.Errorf("Prerelease(1.9.0, 1) = %s, want 1.9.0-rc.1 ordering before 1.9.0", pre)
	}
	if _, err := Prerelease(*planned, 0); err == nil {
		t.Error("Prerelease(1.9.0, 0) succeeded")
	}
}

func TestStrings(t *testing.T) {
	for want, got := range map[string]string{
		"none": ImpactNone.String(), "patch": ImpactPatch.String(), "minor": ImpactMinor.String(),
		"major": ImpactMajor.String(), "Impact(7)": Impact(7).String(),
		"release": Release.String(), "no-release": NoRelease.String(), "violation": Violation.String(),
		"Kind(5)": Kind(5).String(),
	} {
		if got != want {
			t.Errorf("String() = %q, want %q", got, want)
		}
	}
}

// The calculator declares its own impact type and must not depend on any
// package generated from schemas/*.strata.
func TestImportsNoGeneratedSchemaPackage(t *testing.T) {
	pkg, err := build.ImportDir(".", 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range pkg.Imports {
		if strings.HasPrefix(imp, "github.com/brettcallaghan/release/") && imp != "github.com/brettcallaghan/release/internal/semver" {
			t.Errorf("internal/version imports %s; it may import only internal/semver from this module", imp)
		}
	}
}
