package architecture

import (
	"bytes"
	"encoding/json"
	"reflect"
	"strings"
	"testing"
)

func TestTargetCyclesReportCyclicComponents(t *testing.T) {
	t.Parallel()
	tests := []struct {
		fixture    string
		edges      int
		components [][]string
		inside     []string
	}{
		{fixture: "acyclic.json", edges: 3, components: [][]string{}},
		{fixture: "two-node.json", edges: 3, components: [][]string{{"alpha", "beta"}}, inside: []string{"alpha->beta", "beta->alpha"}},
		{fixture: "shared-node.json", edges: 5, components: [][]string{{"alpha", "beta", "gamma"}}, inside: []string{"alpha->beta", "alpha->gamma", "beta->alpha", "gamma->alpha"}},
	}
	for _, test := range tests {
		t.Run(test.fixture, func(t *testing.T) {
			t.Parallel()
			graph := AnalyzeCycles(TargetProjection(loadCyclePolicy(t, test.fixture)))
			if graph.Kind != "target" || graph.Nodes != 4 || graph.Edges != test.edges {
				t.Fatalf("graph summary = %s/%d/%d, want target/4/%d", graph.Kind, graph.Nodes, graph.Edges, test.edges)
			}
			if got := cycleMembers(graph); !reflect.DeepEqual(got, test.components) {
				t.Fatalf("components = %v, want %v", got, test.components)
			}
			if len(graph.Components) == 0 {
				return
			}
			component := graph.Components[0]
			if got := cycleEdgeSummaries(component); !reflect.DeepEqual(got, test.inside) {
				t.Fatalf("component edges = %v, want %v", got, test.inside)
			}
			if component.AfterRatchet != nil {
				t.Fatalf("target component reports a ratchet outcome: %v", component.AfterRatchet)
			}
			first := component.Edges[0]
			want := CycleEdge{Source: "alpha", SourceKind: "domain", Target: "beta", TargetKind: "platform", Provenance: CycleTarget, Rules: []string{"alpha-to-beta"}, ViolationKeys: []string{}}
			if !reflect.DeepEqual(first, want) {
				t.Fatalf("first edge = %+v, want %+v", first, want)
			}
		})
	}
}

func TestMigrationCyclesSeparateBaselineOnlyEdges(t *testing.T) {
	t.Parallel()
	policy, graph, baseline := loadCycleBaselineInputs(t)

	if target := AnalyzeCycles(TargetProjection(policy)); len(target.Components) != 0 {
		t.Fatalf("target graph is cyclic: %v", cycleMembers(target))
	}
	migration := AnalyzeCycles(MigrationProjection(policy, graph, baseline))
	if got, want := cycleMembers(migration), [][]string{{"alpha", "beta"}}; !reflect.DeepEqual(got, want) {
		t.Fatalf("migration components = %v, want %v", got, want)
	}
	component := migration.Components[0]
	provenance := map[string]CycleProvenance{}
	for _, edge := range component.Edges {
		provenance[edge.Source+"->"+edge.Target] = edge.Provenance
	}
	if want := map[string]CycleProvenance{"alpha->beta": CycleTarget, "beta->alpha": CycleBaseline}; !reflect.DeepEqual(provenance, want) {
		t.Fatalf("edge provenance = %v, want %v", provenance, want)
	}
	if len(component.Edges[1].ViolationKeys) != 1 {
		t.Fatalf("baseline edge lost its violation key: %+v", component.Edges[1])
	}
	if component.AfterRatchet == nil || len(component.AfterRatchet) != 0 {
		t.Fatalf("after ratchet = %v, want an empty list", component.AfterRatchet)
	}

	unbaselined := AnalyzeCycles(MigrationProjection(policy, graph, nil))
	if got := unbaselined.Components[0].Edges[1].Provenance; got != CycleNew {
		t.Fatalf("unbaselined edge provenance = %q, want %q", got, CycleNew)
	}
}

func TestCyclesIgnoreSelfEdgesAndExternalNodes(t *testing.T) {
	t.Parallel()
	projection := Projection{Kind: "migration", Nodes: []ProjectionNode{
		{ID: "alpha", Kind: "domain"}, {ID: "external:database", Kind: "external"}, {ID: "unclassified:tmp", Kind: "unclassified"},
	}, Edges: []ProjectionEdge{
		{Source: "alpha", Target: "alpha", Status: ProjectionAllowed},
		{Source: "alpha", Target: "external:database", Status: ProjectionAllowed},
		{Source: "external:database", Target: "alpha", Status: ProjectionAllowed},
		{Source: "alpha", Target: "unclassified:tmp", Status: ProjectionNew},
		{Source: "unclassified:tmp", Target: "alpha", Status: ProjectionNew},
	}}
	graph := AnalyzeCycles(projection)
	if graph.Nodes != 1 || graph.Edges != 0 || len(graph.Components) != 0 {
		t.Fatalf("graph = %+v, want one node without edges or components", graph)
	}
}

func TestCycleReportIsDeterministic(t *testing.T) {
	t.Parallel()
	policy, graph, baseline := loadCycleBaselineInputs(t)
	build := func() CycleReport {
		return CycleReport{SchemaVersion: CycleReportSchemaVersion, Graphs: []CycleGraph{
			AnalyzeCycles(TargetProjection(loadCyclePolicy(t, "shared-node.json"))),
			AnalyzeCycles(MigrationProjection(policy, graph, baseline)),
		}}
	}
	first, err := MarshalCycleReport(build())
	if err != nil {
		t.Fatalf("marshal cycle report: %v", err)
	}
	for range 20 {
		next, err := MarshalCycleReport(build())
		if err != nil {
			t.Fatalf("marshal cycle report: %v", err)
		}
		if !bytes.Equal(first, next) {
			t.Fatalf("cycle report is not byte-stable:\n%s\n%s", first, next)
		}
	}
	text := FormatCycleReport(build())
	for _, want := range []string{
		"target: 4 nodes, 5 edges, 1 cyclic components\n",
		"    members: alpha, beta, gamma\n",
		"    alpha (domain) -> beta (platform) [target] rules: alpha-to-beta\n",
		"migration: 2 nodes, 2 edges, 1 cyclic components\n",
		"    after ratchet: dissolves\n",
		"    beta (platform) -> alpha (domain) [baseline] violations: ",
	} {
		if !strings.Contains(text, want) {
			t.Errorf("text report does not contain %q:\n%s", want, text)
		}
	}
}

func TestCyclesCommandReportsBothGraphs(t *testing.T) {
	t.Parallel()
	project := fixturePath(t, "cycles", "baseline")
	output, err := runArchitecture(t, "cycles", "--json", "--project", project, "--policy", "policy.json", "--baseline", "legacy.jsonl")
	if err != nil {
		t.Fatalf("cycles failed: %v\n%s", err, output)
	}
	var report CycleReport
	if err := json.Unmarshal([]byte(output), &report); err != nil {
		t.Fatalf("decode cycle report: %v\n%s", err, output)
	}
	if report.SchemaVersion != CycleReportSchemaVersion || len(report.Graphs) != 2 {
		t.Fatalf("report = %+v, want schema %d with two graphs", report, CycleReportSchemaVersion)
	}
	if target, migration := report.Graphs[0], report.Graphs[1]; target.Kind != "target" || len(target.Components) != 0 || migration.Kind != "migration" || len(migration.Components) != 1 {
		t.Fatalf("graphs = %+v, want an acyclic target and one migration component", report.Graphs)
	}

	output, err = runArchitecture(t, "cycles", "--graph", "target", "--project", project, "--policy", "policy.json")
	if err != nil || output != "target: 2 nodes, 1 edges, 0 cyclic components\n" {
		t.Fatalf("target text report = %q, %v", output, err)
	}
	if output, err := runArchitecture(t, "cycles", "--graph", "package", "--project", project, "--policy", "policy.json"); err == nil || !strings.Contains(output, "--graph must be target, migration, or both") {
		t.Fatalf("cycles accepted an unknown graph: %q, %v", output, err)
	}
}

func loadCyclePolicy(t *testing.T, fixture string) *Policy {
	t.Helper()
	policy, err := LoadPolicy(fixturePath(t, "cycles", fixture))
	if err != nil {
		t.Fatalf("load cycle policy %s: %v", fixture, err)
	}
	return policy
}

func loadCycleBaselineInputs(t *testing.T) (*Policy, *Graph, *LegacyManifest) {
	t.Helper()
	policy := loadCyclePolicy(t, "baseline/policy.json")
	graph, err := LoadGraph(fixturePath(t, "cycles", "baseline"), policy)
	if err != nil {
		t.Fatalf("load cycle graph: %v", err)
	}
	baseline, err := LoadLegacyManifest(fixturePath(t, "cycles", "baseline", "legacy.jsonl"))
	if err != nil {
		t.Fatalf("load cycle baseline: %v", err)
	}
	return policy, graph, baseline
}

func cycleMembers(graph CycleGraph) [][]string {
	result := make([][]string, 0, len(graph.Components))
	for _, component := range graph.Components {
		result = append(result, component.Members)
	}
	return result
}

func cycleEdgeSummaries(component CycleComponent) []string {
	result := make([]string, 0, len(component.Edges))
	for _, edge := range component.Edges {
		result = append(result, edge.Source+"->"+edge.Target)
	}
	return result
}
