package cli

import (
	"os"
	"path/filepath"
	"testing"
)

// TestDisplayResolvesSymlinks covers a path named through a symbolic link to
// the repository root, as macOS's /var is a link to /private/var while git
// reports the root resolved.
func TestDisplayResolvesSymlinks(t *testing.T) {
	dir := t.TempDir()
	real := filepath.Join(dir, "real")
	if err := os.Mkdir(real, 0o755); err != nil {
		t.Fatal(err)
	}
	link := filepath.Join(dir, "link")
	if err := os.Symlink(real, link); err != nil {
		t.Skipf("symbolic links unavailable: %v", err)
	}
	if err := os.WriteFile(filepath.Join(real, "notes.json"), []byte("{}"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := &streams{root: real}

	cases := map[string]string{
		filepath.Join(link, "notes.json"):    "notes.json",
		filepath.Join(real, "notes.json"):    "notes.json",
		filepath.Join(dir, "elsewhere.json"): filepath.Join(dir, "elsewhere.json"),
	}
	for path, want := range cases {
		if got := s.display(path); got != want {
			t.Errorf("display(%q) = %q, want %q", path, got, want)
		}
	}
}
