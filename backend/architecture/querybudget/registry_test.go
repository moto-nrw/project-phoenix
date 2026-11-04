package querybudget

import (
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"strings"
	"testing"
)

const validBudget = `{"scenario":"modules.example.list","max":3,"exact":true,"description":"One bulk read."}`

func TestReadDiscoversAndValidatesAllFragments(t *testing.T) {
	t.Parallel()
	for _, test := range []struct{ name, second, want string }{
		{"nested discovery", strings.Replace(validBudget, "example.list", "other.list", 1), ""},
		{"global duplicate", validBudget, "duplicate query budget"},
		{"duplicate field", strings.Replace(validBudget, `"max":3`, `"max":3,"max":4`, 1), "duplicate query budget field"},
		{"unknown field", strings.Replace(validBudget, `"exact":true`, `"unexpected":true`, 1), "missing query budget field"},
		{"negative count", strings.Replace(validBudget, `"max":3`, `"max":-1`, 1), "nonnegative max"},
		{"missing exact", strings.Replace(validBudget, `"exact":true,`, "", 1), "requires scenario"},
		{"null count", strings.Replace(validBudget, `"max":3`, `"max":null`, 1), "missing query budget field"},
		{"trailing value", validBudget + `{}`, "trailing"},
		{"invalid JSON", `{`, "unexpected end"},
	} {
		t.Run(test.name, func(t *testing.T) {
			t.Parallel()
			source := budgetFiles(t, map[string]string{"budgets/a.json": validBudget, "budgets/nested/b.json": test.second})
			budgets, err := ReadQueryBudgets(source, "budgets")
			if test.want != "" {
				if err == nil || !strings.Contains(err.Error(), test.want) {
					t.Fatalf("want %q, got %v", test.want, err)
				}
				return
			}
			if err != nil || len(budgets) != 2 || budgets["modules.other.list"].Max != 3 {
				t.Fatalf("budgets=%v err=%v", budgets, err)
			}
		})
	}
	for name, source := range map[string]map[string]string{"missing": {}, "empty": {"budgets/README.md": "docs"}} {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			if _, err := ReadQueryBudgets(budgetFiles(t, source), "budgets"); err == nil {
				t.Fatal("accepted missing/empty registry")
			}
		})
	}
}

func TestReadHistoricalPreservesContractAndFailsClosed(t *testing.T) {
	t.Parallel()
	const source = `package test
 type queryBudget struct {max int; exact bool}
 var queryBudgets = map[string]queryBudget{"list": {max: 3, exact: true}, "zero": {max: 0}}`
	got, err := ReadHistorical([]byte(source))
	want := map[string]Budget{"list": {Scenario: "list", Max: 3, Exact: true}, "zero": {Scenario: "zero"}}
	if err != nil || !reflect.DeepEqual(got, want) {
		t.Fatalf("got %v, %v", got, err)
	}
	for _, malformed := range []string{
		`package test`, strings.Replace(source, `max: 3`, `max: unknown`, 1),
		strings.Replace(source, `exact: true`, `exact: unknown`, 1),
		strings.Replace(source, `"zero"`, `"list"`, 1),
		strings.Replace(source, `max: 3`, `max: 3, max: 4`, 1),
		strings.Replace(source, `max: 3`, `count: 3`, 1),
	} {
		if _, err := ReadHistorical([]byte(malformed)); err == nil {
			t.Fatalf("accepted %s", malformed)
		}
	}
}

func budgetFiles(t *testing.T, files map[string]string) fs.FS {
	t.Helper()
	root := t.TempDir()
	for name, source := range files {
		path := filepath.Join(root, filepath.FromSlash(name))
		if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	return os.DirFS(root)
}

func TestQueryBudgetDirectorySymlinksCannotCreateUnreadableBases(t *testing.T) {
	t.Parallel()
	for _, boundary := range []string{"test", "test/query-budgets"} {
		t.Run(boundary, func(t *testing.T) {
			t.Parallel()
			root := t.TempDir()
			original := filepath.Join(root, filepath.FromSlash(boundary))
			if err := os.MkdirAll(filepath.Join(root, "test", "query-budgets"), 0700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(filepath.Join(root, "test", "query-budgets", "owner.json"), []byte(validBudget), 0600); err != nil {
				t.Fatal(err)
			}
			moved := original + "-source"
			if err := os.Rename(original, moved); err != nil {
				t.Fatal(err)
			}
			if err := os.Symlink(filepath.Base(moved), original); err != nil {
				t.Fatal(err)
			}
			if _, err := ReadQueryBudgets(os.DirFS(root), "test/query-budgets"); err == nil {
				t.Fatal("candidate accepts directory symlink that Git cannot read")
			}
		})
	}
}
