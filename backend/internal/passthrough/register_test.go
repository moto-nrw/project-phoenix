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
		{"-c", "core.hooksPath=/dev/null", "-c", "user.name=Test", "-c", "user.email=test@example.invalid", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "Record base inventory"},
	} {
		if _, err := git(repo, args...); err != nil {
			t.Fatal(err)
		}
	}
	if err := Check(root, "HEAD"); err != nil {
		t.Fatalf("unchanged committed inventory must not require a new ADR: %v", err)
	}
	put(t, root, path, "package owners\nfunc (s *Port) Find() error { return s.store.Renamed() }")
	if err := Check(root, "HEAD"); err == nil || !strings.Contains(err.Error(), "changed method evidence") {
		t.Fatalf("body updates still need a reviewed snapshot diff: %v", err)
	}
	writeRegistry()
	if err := Check(root, "HEAD"); err != nil {
		t.Fatalf("same-form body update must not require a new ADR: %v", err)
	}
}

func TestPassthroughADROnlyForNewOrWorsenedDebt(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name, before, after string
		needsADR            bool
	}{
		{"new forward", "", "forward-debt", true},
		{"new guard", "", "guard-debt", true},
		{"translation to forward", "unclassified-evidence", "forward-debt", true},
		{"translation to guard", "unclassified-evidence", "guard-debt", true},
		{"different debt form", "forward-debt", "guard-debt", true},
		{"same forward form", "forward-debt", "forward-debt", false},
		{"same guard form", "guard-debt", "guard-debt", false},
		{"unclassified body change", "unclassified-evidence", "unclassified-evidence", false},
		{"debt to unclassified", "forward-debt", "unclassified-evidence", false},
		{"new neutral evidence", "", "unchanged-forward-evidence", false},
		{"removed debt", "forward-debt", "", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			base := Registry{Scope: "same"}
			candidate := Registry{Scope: "same"}
			if fixture.before != "" {
				base.Sites = []Site{{ID: "api/Port.Find", Form: fixture.before, Evidence: "old", Decision: "0090"}}
			}
			if fixture.after != "" {
				candidate.Sites = []Site{{ID: "api/Port.Find", Form: fixture.after, Evidence: "new", Decision: "0091"}}
			}
			if problems := CompareRegisters(base, candidate, nil); (len(problems) > 0) != fixture.needsADR {
				t.Fatalf("needs ADR = %t, got %v", fixture.needsADR, problems)
			}
			if problems := CompareRegisters(base, candidate, map[string]bool{"0091": true}); len(problems) != 0 {
				t.Fatalf("newly numbered entry ADR must authorize debt change: %v", problems)
			}
		})
	}
}

func TestPassthroughRemovalNeedsHeadSourceProof(t *testing.T) {
	t.Parallel()
	for _, fixture := range []struct {
		name, source string
		missing      bool
	}{
		{"still forwards", "func (s *Port) Find() { s.store.Find() }", true},
		{"body changed but still delegates", "func (s *Port) Find() { s.store.Renamed() }", true},
		{"deepened", "func (s *Port) Find() { s.store.Find(); s.clock.Now() }", false},
		{"method removed", "", false},
	} {
		t.Run(fixture.name, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			if err := os.MkdirAll(filepath.Join(root, "api"), 0o755); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "api/renamed.go"), []byte("package api\n"+fixture.source), 0o600); err != nil {
				t.Fatal(err)
			}
			problems, err := checkRemovedSites(root, []Site{{ID: "api/Port.Find"}}, nil)
			if err != nil {
				t.Fatal(err)
			}
			if (len(problems) > 0) != fixture.missing {
				t.Fatalf("missing = %t, got %v", fixture.missing, problems)
			}
		})
	}
	t.Run("package removed", func(t *testing.T) {
		t.Parallel()
		problems, err := checkRemovedSites(t.TempDir(), []Site{{ID: "api/Port.Find"}}, nil)
		if err != nil || len(problems) != 0 {
			t.Fatalf("deleted package: %v, %v", problems, err)
		}
	})
}

func TestPassthroughProjectSymlinkCannotSkipHostCheck(t *testing.T) {
	t.Parallel()
	host := t.TempDir()
	alias := filepath.Join(t.TempDir(), "alias")
	if err := os.Symlink(host, alias); err != nil {
		t.Fatal(err)
	}
	for _, paths := range [][2]string{{host, alias}, {alias, host}} {
		err := CheckProject(paths[0], paths[1], "")
		if err == nil || !strings.Contains(err.Error(), "passthrough.json") {
			t.Fatalf("host alias bypassed missing inventory: %v", err)
		}
	}
}
