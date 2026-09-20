package architecture

import (
	"encoding/json"
	"flag"
	"fmt"
	"path/filepath"
	"sort"
	"strings"
)

// CycleReportSchemaVersion identifies the machine-readable cycle report layout.
const CycleReportSchemaVersion = 1

// CycleProvenance says why an owner edge exists in the analysed graph.
type CycleProvenance string

const (
	// CycleTarget marks an owner edge that at least one target rule permits.
	CycleTarget CycleProvenance = "target"
	// CycleBaseline marks an owner edge that exists only through exact legacy baseline entries.
	CycleBaseline CycleProvenance = "baseline"
	// CycleNew marks an owner edge that exists only through violations absent from the baseline.
	CycleNew CycleProvenance = "new"
)

// CycleReport lists the cyclic components of one or more owner graphs.
type CycleReport struct {
	SchemaVersion int          `json:"schema_version"`
	Graphs        []CycleGraph `json:"graphs"`
}

// CycleGraph summarises one owner graph. Edges counts distinct owner pairs without self-edges.
type CycleGraph struct {
	Kind       string           `json:"kind"`
	Nodes      int              `json:"nodes"`
	Edges      int              `json:"edges"`
	Components []CycleComponent `json:"components"`
}

// CycleComponent is a strongly connected component with at least two owners.
// AfterRatchet is set for the migration graph only. It lists the cyclic
// components that remain inside it once only target-permitted edges are kept,
// and is empty when the ratchet dissolves it.
type CycleComponent struct {
	Members      []string    `json:"members"`
	Edges        []CycleEdge `json:"edges"`
	AfterRatchet [][]string  `json:"after_ratchet,omitzero"`
}

// CycleEdge is one owner pair inside a cyclic component.
type CycleEdge struct {
	Source        string          `json:"source"`
	SourceKind    string          `json:"source_kind"`
	Target        string          `json:"target"`
	TargetKind    string          `json:"target_kind"`
	Provenance    CycleProvenance `json:"provenance"`
	Rules         []string        `json:"rules"`
	ViolationKeys []string        `json:"violation_keys"`
}

// AnalyzeCycles reports the cyclic components of an owner projection.
// Self-edges and external or unclassified nodes never take part.
func AnalyzeCycles(projection Projection) CycleGraph {
	kinds := make(map[string]string, len(projection.Nodes))
	for _, node := range projection.Nodes {
		if node.Kind != "external" && node.Kind != "unclassified" {
			kinds[node.ID] = node.Kind
		}
	}
	edges := cycleEdges(projection.Edges, kinds)
	result := CycleGraph{Kind: projection.Kind, Nodes: len(kinds), Edges: len(edges), Components: []CycleComponent{}}
	for _, members := range cyclicComponents(kinds, edges, false) {
		inside := make(map[string]string, len(members))
		for _, member := range members {
			inside[member] = kinds[member]
		}
		component := CycleComponent{Members: members, Edges: []CycleEdge{}}
		for _, edge := range edges {
			if _, source := inside[edge.Source]; !source {
				continue
			}
			if _, target := inside[edge.Target]; target {
				component.Edges = append(component.Edges, edge)
			}
		}
		if projection.Kind == "migration" {
			component.AfterRatchet = cyclicComponents(inside, component.Edges, true)
		}
		result.Components = append(result.Components, component)
	}
	return result
}

func cycleEdges(projectionEdges []ProjectionEdge, kinds map[string]string) []CycleEdge {
	type builder struct {
		edge       CycleEdge
		rules      map[string]struct{}
		violations map[string]struct{}
	}
	builders := make(map[string]*builder)
	for _, edge := range projectionEdges {
		_, source := kinds[edge.Source]
		_, target := kinds[edge.Target]
		if !source || !target || edge.Source == edge.Target {
			continue
		}
		key := edge.Source + "\x00" + edge.Target
		current := builders[key]
		if current == nil {
			current = &builder{
				edge:  CycleEdge{Source: edge.Source, SourceKind: kinds[edge.Source], Target: edge.Target, TargetKind: kinds[edge.Target], Provenance: CycleBaseline},
				rules: make(map[string]struct{}), violations: make(map[string]struct{}),
			}
			builders[key] = current
		}
		switch {
		case edge.Status == ProjectionAllowed:
			current.edge.Provenance = CycleTarget
		case edge.Status == ProjectionNew && current.edge.Provenance == CycleBaseline:
			current.edge.Provenance = CycleNew
		}
		for _, rule := range edge.Rules {
			current.rules[rule] = struct{}{}
		}
		for _, violation := range edge.ViolationKeys {
			current.violations[violation] = struct{}{}
		}
	}
	edges := make([]CycleEdge, 0, len(builders))
	for _, current := range builders {
		current.edge.Rules = sortedKeys(current.rules)
		current.edge.ViolationKeys = sortedKeys(current.violations)
		edges = append(edges, current.edge)
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		return edges[i].Target < edges[j].Target
	})
	return edges
}

func sortedKeys(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

// cyclicComponents runs Tarjan's algorithm and returns every component with at
// least two members, largest first, then by smallest member.
func cyclicComponents(nodes map[string]string, edges []CycleEdge, targetOnly bool) [][]string {
	successors := make(map[string][]string, len(nodes))
	for _, edge := range edges {
		if !targetOnly || edge.Provenance == CycleTarget {
			successors[edge.Source] = append(successors[edge.Source], edge.Target)
		}
	}
	ids := make([]string, 0, len(nodes))
	for id := range nodes {
		ids = append(ids, id)
	}
	sort.Strings(ids)

	index := make(map[string]int, len(ids))
	low := make(map[string]int, len(ids))
	onStack := make(map[string]bool, len(ids))
	var stack []string
	components := [][]string{}
	var visit func(string)
	visit = func(node string) {
		index[node], low[node] = len(index), len(index)
		stack = append(stack, node)
		onStack[node] = true
		for _, next := range successors[node] {
			if _, seen := index[next]; !seen {
				visit(next)
				low[node] = min(low[node], low[next])
			} else if onStack[next] {
				low[node] = min(low[node], index[next])
			}
		}
		if low[node] != index[node] {
			return
		}
		var members []string
		for {
			last := stack[len(stack)-1]
			stack = stack[:len(stack)-1]
			onStack[last] = false
			members = append(members, last)
			if last == node {
				break
			}
		}
		if len(members) > 1 {
			sort.Strings(members)
			components = append(components, members)
		}
	}
	for _, id := range ids {
		if _, seen := index[id]; !seen {
			visit(id)
		}
	}
	sort.Slice(components, func(i, j int) bool {
		if len(components[i]) != len(components[j]) {
			return len(components[i]) > len(components[j])
		}
		return components[i][0] < components[j][0]
	})
	return components
}

// MarshalCycleReport returns stable, indented JSON terminated by a newline.
func MarshalCycleReport(report CycleReport) ([]byte, error) {
	result, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal architecture cycle report: %w", err)
	}
	return append(result, '\n'), nil
}

// FormatCycleReport renders the report as deterministic text.
func FormatCycleReport(report CycleReport) string {
	var out strings.Builder
	for position, graph := range report.Graphs {
		if position > 0 {
			out.WriteByte('\n')
		}
		fmt.Fprintf(&out, "%s: %d nodes, %d edges, %d cyclic components\n", graph.Kind, graph.Nodes, graph.Edges, len(graph.Components))
		for number, component := range graph.Components {
			fmt.Fprintf(&out, "  component %d: %d members, %d edges\n", number+1, len(component.Members), len(component.Edges))
			fmt.Fprintf(&out, "    members: %s\n", strings.Join(component.Members, ", "))
			if graph.Kind == "migration" {
				out.WriteString("    after ratchet: " + formatAfterRatchet(component.AfterRatchet) + "\n")
			}
			for _, edge := range component.Edges {
				fmt.Fprintf(&out, "    %s (%s) -> %s (%s) [%s] %s\n", edge.Source, edge.SourceKind, edge.Target, edge.TargetKind, edge.Provenance, formatCycleEdgeEvidence(edge))
			}
		}
	}
	return out.String()
}

func formatAfterRatchet(components [][]string) string {
	if len(components) == 0 {
		return "dissolves"
	}
	parts := make([]string, 0, len(components))
	for _, members := range components {
		parts = append(parts, fmt.Sprintf("%d members (%s)", len(members), strings.Join(members, ", ")))
	}
	return strings.Join(parts, "; ")
}

func formatCycleEdgeEvidence(edge CycleEdge) string {
	if edge.Provenance == CycleTarget {
		return "rules: " + strings.Join(edge.Rules, ", ")
	}
	return "violations: " + strings.Join(edge.ViolationKeys, ", ")
}

func runCycles(args []string) error {
	flags := flag.NewFlagSet("cycles", flag.ContinueOnError)
	project := flags.String("project", ".", "Go project directory")
	policyPath := flags.String("policy", filepath.Join("architecture", "policy.json"), "architecture policy")
	baselinePath := flags.String("baseline", "", "exact legacy JSONL baseline")
	graphKind := flags.String("graph", "both", "graph to analyse: target, migration, or both")
	asJSON := flags.Bool("json", false, "print the report as JSON")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if flags.NArg() != 0 {
		return fmt.Errorf("cycles: unexpected arguments: %v", flags.Args())
	}
	if *graphKind != "target" && *graphKind != "migration" && *graphKind != "both" {
		return fmt.Errorf("cycles: --graph must be target, migration, or both, got %q", *graphKind)
	}
	absoluteProject, err := filepath.Abs(*project)
	if err != nil {
		return fmt.Errorf("resolve project path: %w", err)
	}
	options := projectionOptions{project: absoluteProject, policy: projectPath(absoluteProject, *policyPath), baseline: projectPath(absoluteProject, *baselinePath)}

	report := CycleReport{SchemaVersion: CycleReportSchemaVersion}
	if *graphKind == "target" {
		policy, err := LoadPolicy(options.policy)
		if err != nil {
			return err
		}
		report.Graphs = append(report.Graphs, AnalyzeCycles(TargetProjection(policy)))
	} else {
		inputs, err := loadProjectionCommandInputs(options)
		if err != nil {
			return err
		}
		if *graphKind == "both" {
			report.Graphs = append(report.Graphs, AnalyzeCycles(TargetProjection(inputs.policy)))
		}
		report.Graphs = append(report.Graphs, AnalyzeCycles(MigrationProjection(inputs.policy, inputs.graph, inputs.baseline)))
	}
	if !*asJSON {
		fmt.Print(FormatCycleReport(report))
		return nil
	}
	encoded, err := MarshalCycleReport(report)
	if err != nil {
		return err
	}
	fmt.Print(string(encoded))
	return nil
}
