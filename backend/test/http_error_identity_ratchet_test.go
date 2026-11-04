package test

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// HTTP error classification has no text-matching exceptions. Use the policy's
// role, not directory names: session middleware and api/common are HTTP too.
func TestHTTPErrorIdentityRatchet(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := artifacts.ReadPolicy(filepath.Join(root, "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	var policy struct{ Packages []struct{ Path, Role string } }
	if err := json.Unmarshal(raw, &policy); err != nil {
		t.Fatal(err)
	}
	for _, pkg := range policy.Packages {
		if pkg.Role != "http" {
			continue
		}
		entries, err := os.ReadDir(filepath.Join(root, pkg.Path))
		if err != nil {
			t.Fatal(err)
		}
		for _, entry := range entries {
			if entry.IsDir() || !strings.HasSuffix(entry.Name(), ".go") || strings.HasSuffix(entry.Name(), "_test.go") {
				continue
			}
			path := filepath.Join(root, pkg.Path, entry.Name())
			source, err := os.ReadFile(path)
			if err != nil {
				t.Fatal(err)
			}
			findings, err := httpErrorTextComparisons(path, source)
			if err != nil {
				t.Fatal(err)
			}
			for _, finding := range findings {
				t.Error(finding)
			}
		}
	}
}

func httpErrorTextComparisons(path string, source []byte) ([]string, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, path, source, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	textNames := map[string]bool{}
	// Track local copies too: msg := err.Error(); switch msg is the same
	// forbidden dependency. Iterate to cover copies declared before their source.
	for changed := true; changed; {
		changed = false
		ast.Inspect(file, func(node ast.Node) bool {
			assign, ok := node.(*ast.AssignStmt)
			if !ok {
				return true
			}
			for i, rhs := range assign.Rhs {
				if i >= len(assign.Lhs) || !httpErrorText(rhs, textNames) {
					continue
				}
				if id, ok := assign.Lhs[i].(*ast.Ident); ok && !textNames[id.Name] {
					textNames[id.Name] = true
					changed = true
				}
			}
			return true
		})
	}
	stringPackages := map[string]bool{}
	for _, imp := range file.Imports {
		value, _ := strconv.Unquote(imp.Path.Value)
		if value != "strings" && value != "slices" {
			continue
		}
		name := value
		if imp.Name != nil {
			name = imp.Name.Name
		}
		stringPackages[name] = true
	}
	var findings []string
	for _, group := range file.Comments {
		for _, comment := range group.List {
			directive := strings.Fields(strings.TrimSpace(strings.TrimPrefix(comment.Text, "//")))
			if len(directive) == 0 {
				continue
			}
			name, linters, hasList := strings.Cut(directive[0], ":")
			if name != "nolint" {
				continue
			}
			forbidden := !hasList
			for _, linter := range strings.Split(linters, ",") {
				forbidden = forbidden || linter == "all" || linter == "errorlint"
			}
			if forbidden {
				findings = append(findings, fmt.Sprintf("%s: HTTP errorlint has no suppressions", fset.Position(comment.Pos())))
			}
		}
	}
	ast.Inspect(file, func(node ast.Node) bool {
		forbidden := false
		switch n := node.(type) {
		case *ast.BinaryExpr:
			forbidden = (n.Op == token.EQL || n.Op == token.NEQ) &&
				(httpErrorText(n.X, textNames) || httpErrorText(n.Y, textNames)) &&
				!httpNil(n.X) && !httpNil(n.Y)
		case *ast.SwitchStmt:
			forbidden = httpErrorText(n.Tag, textNames)
		case *ast.IndexExpr:
			forbidden = httpErrorText(n.Index, textNames)
		case *ast.CallExpr:
			if selector, ok := n.Fun.(*ast.SelectorExpr); ok {
				if pkg, ok := selector.X.(*ast.Ident); ok && stringPackages[pkg.Name] {
					switch selector.Sel.Name {
					case "Contains", "HasPrefix", "HasSuffix", "EqualFold", "Compare":
						for _, arg := range n.Args {
							forbidden = forbidden || httpErrorText(arg, textNames)
						}
					}
				}
			}
		}
		if forbidden {
			findings = append(findings, fmt.Sprintf("%s: classify errors with errors.Is/As, never diagnostic text", fset.Position(node.Pos())))
		}
		return true
	})
	return findings, nil
}

func httpErrorText(expr ast.Expr, names map[string]bool) bool {
	switch value := expr.(type) {
	case *ast.Ident:
		return names[value.Name]
	case *ast.ParenExpr:
		return httpErrorText(value.X, names)
	case *ast.CallExpr:
		selector, ok := value.Fun.(*ast.SelectorExpr)
		return ok && selector.Sel.Name == "Error" && len(value.Args) == 0
	}
	return false
}

func httpNil(expr ast.Expr) bool { id, ok := expr.(*ast.Ident); return ok && id.Name == "nil" }

func TestHTTPErrorTextComparisons(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, body string
		forbidden  bool
	}{
		{"equal", `return err.Error() == "missing"`, true},
		{"reverse unequal", `return "missing" != err.Error()`, true},
		{"contains", `return text.Contains(err.Error(), "missing")`, true},
		{"prefix", `return text.HasPrefix(err.Error(), "missing")`, true},
		{"suffix", `return text.HasSuffix(err.Error(), "missing")`, true},
		{"copied text", `msg := err.Error(); copy := msg; return text.Contains(copy, "missing")`, true},
		{"switch", `switch err.Error() { case "missing": return true }; return false`, true},
		{"map lookup", `return messages[err.Error()]`, true},
		{"slices", `return slices.Contains(messages, err.Error())`, true},
		{"errorlint suppression", "//nolint:errorlint\nreturn false", true},
		{"all-linter suppression", "//nolint\nreturn false", true},
		{"other linter suppression", "//nolint:staticcheck\nreturn false", false},
		{"sentinel", `return errors.Is(err, missing)`, false},
		{"logging", `logger.Error("failed", "error", err.Error()); return false`, false},
		{"ordinary text", `return text.Contains(value, "missing")`, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			source := []byte("package sample\nimport text \"strings\"\nimport \"slices\"\nfunc check(err error) bool {" + tc.body + "}")
			findings, err := httpErrorTextComparisons("sample.go", source)
			if err != nil {
				t.Fatal(err)
			}
			if (len(findings) > 0) != tc.forbidden {
				t.Fatalf("findings = %v, want forbidden %t", findings, tc.forbidden)
			}
		})
	}
}
