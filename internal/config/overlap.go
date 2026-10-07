package config

import (
	"path"
	"unicode/utf8"
)

// versionChar reports whether r can appear in a version: the characters of
// a semver core, prerelease or build identifier, and their separators.
func versionChar(r rune) bool {
	return r >= '0' && r <= '9' || r >= 'a' && r <= 'z' || r >= 'A' && r <= 'Z' ||
		r == '.' || r == '-' || r == '+'
}

// versionChars is every rune versionChar accepts.
var versionChars = func() []rune {
	var out []rune
	for r := rune(0); r < utf8.RuneSelf; r++ {
		if versionChar(r) {
			out = append(out, r)
		}
	}
	return out
}()

// globToken is one element of a path.Match pattern: a literal rune, or a
// wildcard ("*", "?" or a "[...]" class) kept as its source text.
type globToken struct {
	lit  rune
	wild string
}

// tokenize splits a well-formed path.Match pattern into tokens.
func tokenize(glob string) []globToken {
	var out []globToken
	for i := 0; i < len(glob); {
		switch glob[i] {
		case '*', '?':
			out = append(out, globToken{wild: glob[i : i+1]})
			i++
		case '[':
			j := i + 1
			for j < len(glob) && glob[j] != ']' {
				if glob[j] == '\\' {
					j++
				}
				j++
			}
			out = append(out, globToken{wild: glob[i : j+1]})
			i = j + 1
		default:
			if glob[i] == '\\' && i+1 < len(glob) {
				i++
			}
			r, size := utf8.DecodeRuneInString(glob[i:])
			out = append(out, globToken{lit: r})
			i += size
		}
	}
	return out
}

// accepts reports whether token t, other than "*", matches the single rune
// r. A wildcard is tested by path.Match itself, so its class syntax and its
// refusal of "/" are path.Match's.
func (t globToken) accepts(r rune) bool {
	if t.wild == "" {
		return t.lit == r
	}
	ok, _ := path.Match(t.wild, string(r))
	return ok
}

// matchesStageName reports whether a well-formed path.Match glob matches
// some name made of stagePrefix followed by one or more version characters.
// Every stage branch name has that form, so a false answer proves the glob
// matches no stage branch, whatever the version.
//
// It walks the product of the glob's tokens and that language: a language
// state below len(prefix) is the number of prefix runes read, and
// len(prefix) and len(prefix)+1 are "prefix read, no version character yet"
// and "at least one version character read".
func matchesStageName(glob, stagePrefix string) bool {
	tokens := tokenize(glob)
	prefix := []rune(stagePrefix)
	inVersion, accepted := len(prefix), len(prefix)+1

	// next returns the runes the language accepts in a state, and the
	// state each leads to.
	next := func(state int) ([]rune, int) {
		if state < inVersion {
			return prefix[state : state+1], state + 1
		}
		return versionChars, accepted
	}

	type pos struct{ token, state int }
	seen := map[pos]bool{}
	stack := []pos{{0, 0}}
	for len(stack) > 0 {
		p := stack[len(stack)-1]
		stack = stack[:len(stack)-1]
		if seen[p] {
			continue
		}
		seen[p] = true
		if p.token == len(tokens) {
			if p.state == accepted {
				return true
			}
			continue
		}
		t := tokens[p.token]
		runes, to := next(p.state)
		if t.wild == "*" {
			stack = append(stack, pos{p.token + 1, p.state})
			for _, r := range runes {
				if r != '/' {
					stack = append(stack, pos{p.token, to})
					break
				}
			}
			continue
		}
		for _, r := range runes {
			if t.accepts(r) {
				stack = append(stack, pos{p.token + 1, to})
				break
			}
		}
	}
	return false
}
