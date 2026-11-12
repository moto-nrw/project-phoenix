package architecture

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReadsBaseUnderDeclaredModuleRename(t *testing.T) {
	t.Parallel()

	const from, to = "example.test/architecture-fixture", "example.test/renamed-fixture"
	for _, tc := range []struct {
		name         string
		addADR       bool
		changeSource bool
		want         string
	}{
		{name: "pure rename with new ADR", addADR: true},
		{name: "rename without new ADR", want: "needs approval_adr naming an ADR added in this change"},
		{name: "rename with another source change", addADR: true, changeSource: true, want: "changes more than the module path"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := ratchetRepository(t, legacyRecord(2583))
			renameFixtureModule(t, repo, from, to)
			policyPath := filepath.Join(repo, "architecture", "policy.json")
			writeFile(t, policyPath, mutatePolicy(t, readFile(t, policyPath), func(document map[string]any) {
				document["policy_epoch"] = 2
				document["module_rename"] = map[string]any{"from": from, "approval_adr": "0001"}
			}))
			if tc.addADR {
				writeFile(t, filepath.Join(repo, "docs", "adr", "0001-module-rename.md"), "---\nstatus: accepted\n---\n\n# Module rename\n")
			}
			if tc.changeSource {
				source := filepath.Join(repo, "source", "source.go")
				writeFile(t, source, readFile(t, source)+"\nconst Extra = 1\n")
			}

			output, err := runRepositoryCheck(t, repo, baseRef)
			if tc.want == "" {
				if err != nil {
					t.Fatalf("declared pure module rename rejected: %v\n%s", err, output)
				}
				return
			}
			if err == nil || !strings.Contains(output, tc.want) {
				t.Fatalf("module rename accepted without %q: %v\n%s", tc.want, err, output)
			}
		})
	}
}

// renameFixtureModule spells the module path anew in every tracked text file,
// as a module rename change does.
func renameFixtureModule(t *testing.T, repo, from, to string) {
	t.Helper()
	err := filepath.WalkDir(repo, func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() {
			if entry.Name() == ".git" {
				return filepath.SkipDir
			}
			return nil
		}
		contents, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if renamed := strings.ReplaceAll(string(contents), from, to); renamed != string(contents) {
			return os.WriteFile(path, []byte(renamed), 0o600)
		}
		return nil
	})
	if err != nil {
		t.Fatalf("rename fixture module: %v", err)
	}
}
