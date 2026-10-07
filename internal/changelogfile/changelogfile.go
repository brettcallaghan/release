// Package changelogfile is the Changelog Writer: it places a rendered
// section into CHANGELOG.md idempotently and touches nothing else.
//
// It is the only release-core component that writes the changelog, and it
// never invokes git: staging and committing belong to its callers.
package changelogfile

import (
	"bytes"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/brettcallaghan/release/internal/semver"
)

// Name is the changelog file's name in the repository root.
const Name = "CHANGELOG.md"

// preamble heads a changelog the writer creates.
const preamble = "# Changelog\n\n"

// Write places section, the rendered section for version, into CHANGELOG.md
// in the repository root dir, creating the file when it does not exist.
func Write(dir, version, section string) error {
	path := filepath.Join(dir, Name)
	mode := fs.FileMode(0o644)
	existing, err := os.ReadFile(path)
	switch {
	case err == nil:
		if info, err := os.Stat(path); err == nil {
			mode = info.Mode().Perm()
		}
	case os.IsNotExist(err):
		existing = nil
	default:
		return err
	}
	out, err := Place(existing, err == nil, version, section)
	if err != nil {
		return err
	}
	return os.WriteFile(path, out, mode)
}

// Place returns the changelog content with section placed in it: replacing
// the section for the same version when one is present, otherwise inserted
// above every existing section. exists is false for a file that does not
// exist, which is created with a "# Changelog" heading.
func Place(content []byte, exists bool, version, section string) ([]byte, error) {
	want, err := semver.Parse(version)
	if err != nil {
		return nil, fmt.Errorf("cannot place a section for %q: %w", version, err)
	}
	if !exists {
		return []byte(preamble + section), nil
	}

	starts := sectionStarts(content)
	for i, s := range starts {
		if s.version.String() != want.String() {
			continue
		}
		end := len(content)
		replacement := section
		if i+1 < len(starts) {
			end = starts[i+1].offset
			replacement += "\n"
		}
		return splice(content, s.offset, end, replacement), nil
	}
	if len(starts) > 0 {
		at := starts[0].offset
		return splice(content, at, at, section+"\n"), nil
	}

	head := string(content)
	if head != "" && !strings.HasSuffix(head, "\n") {
		head += "\n"
	}
	if head != "" && !strings.HasSuffix(head, "\n\n") {
		head += "\n"
	}
	return []byte(head + section), nil
}

func splice(content []byte, from, to int, with string) []byte {
	out := make([]byte, 0, len(content)-(to-from)+len(with))
	out = append(out, content[:from]...)
	out = append(out, with...)
	return append(out, content[to:]...)
}

type sectionStart struct {
	offset  int
	version semver.Version
}

// sectionStarts finds every line that starts a section: a line beginning
// "## " whose first word, after an optional "[" (with its closing "]") and
// an optional "v", parses as a version.
func sectionStarts(content []byte) []sectionStart {
	var out []sectionStart
	for offset := 0; offset < len(content); {
		end := bytes.IndexByte(content[offset:], '\n')
		next := len(content)
		if end >= 0 {
			next = offset + end + 1
		}
		if v, ok := headingVersion(string(content[offset:next])); ok {
			out = append(out, sectionStart{offset: offset, version: v})
		}
		offset = next
	}
	return out
}

func headingVersion(line string) (semver.Version, bool) {
	rest, ok := strings.CutPrefix(line, "## ")
	if !ok {
		return semver.Version{}, false
	}
	words := strings.Fields(rest)
	if len(words) == 0 {
		return semver.Version{}, false
	}
	word := words[0]
	if bracketed, ok := strings.CutPrefix(word, "["); ok {
		word = strings.TrimSuffix(bracketed, "]")
	}
	word = strings.TrimPrefix(word, "v")
	v, err := semver.Parse(word)
	return v, err == nil
}
