package cli

import (
	"testing"

	"github.com/brettcallaghan/release/internal/version"
)

func TestRefusalExplainsEveryKindOfNoVersion(t *testing.T) {
	for _, tc := range []struct {
		result version.Result
		want   string
	}{
		{version.Result{Kind: version.NoRelease, Reason: "the computed impact is none"}, "no release is warranted: the computed impact is none"},
		{version.Result{Kind: version.Violation, Rule: "RTS-019", Reason: "1.0.0 needs graduation"}, "the version policy refuses this release (RTS-019): 1.0.0 needs graduation"},
		{version.Result{Kind: version.Violation, Reason: "base is a prerelease"}, "the version policy refuses this release: base is a prerelease"},
	} {
		if got := refusal(tc.result); got != tc.want {
			t.Errorf("refusal(%+v) = %q, want %q", tc.result, got, tc.want)
		}
	}
}
