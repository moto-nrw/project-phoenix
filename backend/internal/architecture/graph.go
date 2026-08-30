package architecture

import (
	"encoding/json"
	"fmt"
	"go/parser"
	"go/token"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
)

type Scope string

const (
	ScopeProduction   Scope = "production"
	ScopeInternalTest Scope = "internal_test"
	ScopeExternalTest Scope = "external_test"
)

type Edge struct {
	Scope  Scope
	Source string
	Target string
}

type Graph struct {
	Packages           []string
	Edges              []Edge
	ImportLocations    map[Edge][]Location
	PackageLocations   map[string][]Location
	SemanticViolations []Violation
}

type goListPackage struct {
	ImportPath   string
	Dir          string
	Imports      []string
	TestImports  []string
	XTestImports []string
	GoFiles      []string
	TestGoFiles  []string
	XTestGoFiles []string
}

func LoadGraph(project string, policy *Policy) (*Graph, error) {
	output, err := runGoList(project, policy.Build)
	if err != nil {
		return nil, err
	}
	graph, err := decodeGraph(output, policy.ModulePath, project)
	if err != nil {
		return nil, err
	}
	graph.SemanticViolations, err = analyzeSemantics(project, policy)
	if err != nil {
		return nil, err
	}
	return graph, nil
}

func runGoList(project string, build Build) ([]byte, error) {
	command := exec.Command("go", "list", "-json", "./...")
	command.Dir = project
	command.Env = fixedBuildEnvironment(build)
	output, err := command.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return nil, fmt.Errorf("load Go packages with GOOS=%s GOARCH=%s CGO_ENABLED=0: %s", build.GOOS, build.GOARCH, strings.TrimSpace(string(exitErr.Stderr)))
		}
		return nil, fmt.Errorf("load Go packages: %w", err)
	}
	return output, nil
}

func fixedBuildEnvironment(build Build) []string {
	environment := make([]string, 0, len(os.Environ())+3)
	for _, entry := range os.Environ() {
		if strings.HasPrefix(entry, "GOOS=") || strings.HasPrefix(entry, "GOARCH=") || strings.HasPrefix(entry, "CGO_ENABLED=") {
			continue
		}
		environment = append(environment, entry)
	}
	return append(environment, "GOOS="+build.GOOS, "GOARCH="+build.GOARCH, "CGO_ENABLED=0")
}

func decodeGraph(output []byte, modulePath, project string) (*Graph, error) {
	graph := &Graph{
		ImportLocations:  make(map[Edge][]Location),
		PackageLocations: make(map[string][]Location),
	}
	decoder := json.NewDecoder(strings.NewReader(string(output)))
	for {
		var pkg goListPackage
		if err := decoder.Decode(&pkg); err != nil {
			if err == io.EOF {
				break
			}
			return nil, fmt.Errorf("decode go list output: %w", err)
		}
		if !isOwnPackage(modulePath, pkg.ImportPath) {
			continue
		}
		if err := appendPackageGraph(graph, project, pkg); err != nil {
			return nil, err
		}
	}
	if err := finishGraph(graph); err != nil {
		return nil, err
	}
	return graph, nil
}

func appendPackageGraph(graph *Graph, project string, pkg goListPackage) error {
	graph.Packages = append(graph.Packages, pkg.ImportPath)
	graph.Edges = appendImports(graph.Edges, ScopeProduction, pkg.ImportPath, pkg.Imports)
	graph.Edges = appendImports(graph.Edges, ScopeInternalTest, pkg.ImportPath, pkg.TestImports)
	graph.Edges = appendImports(graph.Edges, ScopeExternalTest, pkg.ImportPath, pkg.XTestImports)
	for _, input := range []struct {
		scope Scope
		files []string
	}{
		{scope: ScopeProduction, files: pkg.GoFiles},
		{scope: ScopeInternalTest, files: pkg.TestGoFiles},
		{scope: ScopeExternalTest, files: pkg.XTestGoFiles},
	} {
		if err := addImportLocations(graph, project, pkg, input.scope, input.files); err != nil {
			return err
		}
	}
	graph.PackageLocations[pkg.ImportPath] = uniqueSortedLocations(graph.PackageLocations[pkg.ImportPath])
	return nil
}

func finishGraph(graph *Graph) error {
	sort.Strings(graph.Packages)
	for _, packagePath := range graph.Packages {
		locations := uniqueSortedLocations(graph.PackageLocations[packagePath])
		if len(locations) == 0 {
			return fmt.Errorf("resolve package location for %s: no active Go source file", packagePath)
		}
		graph.PackageLocations[packagePath] = locations
	}
	sort.Slice(graph.Edges, func(i, j int) bool {
		left, right := graph.Edges[i], graph.Edges[j]
		if left.Scope != right.Scope {
			return left.Scope < right.Scope
		}
		if left.Source != right.Source {
			return left.Source < right.Source
		}
		return left.Target < right.Target
	})
	for _, edge := range graph.Edges {
		locations := uniqueSortedLocations(graph.ImportLocations[edge])
		if len(locations) == 0 {
			return fmt.Errorf("resolve import locations for %s edge %s -> %s: no active Go file imports the target", edge.Scope, edge.Source, edge.Target)
		}
		graph.ImportLocations[edge] = locations
	}
	return nil
}

func addImportLocations(graph *Graph, project string, pkg goListPackage, scope Scope, files []string) error {
	for _, name := range files {
		filename := filepath.Join(pkg.Dir, name)
		fileset := token.NewFileSet()
		file, err := parser.ParseFile(fileset, filename, nil, parser.ImportsOnly)
		if err != nil {
			return fmt.Errorf("resolve import locations in %s: %w", filename, err)
		}
		location, err := sourceLocation(project, fileset.PositionFor(file.Package, false), "package "+file.Name.Name)
		if err != nil {
			return fmt.Errorf("resolve package location for %s: %w", pkg.ImportPath, err)
		}
		graph.PackageLocations[pkg.ImportPath] = append(graph.PackageLocations[pkg.ImportPath], location)
		for _, imported := range file.Imports {
			target, err := strconv.Unquote(imported.Path.Value)
			if err != nil {
				return fmt.Errorf("resolve import location in %s: decode import path: %w", filename, err)
			}
			location, err := sourceLocation(project, fileset.PositionFor(imported.Path.Pos(), false), "import "+target)
			if err != nil {
				return fmt.Errorf("resolve import location for %s -> %s: %w", pkg.ImportPath, target, err)
			}
			edge := Edge{Scope: scope, Source: pkg.ImportPath, Target: target}
			graph.ImportLocations[edge] = append(graph.ImportLocations[edge], location)
		}
	}
	return nil
}

func sourceLocation(project string, position token.Position, declaration string) (Location, error) {
	if !position.IsValid() || position.Line < 1 || position.Filename == "" {
		return Location{}, fmt.Errorf("source position is unavailable")
	}
	relative, ok := relativeSourcePath(project, position.Filename)
	if !ok {
		resolvedProject, projectErr := filepath.EvalSymlinks(project)
		resolvedFile, fileErr := filepath.EvalSymlinks(position.Filename)
		if projectErr != nil || fileErr != nil {
			return Location{}, fmt.Errorf("source path is outside project")
		}
		relative, ok = relativeSourcePath(resolvedProject, resolvedFile)
		if !ok {
			return Location{}, fmt.Errorf("source path is outside project")
		}
	}
	return Location{File: filepath.ToSlash(filepath.Clean(relative)), Line: position.Line, Declaration: declaration}, nil
}

func relativeSourcePath(project, filename string) (string, bool) {
	relative, err := filepath.Rel(project, filename)
	if err != nil || relative == ".." || strings.HasPrefix(relative, ".."+string(filepath.Separator)) || filepath.IsAbs(relative) {
		return "", false
	}
	return relative, true
}

func appendImports(edges []Edge, scope Scope, source string, imports []string) []Edge {
	for _, target := range imports {
		edges = append(edges, Edge{Scope: scope, Source: source, Target: target})
	}
	return edges
}

func isOwnPackage(modulePath, importPath string) bool {
	return importPath == modulePath || strings.HasPrefix(importPath, modulePath+"/")
}
