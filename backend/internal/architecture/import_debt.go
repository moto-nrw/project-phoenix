package architecture

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strconv"
	"strings"
)

// CompareCandidateLegacyBaselines permits a policy tightening to record an
// already-existing, previously allowed import as debt. It never approves an
// import from the candidate tree or a violation that the base already forbade.
func CompareCandidateLegacyBaselines(project, ref string, candidate, base *LegacyManifest, basePolicy, candidatePolicy *Policy) error {
	augmented := &LegacyManifest{byKey: make(map[string]LegacyEntry, len(base.Entries))}
	for key, entry := range base.byKey {
		augmented.byKey[key] = entry
	}
	imports := make(map[string]map[Edge]struct{})
	for _, entry := range candidate.Entries {
		if _, exists := base.byKey[entry.Key()]; exists || entry.Rule != "imports.forbidden" {
			continue
		}
		edge := Edge{Scope: entry.Scope, Source: entry.Source, Target: entry.Target}
		if !importAllowed(basePolicy, edge) || importAllowed(candidatePolicy, edge) {
			continue
		}
		edges, loaded := imports[edge.Source]
		if !loaded {
			var err error
			edges, err = basePackageImports(project, ref, basePolicy, edge.Source)
			if err != nil {
				return err
			}
			imports[edge.Source] = edges
		}
		if _, existed := edges[edge]; existed {
			augmented.byKey[entry.Key()] = entry
		}
	}
	return CompareLegacyBaselines(candidate, augmented)
}

func importAllowed(policy *Policy, edge Edge) bool {
	source, exists := policy.packageMap()[edge.Source]
	if !exists {
		return false
	}
	if isOwnPackage(policy.ModulePath, edge.Target) {
		target, classified := policy.packageMap()[edge.Target]
		return classified && policyAllowsFirstParty(policy, edge.Scope, source, target)
	}
	target, classified := policy.externalPackageMap()[edge.Target]
	return classified && policyAllowsExternal(policy, edge.Scope, source, target)
}

// Materialize the import headers of this package's immutable Go files. go list
// applies filename, build-tag, cgo and test-scope rules without resolving
// dependencies. Declarations (and their embedded assets) are not needed.
func basePackageImports(project, ref string, policy *Policy, source string) (map[Edge]struct{}, error) {
	root, err := gitOutput(project, "rev-parse", "--show-toplevel")
	if err != nil {
		return nil, err
	}
	root = strings.TrimSpace(root)
	sha, err := resolveBaseCommit(root, ref)
	if err != nil {
		return nil, err
	}
	pkg, err := policy.firstPartyPackage(source)
	if err != nil {
		return nil, err
	}
	dir, err := repositoryRelativePath(root, project, filepath.Join(project, pkg.Path))
	if err != nil {
		return nil, err
	}
	names, err := gitOutput(root, "ls-tree", "-r", "--name-only", "-z", sha, "--", filepath.ToSlash(dir))
	if err != nil {
		return nil, err
	}
	snapshot, err := os.MkdirTemp("", "architecture-base-imports-")
	if err != nil {
		return nil, err
	}
	defer func() { _ = os.RemoveAll(snapshot) }()
	importsByFile := make(map[string][]string)
	for _, name := range strings.Split(names, "\x00") {
		if filepath.Ext(name) != ".go" || filepath.Clean(filepath.Dir(name)) != filepath.Clean(dir) {
			continue
		}
		blob, err := gitOutput(root, "cat-file", "blob", sha+":"+name)
		if err != nil {
			return nil, err
		}
		files := token.NewFileSet()
		header, err := parser.ParseFile(files, name, blob, parser.ImportsOnly)
		if err != nil {
			return nil, fmt.Errorf("parse base imports in %s: %w", name, err)
		}
		for _, spec := range header.Imports {
			target, err := strconv.Unquote(spec.Path.Value)
			if err != nil {
				return nil, fmt.Errorf("parse base import in %s: %w", name, err)
			}
			importsByFile[filepath.Base(name)] = append(importsByFile[filepath.Base(name)], target)
		}
		content := []byte(blob[:files.Position(header.End()).Offset] + "\n")
		if err := os.WriteFile(filepath.Join(snapshot, filepath.Base(name)), content, 0600); err != nil {
			return nil, err
		}
	}
	environment := append(fixedBuildEnvironment(policy.Build), "GO111MODULE=off")
	output, err := processOutput(snapshot, environment, "go", "list", "-find", "-json", ".")
	if err != nil {
		return nil, fmt.Errorf("read base imports for %s: %w: %s", source, err, strings.TrimSpace(string(output)))
	}
	var pkgImports goListPackage
	if err := json.Unmarshal(output, &pkgImports); err != nil {
		return nil, fmt.Errorf("decode base imports for %s: %w", source, err)
	}
	edges := make(map[Edge]struct{})
	for scope, names := range map[Scope][]string{
		ScopeProduction: pkgImports.GoFiles, ScopeInternalTest: pkgImports.TestGoFiles, ScopeExternalTest: pkgImports.XTestGoFiles,
	} {
		for _, name := range names {
			for _, target := range importsByFile[name] {
				edges[Edge{Scope: scope, Source: source, Target: target}] = struct{}{}
			}
		}
	}
	return edges, nil
}
