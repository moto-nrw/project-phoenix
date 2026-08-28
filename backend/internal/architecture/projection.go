package architecture

import (
	"encoding/json"
	"fmt"
	"sort"
	"strings"
)

// ProjectionStatus describes how a dependency relates to the current policy and baseline.
type ProjectionStatus string

const (
	// ProjectionAllowed marks a dependency permitted by the target policy.
	ProjectionAllowed ProjectionStatus = "allowed"
	// ProjectionLegacy marks a violation recorded in the exact legacy baseline.
	ProjectionLegacy ProjectionStatus = "legacy"
	// ProjectionNew marks a current violation absent from the exact legacy baseline.
	ProjectionNew ProjectionStatus = "new"
)

// Projection is the deterministic machine-readable source for an architecture diagram.
type Projection struct {
	SchemaVersion int                   `json:"schema_version"`
	Kind          string                `json:"kind"`
	Build         Build                 `json:"build"`
	Focus         string                `json:"focus,omitempty"`
	Nodes         []ProjectionNode      `json:"nodes"`
	Edges         []ProjectionEdge      `json:"edges"`
	Violations    []ProjectionViolation `json:"violations"`
}

// ProjectionNode is a rendered module, package, or external capability.
type ProjectionNode struct {
	ID       string   `json:"id"`
	Kind     string   `json:"kind"`
	Packages []string `json:"packages"`
}

// ProjectionEdge groups dependencies with the same endpoints and status.
type ProjectionEdge struct {
	Source        string           `json:"source"`
	Target        string           `json:"target"`
	Status        ProjectionStatus `json:"status"`
	Imports       int              `json:"imports"`
	Rules         []string         `json:"rules"`
	ViolationKeys []string         `json:"violation_keys"`
}

// ProjectionViolation preserves an evaluator violation and its ownership metadata.
type ProjectionViolation struct {
	Key           string           `json:"key"`
	Rule          string           `json:"rule"`
	Source        string           `json:"source"`
	Target        string           `json:"target"`
	SourceOwner   string           `json:"source_owner"`
	TargetOwner   string           `json:"target_owner"`
	SourcePackage string           `json:"source_package,omitempty"`
	TargetPackage string           `json:"target_package,omitempty"`
	Status        ProjectionStatus `json:"status"`
}

// TargetProjection builds the policy-only target module graph.
func TargetProjection(policy *Policy) Projection {
	projection := newOwnerProjection("target", policy, true, targetOwnerKind)
	allowedNodes := projectionNodeSet(projection.Nodes)
	for _, edge := range allowedOwnerEdges(policy) {
		if _, source := allowedNodes[edge.Source]; !source {
			continue
		}
		if _, target := allowedNodes[edge.Target]; target {
			projection.Edges = append(projection.Edges, edge)
		}
	}
	return projection
}

func allowedOwnerEdges(policy *Policy) []ProjectionEdge {
	edges := make(map[string]*projectionEdgeBuilder)
	for _, rule := range policy.Rules {
		if rule.TargetClass != "" || !ruleHasScope(rule, ScopeProduction) {
			continue
		}
		for _, source := range matchingOwners(policy, rule.SourceOwner, rule.SourceOwnerKind) {
			for _, target := range targetOwners(policy, rule, source) {
				projectionEdge(edges, source, target, ProjectionAllowed).rules[rule.ID] = struct{}{}
			}
		}
	}
	return finishEdges(edges)
}

// MigrationProjection condenses the current graph and all production violations by owner.
func MigrationProjection(policy *Policy, graph *Graph, baseline *LegacyManifest) Projection {
	projection := newOwnerProjection("migration", policy, false, nil)
	violations := Check(policy, graph)
	statuses := violationStatuses(violations, baseline)
	projection.Violations = projectViolations(policy, violations, statuses)
	edges := make(map[string]*projectionEdgeBuilder)
	addImportEdges(policy, graph, statuses, edges)
	addSemanticEdges(policy, graph.SemanticViolations, statuses, edges)
	addUnrepresentedViolationEdges(policy, violations, statuses, edges, ownerViolationEndpoint)
	addMissingOwnerNodes(policy, &projection, edges)
	projection.Edges = finishEdges(edges)
	return projection
}

// DependenciesProjection builds a one-hop current dependency graph around an exact focus.
func DependenciesProjection(policy *Policy, graph *Graph, baseline *LegacyManifest, rawFocus string) (Projection, error) {
	focus, err := resolveFocus(policy, graph, rawFocus)
	if err != nil {
		return Projection{}, err
	}
	if focus.kind == "module" {
		projection := MigrationProjection(policy, graph, baseline)
		projection.Kind, projection.Focus = "dependencies", rawFocus
		return filterProjection(projection, focus.value), nil
	}
	projection := packageProjection(policy, graph, baseline)
	projection.Kind, projection.Focus = "dependencies", rawFocus
	return filterProjection(projection, focus.value), nil
}

// MarshalProjection returns stable, indented JSON terminated by a newline.
func MarshalProjection(projection Projection) ([]byte, error) {
	result, err := json.MarshalIndent(projection, "", "  ")
	if err != nil {
		return nil, fmt.Errorf("marshal architecture projection: %w", err)
	}
	return append(result, '\n'), nil
}

func newOwnerProjection(kind string, policy *Policy, includeEmpty bool, includeOwner func(string) bool) Projection {
	packages := make(map[string][]string)
	kinds := make(map[string]string, len(policy.Owners))
	for _, owner := range policy.Owners {
		if includeOwner != nil && !includeOwner(owner.Kind) {
			continue
		}
		kinds[owner.ID] = owner.Kind
		if includeEmpty {
			packages[owner.ID] = []string{}
		}
	}
	for _, pkg := range policy.Packages {
		if _, included := kinds[pkg.Owner]; includeOwner != nil && !included {
			continue
		}
		packages[pkg.Owner] = append(packages[pkg.Owner], pkg.Path)
	}
	nodes := make([]ProjectionNode, 0, len(packages))
	for owner, paths := range packages {
		sort.Strings(paths)
		nodes = append(nodes, ProjectionNode{ID: owner, Kind: kinds[owner], Packages: paths})
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return Projection{SchemaVersion: 1, Kind: kind, Build: policy.Build, Nodes: nodes, Edges: []ProjectionEdge{}, Violations: []ProjectionViolation{}}
}

func targetOwnerKind(kind string) bool {
	return kind == "domain" || kind == "platform" || kind == "workflow" || kind == "projection" || kind == "kernel"
}

func projectionNodeSet(nodes []ProjectionNode) map[string]struct{} {
	result := make(map[string]struct{}, len(nodes))
	for _, node := range nodes {
		result[node.ID] = struct{}{}
	}
	return result
}

func matchingOwners(policy *Policy, id, kind string) []string {
	var result []string
	for _, owner := range policy.Owners {
		if ownerMatches(id, kind, owner) {
			result = append(result, owner.ID)
		}
	}
	sort.Strings(result)
	return result
}

func targetOwners(policy *Policy, rule Rule, source string) []string {
	if !rule.SameOwner {
		return matchingOwners(policy, rule.TargetOwner, rule.TargetOwnerKind)
	}
	for _, owner := range policy.Owners {
		if owner.ID == source && ownerMatches(rule.TargetOwner, rule.TargetOwnerKind, owner) {
			return []string{source}
		}
	}
	return nil
}

func addMissingOwnerNodes(policy *Policy, projection *Projection, edges map[string]*projectionEdgeBuilder) {
	existing := make(map[string]struct{}, len(projection.Nodes))
	for _, node := range projection.Nodes {
		existing[node.ID] = struct{}{}
	}
	for _, edge := range edges {
		for _, owner := range []string{edge.source, edge.target} {
			if _, ok := existing[owner]; ok {
				continue
			}
			projection.Nodes = append(projection.Nodes, projectionOwnerNode(policy, owner))
			existing[owner] = struct{}{}
		}
	}
	sort.Slice(projection.Nodes, func(i, j int) bool { return projection.Nodes[i].ID < projection.Nodes[j].ID })
}

func projectionOwnerNode(policy *Policy, ownerID string) ProjectionNode {
	node := ProjectionNode{ID: ownerID, Packages: []string{}}
	if strings.HasPrefix(ownerID, "external:") {
		node.Kind = "external"
		return node
	}
	if strings.HasPrefix(ownerID, "unclassified:") {
		node.Kind = "unclassified"
		return node
	}
	for _, owner := range policy.Owners {
		if owner.ID == ownerID {
			node.Kind = owner.Kind
			break
		}
	}
	for _, pkg := range policy.Packages {
		if pkg.Owner == ownerID {
			node.Packages = append(node.Packages, pkg.Path)
		}
	}
	sort.Strings(node.Packages)
	return node
}

type projectionEdgeBuilder struct {
	source, target string
	status         ProjectionStatus
	imports        int
	rules          map[string]struct{}
	violations     map[string]struct{}
}

func projectionEdge(edges map[string]*projectionEdgeBuilder, source, target string, status ProjectionStatus) *projectionEdgeBuilder {
	key := source + "\x00" + target + "\x00" + string(status)
	if edge := edges[key]; edge != nil {
		return edge
	}
	edge := &projectionEdgeBuilder{source: source, target: target, status: status, rules: make(map[string]struct{}), violations: make(map[string]struct{})}
	edges[key] = edge
	return edge
}

func finishEdges(builders map[string]*projectionEdgeBuilder) []ProjectionEdge {
	edges := make([]ProjectionEdge, 0, len(builders))
	for _, builder := range builders {
		edges = append(edges, ProjectionEdge{
			Source: builder.source, Target: builder.target, Status: builder.status, Imports: builder.imports,
			Rules: sortedSet(builder.rules), ViolationKeys: sortedSet(builder.violations),
		})
	}
	sort.Slice(edges, func(i, j int) bool {
		if edges[i].Source != edges[j].Source {
			return edges[i].Source < edges[j].Source
		}
		if edges[i].Target != edges[j].Target {
			return edges[i].Target < edges[j].Target
		}
		return projectionStatusRank(edges[i].Status) < projectionStatusRank(edges[j].Status)
	})
	return edges
}

func sortedSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func violationStatuses(violations []Violation, baseline *LegacyManifest) map[string]ProjectionStatus {
	legacy := make(map[string]struct{})
	if baseline != nil {
		for _, entry := range baseline.Entries {
			legacy[entry.Key()] = struct{}{}
		}
	}
	statuses := make(map[string]ProjectionStatus, len(violations))
	for _, violation := range violations {
		statuses[violation.Key()] = ProjectionNew
		if _, exists := legacy[violation.Key()]; exists {
			statuses[violation.Key()] = ProjectionLegacy
		}
	}
	return statuses
}

func addImportEdges(policy *Policy, graph *Graph, statuses map[string]ProjectionStatus, edges map[string]*projectionEdgeBuilder) {
	packages := policy.packageMap()
	external := policy.externalPackageMap()
	for _, edge := range graph.Edges {
		if edge.Scope != ScopeProduction {
			continue
		}
		source, sourceOK := packages[edge.Source]
		if !sourceOK {
			continue
		}
		targetID := ""
		var rules []*Rule
		if target, ok := packages[edge.Target]; ok {
			targetID = target.Owner
			rules = policy.firstPartyRules(edge.Scope, source, target)
		} else if target, ok := external[edge.Target]; ok {
			targetID = "external:" + target.Class
			rules = policy.externalRules(edge.Scope, source, target.Class)
		} else if !isOwnPackage(policy.ModulePath, edge.Target) {
			targetID = "external:" + edge.Target
		}
		if targetID == "" {
			continue
		}
		status, key := importEdgeStatus(statuses, edge)
		builder := projectionEdge(edges, source.Owner, targetID, status)
		builder.imports++
		if key != "" {
			builder.violations[key] = struct{}{}
			continue
		}
		for _, rule := range rules {
			builder.rules[rule.ID] = struct{}{}
		}
	}
}

func importEdgeStatus(statuses map[string]ProjectionStatus, edge Edge) (ProjectionStatus, string) {
	for _, rule := range []string{"imports.forbidden", "external.unclassified", "policy.rules-overlap"} {
		key := (Violation{Scope: edge.Scope, Rule: rule, Source: edge.Source, Target: edge.Target}).Key()
		if status, exists := statuses[key]; exists {
			return status, key
		}
	}
	return ProjectionAllowed, ""
}

func addSemanticEdges(policy *Policy, violations []Violation, statuses map[string]ProjectionStatus, edges map[string]*projectionEdgeBuilder) {
	for _, violation := range violations {
		if violation.Scope != ScopeProduction {
			continue
		}
		sourceOwner := policy.ownerForPackage(violation.Source)
		if sourceOwner == "" {
			continue
		}
		targetOwner := policy.ownerForViolationTarget(violation.Target, sourceOwner)
		builder := projectionEdge(edges, sourceOwner, targetOwner, statuses[violation.Key()])
		builder.rules[violation.Rule] = struct{}{}
		builder.violations[violation.Key()] = struct{}{}
	}
}

func projectionStatusRank(status ProjectionStatus) int {
	switch status {
	case ProjectionNew:
		return 2
	case ProjectionLegacy:
		return 1
	default:
		return 0
	}
}

type violationEndpoint func(*Policy, Violation) (string, string)

func addUnrepresentedViolationEdges(policy *Policy, violations []Violation, statuses map[string]ProjectionStatus, edges map[string]*projectionEdgeBuilder, endpoints violationEndpoint) {
	represented := make(map[string]struct{})
	for _, edge := range edges {
		for key := range edge.violations {
			represented[key] = struct{}{}
		}
	}
	for _, violation := range violations {
		if violation.Scope != ScopeProduction {
			continue
		}
		if _, exists := represented[violation.Key()]; exists {
			continue
		}
		source, target := endpoints(policy, violation)
		builder := projectionEdge(edges, source, target, statuses[violation.Key()])
		builder.rules[violation.Rule] = struct{}{}
		builder.violations[violation.Key()] = struct{}{}
	}
}

func ownerViolationEndpoint(policy *Policy, violation Violation) (string, string) {
	source := policy.ownerForPackage(violation.Source)
	if source == "" {
		source = policy.classificationEndpoint(violation.Source)
	}
	target := policy.ownerForViolationTarget(violation.Target, source)
	if isClassificationViolation(violation.Rule) || strings.HasPrefix(violation.Rule, "external.") || strings.HasPrefix(violation.Rule, "imports.") || violation.Rule == "policy.rules-overlap" {
		target = policy.classificationEndpoint(violation.Target)
	}
	if target == "" {
		target = source
	}
	return source, target
}

func (p *Policy) classificationEndpoint(path string) string {
	if owner := p.ownerForPackage(path); owner != "" {
		return owner
	}
	if external, ok := p.externalPackageMap()[path]; ok {
		return "external:" + external.Class
	}
	if isOwnPackage(p.ModulePath, path) {
		return "unclassified:" + strings.TrimPrefix(strings.TrimPrefix(path, p.ModulePath), "/")
	}
	return "external:" + path
}

func isClassificationViolation(rule string) bool {
	return strings.HasPrefix(rule, "packages.")
}

func projectViolations(policy *Policy, violations []Violation, statuses map[string]ProjectionStatus) []ProjectionViolation {
	result := make([]ProjectionViolation, 0, len(violations))
	for _, violation := range violations {
		sourceOwner, targetOwner := ownerViolationEndpoint(policy, violation)
		result = append(result, ProjectionViolation{
			Key: violation.Key(), Rule: violation.Rule, Source: violation.Source, Target: violation.Target,
			SourceOwner: sourceOwner, TargetOwner: targetOwner,
			SourcePackage: policy.relativePackageForTarget(violation.Source), TargetPackage: policy.relativePackageForTarget(violation.Target),
			Status: statuses[violation.Key()],
		})
	}
	return result
}

func (p *Policy) ownerForPackage(path string) string {
	if pkg, ok := p.packageMap()[path]; ok {
		return pkg.Owner
	}
	return ""
}

func (p *Policy) ownerForViolationTarget(target, fallback string) string {
	for _, object := range p.DataObjects {
		if object.Name == target {
			return object.WriteOwner
		}
	}
	packages := p.packageMap()
	for path, pkg := range packages {
		if target == path || strings.HasPrefix(target, path+".") {
			return pkg.Owner
		}
	}
	return fallback
}

func (p *Policy) relativePackageForTarget(target string) string {
	for path, pkg := range p.packageMap() {
		if target == path || strings.HasPrefix(target, path+".") {
			return pkg.Path
		}
	}
	return ""
}

type resolvedFocus struct{ kind, value string }

func resolveFocus(policy *Policy, graph *Graph, raw string) (resolvedFocus, error) {
	if value, ok := strings.CutPrefix(raw, "module:"); ok {
		return resolveModuleFocus(policy, graph, raw, value)
	}
	if value, ok := strings.CutPrefix(raw, "package:"); ok {
		return resolvePackageFocus(policy, graph, raw, value)
	}
	module := policy.hasCurrentOwner(graph, raw)
	path, pkg := policy.resolvePackage(raw)
	pkg = pkg && graph.hasPackage(policy.absolutePackage(path))
	if module && pkg {
		return resolvedFocus{}, fmt.Errorf("ambiguous focus %q matches both a module and a package; use module:%s or package:%s", raw, raw, raw)
	}
	if module {
		return resolvedFocus{kind: "module", value: raw}, nil
	}
	if pkg {
		return resolvedFocus{kind: "package", value: path}, nil
	}
	return resolvedFocus{}, fmt.Errorf("unknown focus %q: expected an exact module owner or package path", raw)
}

func resolveModuleFocus(policy *Policy, graph *Graph, raw, value string) (resolvedFocus, error) {
	if value == "" || !policy.hasOwner(value) {
		return resolvedFocus{}, fmt.Errorf("unknown focus %q: module %q is not declared by the policy", raw, value)
	}
	if !policy.hasCurrentOwner(graph, value) {
		return resolvedFocus{}, fmt.Errorf("unknown focus %q: module %q has no classified package in the current graph", raw, value)
	}
	return resolvedFocus{kind: "module", value: value}, nil
}

func resolvePackageFocus(policy *Policy, graph *Graph, raw, value string) (resolvedFocus, error) {
	path, ok := policy.resolvePackage(value)
	if value == "" || !ok {
		return resolvedFocus{}, fmt.Errorf("unknown focus %q: package %q is not classified by the policy", raw, value)
	}
	if !graph.hasPackage(policy.absolutePackage(path)) {
		return resolvedFocus{}, fmt.Errorf("unknown focus %q: package %q is not present in the current graph", raw, value)
	}
	return resolvedFocus{kind: "package", value: path}, nil
}

func (p *Policy) hasCurrentOwner(graph *Graph, id string) bool {
	for _, pkg := range p.Packages {
		if pkg.Owner == id && graph.hasPackage(p.absolutePackage(pkg.Path)) {
			return true
		}
	}
	return false
}

func (g *Graph) hasPackage(path string) bool {
	for _, current := range g.Packages {
		if current == path {
			return true
		}
	}
	return false
}

func (p *Policy) hasOwner(id string) bool {
	for _, owner := range p.Owners {
		if owner.ID == id {
			return true
		}
	}
	return false
}

func (p *Policy) resolvePackage(value string) (string, bool) {
	for _, pkg := range p.Packages {
		if value == pkg.Path || value == p.absolutePackage(pkg.Path) {
			return pkg.Path, true
		}
	}
	return "", false
}

func filterProjection(projection Projection, focus string) Projection {
	keep := map[string]struct{}{focus: {}}
	incident := make([]ProjectionEdge, 0, len(projection.Edges))
	for _, edge := range projection.Edges {
		if edge.Source == focus || edge.Target == focus {
			keep[edge.Source], keep[edge.Target] = struct{}{}, struct{}{}
			incident = append(incident, edge)
		}
	}
	projection.Nodes = filterNodes(projection.Nodes, keep)
	projection.Edges = incident
	projection.Violations = incidentViolations(projection.Violations, focus, projection.Nodes)
	return projection
}

func filterNodes(nodes []ProjectionNode, keep map[string]struct{}) []ProjectionNode {
	result := make([]ProjectionNode, 0, len(keep))
	for _, node := range nodes {
		if _, ok := keep[node.ID]; ok {
			result = append(result, node)
		}
	}
	return result
}

func incidentViolations(violations []ProjectionViolation, focus string, nodes []ProjectionNode) []ProjectionViolation {
	packageFocus := false
	for _, node := range nodes {
		if node.ID == focus {
			packageFocus = node.Kind == "package"
			break
		}
	}
	result := make([]ProjectionViolation, 0, len(violations))
	for _, violation := range violations {
		ownerMatch := violation.SourceOwner == focus || violation.TargetOwner == focus
		packageMatch := violation.SourcePackage == focus || violation.TargetPackage == focus
		if packageFocus && packageMatch || !packageFocus && ownerMatch {
			result = append(result, violation)
		}
	}
	return result
}

func packageProjection(policy *Policy, graph *Graph, baseline *LegacyManifest) Projection {
	violations := Check(policy, graph)
	statuses := violationStatuses(violations, baseline)
	projection := Projection{SchemaVersion: 1, Kind: "dependencies", Build: policy.Build, Nodes: packageNodes(policy, graph), Violations: projectViolations(policy, violations, statuses)}
	edges := make(map[string]*projectionEdgeBuilder)
	packages := policy.packageMap()
	external := policy.externalPackageMap()
	for _, edge := range graph.Edges {
		if edge.Scope != ScopeProduction {
			continue
		}
		source, sourceOK := packages[edge.Source]
		if !sourceOK {
			continue
		}
		targetID := ""
		if target, ok := packages[edge.Target]; ok {
			targetID = target.Path
		} else if target, ok := external[edge.Target]; ok {
			targetID = "external:" + target.Class
		} else if !isOwnPackage(policy.ModulePath, edge.Target) {
			targetID = "external:" + edge.Target
		}
		if targetID == "" {
			continue
		}
		status, key := importEdgeStatus(statuses, edge)
		builder := projectionEdge(edges, source.Path, targetID, status)
		builder.imports++
		if key != "" {
			builder.violations[key] = struct{}{}
		}
	}
	addPackageSemanticEdges(policy, &projection, graph.SemanticViolations, statuses, edges)
	addUnrepresentedViolationEdges(policy, violations, statuses, edges, packageViolationEndpoint)
	addMissingPackageNodes(&projection, edges)
	projection.Edges = finishEdges(edges)
	return projection
}

func addPackageSemanticEdges(policy *Policy, projection *Projection, violations []Violation, statuses map[string]ProjectionStatus, edges map[string]*projectionEdgeBuilder) {
	for _, violation := range violations {
		source := policy.relativePackageForTarget(violation.Source)
		if violation.Scope != ScopeProduction || source == "" {
			continue
		}
		targetOwner := policy.ownerForViolationTarget(violation.Target, policy.ownerForPackage(violation.Source))
		target := policy.relativePackageForTarget(violation.Target)
		if target == "" {
			target = "owner:" + targetOwner
			addOwnerNode(policy, projection, targetOwner)
		}
		builder := projectionEdge(edges, source, target, statuses[violation.Key()])
		builder.rules[violation.Rule] = struct{}{}
		builder.violations[violation.Key()] = struct{}{}
	}
}

func packageViolationEndpoint(policy *Policy, violation Violation) (string, string) {
	source := policy.relativePackageForTarget(violation.Source)
	if source == "" {
		source = policy.classificationEndpoint(violation.Source)
	}
	target := policy.relativePackageForTarget(violation.Target)
	if target == "" {
		if strings.HasPrefix(violation.Rule, "tables.") || strings.HasPrefix(violation.Rule, "contracts.") || strings.HasPrefix(violation.Rule, "database.") || strings.HasPrefix(violation.Rule, "composition.") {
			target = "owner:" + policy.ownerForViolationTarget(violation.Target, policy.ownerForPackage(violation.Source))
		} else {
			target = policy.classificationEndpoint(violation.Target)
		}
	}
	return source, target
}

func addMissingPackageNodes(projection *Projection, edges map[string]*projectionEdgeBuilder) {
	existing := projectionNodeSet(projection.Nodes)
	for _, edge := range edges {
		for _, id := range []string{edge.source, edge.target} {
			if _, ok := existing[id]; ok {
				continue
			}
			kind := "external"
			if strings.HasPrefix(id, "unclassified:") {
				kind = "unclassified"
			} else if strings.HasPrefix(id, "owner:") {
				kind = "owner"
			}
			projection.Nodes = append(projection.Nodes, ProjectionNode{ID: id, Kind: kind, Packages: []string{}})
			existing[id] = struct{}{}
		}
	}
	sort.Slice(projection.Nodes, func(i, j int) bool { return projection.Nodes[i].ID < projection.Nodes[j].ID })
}

func addOwnerNode(policy *Policy, projection *Projection, owner string) {
	id := "owner:" + owner
	for _, node := range projection.Nodes {
		if node.ID == id {
			return
		}
	}
	var packages []string
	for _, pkg := range policy.Packages {
		if pkg.Owner == owner {
			packages = append(packages, policy.absolutePackage(pkg.Path))
		}
	}
	sort.Strings(packages)
	projection.Nodes = append(projection.Nodes, ProjectionNode{ID: id, Kind: "owner", Packages: packages})
	sort.Slice(projection.Nodes, func(i, j int) bool { return projection.Nodes[i].ID < projection.Nodes[j].ID })
}

func packageNodes(policy *Policy, graph *Graph) []ProjectionNode {
	packages := policy.packageMap()
	nodes := make([]ProjectionNode, 0, len(graph.Packages))
	for _, path := range graph.Packages {
		if pkg, ok := packages[path]; ok {
			nodes = append(nodes, ProjectionNode{ID: pkg.Path, Kind: "package", Packages: []string{path}})
		}
	}
	sort.Slice(nodes, func(i, j int) bool { return nodes[i].ID < nodes[j].ID })
	return nodes
}
