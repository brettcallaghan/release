package semver

import (
	"errors"
	"strings"
	"testing"
)

func mustParse(t *testing.T, s string) Version {
	t.Helper()
	v, err := Parse(s)
	if err != nil {
		t.Fatalf("Parse(%q): %v", s, err)
	}
	return v
}

func TestParseFormatRoundTrip(t *testing.T) {
	for _, s := range []string{
		"0.0.0",
		"1.9.0",
		"10.20.30",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-0.3.7",
		"1.0.0-x.7.z.92",
		"1.0.0-x-y-z.--",
		"1.0.0+20130313144700",
		"1.0.0-beta+exp.sha.5114f85",
		"1.0.0+21AF26D3----117B344092BD",
		"1.0.0+001",
		"18446744073709551615.0.0",
	} {
		t.Run(s, func(t *testing.T) {
			if got := mustParse(t, s).String(); got != s {
				t.Errorf("Parse(%q).String() = %q", s, got)
			}
		})
	}
}

func TestParseFields(t *testing.T) {
	v := mustParse(t, "1.2.3-rc.4+build.5")
	if v.Major != 1 || v.Minor != 2 || v.Patch != 3 {
		t.Errorf("core = %d.%d.%d, want 1.2.3", v.Major, v.Minor, v.Patch)
	}
	if strings.Join(v.Prerelease, ".") != "rc.4" || strings.Join(v.Build, ".") != "build.5" {
		t.Errorf("prerelease %v build %v", v.Prerelease, v.Build)
	}
	if !v.IsPrerelease() || mustParse(t, "1.2.3").IsPrerelease() {
		t.Error("IsPrerelease wrong")
	}
}

// AC: AC-VER-004.5
func TestParseRefusesTagPrefix(t *testing.T) {
	_, err := Parse("v1.9.0")
	if !errors.Is(err, ErrInvalid) {
		t.Fatalf("Parse(%q) error = %v, want ErrInvalid", "v1.9.0", err)
	}
	if got := mustParse(t, "1.9.0").String(); got != "1.9.0" {
		t.Errorf("Parse(1.9.0).String() = %q", got)
	}
}

func TestParseRefusesInvalid(t *testing.T) {
	for _, s := range []string{
		"",
		"v1.9.0",
		"V1.9.0",
		"1",
		"1.2",
		"1.2.3.4",
		"01.2.3",
		"1.02.3",
		"1.2.03",
		"1.2.3-",
		"1.2.3+",
		"1.2.3-01",
		"1.2.3-rc..1",
		"1.2.3-rc.1.",
		"1.2.3+build..1",
		"1.2.3-rc_1",
		"1.2.3+bü",
		"-1.2.3",
		"1.-2.3",
		"a.b.c",
		" 1.2.3",
		"1.2.3 ",
		"18446744073709551616.0.0",
	} {
		t.Run(s, func(t *testing.T) {
			if v, err := Parse(s); !errors.Is(err, ErrInvalid) {
				t.Errorf("Parse(%q) = %v, %v; want ErrInvalid", s, v, err)
			}
		})
	}
}

func TestCompareSemverPrecedence(t *testing.T) {
	// Ascending precedence, from the Semantic Versioning 2.0.0 specification
	// section 11, extended with core-version steps.
	ordered := []string{
		"0.9.9",
		"1.0.0-alpha",
		"1.0.0-alpha.1",
		"1.0.0-alpha.beta",
		"1.0.0-beta",
		"1.0.0-beta.2",
		"1.0.0-beta.11",
		"1.0.0-rc.1",
		"1.0.0",
		"1.0.1",
		"1.1.0",
		"2.0.0",
		"10.0.0",
	}
	for i := range ordered {
		for j := range ordered {
			a, b := mustParse(t, ordered[i]), mustParse(t, ordered[j])
			want := cmpInt(i, j)
			if got := Compare(a, b); got != want {
				t.Errorf("Compare(%s, %s) = %d, want %d", a, b, got, want)
			}
			if got := a.Compare(b); got != want {
				t.Errorf("%s.Compare(%s) = %d, want %d", a, b, got, want)
			}
			if got := a.Less(b); got != (want < 0) {
				t.Errorf("%s.Less(%s) = %v", a, b, got)
			}
		}
	}
}

func TestCompareIgnoresBuildMetadata(t *testing.T) {
	pairs := [][2]string{
		{"1.0.0+a", "1.0.0+b"},
		{"1.0.0", "1.0.0+exp.sha.5114f85"},
		{"1.0.0-rc.1+x", "1.0.0-rc.1"},
	}
	for _, p := range pairs {
		if got := Compare(mustParse(t, p[0]), mustParse(t, p[1])); got != 0 {
			t.Errorf("Compare(%s, %s) = %d, want 0", p[0], p[1], got)
		}
	}
}

func TestCompareLongNumericIdentifiers(t *testing.T) {
	a := mustParse(t, "1.0.0-rc.99999999999999999999")
	b := mustParse(t, "1.0.0-rc.100000000000000000000")
	if !a.Less(b) {
		t.Errorf("%s should precede %s", a, b)
	}
}

// AC: AC-VER-004.1
func TestDerivedNames(t *testing.T) {
	v := mustParse(t, "1.9.0")
	if got := TagName("v", v); got != "v1.9.0" {
		t.Errorf("TagName = %q, want v1.9.0", got)
	}
	if got := StageBranchName("cut/", v); got != "cut/1.9.0" {
		t.Errorf("StageBranchName = %q, want cut/1.9.0", got)
	}
}

// AC: AC-VER-004.2
func TestStageBranchNameParsesBack(t *testing.T) {
	v := mustParse(t, "1.9.0")
	got, err := ParseStageBranchName("cut/", StageBranchName("cut/", v))
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "1.9.0" {
		t.Errorf("parsed %q, want 1.9.0", got)
	}
}

// AC: AC-VER-004.3
func TestStageBranchNameUsesConfiguredPrefix(t *testing.T) {
	if got := StageBranchName("release-prep/", mustParse(t, "1.9.0")); got != "release-prep/1.9.0" {
		t.Errorf("StageBranchName = %q, want release-prep/1.9.0", got)
	}
}

// AC: AC-VER-004.9
func TestTagNameUsesConfiguredEmptyPrefix(t *testing.T) {
	if got := TagName("", mustParse(t, "1.9.0")); got != "1.9.0" {
		t.Errorf("TagName = %q, want 1.9.0", got)
	}
}

func TestNameRoundTrips(t *testing.T) {
	versions := []string{"0.0.1", "1.9.0", "2.0.0-rc.3", "1.2.3-beta.1+build.7"}
	forms := []struct {
		name   string
		derive func(string, Version) string
		parse  func(string, string) (Version, error)
	}{
		{"tag", TagName, ParseTagName},
		{"stage", StageBranchName, ParseStageBranchName},
	}
	for _, f := range forms {
		for _, prefix := range []string{"", "v", "cut/", "release-prep/"} {
			for _, s := range versions {
				v := mustParse(t, s)
				name := f.derive(prefix, v)
				back, err := f.parse(prefix, name)
				if err != nil {
					t.Errorf("%s prefix %q: parse(%q): %v", f.name, prefix, name, err)
					continue
				}
				if back.String() != s || Compare(back, v) != 0 {
					t.Errorf("%s prefix %q: %q parsed back to %q, want %q", f.name, prefix, name, back, s)
				}
			}
		}
	}
}

func TestParseNameRefusesWrongPrefix(t *testing.T) {
	cases := []struct {
		parse        func(string, string) (Version, error)
		prefix, name string
	}{
		{ParseTagName, "v", "1.9.0"},
		{ParseTagName, "v", "release-1.9.0"},
		{ParseTagName, "", "v1.9.0"},
		{ParseTagName, "v", "v1.9"},
		{ParseStageBranchName, "cut/", "release-prep/1.9.0"},
		{ParseStageBranchName, "cut/", "cut/v1.9.0"},
		{ParseStageBranchName, "", "cut/1.9.0"},
	}
	for _, c := range cases {
		if v, err := c.parse(c.prefix, c.name); !errors.Is(err, ErrInvalid) {
			t.Errorf("prefix %q name %q: got %v, %v; want ErrInvalid", c.prefix, c.name, v, err)
		}
	}
}

// AC: AC-VER-004.6
func TestPrereleaseForNamesPlannedVersionAndOrdinal(t *testing.T) {
	got, err := PrereleaseFor(mustParse(t, "1.9.0"), 1)
	if err != nil {
		t.Fatal(err)
	}
	if got.String() != "1.9.0-rc.1" {
		t.Errorf("PrereleaseFor(1.9.0, 1) = %q, want 1.9.0-rc.1", got)
	}
	if again := mustParse(t, got.String()); Compare(again, got) != 0 {
		t.Errorf("%q does not parse back to itself", got)
	}
}

// AC: AC-VER-004.7
func TestPrereleaseOrdersBeforePlannedVersion(t *testing.T) {
	planned := mustParse(t, "1.9.0")
	for _, n := range []int{1, 2, 10} {
		pre, err := PrereleaseFor(planned, n)
		if err != nil {
			t.Fatal(err)
		}
		if !pre.Less(planned) {
			t.Errorf("%s should order before %s", pre, planned)
		}
		if prev := mustParse(t, "1.8.99"); !prev.Less(pre) {
			t.Errorf("%s should order after %s", pre, prev)
		}
	}
	rc2, _ := PrereleaseFor(planned, 2)
	rc10, _ := PrereleaseFor(planned, 10)
	if !rc2.Less(rc10) {
		t.Errorf("%s should order before %s", rc2, rc10)
	}
}

func TestPrereleaseForRefuses(t *testing.T) {
	cases := []struct {
		planned string
		n       int
	}{
		{"1.9.0", 0},
		{"1.9.0", -1},
		{"1.9.0-rc.1", 2},
		{"1.9.0+build", 1},
	}
	for _, c := range cases {
		if v, err := PrereleaseFor(mustParse(t, c.planned), c.n); !errors.Is(err, ErrInvalid) {
			t.Errorf("PrereleaseFor(%s, %d) = %v, %v; want ErrInvalid", c.planned, c.n, v, err)
		}
	}
}
