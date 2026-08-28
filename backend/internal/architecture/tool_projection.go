package architecture

import (
	"fmt"
	"sort"
	"strings"
)

// GoArchLintProjection emits a coarse additional guard derived from the canonical policy.
func GoArchLintProjection(policy *Policy) []byte {
	packages := packagesByOwner(policy)
	projectDeps := projectDependencies(policy)
	vendorDeps := vendorDependencies(policy)
	var output strings.Builder
	output.WriteString("version: 3\nworkdir: .\nallow:\n  depOnAnyVendor: false\n  deepScan: false\nexcludeFiles:\n  - \"^.*_test\\\\.go$\"\n  - \"^.*internal/architecture/testdata/.*\\\\.go$\"\ncomponents:\n")
	for _, owner := range sortedMapKeys(packages) {
		fmt.Fprintf(&output, "  %s:\n    in:\n", owner)
		for _, path := range packages[owner] {
			fmt.Fprintf(&output, "      - %s\n", yamlScalar(path))
		}
	}
	writeVendors(&output, policy)
	output.WriteString("deps:\n")
	for _, owner := range sortedMapKeys(packages) {
		if len(projectDeps[owner]) == 0 && len(vendorDeps[owner]) == 0 {
			continue
		}
		fmt.Fprintf(&output, "  %s:\n", owner)
		writeInlineList(&output, "mayDependOn", projectDeps[owner])
		writeInlineList(&output, "canUse", vendorDeps[owner])
	}
	return []byte(output.String())
}

// GodaQuery emits a focused query pinned to the evaluator's build context.
func GodaQuery(policy *Policy, graph *Graph, rawFocus string) (string, error) {
	focus, err := resolveFocus(policy, graph, rawFocus)
	if err != nil {
		return "", err
	}
	packages := []string{policy.absolutePackage(focus.value)}
	if focus.kind == "module" {
		packages = nil
		for _, pkg := range policy.Packages {
			if pkg.Owner == focus.value {
				packages = append(packages, policy.absolutePackage(pkg.Path))
			}
		}
	}
	sort.Strings(packages)
	if len(packages) == 0 {
		return "", fmt.Errorf("module %q is declared by the policy but has no classified package in the current graph", focus.value)
	}
	selected := godaUnion(packages)
	expression := fmt.Sprintf("add(%s:+import,incoming(./...:module,%s))", selected, selected)
	return fmt.Sprintf("cgo_enabled=0(goarch=%s(goos=%s(%s)))\n", policy.Build.GOARCH, policy.Build.GOOS, expression), nil
}

func packagesByOwner(policy *Policy) map[string][]string {
	result := make(map[string][]string)
	for _, pkg := range policy.Packages {
		result[pkg.Owner] = append(result[pkg.Owner], pkg.Path)
	}
	for owner := range result {
		sort.Strings(result[owner])
	}
	return result
}

func projectDependencies(policy *Policy) map[string][]string {
	sets := make(map[string]map[string]struct{})
	packages := packagesByOwner(policy)
	for _, edge := range allowedOwnerEdges(policy) {
		if len(packages[edge.Source]) == 0 || len(packages[edge.Target]) == 0 {
			continue
		}
		if sets[edge.Source] == nil {
			sets[edge.Source] = make(map[string]struct{})
		}
		sets[edge.Source][edge.Target] = struct{}{}
	}
	return sortedDependencySets(sets)
}

func vendorDependencies(policy *Policy) map[string][]string {
	sets := make(map[string]map[string]struct{})
	for _, pkg := range policy.Packages {
		for _, rule := range policy.Rules {
			if rule.TargetClass == "" || !ruleHasScope(rule, ScopeProduction) || !policy.matchesSource(rule, pkg) {
				continue
			}
			if sets[pkg.Owner] == nil {
				sets[pkg.Owner] = make(map[string]struct{})
			}
			sets[pkg.Owner][rule.TargetClass] = struct{}{}
		}
	}
	return sortedDependencySets(sets)
}

func sortedDependencySets(sets map[string]map[string]struct{}) map[string][]string {
	result := make(map[string][]string, len(sets))
	for owner, values := range sets {
		result[owner] = sortedSet(values)
	}
	return result
}

func writeVendors(output *strings.Builder, policy *Policy) {
	classes := make(map[string][]string)
	for _, pkg := range policy.ExternalPackages {
		classes[pkg.Class] = append(classes[pkg.Class], pkg.Path)
	}
	if len(classes) == 0 {
		return
	}
	output.WriteString("vendors:\n")
	for _, class := range sortedMapKeys(classes) {
		sort.Strings(classes[class])
		fmt.Fprintf(output, "  %s:\n    in:\n", class)
		for _, path := range classes[class] {
			fmt.Fprintf(output, "      - %s\n", yamlScalar(path))
		}
	}
}

func writeInlineList(output *strings.Builder, label string, values []string) {
	if len(values) > 0 {
		fmt.Fprintf(output, "    %s: [%s]\n", label, strings.Join(values, ", "))
	}
}

func sortedMapKeys[V any](values map[string]V) []string {
	keys := make([]string, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Strings(keys)
	return keys
}

func yamlScalar(value string) string {
	return fmt.Sprintf("%q", value)
}

func godaUnion(packages []string) string {
	if len(packages) == 1 {
		return packages[0]
	}
	return "add(" + strings.Join(packages, ",") + ")"
}
