package architecture

import (
	"fmt"
	"path/filepath"
	"strings"

	"golang.org/x/tools/go/packages"
)

func loadTypedPackages(project string, build Build) ([]*packages.Package, error) {
	return loadTypedPatterns(project, build, nil, "./...")
}

// loadTypedPatterns loads patterns as loadTypedPackages loads the tree, with
// overlay replacing or adding files; rule tests check mutations through it.
func loadTypedPatterns(project string, build Build, overlay map[string][]byte, patterns ...string) ([]*packages.Package, error) {
	environment := fixedBuildEnvironment(build)
	dependencyRoots, err := dependencySourceRoots(project, environment)
	if err != nil {
		return nil, err
	}
	// Every first-party package is an initial ./... root with typed syntax.
	// Dependencies need exported types, not their own syntax or type-info maps.
	config := &packages.Config{
		Mode: packages.NeedName | packages.NeedFiles | packages.NeedCompiledGoFiles | packages.NeedImports |
			packages.NeedSyntax | packages.NeedTypes | packages.NeedTypesInfo,
		Dir:       project,
		Env:       environment,
		ParseFile: parseWithoutDependencyBodies(dependencyRoots),
		Overlay:   overlay,
	}
	loaded, err := packages.Load(config, patterns...)
	if err != nil {
		return nil, fmt.Errorf("load Go types: %w", err)
	}
	for _, pkg := range loaded {
		if len(pkg.Errors) > 0 {
			return nil, fmt.Errorf("load Go types for %s: %s", pkg.PkgPath, pkg.Errors[0])
		}
	}
	return loaded, nil
}

// dependencySourceRoots returns the directories that hold standard-library and
// module-cache sources. Packages loaded from there are never analyzed; they
// are type-checked only so the project's own packages resolve against them.
func dependencySourceRoots(project string, environment []string) ([]string, error) {
	output, err := processOutput(project, environment, "go", "env", "GOROOT", "GOMODCACHE")
	if err != nil {
		return nil, fmt.Errorf("locate dependency sources: %w: %s", err, strings.TrimSpace(string(output)))
	}
	var roots []string
	for _, line := range strings.Split(string(output), "\n") {
		if root := strings.TrimSpace(line); root != "" {
			roots = append(roots, filepath.Clean(root)+string(filepath.Separator))
		}
	}
	return roots, nil
}
