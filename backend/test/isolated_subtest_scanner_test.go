package test

import (
	"bytes"
	"fmt"
	"go/ast"
	"go/token"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// This is a conservative source check, not runtime detection of t.Parallel.
// It follows named package-local functions across files and inline/named Run
// callbacks, plus the public SetupIsolatedTestDB/NewFaultProbe entry points.
// Function values, interface/method dispatch and other packages' helper graphs
// are not resolved. The deadline on slot acquisition bounds missed patterns.
// PerTestDatabases packages never acquire this limiter, so their clone-holding
// parents cannot cause the slot/parallel-scheduler cycle checked here.
func checkIsolatedParallelSubtests(t *testing.T, root string) []string {
	t.Helper()
	var files []sourceFile
	require.NoError(t, walkGoFilesRaw(root, func(rel, pkg string, source []byte) {
		if !inDirNamed(rel, "vendor", "testdata") {
			files = append(files, sourceFile{rel: rel, pkg: pkg, content: source})
		}
	}))
	return isolatedParallelSubtestFindings(t, files)
}

type isolatedSubtestNode struct {
	location, name                 string
	calls, children                []string
	clone, parallel, parallelChild bool
}

func isolatedParallelSubtestFindings(t *testing.T, files []sourceFile) []string {
	t.Helper()
	// Only packages containing an explicit clone entry point need an AST walk.
	// Reuse the shared parser cache in candidate packages. TestMain's direct
	// PerTestDatabases opt-in is recognized to exclude that slot-free mode.
	dirs := map[string]bool{}
	for _, file := range files {
		for _, marker := range []string{"SetupIsolatedTestDB", "NewFaultProbe", "PerTestDatabases"} {
			if bytes.Contains(file.content, []byte(marker)) {
				dirs[file.pkg] = true
			}
		}
	}
	type parsed struct {
		file     *ast.File
		fset     *token.FileSet
		rel, pkg string
	}
	var parsedFiles []parsed
	perDatabase := map[string]bool{}
	for _, file := range files {
		if !dirs[file.pkg] {
			continue
		}
		fset, syntax, err := parseGoSourceCached(file.rel, file.content)
		require.NoError(t, err)
		pkg := file.pkg + "/" + syntax.Name.Name
		parsedFiles = append(parsedFiles, parsed{syntax, fset, file.rel, pkg})
		for _, decl := range syntax.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Name.Name != "TestMain" {
				continue
			}
			ast.Inspect(fn.Body, func(node ast.Node) bool {
				if call, ok := node.(*ast.CallExpr); ok && isolatedCallName(call) == "PerTestDatabases" {
					perDatabase[pkg] = true
				}
				return true
			})
		}
	}
	nodes := map[string]*isolatedSubtestNode{}
	var tests []*isolatedSubtestNode
	var add func(*ast.BlockStmt, parsed, string, string) *isolatedSubtestNode
	add = func(body *ast.BlockStmt, file parsed, name, key string) *isolatedSubtestNode {
		node := &isolatedSubtestNode{name: name, location: fmt.Sprintf("%s:%d", file.rel, file.fset.Position(body.Pos()).Line)}
		nodes[key] = node
		ast.Inspect(body, func(syntax ast.Node) bool {
			if _, ok := syntax.(*ast.FuncLit); ok {
				return false
			}
			call, ok := syntax.(*ast.CallExpr)
			if !ok {
				return true
			}
			called := isolatedCallName(call)
			if called == "SetupIsolatedTestDB" || called == "NewFaultProbe" {
				node.clone = true
			}
			if called == "Parallel" {
				node.parallel = true
			}
			if ident, ok := call.Fun.(*ast.Ident); ok {
				node.calls = append(node.calls, file.pkg+"/"+ident.Name)
			}
			if called == "Run" && len(call.Args) == 2 {
				switch child := call.Args[1].(type) {
				case *ast.FuncLit:
					childKey := fmt.Sprintf("%s@%d", key, child.Pos())
					add(child.Body, file, name+"/subtest", childKey)
					node.children = append(node.children, childKey)
				case *ast.Ident:
					node.children = append(node.children, file.pkg+"/"+child.Name)
				}
			}
			return true
		})
		return node
	}
	for _, file := range parsedFiles {
		if perDatabase[file.pkg] {
			continue
		}
		for _, decl := range file.file.Decls {
			fn, ok := decl.(*ast.FuncDecl)
			if !ok || fn.Body == nil || fn.Recv != nil {
				continue
			}
			node := add(fn.Body, file, fn.Name.Name, file.pkg+"/"+fn.Name.Name)
			if strings.HasSuffix(file.rel, "_test.go") && strings.HasPrefix(fn.Name.Name, "Test") && fn.Name.Name != "TestMain" {
				tests = append(tests, node)
			}
		}
	}
	for changed := true; changed; {
		changed = false
		for _, node := range nodes {
			clone, parallel, children := node.clone, node.parallel, node.parallelChild
			for _, key := range node.calls {
				if callee := nodes[key]; callee != nil {
					clone = clone || callee.clone
					parallel = parallel || callee.parallel
					children = children || callee.parallelChild
				}
			}
			for _, key := range node.children {
				if child := nodes[key]; child != nil {
					children = children || child.parallel || child.parallelChild
				}
			}
			if clone != node.clone || parallel != node.parallel || children != node.parallelChild {
				changed = true
			}
			node.clone, node.parallel, node.parallelChild = clone, parallel, children
		}
	}
	var findings []string
	// A clone-holding parent can itself be a Run callback. Do not propagate
	// child clone ownership to its ancestor: inspect each callback separately.
	for _, node := range nodes {
		for _, key := range node.children {
			if child := nodes[key]; child != nil {
				tests = append(tests, child)
			}
		}
	}
	seen := map[*isolatedSubtestNode]bool{}
	for _, node := range tests {
		if !seen[node] && node.clone && node.parallelChild {
			findings = append(findings, fmt.Sprintf("%s: %s holds a slot-limited isolated clone with parallel descendants", node.location, node.name))
		}
		seen[node] = true
	}
	sort.Strings(findings)
	return findings
}

func isolatedCallName(call *ast.CallExpr) string {
	switch fun := call.Fun.(type) {
	case *ast.Ident:
		return fun.Name
	case *ast.SelectorExpr:
		return fun.Sel.Name
	}
	return ""
}

func TestIsolatedParallelSubtestScanner(t *testing.T) {
	t.Parallel()
	const header = "package fixture\nimport (\"testing\"; testpkg \"github.com/moto-nrw/project-phoenix/test\")\n"
	for _, tc := range []struct {
		name, source, helper string
		findings             int
	}{
		{name: "parent clone and child parallel", findings: 1, source: `func TestParent(t *testing.T) { testpkg.SetupIsolatedTestDB(t); t.Run("child", func(t *testing.T) { t.Parallel() }) }`},
		{name: "helpers across files", findings: 1, source: `func TestParent(t *testing.T) { fixture(t); children(t) }`, helper: `func fixture(t *testing.T) { testpkg.NewFaultProbe(t) }; func children(t *testing.T) { t.Run("child", parallelChild) }; func parallelChild(t *testing.T) { t.Parallel() }`},
		{name: "nested descendant", findings: 1, source: `func TestParent(t *testing.T) { testpkg.SetupIsolatedTestDB(t); t.Run("child", func(t *testing.T) { t.Run("grandchild", func(t *testing.T) { t.Parallel() }) }) }`},
		{name: "nested clone parent", findings: 1, source: `func TestRoot(t *testing.T) { t.Run("parent", func(t *testing.T) { testpkg.SetupIsolatedTestDB(t); t.Run("child", func(t *testing.T) { t.Parallel() }) }) }`},
		{name: "named nested clone parent", findings: 1, source: `func TestRoot(t *testing.T) { t.Run("parent", parent) }`, helper: `func parent(t *testing.T) { testpkg.SetupIsolatedTestDB(t); t.Run("child", func(t *testing.T) { t.Parallel() }) }`},
		{name: "per database mode has no limiter", source: `func TestMain(m *testing.M) { testpkg.PerTestDatabases() }; func TestParent(t *testing.T) { testpkg.SetupTestDB(t); t.Run("child", func(t *testing.T) { t.Parallel() }) }`},
		{name: "explicit clone in per database mode has no limiter", source: `func TestMain(m *testing.M) { testpkg.PerTestDatabases() }; func TestParent(t *testing.T) { testpkg.SetupIsolatedTestDB(t); t.Run("child", func(t *testing.T) { t.Parallel() }) }`},
		{name: "ordinary package database", source: `func TestParent(t *testing.T) { testpkg.SetupTestDB(t); t.Run("child", func(t *testing.T) { t.Parallel() }) }`},
		{name: "only parent parallel", source: `func TestParent(t *testing.T) { t.Parallel(); testpkg.SetupIsolatedTestDB(t); t.Run("child", func(t *testing.T) {}) }`},
		{name: "only child clones", source: `func TestParent(t *testing.T) { t.Run("child", func(t *testing.T) { t.Parallel(); testpkg.SetupIsolatedTestDB(t) }) }`},
	} {
		t.Run(tc.name, func(t *testing.T) {
			files := []sourceFile{{rel: filepath.Join(tc.name, "fixture_test.go"), pkg: tc.name, content: []byte(header + tc.source)}}
			if tc.helper != "" {
				files = append(files, sourceFile{rel: filepath.Join(tc.name, "helper_test.go"), pkg: tc.name, content: []byte(header + tc.helper)})
			}
			findings := isolatedParallelSubtestFindings(t, files)
			require.Len(t, findings, tc.findings, "%v", findings)
		})
	}
}
