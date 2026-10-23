#!/usr/bin/env python3
"""Reproduce ADR 0082 root pass-through shapes from the repository root.

Uses the ratchet's unchanged AST predicate; --sites prints each body for review.
The semantic classification is a reviewed snapshot, not a new CI gate or proof
that a translation is necessary. Re-review it when the measured SHA changes.
"""
from pathlib import Path
import collections
import json
import subprocess
import sys
import tempfile
source = Path('backend/test/module_passthrough_ratchet_test.go').read_text()
header = '''package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

const modulePassthroughMaxBodyLines = 14

func main() {
	rows := []map[string]any{}
	for _, tree := range []string{"roots", "api", "cmd", "."} {
		filepath.WalkDir("backend/"+tree, func(path string, d os.DirEntry, err error) error {
			if err != nil {
				panic(err)
			}
			if d.IsDir() {
				if path != "backend/"+tree && (tree == "." || strings.HasPrefix(d.Name(), ".")) {
					return filepath.SkipDir
				}
				return nil
			}
			if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return nil
			}
			fs := token.NewFileSet()
			f, err := parser.ParseFile(fs, path, nil, 0)
			if err != nil {
				panic(err)
			}
			for _, decl := range f.Decls {
				fn, ok := decl.(*ast.FuncDecl)
				if !ok || !modulePassthroughIsDelegation(fn, fs) {
					continue
				}
				var b bytes.Buffer
				format.Node(&b, fs, fn)
				kind := classify(fn)
				if strings.Contains(path, "/api/common/") || strings.HasSuffix(path, "/api/iot_http_metrics.go") || strings.HasSuffix(path, "/api/serve_host.go") {
					kind = "runtime"
				}
				rows = append(rows, map[string]any{"file": strings.TrimPrefix(path, "backend/"), "line": fs.Position(fn.Pos()).Line, "name": fn.Name.Name, "kind": kind, "body": b.String()})
			}
			return nil
		})
	}
	b, err := json.MarshalIndent(rows, "", "  ")
	if err != nil {
		panic(err)
	}
	fmt.Println(string(b))
}
func pure(fn *ast.FuncDecl) bool {
	if len(fn.Body.List) == 2 {
		a, ok := fn.Body.List[0].(*ast.AssignStmt)
		if !ok || len(a.Rhs) != 1 {
			return false
		}
		c, ok := a.Rhs[0].(*ast.CallExpr)
		if !ok {
			return false
		}
		r, ok := fn.Body.List[1].(*ast.ReturnStmt)
		if !ok || len(r.Results) != len(a.Lhs) {
			return false
		}
		for i, v := range r.Results {
			x, xok := v.(*ast.Ident)
			y, yok := a.Lhs[i].(*ast.Ident)
			if !xok || !yok || x.Name != y.Name {
				return false
			}
		}
		return pureCall(fn, c)
	}
	if len(fn.Body.List) != 1 {
		return false
	}
	var call *ast.CallExpr
	switch s := fn.Body.List[0].(type) {
	case *ast.ReturnStmt:
		if len(s.Results) != 1 {
			return false
		}
		call, _ = s.Results[0].(*ast.CallExpr)
	case *ast.ExprStmt:
		call, _ = s.X.(*ast.CallExpr)
	}
	return pureCall(fn, call)
}
func pureCall(fn *ast.FuncDecl, call *ast.CallExpr) bool {
	if call == nil {
		return false
	}
	sel, ok := call.Fun.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	field, ok := sel.X.(*ast.SelectorExpr)
	if !ok {
		return false
	}
	root := modulePassthroughSelectorRoot(field)
	if root == nil || root.Name != modulePassthroughReceiverName(fn) {
		return false
	}
	params := []string{}
	for _, f := range fn.Type.Params.List {
		for _, n := range f.Names {
			params = append(params, n.Name)
		}
	}
	if len(params) != len(call.Args) {
		return false
	}
	for i, arg := range call.Args {
		id, ok := arg.(*ast.Ident)
		if !ok || id.Name != params[i] {
			return false
		}
	}
	return true
}

func classify(fn *ast.FuncDecl) string {
	if pure(fn) {
		return "forward"
	}
	receiver := render(fn.Recv.List[0].Type)
	key := receiver + "." + fn.Name.Name
	if key == "settingOverrideRoot.StoredOverride" {
		return "transaction"
	}
	if key == "*sharedOperatorSession.LoginOperator" {
		return "synchronization"
	}
	// Reviewed guard-only sites. No value/argument/result conversion occurs here.
	guards := map[string]bool{
		"mfaPolicyLock.LockMFAPolicySharedForTenant":     true,
		"PushSubscriptionCleanup.DeleteStaffByAccount":   true,
		"PushSubscriptionCleanup.DeleteSchoolByAccount":  true,
		"PushSubscriptionCleanup.DeleteParentByAccount":  true,
		"PushSubscriptionCleanup.DeleteByTokenFamily":    true,
		"PushSubscriptionCleanup.DeleteUnboundByAccount": true,
		"PushSubscriptionCleanup.DeleteOrphaned":         true,
		"*ParentRequestLedger.ListForRequest":            true,
		"ParentSettingsBatch.ResolveManyForTenant":       true,
		"*ParentRequestReviewPolicy.Scope":               true,
		"*ParentRequestReviewPolicy.AbsenceScope":        true,
		"*ParentRequestReviewPolicy.AccessLevel":         true,
		"StudentPhotoRuntime.UnlinkStoredPhoto":          true,
		"StudentPhotos.CommitDelete":                     true,
		"StudentPhotos.LookupForRead":                    true,
	}
	if guards[key] {
		return "guard"
	}
	return "translation"
}
func render(n ast.Node) string {
	var b bytes.Buffer
	format.Node(&b, token.NewFileSet(), n)
	return b.String()
}
'''
with tempfile.TemporaryDirectory(prefix="root-passthroughs-") as tmp:
    path = Path(tmp) / "scan.go"
    path.write_text(header + source[source.index("func modulePassthroughIsDelegation"): ])
    rows = json.loads(subprocess.check_output([
        "scripts/run-go-toolchain.sh", "go", "run", str(path)], text=True))
if "--sites" in sys.argv:
    print(json.dumps(rows, indent=2))
else:
    counts = collections.defaultdict(collections.Counter)
    for row in rows:
        counts[str(Path(row["file"]).parent)][row["kind"]] += 1
    for package, values in sorted(counts.items()):
        print(package, sum(values.values()), dict(sorted(values.items())))
