// Package schemacheck validates a changelog corpus or an evidence bundle
// against the JSON Schema generated from schemas/changelog.strata and
// schemas/evidence.strata: the check the changelog contract requires before
// anything reads an instance. It validates the document as
// written, so a check passes or fails on exactly the bytes a consumer would
// decode.
package schemacheck

import (
	"bytes"
	"fmt"
	"sync"

	"github.com/santhosh-tekuri/jsonschema/v6"

	"github.com/brettcallaghan/release/internal/schema"
)

// The names the embedded schemas are registered under. Nothing is fetched
// from them.
const (
	changelogURL = "release:///changelog.schema.json"
	evidenceURL  = "release:///evidence.schema.json"
)

// lazy compiles one embedded schema once.
type lazy struct {
	once     sync.Once
	compiled *jsonschema.Schema
	err      error
}

func (l *lazy) get(name, url string, raw []byte) (*jsonschema.Schema, error) {
	l.once.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(raw))
		if err != nil {
			l.err = fmt.Errorf("reading the embedded %s schema: %w", name, err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(url, doc); err != nil {
			l.err = fmt.Errorf("loading the embedded %s schema: %w", name, err)
			return
		}
		l.compiled, l.err = c.Compile(url)
	})
	return l.compiled, l.err
}

var (
	changelogSchema lazy
	evidenceSchema  lazy
)

// validate returns nil WHEN data is a JSON document that validates against s.
func validate(s *jsonschema.Schema, what, schemaName string, data []byte) error {
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("the %s is not a JSON document: %w", what, err)
	}
	if err := s.Validate(doc); err != nil {
		return fmt.Errorf("the %s does not validate against the %s schema: %w", what, schemaName, err)
	}
	return nil
}

// Changelog returns nil WHEN data is a JSON document that validates against
// the changelog corpus schema, and otherwise an error naming every violation.
func Changelog(data []byte) error {
	s, err := changelogSchema.get("changelog", changelogURL, schema.Changelog)
	if err != nil {
		return err
	}
	return validate(s, "corpus", "changelog", data)
}

// Evidence returns nil WHEN data is a JSON document that validates against
// the evidence bundle schema, and otherwise an error naming every violation.
func Evidence(data []byte) error {
	s, err := evidenceSchema.get("evidence", evidenceURL, schema.Evidence)
	if err != nil {
		return err
	}
	return validate(s, "evidence bundle", "evidence", data)
}
