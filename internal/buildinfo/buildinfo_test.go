package buildinfo

import "testing"

func TestNormalize(t *testing.T) {
	for _, tc := range []struct{ in, want string }{
		{"", "devel"},
		{"(devel)", "devel"},
		{"v1.2.3", "v1.2.3"},
		{"v0.0.0-20261007000000-abcdef123456", "v0.0.0-20261007000000-abcdef123456"},
	} {
		if got := normalize(tc.in); got != tc.want {
			t.Errorf("normalize(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestVersionPrefersTheStampedValue(t *testing.T) {
	saved := version
	t.Cleanup(func() { version = saved })

	version = "v9.8.7"
	if got := Version(); got != "v9.8.7" {
		t.Errorf("Version() = %q, want the stamped v9.8.7", got)
	}
}

func TestVersionWithoutAStampIsNeverEmpty(t *testing.T) {
	saved := version
	t.Cleanup(func() { version = saved })

	version = ""
	if got := Version(); got == "" {
		t.Error("Version() returned an empty string")
	}
}
