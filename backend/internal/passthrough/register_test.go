package passthrough

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestPassthroughChecksCommittedBaseInventory(t *testing.T) {
	t.Parallel()
	put := func(t *testing.T, root, path, source string) {
		t.Helper()
		path = filepath.Join(root, path)
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0o600); err != nil {
			t.Fatal(err)
		}
	}
	repo := t.TempDir()
	root := filepath.Join(repo, "backend")
	path := "roots/owners/port.go"
	put(t, root, path, "package owners\nfunc (s *Port) Find() error { return s.store.Find() }")
	put(t, repo, "docs/adr/0090-method-evidence.md", "# Method evidence\n")
	writeRegistry := func() {
		t.Helper()
		var output bytes.Buffer
		if err := WriteSnapshot(&output, root, "0090"); err != nil {
			t.Fatal(err)
		}
		put(t, root, "architecture/passthrough.json", output.String())
	}
	writeRegistry()
	for _, args := range [][]string{
		{"init", "--quiet"},
		{"add", "."},
		{"-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Record base inventory"},
	} {
		if _, err := git(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(root, "HEAD"); err != nil {
		t.Fatalf("unchanged committed inventory must not require a new ADR: %v", err)
	}
	put(t, root, path, "package owners\nfunc (s *Port) Find() error { return s.store.Renamed() }")
	writeRegistry()
	if err := Check(root, "HEAD"); err == nil || !strings.Contains(err.Error(), "new/changed registration needs a new ADR: roots/owners/Port.Find") {
		t.Fatalf("changed inventory must still require a new ADR: %v", err)
	}
}
