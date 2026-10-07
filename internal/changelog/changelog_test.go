package changelog

import "testing"

func TestWebURL(t *testing.T) {
	cases := map[string]string{
		"https://github.com/o/r":       "https://github.com/o/r",
		"https://github.com/o/r.git":   "https://github.com/o/r",
		"https://github.com/o/r/":      "https://github.com/o/r",
		"git@github.com:o/r.git":       "https://github.com/o/r",
		"ssh://git@github.com/o/r.git": "https://github.com/o/r",
		"https://gitlab.com/o/r.git":   "",
		"/srv/git/r.git":               "",
		"https://github.com/o/r/extra": "",
		"  git@github.com:o/r.git\n":   "https://github.com/o/r",
	}
	for remote, want := range cases {
		if got := WebURL(remote); got != want {
			t.Errorf("WebURL(%q) = %q, want %q", remote, got, want)
		}
	}
}
