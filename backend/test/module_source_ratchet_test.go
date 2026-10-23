package test

import (
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// walkModuleRatchetSources shares the size, length and complexity surface.
// Include root packages recursively, even hooks and untagged test helpers:
// moving code out of an owner must not silence a gate (ADR 0082).
// Only direct files belong to "."; walking all backend trees would also
// pull unrelated models, migrations and tools into these module ratchets.
func walkModuleRatchetSources(backendRoot string, visit func(path, rel string) error) error {
	for _, tree := range []string{"modules", "workflows", "roots", "api", "cmd", "."} {
		root := filepath.Join(backendRoot, tree)
		err := filepath.WalkDir(root, func(path string, entry fs.DirEntry, err error) error {
			if err != nil {
				return err
			}
			if entry.IsDir() {
				if path != root && (tree == "." || strings.HasPrefix(entry.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			rel, err := filepath.Rel(backendRoot, path)
			if err != nil {
				return err
			}
			return visit(path, filepath.ToSlash(rel))
		})
		if err != nil {
			return err
		}
	}
	return nil
}

// Each new root surface must reject an unregistered file, long function and
// complex function. Test hooks are deliberately part of the source scan.
func TestModuleRootSourceRatchetsRejectNewViolations(t *testing.T) {
	t.Parallel()
	root := t.TempDir()
	for _, tree := range []string{"modules", "workflows", "roots", "api", "cmd"} {
		if err := os.MkdirAll(filepath.Join(root, tree), 0755); err != nil {
			t.Fatal(err)
		}
	}
	source := "//go:build testhooks\n\npackage fixture\n\nfunc Oversized(b bool) {\n" +
		strings.Repeat("if b {\nprintln(b)\n}\n", 16) +
		strings.Repeat("// body line\n", 14) + "}\n" + strings.Repeat("// padding\n", 800)
	paths := []string{"main.go", "roots/newbinding/nested/example.go", "api/nested/example.go", "cmd/nested/example.go"}
	for _, rel := range paths {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	for _, rel := range []string{"api/ignored_test.go", "roots/.hidden/ignored.go", "outside/ignored.go"} {
		path := filepath.Join(root, rel)
		if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(source), 0600); err != nil {
			t.Fatal(err)
		}
	}
	sizes, err := moduleFileSizeScan(root)
	if err != nil {
		t.Fatal(err)
	}
	lengths, err := moduleFuncLenScan(root)
	if err != nil {
		t.Fatal(err)
	}
	scores, err := moduleComplexityScan(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, rel := range paths {
		if sizes[rel] <= moduleFileSizeThreshold || lengths[rel+":Oversized"] <= moduleFuncLenThreshold || scores[rel+":Oversized"] <= moduleComplexityThreshold {
			t.Errorf("new root violation escaped: %s (size %d, length %d, complexity %d)", rel, sizes[rel], lengths[rel+":Oversized"], scores[rel+":Oversized"])
		}
	}
	if len(sizes) != len(paths) || len(lengths) != len(paths) || len(scores) != len(paths) {
		t.Fatalf("unexpected scope: %v, %v, %v", sizes, lengths, scores)
	}
	if len(moduleFileSizeViolations(sizes, nil)) != len(paths) || len(moduleFuncLenViolations(lengths, nil)) != len(paths) || len(moduleComplexityViolations(scores, nil)) != len(paths) {
		t.Fatal("unregistered root violations must fail all three ratchets")
	}
}
