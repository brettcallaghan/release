package cli

import (
	"io/fs"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/BurntSushi/toml"
)

// repoRoot is the repository this package's tests read the specs and the
// other packages' tests from.
const repoRoot = "../.."

var (
	// declaredAC is an acceptance criterion as an FRD declares it: a
	// Gherkin tag on its scenario.
	declaredAC = regexp.MustCompile(`@(AC-[A-Z0-9]+-[0-9]+\.[0-9]+)\b`)
	// acID is an acceptance criterion id wherever a citation names one.
	acID = regexp.MustCompile(`\bAC-[A-Z0-9]+-[0-9]+\.[0-9]+\b`)
	// acComment is the comment a Go test cites its criteria in.
	acComment = regexp.MustCompile(`//\s*AC:(.*)`)
)

// TestEveryCitedCriterionIsDeclared fails when a snapshot case or a Go test
// cites an acceptance criterion that no FRD under specs/features declares,
// so a criterion removed from the specs cannot leave a citation naming it.
// slug#anchor citations are not checked; resolving anchors needs the corpus
// tooling.
func TestEveryCitedCriterionIsDeclared(t *testing.T) {
	declared := declaredCriteria(t)

	cases, err := filepath.Glob(filepath.Join(casesDir, "*", "case.toml"))
	if err != nil {
		t.Fatal(err)
	}
	if len(cases) == 0 {
		t.Fatalf("no case.toml under %s", casesDir)
	}
	for _, path := range cases {
		var m caseManifest
		if _, err := toml.DecodeFile(path, &m); err != nil {
			t.Fatalf("reading %s: %v", path, err)
		}
		for _, c := range m.Cites {
			if acID.MatchString(c) && !declared[c] {
				t.Errorf("%s cites %s, which no specs/features/*/*-frd.md declares", rel(path), c)
			}
		}
	}

	err = filepath.WalkDir(repoRoot, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if path != repoRoot && strings.HasPrefix(d.Name(), ".") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, "_test.go") {
			return nil
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		for i, line := range strings.Split(string(data), "\n") {
			comment := acComment.FindStringSubmatch(line)
			if comment == nil {
				continue
			}
			for _, id := range acID.FindAllString(comment[1], -1) {
				if !declared[id] {
					t.Errorf("%s:%d cites %s, which no specs/features/*/*-frd.md declares", rel(path), i+1, id)
				}
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

// declaredCriteria reads every acceptance criterion id the FRDs declare.
func declaredCriteria(t *testing.T) map[string]bool {
	t.Helper()
	frds, err := filepath.Glob(filepath.Join(repoRoot, "specs", "features", "*", "*-frd.md"))
	if err != nil {
		t.Fatal(err)
	}
	if len(frds) == 0 {
		t.Fatal("no specs/features/*/*-frd.md; the test must run inside the repository")
	}
	declared := map[string]bool{}
	for _, frd := range frds {
		data, err := os.ReadFile(frd)
		if err != nil {
			t.Fatal(err)
		}
		for _, m := range declaredAC.FindAllStringSubmatch(string(data), -1) {
			declared[m[1]] = true
		}
	}
	if len(declared) == 0 {
		t.Fatal("the FRDs declare no @AC- tags")
	}
	return declared
}

func rel(path string) string {
	if r, err := filepath.Rel(repoRoot, path); err == nil {
		return r
	}
	return path
}
