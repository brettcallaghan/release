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

func TestEntryWithoutKindValidates(t *testing.T) {
	doc := strings.Replace(assembled, `"kind": "FIX",`, ``, 1)
	if doc == assembled {
		t.Fatal("the replacement did not apply; the case tests nothing")
	}
	if err := Changelog([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

func TestCorpusWithOmittedValidates(t *testing.T) {
	doc := strings.Replace(assembled, `"entries"`, `"omitted": [{"hash": "abc", "reason": "a merge commit"}], "entries"`, 1)
	if err := Changelog([]byte(doc)); err != nil {
		t.Fatal(err)
	}
}

func TestOmittedItemWithoutHashIsRefused(t *testing.T) {
	doc := strings.Replace(assembled, `"entries"`, `"omitted": [{"reason": "a merge commit"}], "entries"`, 1)
	if err := Changelog([]byte(doc)); err == nil {
		t.Error("validated; want a refusal")
	}
}

// bundle is a minimal valid evidence bundle.
const bundle = `{
  "schema_version": "1.0.0",
  "purpose": "RELEASE",
  "range": {"to": "0123456789abcdef0123456789abcdef01234567"},
  "commits": [],
  "sources": [],
  "budget": {"limit": 100, "total": 10, "used": 10, "omitted": [
    {"commit": "abc", "path": "go.sum", "reason": "EXCLUDED", "truncated": false}
  ]}
}`

func TestMinimalEvidenceBundleValidates(t *testing.T) {
	if err := Evidence([]byte(bundle)); err != nil {
		t.Fatal(err)
	}
}

func TestEvidenceRefusals(t *testing.T) {
	cases := map[string]string{
		"reason not an OmissionReason": strings.Replace(bundle, `"EXCLUDED"`, `"TOO_BIG"`, 1),
		"unknown purpose":              strings.Replace(bundle, `"RELEASE"`, `"PREVIEW"`, 1),
		"no purpose":                   strings.Replace(bundle, `"purpose": "RELEASE",`, ``, 1),
		"no budget":                    strings.Replace(bundle, `"budget": {"limit": 100, "total": 10, "used": 10, "omitted": [`, `"x": {"omitted": [`, 1),
		"not JSON":                     `{"schema_version":`,
	}
	for name, doc := range cases {
		t.Run(name, func(t *testing.T) {
			if doc == bundle {
				t.Fatal("the replacement did not apply; the case tests nothing")
			}
			if err := Evidence([]byte(doc)); err == nil {
				t.Error("validated; want a refusal")
			}
		})
	}
}
