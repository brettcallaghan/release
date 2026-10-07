package schemacheck

import (
	"strings"
	"testing"
)

// assembled is a corpus as the tooling assembles it: no proposed_bump.
const assembled = `{
  "schema_version": "1.0.0",
  "entries": [
    {
      "kind": "FIX",
      "summary": "handle empty input",
      "breaking": false,
      "derived": true,
      "sources": [{"__type": "PullRequest", "number": 11}]
    }
  ],
  "sources": [{"adapter": "COMMITS", "to": "0123456789abcdef0123456789abcdef01234567"}]
}`

func TestAssembledCorpusWithoutProposedBumpValidates(t *testing.T) {
	if err := Changelog([]byte(assembled)); err != nil {
		t.Fatal(err)
	}
}

func TestAuthoredCorpusValidates(t *testing.T) {
	authored := strings.Replace(assembled, `"schema_version": "1.0.0",`, `"schema_version": "1.0.0", "proposed_bump": "PATCH", "version": "1.2.3",`, 1)
	if err := Changelog([]byte(authored)); err != nil {
		t.Fatal(err)
	}
}

func TestRefusals(t *testing.T) {
	cases := map[string]string{
		"unknown adapter":      strings.Replace(assembled, `"COMMITS"`, `"TICKETS"`, 1),
		"adapter run, no head": strings.Replace(assembled, `, "to": "0123456789abcdef0123456789abcdef01234567"`, ``, 1),
		"no sources": strings.Replace(assembled, `,
  "sources": [{"adapter": "COMMITS", "to": "0123456789abcdef0123456789abcdef01234567"}]`, ``, 1),
		"entry without derived": strings.Replace(assembled, `"derived": true,`, ``, 1),
		"unknown bump":          strings.Replace(assembled, `"schema_version": "1.0.0",`, `"schema_version": "1.0.0", "proposed_bump": "HUGE",`, 1),
		"not JSON":              `{"schema_version":`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if doc == assembled {
				t.Fatal("the replacement did not apply; the case tests nothing")
			}
			if err := Changelog([]byte(doc)); err == nil {
				t.Error("validated; want a refusal")
			}
		})
	}
}
