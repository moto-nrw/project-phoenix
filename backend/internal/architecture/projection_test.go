package architecture_test

import (
	"bytes"
	"encoding/json"
	"encoding/xml"
	"io"
	"slices"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/architecture"
)

func TestTargetProjectionContainsOnlyPolicyAllowedOwnerEdges(t *testing.T) {
	t.Parallel()

	policy := loadProjectionPolicy(t)
	projection := architecture.TargetProjection(policy)

	if got := projectionEdgeSummaries(projection); !slices.Equal(got, []string{"alpha->beta:allowed"}) {
		t.Fatalf("target edges = %v, want only the policy edge", got)
	}
	if got := projectionNodeIDs(projection); !slices.Equal(got, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("target nodes = %v", got)
	}
}

func TestTargetProjectionIncludesDeclaredOwnersWithoutCurrentPackages(t *testing.T) {
	t.Parallel()

	policy := loadProjectionPolicy(t)
	policy.Owners = append(policy.Owners, architecture.Owner{ID: "future", Kind: "domain"})
	policy.Rules = append(policy.Rules, architecture.Rule{
		ID: "beta-to-future", Description: "Beta may call the future module.", Scopes: []string{"production"},
		SourceOwner: "beta", SourceRole: "application", TargetOwner: "future", TargetRole: "domain",
	})
	projection := architecture.TargetProjection(policy)

	if !slices.Contains(projectionNodeIDs(projection), "future") {
		t.Fatalf("target projection omits declared future owner: %#v", projection.Nodes)
	}
	if !slices.Contains(projectionEdgeSummaries(projection), "beta->future:allowed") {
		t.Fatalf("target projection omits declared future edge: %#v", projection.Edges)
	}
}

func TestTargetProjectionExcludesNonModuleOwners(t *testing.T) {
	t.Parallel()

	policy := loadProjectionPolicy(t)
	policy.Owners = append(policy.Owners, architecture.Owner{ID: "http-adapter", Kind: "inbound"})
	policy.Packages = append(policy.Packages, architecture.Package{Path: "http", Owner: "http-adapter", Role: "application"})
	projection := architecture.TargetProjection(policy)

	if slices.Contains(projectionNodeIDs(projection), "http-adapter") {
		t.Fatalf("target projection contains non-module owner: %#v", projection.Nodes)
	}
}

func TestMigrationProjectionClassifiesAllowedLegacyAndNewEdges(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	projection := architecture.MigrationProjection(policy, graph, baseline)

	want := []string{"alpha->beta:allowed", "alpha->gamma:new", "beta->gamma:legacy"}
	if got := projectionEdgeSummaries(projection); !slices.Equal(got, want) {
		t.Fatalf("migration edges = %v, want %v", got, want)
	}

	svg := renderValidSVG(t, projection)
	for _, marker := range []string{`stroke="#737373"`, `stroke="#d65a31"`, `stroke="#d00000"`, `stroke-dasharray="8 6"`} {
		if !bytes.Contains(svg, []byte(marker)) {
			t.Errorf("migration SVG does not contain %s", marker)
		}
	}
}

func TestMigrationProjectionPreservesEveryStatusBetweenTheSameOwners(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	legacy := architecture.Violation{
		Scope: architecture.ScopeProduction, Rule: "tables.foreign-read",
		Source: policy.ModulePath + "/alpha", Target: "beta.legacy",
	}
	current := architecture.Violation{
		Scope: architecture.ScopeProduction, Rule: "tables.foreign-write",
		Source: policy.ModulePath + "/alpha", Target: "beta.current",
	}
	policy.DataObjects = append(policy.DataObjects,
		architecture.DataObject{Name: legacy.Target, WriteOwner: "beta"},
		architecture.DataObject{Name: current.Target, WriteOwner: "beta"},
	)
	graph.SemanticViolations = append(graph.SemanticViolations, legacy, current)
	baseline.Entries = append(baseline.Entries, architecture.LegacyEntry{Violation: legacy, Issue: "https://github.com/moto-nrw/project-phoenix/issues/2584"})

	projection := architecture.MigrationProjection(policy, graph, baseline)
	var statuses []architecture.ProjectionStatus
	for _, edge := range projection.Edges {
		if edge.Source == "alpha" && edge.Target == "beta" {
			statuses = append(statuses, edge.Status)
		}
	}
	if want := []architecture.ProjectionStatus{architecture.ProjectionAllowed, architecture.ProjectionLegacy, architecture.ProjectionNew}; !slices.Equal(statuses, want) {
		t.Fatalf("alpha -> beta statuses = %v, want %v", statuses, want)
	}
}

func TestMigrationProjectionDrawsExternalAndUnclassifiedViolations(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	unknownPackage := policy.ModulePath + "/unknown"
	unknownExternal := "example.test/unknown-external"
	graph.Packages = append(graph.Packages, unknownPackage)
	graph.Edges = append(graph.Edges, architecture.Edge{
		Scope: architecture.ScopeProduction, Source: policy.ModulePath + "/alpha", Target: unknownExternal,
	})

	projection := architecture.MigrationProjection(policy, graph, baseline)
	edgeKeys := make(map[string]struct{})
	for _, edge := range projection.Edges {
		for _, key := range edge.ViolationKeys {
			edgeKeys[key] = struct{}{}
		}
	}
	for _, violation := range projection.Violations {
		if _, visible := edgeKeys[violation.Key]; !visible {
			t.Errorf("violation %q has no migration edge", violation.Key)
		}
	}
}

func TestMigrationProjectionIncludesSemanticViolationsAndOwnership(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	semantic := architecture.Violation{
		Scope: architecture.ScopeProduction, Rule: "tables.foreign-write",
		Source: policy.ModulePath + "/alpha", Target: "gamma.records",
	}
	graph.SemanticViolations = append(graph.SemanticViolations, semantic)
	projection := architecture.MigrationProjection(policy, graph, baseline)

	var found bool
	for _, violation := range projection.Violations {
		if violation.Key == semantic.Key() {
			found = violation.SourceOwner == "alpha" && violation.TargetOwner == "gamma" && violation.Status == architecture.ProjectionNew
		}
	}
	if !found {
		t.Fatalf("semantic violation ownership missing from machine projection: %#v", projection.Violations)
	}
}

func TestMigrationProjectionAddsSemanticTargetOwnersWithoutPackages(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	policy.Owners = append(policy.Owners, architecture.Owner{ID: "future", Kind: "domain"})
	policy.DataObjects = append(policy.DataObjects, architecture.DataObject{Name: "future.records", WriteOwner: "future"})
	graph.SemanticViolations = append(graph.SemanticViolations, architecture.Violation{
		Scope: architecture.ScopeProduction, Rule: "tables.foreign-write",
		Source: policy.ModulePath + "/alpha", Target: "future.records",
	})
	projection := architecture.MigrationProjection(policy, graph, baseline)

	if !slices.Contains(projectionNodeIDs(projection), "future") {
		t.Fatalf("migration projection omits semantic target owner: %#v", projection.Nodes)
	}
	if _, err := architecture.RenderSVG(projection); err != nil {
		t.Fatalf("render migration projection with target-only semantic owner: %v", err)
	}
}

func TestSVGAndMachineProjectionAreByteStable(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	projection := architecture.MigrationProjection(policy, graph, baseline)
	first := renderValidSVG(t, projection)
	second := renderValidSVG(t, projection)
	if !bytes.Equal(first, second) {
		t.Fatal("identical projections produced different SVG bytes")
	}

	firstJSON, err := architecture.MarshalProjection(projection)
	if err != nil {
		t.Fatalf("marshal projection: %v", err)
	}
	secondJSON, err := architecture.MarshalProjection(projection)
	if err != nil {
		t.Fatalf("marshal projection again: %v", err)
	}
	if !bytes.Equal(firstJSON, secondJSON) || !json.Valid(firstJSON) {
		t.Fatal("machine projection is not deterministic valid JSON")
	}
}

func TestDependenciesProjectionValidatesAndFiltersFocus(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	module, err := architecture.DependenciesProjection(policy, graph, baseline, "module:beta")
	if err != nil {
		t.Fatalf("focus module: %v", err)
	}
	if got := projectionNodeIDs(module); !slices.Equal(got, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("module focus nodes = %v", got)
	}
	for _, edge := range module.Edges {
		if edge.Source != "beta" && edge.Target != "beta" {
			t.Fatalf("module focus contains neighbor-only edge: %#v", edge)
		}
	}

	pkg, err := architecture.DependenciesProjection(policy, graph, baseline, "package:alpha")
	if err != nil {
		t.Fatalf("focus package: %v", err)
	}
	if got := projectionNodeIDs(pkg); !slices.Equal(got, []string{"alpha", "beta", "gamma"}) {
		t.Fatalf("package focus nodes = %v", got)
	}
	for _, edge := range pkg.Edges {
		if edge.Source != "alpha" && edge.Target != "alpha" {
			t.Fatalf("package focus contains neighbor-only edge: %#v", edge)
		}
	}

	for focus, want := range map[string]string{
		"beta":    "ambiguous focus",
		"missing": "unknown focus",
	} {
		if _, err := architecture.DependenciesProjection(policy, graph, baseline, focus); err == nil || !strings.Contains(err.Error(), want) {
			t.Errorf("focus %q error = %v, want %q", focus, err, want)
		}
	}
}

func TestDependenciesProjectionRejectsStalePolicyFocus(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	policy.Owners = append(policy.Owners, architecture.Owner{ID: "future", Kind: "domain"})
	policy.Packages = append(policy.Packages, architecture.Package{Path: "future", Owner: "future", Role: "domain"})
	for _, focus := range []string{"module:future", "package:future"} {
		if _, err := architecture.DependenciesProjection(policy, graph, baseline, focus); err == nil || !strings.Contains(err.Error(), "current graph") {
			t.Errorf("focus %q error = %v, want current graph error", focus, err)
		}
	}
}

func TestPackageDependenciesProjectionDrawsSemanticViolations(t *testing.T) {
	t.Parallel()

	policy, graph, baseline := loadProjectionInputs(t)
	semantic := architecture.Violation{
		Scope: architecture.ScopeProduction, Rule: "tables.foreign-write",
		Source: policy.ModulePath + "/alpha", Target: "gamma.records",
	}
	graph.SemanticViolations = append(graph.SemanticViolations, semantic)
	projection, err := architecture.DependenciesProjection(policy, graph, baseline, "package:alpha")
	if err != nil {
		t.Fatalf("focus package with semantic violation: %v", err)
	}

	if !slices.Contains(projectionNodeIDs(projection), "owner:gamma") {
		t.Fatalf("package focus omits semantic target owner: %#v", projection.Nodes)
	}
	if !slices.Contains(projectionEdgeSummaries(projection), "alpha->owner:gamma:new") {
		t.Fatalf("package focus omits semantic violation edge: %#v", projection.Edges)
	}
}

func TestToolProjectionsComeFromPolicyAndBuildContext(t *testing.T) {
	t.Parallel()

	policy := loadProjectionPolicy(t)
	config := string(architecture.GoArchLintProjection(policy))
	for _, want := range []string{"components:", "alpha:", "beta:", "mayDependOn: [beta]"} {
		if !strings.Contains(config, want) {
			t.Errorf("go-arch-lint projection does not contain %q:\n%s", want, config)
		}
	}
	if strings.Contains(config, "mayDependOn: [gamma]") {
		t.Fatalf("go-arch-lint projection invented a target edge:\n%s", config)
	}

	_, graph, _ := loadProjectionInputs(t)
	query, err := architecture.GodaQuery(policy, graph, "module:alpha")
	if err != nil {
		t.Fatalf("generate Goda query: %v", err)
	}
	policy.Owners = append(policy.Owners, architecture.Owner{ID: "future", Kind: "domain"})
	policy.Packages = append(policy.Packages, architecture.Package{Path: "future", Owner: "future", Role: "domain"})
	if _, err := architecture.GodaQuery(policy, graph, "package:future"); err == nil || !strings.Contains(err.Error(), "current graph") {
		t.Fatalf("Goda accepted stale package focus: %v", err)
	}
	for _, want := range []string{"goos=linux", "goarch=amd64", "cgo_enabled=0", policy.ModulePath + "/alpha"} {
		if !strings.Contains(query, want) {
			t.Errorf("Goda query does not contain %q: %s", want, query)
		}
	}
}

func loadProjectionPolicy(t *testing.T) *architecture.Policy {
	t.Helper()
	policy, err := architecture.LoadPolicy(fixturePath(t, "projection", "policy.json"))
	if err != nil {
		t.Fatalf("load projection policy: %v", err)
	}
	return policy
}

func loadProjectionInputs(t *testing.T) (*architecture.Policy, *architecture.Graph, *architecture.LegacyManifest) {
	t.Helper()
	policy := loadProjectionPolicy(t)
	graph, err := architecture.LoadGraph(fixturePath(t, "projection"), policy)
	if err != nil {
		t.Fatalf("load projection graph: %v", err)
	}
	baseline, err := architecture.LoadLegacyManifest(fixturePath(t, "projection", "legacy.jsonl"))
	if err != nil {
		t.Fatalf("load projection baseline: %v", err)
	}
	return policy, graph, baseline
}

func projectionEdgeSummaries(projection architecture.Projection) []string {
	result := make([]string, 0, len(projection.Edges))
	for _, edge := range projection.Edges {
		result = append(result, edge.Source+"->"+edge.Target+":"+string(edge.Status))
	}
	return result
}

func projectionNodeIDs(projection architecture.Projection) []string {
	result := make([]string, 0, len(projection.Nodes))
	for _, node := range projection.Nodes {
		result = append(result, node.ID)
	}
	return result
}

func renderValidSVG(t *testing.T, projection architecture.Projection) []byte {
	t.Helper()
	result, err := architecture.RenderSVG(projection)
	if err != nil {
		t.Fatalf("render SVG: %v", err)
	}
	assertValidSVG(t, result)
	return result
}

func assertValidSVG(t *testing.T, result []byte) {
	t.Helper()
	decoder := xml.NewDecoder(bytes.NewReader(result))
	for {
		if _, err := decoder.Token(); err != nil {
			if err == io.EOF {
				return
			}
			t.Fatalf("invalid SVG XML: %v\n%s", err, result)
		}
	}
}
