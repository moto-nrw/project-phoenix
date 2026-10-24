package architecture

import (
	"fmt"
	"strings"
)

// acyclicTargetEpoch is the policy epoch from which the target graph has no
// cyclic component (ADR 0058 point 1, #3417). A rule that closes a cycle is
// not target: the dependency it allowed is an exact legacy.jsonl key with an
// open carrier. Older epochs stay loadable as immutable PR bases.
const acyclicTargetEpoch = 48

// validateAcyclicTarget rejects a policy whose target projection holds a
// cyclic component and names the rules that carry its edges.
func (p *Policy) validateAcyclicTarget() error {
	graph := AnalyzeCycles(TargetProjection(p))
	if len(graph.Components) == 0 {
		return nil
	}
	parts := make([]string, 0, len(graph.Components))
	for _, component := range graph.Components {
		edges := make([]string, 0, len(component.Edges))
		for _, edge := range component.Edges {
			edges = append(edges, fmt.Sprintf("%s -> %s (%s)", edge.Source, edge.Target, strings.Join(edge.Rules, ", ")))
		}
		parts = append(parts, fmt.Sprintf("{%s} through %s", strings.Join(component.Members, ", "), strings.Join(edges, "; ")))
	}
	return fmt.Errorf("the target graph has %d cyclic component(s): %s: owner dependencies form a DAG, convert a rule that closes a cycle into exact legacy.jsonl keys (ADR 0058)",
		len(graph.Components), strings.Join(parts, " | "))
}
