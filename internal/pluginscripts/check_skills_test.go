package pluginscripts

import (
	"bytes"
	"errors"
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

// The release standards the cut-release skill carries, by file name.
var standards = []string{
	"repository-release-policy.md",
	"release-tooling-standards.md",
	"cli-distribution-policy.md",
}

// checkSkillsTree copies what scripts/check-skills.sh reads into a
// temporary repository root, so a case can edit a source without touching
// the real one.
func checkSkillsTree(t *testing.T) string {
	t.Helper()
	if runtime.GOOS == "windows" {
		t.Skip("check-skills.sh runs under bash on unix-like systems")
	}
	for _, need := range []string{"bash", "python3"} {
		if _, err := exec.LookPath(need); err != nil {
			t.Skipf("%s is not on the PATH", need)
		}
	}
	repo := filepath.Join("..", "..")
	root := t.TempDir()
	for _, dir := range []string{"scripts", "plugin", ".claude-plugin", "schemas", "specs/contracts", "specs/standards"} {
		copyTree(t, filepath.Join(repo, dir), filepath.Join(root, dir))
	}
	return root
}

func runCheckSkills(t *testing.T, root string) (string, int) {
	t.Helper()
	cmd := exec.Command("bash", filepath.Join(root, "scripts", "check-skills.sh"), "--strict")
	var out bytes.Buffer
	cmd.Stdout, cmd.Stderr = &out, &out
	err := cmd.Run()
	var exitErr *exec.ExitError
	switch {
	case err == nil:
		return out.String(), 0
	case errors.As(err, &exitErr):
		return out.String(), exitErr.ExitCode()
	}
	t.Fatalf("running check-skills.sh: %v", err)
	return "", 0
}

// AC: AC-PLG-006.1
func TestCutReleaseCarriesTheStandardsByteForByte(t *testing.T) {
	for _, name := range standards {
		source, err := os.ReadFile(filepath.Join("..", "..", "specs", "standards", name))
		if err != nil {
			t.Fatal(err)
		}
		copied, err := os.ReadFile(filepath.Join(pluginRoot, "skills", "cut-release", "references", name))
		if err != nil {
			t.Fatalf("the skill does not carry %s: %v", name, err)
		}
		if !bytes.Equal(source, copied) {
			t.Errorf("references/%s differs from specs/standards/%s", name, name)
		}
	}
	root := checkSkillsTree(t)
	if out, code := runCheckSkills(t, root); code != 0 {
		t.Errorf("check-skills.sh --strict exit = %d, want 0:\n%s", code, out)
	}
}

// AC: AC-PLG-006.2
func TestStandardEditedWithoutItsCopyFailsNamingTheFile(t *testing.T) {
	for _, name := range standards {
		t.Run(name, func(t *testing.T) {
			root := checkSkillsTree(t)
			source := filepath.Join(root, "specs", "standards", name)
			f, err := os.OpenFile(source, os.O_APPEND|os.O_WRONLY, 0)
			if err != nil {
				t.Fatal(err)
			}
			_, _ = f.WriteString("\nAn edit made to the source alone.\n")
			_ = f.Close()

			out, code := runCheckSkills(t, root)
			if code != 1 {
				t.Errorf("check-skills.sh --strict exit = %d, want 1:\n%s", code, out)
			}
			copyPath := "plugin/skills/cut-release/references/" + name
			if !strings.Contains(out, "ERROR "+copyPath+": differs from specs/standards/"+name) {
				t.Errorf("output does not name %s as differing from its source:\n%s", copyPath, out)
			}
		})
	}
}

// copyTree copies files and directories, keeping modes; symlinks are not
// followed or copied.
func copyTree(t *testing.T, src, dst string) {
	t.Helper()
	err := filepath.Walk(src, func(p string, info fs.FileInfo, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(src, p)
		target := filepath.Join(dst, rel)
		switch {
		case info.IsDir():
			return os.MkdirAll(target, 0o755)
		case info.Mode().IsRegular():
			data, err := os.ReadFile(p)
			if err != nil {
				return err
			}
			return os.WriteFile(target, data, info.Mode().Perm())
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
