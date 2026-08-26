package architecture

import (
	"encoding/json"
	"fmt"
	"go/token"
	"io"
	"os"
	"regexp"
	"slices"
	"strings"
)

type Policy struct {
	SchemaVersion     int               `json:"schema_version"`
	PolicyEpoch       int               `json:"policy_epoch"`
	ModulePath        string            `json:"module_path"`
	Build             Build             `json:"build"`
	Owners            []Owner           `json:"owners"`
	Roles             []string          `json:"roles"`
	Packages          []Package         `json:"packages"`
	DataObjects       []DataObject      `json:"data_objects"`
	ReadProjections   []ReadProjection  `json:"read_projections"`
	LegacyComposition []LegacyReference `json:"legacy_composition"`
	SharedKernelTypes []string          `json:"shared_kernel_types"`
	ExternalClasses   []string          `json:"external_classes"`
	ExternalPackages  []ExternalPackage `json:"external_packages"`
	Rules             []Rule            `json:"rules"`
}

type DataObject struct {
	Name       string `json:"name"`
	WriteOwner string `json:"write_owner"`
}

type ReadProjection struct {
	ID          string   `json:"id"`
	Package     string   `json:"package"`
	DataObjects []string `json:"data_objects"`
	TenantSafe  bool     `json:"tenant_safe"`
}

type LegacyReference struct {
	Package string   `json:"package"`
	Symbols []string `json:"symbols"`
}

type Build struct {
	GOOS       string `json:"goos"`
	GOARCH     string `json:"goarch"`
	CGOEnabled *bool  `json:"cgo_enabled"`
}

type Owner struct {
	ID   string `json:"id"`
	Kind string `json:"kind"`
}

type Package struct {
	Path             string `json:"path"`
	Owner            string `json:"owner"`
	Role             string `json:"role"`
	InternalTestRole string `json:"internal_test_role"`
	ExternalTestRole string `json:"external_test_role"`
}

type ExternalPackage struct {
	Path  string `json:"path"`
	Class string `json:"class"`
}

type Rule struct {
	ID              string   `json:"id"`
	Description     string   `json:"description"`
	Scopes          []string `json:"scopes"`
	SourceOwner     string   `json:"source_owner"`
	SourceOwnerKind string   `json:"source_owner_kind"`
	SourceRole      string   `json:"source_role"`
	TargetOwner     string   `json:"target_owner"`
	TargetOwnerKind string   `json:"target_owner_kind"`
	TargetRole      string   `json:"target_role"`
	TargetClass     string   `json:"target_class"`
	SameOwner       bool     `json:"same_owner"`
}

var (
	identifierPattern = regexp.MustCompile(`^[a-z][a-z0-9]*(?:[._-][a-z0-9]+)*$`)
	allowedOwnerKinds = map[string]bool{
		"domain": true, "platform": true, "workflow": true, "projection": true,
		"inbound": true, "composition": true, "kernel": true, "migration": true, "test-support": true,
	}
	allowedRoles = map[string]bool{
		"public": true, "contract": true, "domain": true, "application": true,
		"port": true, "adapter": true, "postgres": true, "compose": true,
		"http": true, "worker": true, "cli": true, "migration": true, "test-support": true,
		"module-internal-test": true, "module-behavior-test": true,
		"workflow-decision-test": true, "workflow-integration-test": true,
		"adapter-test": true, "e2e-test": true,
	}
	allowedScopes = map[string]bool{
		string(ScopeProduction): true, string(ScopeInternalTest): true, string(ScopeExternalTest): true,
	}
)

func LoadPolicy(path string) (*Policy, error) {
	file, err := os.Open(path) // #nosec G304 -- path is an explicit CLI input
	if err != nil {
		return nil, fmt.Errorf("open policy: %w", err)
	}
	defer func() { _ = file.Close() }()

	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var policy Policy
	if err := decoder.Decode(&policy); err != nil {
		return nil, fmt.Errorf("decode policy: %w", err)
	}
	if err := requireJSONEnd(decoder); err != nil {
		return nil, err
	}
	if err := policy.Validate(); err != nil {
		return nil, fmt.Errorf("validate policy: %w", err)
	}
	return &policy, nil
}

func (p *Policy) Validate() error {
	if err := p.validateHeader(); err != nil {
		return err
	}
	owners, err := p.validateOwners()
	if err != nil {
		return err
	}
	roles, err := validateUniqueValues("role", p.Roles, allowedRoles)
	if err != nil {
		return err
	}
	classes, err := validateUniqueValues("external class", p.ExternalClasses, nil)
	if err != nil {
		return err
	}
	if err := p.validatePackages(owners, roles); err != nil {
		return err
	}
	dataObjects, err := p.validateDataObjects(owners)
	if err != nil {
		return err
	}
	if err := p.validateReadProjections(owners, dataObjects); err != nil {
		return err
	}
	if err := p.validateLegacyComposition(owners); err != nil {
		return err
	}
	if err := p.validateExternalPackages(classes); err != nil {
		return err
	}
	return p.validateRules(owners, roles, classes)
}

func (p *Policy) validateHeader() error {
	if p.SchemaVersion != 2 {
		return fmt.Errorf("schema_version must be 2, got %d", p.SchemaVersion)
	}
	if p.PolicyEpoch < 1 {
		return fmt.Errorf("policy_epoch must be a positive integer")
	}
	if p.ModulePath == "" || !strings.Contains(p.ModulePath, ".") || strings.HasSuffix(p.ModulePath, "/") {
		return fmt.Errorf("module_path must be a canonical Go module path")
	}
	if p.Build.GOOS != "linux" {
		return fmt.Errorf("build.goos must be %q, got %q", "linux", p.Build.GOOS)
	}
	if p.Build.GOARCH != "amd64" {
		return fmt.Errorf("build.goarch must be %q, got %q", "amd64", p.Build.GOARCH)
	}
	if p.Build.CGOEnabled == nil || *p.Build.CGOEnabled {
		return fmt.Errorf("build.cgo_enabled must be explicitly false")
	}
	if p.Owners == nil || p.Roles == nil || p.Packages == nil || p.DataObjects == nil || p.ReadProjections == nil || p.LegacyComposition == nil || p.SharedKernelTypes == nil || p.ExternalClasses == nil || p.ExternalPackages == nil || p.Rules == nil {
		return fmt.Errorf("owners, roles, packages, data_objects, read_projections, legacy_composition, shared_kernel_types, external_classes, external_packages, and rules are required")
	}
	if len(p.Owners) == 0 || len(p.Roles) == 0 || len(p.Packages) == 0 {
		return fmt.Errorf("owners, roles, and packages must not be empty")
	}
	wantKernel := []string{"CorrelationID", "Date", "TenantID", "WallClock"}
	if !slices.Equal(p.SharedKernelTypes, wantKernel) {
		return fmt.Errorf("shared_kernel_types must be exactly %v", wantKernel)
	}
	return nil
}

func (p *Policy) validateDataObjects(owners map[string]Owner) (map[string]DataObject, error) {
	objects := make(map[string]DataObject, len(p.DataObjects))
	for _, object := range p.DataObjects {
		if !isDataObjectName(object.Name) {
			return nil, fmt.Errorf("data object name %q must be schema-qualified", object.Name)
		}
		if object.WriteOwner == "" {
			return nil, fmt.Errorf("data object %q has no write owner", object.Name)
		}
		if _, ok := owners[object.WriteOwner]; !ok {
			return nil, fmt.Errorf("data object %q references unknown write owner %q", object.Name, object.WriteOwner)
		}
		if kind := owners[object.WriteOwner].Kind; kind != "domain" && kind != "platform" && kind != "migration" {
			return nil, fmt.Errorf("data object %q write owner %q has non-owning kind %q", object.Name, object.WriteOwner, kind)
		}
		if previous, exists := objects[object.Name]; exists {
			if previous.WriteOwner != object.WriteOwner {
				return nil, fmt.Errorf("data object %q has conflicting write owners %q and %q", object.Name, previous.WriteOwner, object.WriteOwner)
			}
			return nil, fmt.Errorf("data object %q is declared more than once", object.Name)
		}
		objects[object.Name] = object
	}
	return objects, nil
}

func (p *Policy) validateReadProjections(owners map[string]Owner, objects map[string]DataObject) error {
	seen := make(map[string]struct{}, len(p.ReadProjections))
	seenGrants := make(map[string]string)
	packages := p.packageMap()
	for _, projection := range p.ReadProjections {
		if !identifierPattern.MatchString(projection.ID) {
			return fmt.Errorf("read projection id %q is invalid", projection.ID)
		}
		if _, exists := seen[projection.ID]; exists {
			return fmt.Errorf("read projection %q is declared more than once", projection.ID)
		}
		seen[projection.ID] = struct{}{}
		pkg, ok := packages[p.absolutePackage(projection.Package)]
		if !ok {
			return fmt.Errorf("read projection %q references unknown package %q", projection.ID, projection.Package)
		}
		if owners[pkg.Owner].Kind != "projection" {
			return fmt.Errorf("read projection %q package %q is not owned by a projection", projection.ID, projection.Package)
		}
		if pkg.Role != "adapter" && pkg.Role != "postgres" {
			return fmt.Errorf("read projection %q package %q must have role %q or %q", projection.ID, projection.Package, "adapter", "postgres")
		}
		if !projection.TenantSafe {
			return fmt.Errorf("read projection %q must be explicitly tenant-safe", projection.ID)
		}
		if len(projection.DataObjects) == 0 {
			return fmt.Errorf("read projection %q has no data objects", projection.ID)
		}
		if err := p.validateProjectionObjects(projection, objects, seenGrants); err != nil {
			return err
		}
	}
	return nil
}

func (p *Policy) validateProjectionObjects(projection ReadProjection, objects map[string]DataObject, seenGrants map[string]string) error {
	projectionObjects := make(map[string]struct{}, len(projection.DataObjects))
	for _, name := range projection.DataObjects {
		if _, ok := objects[name]; !ok {
			return fmt.Errorf("read projection %q references unknown data object %q", projection.ID, name)
		}
		if _, exists := projectionObjects[name]; exists {
			return fmt.Errorf("read projection %q names data object %q more than once", projection.ID, name)
		}
		grant := p.absolutePackage(projection.Package) + "|" + name
		if previous, exists := seenGrants[grant]; exists {
			return fmt.Errorf("read projections %q and %q both grant package %q access to data object %q", previous, projection.ID, projection.Package, name)
		}
		seenGrants[grant] = projection.ID
		projectionObjects[name] = struct{}{}
	}
	return nil
}

func (p *Policy) validateLegacyComposition(owners map[string]Owner) error {
	packages := p.packageMap()
	seen := make(map[string]struct{})
	for _, legacy := range p.LegacyComposition {
		pkg, ok := packages[p.absolutePackage(legacy.Package)]
		if !ok {
			return fmt.Errorf("legacy composition references unknown package %q", legacy.Package)
		}
		if owners[pkg.Owner].Kind != "composition" || pkg.Role != "compose" {
			return fmt.Errorf("legacy composition package %q must have a composition owner and compose role", legacy.Package)
		}
		if len(legacy.Symbols) == 0 {
			return fmt.Errorf("legacy composition package %q has no symbols", legacy.Package)
		}
		for _, symbol := range legacy.Symbols {
			if !token.IsExported(symbol) {
				return fmt.Errorf("legacy composition symbol %q.%s is not exported", legacy.Package, symbol)
			}
			key := legacy.Package + "." + symbol
			if _, exists := seen[key]; exists {
				return fmt.Errorf("legacy composition symbol %q is declared more than once", key)
			}
			seen[key] = struct{}{}
		}
	}
	return nil
}

func isDataObjectName(name string) bool {
	parts := strings.Split(name, ".")
	return len(parts) == 2 && identifierPattern.MatchString(parts[0]) && identifierPattern.MatchString(parts[1])
}

func (p *Policy) validateOwners() (map[string]Owner, error) {
	owners := make(map[string]Owner, len(p.Owners))
	for _, owner := range p.Owners {
		if !identifierPattern.MatchString(owner.ID) {
			return nil, fmt.Errorf("owner id %q is invalid", owner.ID)
		}
		if _, exists := owners[owner.ID]; exists {
			return nil, fmt.Errorf("owner %q is declared more than once", owner.ID)
		}
		if !allowedOwnerKinds[owner.Kind] {
			return nil, fmt.Errorf("owner %q has invalid kind %q", owner.ID, owner.Kind)
		}
		owners[owner.ID] = owner
	}
	return owners, nil
}

func validateUniqueValues(label string, values []string, allowed map[string]bool) (map[string]struct{}, error) {
	seen := make(map[string]struct{}, len(values))
	for _, value := range values {
		if !identifierPattern.MatchString(value) || (allowed != nil && !allowed[value]) {
			return nil, fmt.Errorf("invalid %s %q", label, value)
		}
		if _, exists := seen[value]; exists {
			return nil, fmt.Errorf("%s %q is declared more than once", label, value)
		}
		seen[value] = struct{}{}
	}
	return seen, nil
}

func (p *Policy) validatePackages(owners map[string]Owner, roles map[string]struct{}) error {
	seen := make(map[string]struct{}, len(p.Packages))
	for _, pkg := range p.Packages {
		if pkg.Path == "" || strings.HasPrefix(pkg.Path, "/") || strings.Contains(pkg.Path, "..") {
			return fmt.Errorf("package path %q must be relative to module_path", pkg.Path)
		}
		if _, exists := seen[pkg.Path]; exists {
			return fmt.Errorf("package %q is classified more than once", pkg.Path)
		}
		seen[pkg.Path] = struct{}{}
		if _, ok := owners[pkg.Owner]; !ok {
			return fmt.Errorf("package %q references unknown owner %q", pkg.Path, pkg.Owner)
		}
		packageRoles := []struct{ scope, role string }{
			{scope: "production", role: pkg.Role},
			{scope: "internal_test", role: pkg.InternalTestRole},
			{scope: "external_test", role: pkg.ExternalTestRole},
		}
		for _, scopedRole := range packageRoles {
			if _, ok := roles[scopedRole.role]; !ok || !allowedRoles[scopedRole.role] {
				return fmt.Errorf("package %q has invalid %s role %q", pkg.Path, scopedRole.scope, scopedRole.role)
			}
		}
		if owners[pkg.Owner].Kind == "kernel" && pkg.Role != "contract" {
			return fmt.Errorf("shared-kernel package %q must have role %q", pkg.Path, "contract")
		}
	}
	return nil
}

func (p *Policy) validateExternalPackages(classes map[string]struct{}) error {
	seen := make(map[string]struct{}, len(p.ExternalPackages))
	for _, pkg := range p.ExternalPackages {
		if pkg.Path == "" || isOwnPackage(p.ModulePath, pkg.Path) {
			return fmt.Errorf("external package path %q is invalid", pkg.Path)
		}
		if _, exists := seen[pkg.Path]; exists {
			return fmt.Errorf("external package %q is classified more than once", pkg.Path)
		}
		seen[pkg.Path] = struct{}{}
		if _, ok := classes[pkg.Class]; !ok {
			return fmt.Errorf("external package %q has unknown class %q", pkg.Path, pkg.Class)
		}
	}
	return nil
}

func (p *Policy) validateRules(owners map[string]Owner, roles, classes map[string]struct{}) error {
	seenIDs := make(map[string]struct{}, len(p.Rules))
	validated := make([]Rule, 0, len(p.Rules))
	for _, rule := range p.Rules {
		if err := validateRule(rule, seenIDs, owners, roles, classes); err != nil {
			return err
		}
		for _, previous := range validated {
			if scope, overlaps := overlappingScope(previous, rule, owners); overlaps {
				return fmt.Errorf("rule %q overlaps rule %q for scope %q", rule.ID, previous.ID, scope)
			}
		}
		validated = append(validated, rule)
	}
	return nil
}

func validateRule(rule Rule, seenIDs map[string]struct{}, owners map[string]Owner, roles, classes map[string]struct{}) error {
	if !identifierPattern.MatchString(rule.ID) {
		return fmt.Errorf("rule id %q is invalid", rule.ID)
	}
	if _, exists := seenIDs[rule.ID]; exists {
		return fmt.Errorf("rule id %q is declared more than once", rule.ID)
	}
	seenIDs[rule.ID] = struct{}{}
	if strings.TrimSpace(rule.Description) == "" {
		return fmt.Errorf("rule %q has no description", rule.ID)
	}
	if err := validateOwnerSelector(rule.ID, "source", rule.SourceOwner, rule.SourceOwnerKind, owners); err != nil {
		return err
	}
	if rule.SourceRole != "" {
		if _, ok := roles[rule.SourceRole]; !ok {
			return fmt.Errorf("rule %q has invalid source role %q", rule.ID, rule.SourceRole)
		}
	}
	if err := validateRuleTarget(rule, owners, roles, classes); err != nil {
		return err
	}
	if len(rule.Scopes) == 0 {
		return fmt.Errorf("rule %q has no scopes", rule.ID)
	}
	for _, scope := range rule.Scopes {
		if !allowedScopes[scope] {
			return fmt.Errorf("rule %q has invalid scope %q", rule.ID, scope)
		}
	}
	return nil
}

func overlappingScope(left, right Rule, owners map[string]Owner) (string, bool) {
	if left.TargetClass != right.TargetClass || (left.TargetClass == "" && left.TargetRole != right.TargetRole) {
		return "", false
	}
	if !rolesOverlap(left.SourceRole, right.SourceRole) || !ownerSelectorsOverlap(left.SourceOwner, left.SourceOwnerKind, right.SourceOwner, right.SourceOwnerKind, owners) {
		return "", false
	}
	if left.TargetClass == "" && !targetSelectorsOverlap(left, right, owners) {
		return "", false
	}
	for _, scope := range left.Scopes {
		if slices.Contains(right.Scopes, scope) {
			return scope, true
		}
	}
	return "", false
}

func targetSelectorsOverlap(left, right Rule, owners map[string]Owner) bool {
	if !left.SameOwner && !right.SameOwner {
		return ownerSelectorsOverlap(left.TargetOwner, left.TargetOwnerKind, right.TargetOwner, right.TargetOwnerKind, owners)
	}
	for _, owner := range owners {
		if ownerMatches(left.SourceOwner, left.SourceOwnerKind, owner) && ownerMatches(right.SourceOwner, right.SourceOwnerKind, owner) &&
			ownerMatches(left.TargetOwner, left.TargetOwnerKind, owner) && ownerMatches(right.TargetOwner, right.TargetOwnerKind, owner) {
			return true
		}
	}
	return false
}

func ownerSelectorsOverlap(leftID, leftKind, rightID, rightKind string, owners map[string]Owner) bool {
	for _, owner := range owners {
		if ownerMatches(leftID, leftKind, owner) && ownerMatches(rightID, rightKind, owner) {
			return true
		}
	}
	return false
}

func ownerMatches(wantID, wantKind string, owner Owner) bool {
	return (wantID == "" || wantID == owner.ID) && (wantKind == "" || wantKind == owner.Kind)
}

func rolesOverlap(left, right string) bool {
	return left == "" || right == "" || left == right
}

func validateOwnerSelector(ruleID, side, ownerID, ownerKind string, owners map[string]Owner) error {
	if ownerID != "" && ownerKind != "" {
		return fmt.Errorf("rule %q sets both %s_owner and %s_owner_kind", ruleID, side, side)
	}
	if ownerID != "" {
		if _, ok := owners[ownerID]; !ok {
			return fmt.Errorf("rule %q references unknown %s owner %q", ruleID, side, ownerID)
		}
	}
	if ownerKind != "" && !allowedOwnerKinds[ownerKind] {
		return fmt.Errorf("rule %q has invalid %s owner kind %q", ruleID, side, ownerKind)
	}
	return nil
}

func validateRuleTarget(rule Rule, owners map[string]Owner, roles, classes map[string]struct{}) error {
	if rule.TargetClass != "" {
		if rule.TargetOwner != "" || rule.TargetOwnerKind != "" || rule.TargetRole != "" || rule.SameOwner {
			return fmt.Errorf("rule %q mixes first-party and external targets", rule.ID)
		}
		if _, ok := classes[rule.TargetClass]; !ok {
			return fmt.Errorf("rule %q references unknown external class %q", rule.ID, rule.TargetClass)
		}
		return nil
	}
	if rule.TargetRole == "" {
		return fmt.Errorf("rule %q must target a first-party role or external class", rule.ID)
	}
	if _, ok := roles[rule.TargetRole]; !ok {
		return fmt.Errorf("rule %q has invalid target role %q", rule.ID, rule.TargetRole)
	}
	if err := validateOwnerSelector(rule.ID, "target", rule.TargetOwner, rule.TargetOwnerKind, owners); err != nil {
		return err
	}
	if rule.TargetOwner == "" && rule.TargetOwnerKind == "" && !rule.SameOwner {
		return fmt.Errorf("rule %q must select a target owner or require same_owner", rule.ID)
	}
	return nil
}

func requireJSONEnd(decoder *json.Decoder) error {
	var trailing any
	err := decoder.Decode(&trailing)
	if err == io.EOF {
		return nil
	}
	if err != nil {
		return fmt.Errorf("decode trailing policy data: %w", err)
	}
	return fmt.Errorf("policy contains multiple JSON values")
}
