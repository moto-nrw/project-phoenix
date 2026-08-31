package architecture

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
)

func candidateDeletedLegacySymbols(project string, base, candidate *Policy) (map[string]struct{}, error) {
	candidateSymbols := compositionSymbols(candidate)
	deleted := make(map[string]struct{})
	for _, legacy := range base.LegacyComposition {
		declared, err := packageDeclarations(filepath.Join(project, filepath.FromSlash(legacy.Package)))
		if err != nil {
			return nil, err
		}
		for _, symbol := range legacy.Symbols {
			key := base.absolutePackage(legacy.Package) + "." + symbol
			if _, guarded := candidateSymbols[key]; guarded {
				continue
			}
			if _, exists := declared[symbol]; !exists {
				deleted[key] = struct{}{}
			}
		}
	}
	return deleted, nil
}

func packageDeclarations(dir string) (map[string]struct{}, error) {
	declarations := make(map[string]struct{})
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return declarations, nil
		}
		return nil, fmt.Errorf("inspect legacy composition package %s: %w", dir, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || filepath.Ext(entry.Name()) != ".go" || strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		filename := filepath.Join(dir, entry.Name())
		file, err := parser.ParseFile(token.NewFileSet(), filename, nil, 0)
		if err != nil {
			return nil, fmt.Errorf("parse legacy composition package %s: %w", filename, err)
		}
		for _, declaration := range file.Decls {
			switch value := declaration.(type) {
			case *ast.FuncDecl:
				if value.Recv == nil {
					declarations[value.Name.Name] = struct{}{}
				}
			case *ast.GenDecl:
				for _, spec := range value.Specs {
					switch value := spec.(type) {
					case *ast.TypeSpec:
						declarations[value.Name.Name] = struct{}{}
					case *ast.ValueSpec:
						for _, name := range value.Names {
							declarations[name.Name] = struct{}{}
						}
					}
				}
			}
		}
	}
	return declarations, nil
}
