package test

import (
	"bytes"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"hash/fnv"
	"os"
	"path/filepath"
	"reflect"
	"sort"
	"strings"
	"testing"
)

// Every exception names one losable notice, with its reason and exact operation
// body. Moving a durable write into an existing callback changes its fingerprint.
// The register is a subset of development's register after this initial cutover.
func TestDurableSideEffectsRatchet(t *testing.T) {
	t.Parallel()
	root, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile(filepath.Join(root, sideEffectNoticeRegister))
	if err != nil {
		t.Fatal(err)
	}
	notices := map[string]losableSideEffectNotice{}
	if err := json.Unmarshal(raw, &notices); err != nil {
		t.Fatal(err)
	}
	findings, err := durableSideEffectScan(root)
	if err != nil {
		t.Fatal(err)
	}
	var failures []string
	for key, fingerprint := range findings {
		notice, ok := notices[key]
		if !ok {
			failures = append(failures, key+" @"+fingerprint)
			continue
		}
		if strings.TrimSpace(notice.Reason) == "" {
			failures = append(failures, "missing reason: "+key)
		}
		if notice.Fingerprint != fingerprint {
			failures = append(failures, "changed notice, reassess durable work: "+key+" @"+fingerprint)
		}
	}
	for key := range notices {
		if _, ok := findings[key]; !ok {
			failures = append(failures, "stale notice, remove: "+key)
		}
	}
	sort.Strings(failures)
	if len(failures) > 0 {
		t.Errorf("Durable work belongs in the triggering transaction/outbox; exact exceptions are losable notices only:\n%s", strings.Join(failures, "\n"))
	}
	if err := sideEffectRegisterOnlyShrinks(root, notices); err != nil {
		t.Error(err)
	}
}

type sideEffectPackage struct{ Path, Role string }

func durableSideEffectScan(root string) (map[string]string, error) {
	raw, err := os.ReadFile(filepath.Join(root, "architecture", "policy.json"))
	if err != nil {
		return nil, err
	}
	var policy struct{ Packages []sideEffectPackage }
	if err := json.Unmarshal(raw, &policy); err != nil {
		return nil, err
	}
	sort.Slice(policy.Packages, func(i, j int) bool { return len(policy.Packages[i].Path) > len(policy.Packages[j].Path) })
	findings := map[string]string{}
	err = filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() {
			if path != root && (strings.HasPrefix(entry.Name(), ".") || entry.Name() == "vendor" || entry.Name() == "node_modules" || entry.Name() == "tmp") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		rel, err := filepath.Rel(root, path)
		if err != nil {
			return err
		}
		rel = filepath.ToSlash(rel)
		role := sideEffectRole(filepath.ToSlash(filepath.Dir(rel)), policy.Packages)
		if role != "http" && role != "application" && role != "compose" {
			return nil
		}
		_, file, err := parseGoSourceCached(path, nil)
		if err != nil {
			return err
		}
		for _, group := range file.Comments {
			for _, comment := range group.List {
				if comment.Text == "//go:build testhooks" {
					return nil
				}
			}
		}
		for key, fingerprint := range sideEffectNodes(rel, file) {
			findings[key] = fingerprint
		}
		return nil
	})
	return findings, err
}

func sideEffectRole(dir string, packages []sideEffectPackage) string {
	for _, pkg := range packages {
		if dir == pkg.Path || strings.HasPrefix(dir, pkg.Path+"/") {
			return pkg.Role
		}
	}
	return ""
}

func sideEffectNodes(path string, file *ast.File) map[string]string {
	result := map[string]string{}
	collect := func(name string, body ast.Node) {
		counts := map[string]int{}
		ast.Inspect(body, func(node ast.Node) bool {
			operation := ""
			switch node := node.(type) {
			case *ast.GoStmt:
				operation = "go"
			case *ast.CallExpr:
				var called string
				switch target := node.Fun.(type) {
				case *ast.SelectorExpr:
					called = target.Sel.Name
				case *ast.Ident:
					called = target.Name
				}
				switch called {
				case "Dispatch", "AfterCommit", "RegisterAfterCommit":
					operation = called
				}
			}
			if operation != "" {
				var formatted bytes.Buffer
				if err := ast.Fprint(&formatted, nil, node, func(name string, value reflect.Value) bool {
					return value.Type() != reflect.TypeFor[token.Pos]() && name != "Obj" && name != "Scope" && name != "Doc" && name != "Comment" && name != "Comments"
				}); err != nil {
					panic(err)
				}
				hash := fnv.New128a()
				hash.Write(formatted.Bytes())
				fingerprint := hex.EncodeToString(hash.Sum(nil))
				identity := operation + "@" + fingerprint
				counts[identity]++
				result[fmt.Sprintf("%s:%s:%s#%d", path, name, identity, counts[identity])] = fingerprint
			}
			return true
		})
	}
	for _, declaration := range file.Decls {
		switch declaration := declaration.(type) {
		case *ast.FuncDecl:
			if declaration.Body == nil {
				continue
			}
			name := declaration.Name.Name
			if declaration.Recv != nil {
				receiver := declaration.Recv.List[0].Type
				if pointer, ok := receiver.(*ast.StarExpr); ok {
					receiver = pointer.X
				}
				if identifier, ok := receiver.(*ast.Ident); ok {
					name = identifier.Name + "." + name
				}
			}
			collect(name, declaration.Body)
		case *ast.GenDecl:
			if declaration.Tok != token.VAR {
				continue
			}
			for _, specification := range declaration.Specs {
				values, ok := specification.(*ast.ValueSpec)
				if !ok {
					continue
				}
				for index, value := range values.Values {
					collect("var:"+values.Names[min(index, len(values.Names)-1)].Name, value)
				}
			}
		}
	}
	return result
}

func TestSideEffectScannerFindsNestedAndPackageLevelOperations(t *testing.T) {
	t.Parallel()
	source := `package fixture
var deferred = func(){ go work() }
func run(){ go work(); mail.Dispatch(ctx,req); hooks.AfterCommit(ctx,func(){tenant.RegisterAfterCommit(ctx,func(){go notice()})}) }`
	file, err := parser.ParseFile(token.NewFileSet(), "fixture.go", source, 0)
	if err != nil {
		t.Fatal(err)
	}
	found := sideEffectNodes("fixture.go", file)
	for prefix, want := range map[string]int{"var:deferred:go@": 1, "run:go@": 2, "run:Dispatch@": 1, "run:AfterCommit@": 1, "run:RegisterAfterCommit@": 1} {
		count := 0
		for key, fingerprint := range found {
			if strings.HasPrefix(key, "fixture.go:"+prefix) && len(fingerprint) == 32 {
				count++
			}
		}
		if count != want {
			t.Errorf("operation %s: got %d, want %d", prefix, count, want)
		}
	}
	if len(found) != 6 {
		t.Fatalf("unexpected operations: %v", found)
	}
	changed, err := parser.ParseFile(token.NewFileSet(), "fixture.go", strings.Replace(source, "go notice()", "go persistAudit()", 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	changedNodes := sideEffectNodes("fixture.go", changed)
	for key := range found {
		if strings.Contains(key, ":AfterCommit@") {
			if _, ok := changedNodes[key]; ok {
				t.Fatal("changing a callback body must invalidate its notice entry")
			}
		}
	}
	removed, err := parser.ParseFile(token.NewFileSet(), "fixture.go", strings.Replace(source, "func run(){ go work();", "func run(){", 1), 0)
	if err != nil {
		t.Fatal(err)
	}
	for key := range sideEffectNodes("fixture.go", removed) {
		if _, ok := found[key]; !ok {
			t.Fatalf("deleting an earlier notice must not rename retained operations: %s", key)
		}
	}
}

func TestSideEffectNoticeRegisterRejectsEqualSizedReplacements(t *testing.T) {
	t.Parallel()
	notice := losableSideEffectNotice{Reason: "realtime invalidation", Fingerprint: "exact operation"}
	base := map[string]losableSideEffectNotice{"original": notice}
	if err := sideEffectRegisterSubset(map[string]losableSideEffectNotice{}, base); err != nil {
		t.Fatal(err)
	}
	if err := sideEffectRegisterSubset(map[string]losableSideEffectNotice{"replacement": notice}, base); err == nil {
		t.Fatal("an equal-sized replacement is not a shrinking register")
	}
	changed := notice
	changed.Fingerprint = "durable operation"
	if err := sideEffectRegisterSubset(map[string]losableSideEffectNotice{"original": changed}, base); err == nil {
		t.Fatal("a changed operation is not the original losable notice")
	}
}
