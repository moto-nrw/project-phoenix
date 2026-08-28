package architecture

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"sort"
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
	SemanticViolations []Violation
}

type goListPackage struct {
	ImportPath   string
	Imports      []string
	TestImports  []string
	XTestImports []string
}

func LoadGraph(project string, policy *Policy) (*Graph, error) {
	output, err := runGoList(project, policy.Build)
	if err != nil {
		return nil, err
	}
	graph, err := decodeGraph(output, policy.ModulePath)
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

func decodeGraph(output []byte, modulePath string) (*Graph, error) {
	graph := &Graph{}
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
		graph.Packages = append(graph.Packages, pkg.ImportPath)
		graph.Edges = appendImports(graph.Edges, ScopeProduction, pkg.ImportPath, pkg.Imports)
		graph.Edges = appendImports(graph.Edges, ScopeInternalTest, pkg.ImportPath, pkg.TestImports)
		graph.Edges = appendImports(graph.Edges, ScopeExternalTest, pkg.ImportPath, pkg.XTestImports)
	}

	sort.Strings(graph.Packages)
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
	return graph, nil
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
