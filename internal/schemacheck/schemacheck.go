// Package schemacheck validates a changelog corpus against the JSON Schema
// generated from schemas/changelog.strata: the check the changelog contract
// requires before anything reads an instance. It validates the document as
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

// changelogURL is the name the embedded schema is registered under. Nothing
// is fetched from it.
const changelogURL = "release:///changelog.schema.json"

var (
	compileOnce sync.Once
	compiled    *jsonschema.Schema
	compileErr  error
)

func changelogSchema() (*jsonschema.Schema, error) {
	compileOnce.Do(func() {
		doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(schema.Changelog))
		if err != nil {
			compileErr = fmt.Errorf("reading the embedded changelog schema: %w", err)
			return
		}
		c := jsonschema.NewCompiler()
		if err := c.AddResource(changelogURL, doc); err != nil {
			compileErr = fmt.Errorf("loading the embedded changelog schema: %w", err)
			return
		}
		compiled, compileErr = c.Compile(changelogURL)
	})
	return compiled, compileErr
}

// Changelog returns nil WHEN data is a JSON document that validates against
// the changelog corpus schema, and otherwise an error naming every violation.
func Changelog(data []byte) error {
	s, err := changelogSchema()
	if err != nil {
		return err
	}
	doc, err := jsonschema.UnmarshalJSON(bytes.NewReader(data))
	if err != nil {
		return fmt.Errorf("the corpus is not a JSON document: %w", err)
	}
	if err := s.Validate(doc); err != nil {
		return fmt.Errorf("the corpus does not validate against the changelog schema: %w", err)
	}
	return nil
}
