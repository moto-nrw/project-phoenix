package test

import (
	"encoding/json"
	"flag"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"testing"
)

const compositionEvidenceCommit = "8dc3a9ca8ac7cb8edbfa3c17760d92c02751bc3e"

var (
	updateCompositionInventory = flag.Bool("update-composition-inventory", false, "rewrite discovered composition inventory entries")
	compositionEvidenceRoot    = flag.String("composition-evidence-root", "", "optional fixed-commit backend root used only to capture immutable evidence")
)

var expectedCompositionRoots = []compositionRoot{
	{ID: "serve", Kind: "http", File: "api/server.go", Declaration: "NewServer", SmokeTest: "api/route_table_golden_test.go#TestRouteTableGolden", Tables: []string{}},
	{ID: "worker", Kind: "embedded-worker", File: "api/server.go", Declaration: "newScheduler", SmokeTest: "test/composition_inventory_test.go#TestWorkerJobRegistryInventory", Tables: []string{}},
	{ID: "commands", Kind: "cli", File: "cmd/root.go", Declaration: "RootCmd", SmokeTest: "cmd/root_test.go#TestRootCmd_HasCommands", Tables: []string{}},
}

var expectedCompositionSmokeTests = []string{
	"api/route_table_golden_test.go#TestRouteTableGolden",
	"api/route_table_golden_test.go#TestIoTAuthMatrixGolden",
	"api/school_scope_matrix_test.go#TestSchoolScopeRejectedOnAllAPIRoutes",
	"api/testutil/helpers_test.go#TestSetupAPITest",
	"cmd/root_test.go#TestRootCmd_HasCommands",
	"database/repositories/factory_test.go#TestNewFactory",
	"internal/architecture/composition_inventory_test.go#TestCompositionLegacyCallerInventory",
	"services/factory_test.go#TestNewFactory",
	"test/composition_inventory_test.go#TestCompositionInventory",
	"test/composition_inventory_test.go#TestWorkerJobRegistryInventory",
	"test/e2e/calendar/parent_calendar_flow_test.go#TestParentCalendarHTTPFlow_ViewICSAndFeed",
	"test/e2e/timetable/flow_a_happy_path_test.go#TestFlowA_PlanToReport",
}

var compositionTestPackages = []string{
	"api",
	"api/testutil",
	"cmd",
	"database/repositories",
	"services",
	"test",
	"test/e2e/calendar",
	"test/e2e/timetable",
}

var compositionTestSmokeByPackage = map[string]string{
	"api":                   "api/route_table_golden_test.go#TestRouteTableGolden",
	"api/testutil":          "api/testutil/helpers_test.go#TestSetupAPITest",
	"cmd":                   "cmd/root_test.go#TestRootCmd_HasCommands",
	"database/repositories": "database/repositories/factory_test.go#TestNewFactory",
	"services":              "services/factory_test.go#TestNewFactory",
	"test":                  "test/composition_inventory_test.go#TestCompositionInventory",
	"test/e2e/calendar":     "test/e2e/calendar/parent_calendar_flow_test.go#TestParentCalendarHTTPFlow_ViewICSAndFeed",
	"test/e2e/timetable":    "test/e2e/timetable/flow_a_happy_path_test.go#TestFlowA_PlanToReport",
}

type compositionInventory struct {
	SchemaVersion            int                 `json:"schema_version"`
	EvidenceCommit           string              `json:"evidence_commit"`
	ProductionRoots          []compositionRoot   `json:"production_roots"`
	TestRoots                []compositionRoot   `json:"test_roots"`
	Commands                 []string            `json:"commands"`
	WorkerJobIDs             []string            `json:"worker_job_ids"`
	EvidenceLegacyCallers    json.RawMessage     `json:"evidence_legacy_callers"`
	LegacyCallers            json.RawMessage     `json:"legacy_callers"`
	EvidenceConstructorCalls []constructorCaller `json:"evidence_constructor_calls"`
	ConstructorCalls         []constructorCaller `json:"constructor_calls"`
	SmokeTests               []string            `json:"smoke_tests"`
	RuntimeBaseline          runtimeBaseline     `json:"runtime_baseline"`
}

type compositionRoot struct {
	ID          string   `json:"id"`
	Kind        string   `json:"kind"`
	File        string   `json:"file"`
	Declaration string   `json:"declaration"`
	SmokeTest   string   `json:"smoke_test"`
	Tables      []string `json:"tables"`
}

type constructorCaller struct {
	Scope       string   `json:"scope"`
	File        string   `json:"file"`
	Declaration string   `json:"declaration"`
	Target      string   `json:"target"`
	Lines       []int    `json:"lines"`
	Tables      []string `json:"tables"`
}

type runtimeBaseline struct {
	MeasuredAt                  string   `json:"measured_at"`
	ServeRootConstructionMillis int      `json:"serve_root_construction_ms"`
	RegisteredRouteCount        int      `json:"registered_route_count"`
	RegisteredJobIDs            []string `json:"registered_job_ids"`
	BackendTestSuiteSeconds     float64  `json:"backend_test_suite_seconds"`
	MeasurementEnvironment      string   `json:"measurement_environment"`
	MeasurementCommands         []string `json:"measurement_commands"`
}

type compositionPolicy struct {
	DataObjects []struct {
		Name string `json:"name"`
	} `json:"data_objects"`
}

func TestCompositionInventory(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	manifestPath := filepath.Join(backendRoot, "architecture", "composition.json")
	inventory := loadCompositionInventory(t, manifestPath)
	validateCompositionMetadata(t, backendRoot, inventory)

	policy := loadCompositionPolicy(t, filepath.Join(backendRoot, "architecture", "policy.json"))
	actualCalls := discoverConstructorCallers(t, backendRoot, policy)

	if *updateCompositionInventory {
		updateDiscoveredInventory(t, manifestPath, backendRoot, inventory, actualCalls)
		return
	}

	assertJSONEqual(t, "composition constructor calls", inventory.ConstructorCalls, actualCalls)
	assertJSONEqual(t, "composition smoke tests", expectedCompositionSmokeTests, inventory.SmokeTests)
	assertSmokeTestsExist(t, backendRoot, inventory.SmokeTests)
}

func validateCompositionMetadata(t *testing.T, backendRoot string, inventory compositionInventory) {
	t.Helper()
	if inventory.SchemaVersion != 1 {
		t.Fatalf("composition inventory schema_version = %d, want 1", inventory.SchemaVersion)
	}
	if inventory.EvidenceCommit != compositionEvidenceCommit {
		t.Fatalf("composition inventory evidence_commit = %q, want %q", inventory.EvidenceCommit, compositionEvidenceCommit)
	}
	if err := inventory.validateRuntimeBaseline(); err != nil {
		t.Fatal(err)
	}
	if len(inventory.ProductionRoots) == 0 || len(inventory.Commands) == 0 {
		t.Fatal("composition inventory must list production roots and commands")
	}
	missingEvidence := len(inventory.EvidenceLegacyCallers) == 0 || len(inventory.EvidenceConstructorCalls) == 0
	capturingEvidence := *updateCompositionInventory && *compositionEvidenceRoot != ""
	if len(inventory.LegacyCallers) == 0 || (missingEvidence && !capturingEvidence) {
		t.Fatal("composition inventory must contain fixed and live caller inventories")
	}
	assertJSONEqual(t, "production composition roots", expectedCompositionRoots, inventory.ProductionRoots)
	assertRootsExist(t, backendRoot, inventory.ProductionRoots)
	actualTestRoots := discoverTestRoots(t, backendRoot, inventory.SmokeTests)
	if !*updateCompositionInventory {
		assertJSONEqual(t, "test composition roots", inventory.TestRoots, actualTestRoots)
	}
	assertRootsExist(t, backendRoot, inventory.TestRoots)
}

func updateDiscoveredInventory(t *testing.T, manifestPath, backendRoot string, inventory compositionInventory, calls []constructorCaller) {
	t.Helper()
	actualJobs := discoverWorkerJobIDs(t, backendRoot)
	inventory.ConstructorCalls = calls
	if *compositionEvidenceRoot != "" {
		evidenceRoot := filepath.Clean(*compositionEvidenceRoot)
		evidencePolicy := loadCompositionPolicy(t, filepath.Join(evidenceRoot, "architecture", "policy.json"))
		inventory.EvidenceConstructorCalls = discoverConstructorCallers(t, evidenceRoot, evidencePolicy)
	}
	inventory.TestRoots = discoverTestRoots(t, backendRoot, inventory.SmokeTests)
	inventory.WorkerJobIDs = actualJobs
	inventory.RuntimeBaseline.RegisteredJobIDs = actualJobs
	writeCompositionInventory(t, manifestPath, inventory)
}

func TestWorkerJobRegistryInventory(t *testing.T) {
	t.Parallel()

	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	inventory := loadCompositionInventory(t, filepath.Join(backendRoot, "architecture", "composition.json"))
	actualJobs := discoverWorkerJobIDs(t, backendRoot)
	assertJSONEqual(t, "worker job IDs", inventory.WorkerJobIDs, actualJobs)
	assertJSONEqual(t, "runtime baseline job IDs", inventory.RuntimeBaseline.RegisteredJobIDs, actualJobs)
}

func loadCompositionInventory(t *testing.T, path string) compositionInventory {
	t.Helper()
	file, err := os.Open(path) // #nosec G304 -- fixed repository manifest
	if err != nil {
		t.Fatalf("open composition inventory: %v", err)
	}
	defer func() { _ = file.Close() }()
	decoder := json.NewDecoder(file)
	decoder.DisallowUnknownFields()
	var inventory compositionInventory
	if err := decoder.Decode(&inventory); err != nil {
		t.Fatalf("decode composition inventory: %v", err)
	}
	return inventory
}

func loadCompositionPolicy(t *testing.T, path string) *compositionPolicy {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- fixed repository policy
	if err != nil {
		t.Fatal(err)
	}
	var policy compositionPolicy
	if err := json.Unmarshal(content, &policy); err != nil {
		t.Fatal(err)
	}
	return &policy
}

func sourceDeclarationName(declaration ast.Decl) string {
	switch typed := declaration.(type) {
	case *ast.FuncDecl:
		if typed.Recv == nil {
			return typed.Name.Name
		}
		return receiverName(typed.Recv.List[0].Type) + "." + typed.Name.Name
	case *ast.GenDecl:
		var names []string
		for _, specification := range typed.Specs {
			switch value := specification.(type) {
			case *ast.TypeSpec:
				names = append(names, value.Name.Name)
			case *ast.ValueSpec:
				for _, name := range value.Names {
					names = append(names, name.Name)
				}
			}
		}
		return typed.Tok.String() + " " + strings.Join(names, ", ")
	default:
		return "declaration"
	}
}

func receiverName(expression ast.Expr) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return typed.Name
	case *ast.StarExpr:
		return receiverName(typed.X)
	default:
		return "method"
	}
}

var compositionConstructors = map[string]map[string]string{
	"github.com/moto-nrw/project-phoenix/api":                   {"New": "api.New", "NewServer": "api.NewServer"},
	"github.com/moto-nrw/project-phoenix/api/testutil":          {"SetupAPITest": "api/testutil.SetupAPITest"},
	"github.com/moto-nrw/project-phoenix/database/repositories": {"NewFactory": "database/repositories.NewFactory"},
	"github.com/moto-nrw/project-phoenix/services":              {"NewFactory": "services.NewFactory"},
	"github.com/moto-nrw/project-phoenix/services/scheduler":    {"NewScheduler": "services/scheduler.NewScheduler"},
}

type callerAggregate struct {
	lines  []int
	tables map[string]struct{}
}

func discoverConstructorCallers(t *testing.T, backendRoot string, policy *compositionPolicy) []constructorCaller {
	t.Helper()
	aggregates := make(map[string]*callerAggregate)
	for _, root := range []string{"api", "cmd", "services", filepath.Join("database", "repositories"), "test"} {
		scanConstructorRoot(t, backendRoot, root, policy, aggregates)
	}
	result := make([]constructorCaller, 0, len(aggregates))
	for key, aggregate := range aggregates {
		parts := strings.SplitN(key, "\x00", 4)
		sort.Ints(aggregate.lines)
		result = append(result, constructorCaller{
			Scope: parts[0], File: parts[1], Declaration: parts[2], Target: parts[3],
			Lines: aggregate.lines, Tables: sortedStringSet(aggregate.tables),
		})
	}
	sort.Slice(result, func(i, j int) bool {
		left := result[i].Scope + "\x00" + result[i].File + "\x00" + result[i].Declaration + "\x00" + result[i].Target
		right := result[j].Scope + "\x00" + result[j].File + "\x00" + result[j].Declaration + "\x00" + result[j].Target
		return left < right
	})
	return result
}

func scanConstructorRoot(t *testing.T, backendRoot, relativeRoot string, policy *compositionPolicy, aggregates map[string]*callerAggregate) {
	t.Helper()
	root := filepath.Join(backendRoot, relativeRoot)
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil {
			return walkErr
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") {
			return nil
		}
		return scanConstructorFile(backendRoot, path, policy, aggregates)
	})
	if err != nil {
		t.Fatalf("scan composition constructors under %s: %v", relativeRoot, err)
	}
}

func scanConstructorFile(backendRoot, path string, policy *compositionPolicy, aggregates map[string]*callerAggregate) error {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return err
	}
	aliases := compositionImportAliases(file)
	relative, err := filepath.Rel(backendRoot, path)
	if err != nil {
		return err
	}
	relative = filepath.ToSlash(relative)
	scope := compositionCallerScope(relative)
	localPackage := compositionPackagePath(relative, file.Name.Name)
	for _, declaration := range file.Decls {
		scanConstructorDeclaration(fileSet, relative, scope, localPackage, declaration, aliases, policy, aggregates)
	}
	return nil
}

func compositionCallerScope(relative string) string {
	if strings.HasSuffix(relative, "_test.go") || strings.HasPrefix(relative, "api/testutil/") || strings.HasPrefix(relative, "test/") {
		return "test"
	}
	return "production"
}

func compositionPackagePath(relative, packageName string) string {
	directory := filepath.ToSlash(filepath.Dir(relative))
	if packageName != filepath.Base(directory) {
		return ""
	}
	return "github.com/moto-nrw/project-phoenix/" + directory
}

func compositionImportAliases(file *ast.File) map[string]string {
	return selectedImportAliases(file, compositionConstructors)
}

func selectedImportAliases(file *ast.File, selected map[string]map[string]string) map[string]string {
	aliases := make(map[string]string)
	for _, imported := range file.Imports {
		path, err := strconv.Unquote(imported.Path.Value)
		if err != nil || selected[path] == nil {
			continue
		}
		name := filepath.Base(path)
		if imported.Name != nil {
			name = imported.Name.Name
		}
		aliases[name] = path
	}
	return aliases
}

func scanConstructorDeclaration(fileSet *token.FileSet, file, scope, localPackage string, declaration ast.Decl, aliases map[string]string, policy *compositionPolicy, aggregates map[string]*callerAggregate) {
	declarationName := sourceDeclarationName(declaration)
	tables := namedDataObjects(declaration, policy)
	ast.Inspect(declaration, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok {
			return true
		}
		target := compositionConstructorTarget(call.Fun, localPackage, aliases)
		if target == "" {
			return true
		}
		key := scope + "\x00" + file + "\x00" + declarationName + "\x00" + target
		aggregate := aggregates[key]
		if aggregate == nil {
			aggregate = &callerAggregate{tables: make(map[string]struct{})}
			aggregates[key] = aggregate
		}
		aggregate.lines = append(aggregate.lines, fileSet.PositionFor(call.Pos(), false).Line)
		for _, table := range tables {
			aggregate.tables[table] = struct{}{}
		}
		return true
	})
}

func compositionConstructorTarget(expression ast.Expr, localPackage string, aliases map[string]string) string {
	switch typed := expression.(type) {
	case *ast.Ident:
		return compositionConstructors[localPackage][typed.Name]
	case *ast.SelectorExpr:
		identifier, ok := typed.X.(*ast.Ident)
		if !ok {
			return ""
		}
		return compositionConstructors[aliases[identifier.Name]][typed.Sel.Name]
	default:
		return ""
	}
}

func namedDataObjects(declaration ast.Decl, policy *compositionPolicy) []string {
	result := make(map[string]struct{})
	ast.Inspect(declaration, func(node ast.Node) bool {
		literal, ok := node.(*ast.BasicLit)
		if !ok || literal.Kind != token.STRING {
			return true
		}
		value, err := strconv.Unquote(literal.Value)
		if err != nil {
			return true
		}
		for _, object := range policy.DataObjects {
			if strings.Contains(value, object.Name) {
				result[object.Name] = struct{}{}
			}
		}
		return true
	})
	return sortedStringSet(result)
}

func discoverWorkerJobIDs(t *testing.T, backendRoot string) []string {
	t.Helper()
	jobs := make(map[string]struct{})
	root := filepath.Join(backendRoot, "services", "scheduler")
	err := filepath.WalkDir(root, func(path string, entry os.DirEntry, walkErr error) error {
		if walkErr != nil || entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return walkErr
		}
		return collectRegisteredJobs(path, jobs)
	})
	if err != nil {
		t.Fatalf("scan scheduler registry: %v", err)
	}
	return sortedStringSet(jobs)
}

func collectRegisteredJobs(path string, jobs map[string]struct{}) error {
	fileSet := token.NewFileSet()
	file, err := parser.ParseFile(fileSet, path, nil, 0)
	if err != nil {
		return err
	}
	ast.Inspect(file, func(node ast.Node) bool {
		call, ok := node.(*ast.CallExpr)
		if !ok || len(call.Args) == 0 {
			return true
		}
		selector, ok := call.Fun.(*ast.SelectorExpr)
		literal, literalOK := call.Args[0].(*ast.BasicLit)
		if !ok || selector.Sel.Name != "registerTask" || !literalOK || literal.Kind != token.STRING {
			return true
		}
		if value, unquoteErr := strconv.Unquote(literal.Value); unquoteErr == nil {
			jobs[value] = struct{}{}
		}
		return true
	})
	return nil
}

func discoverTestRoots(t *testing.T, backendRoot string, smokeTests []string) []compositionRoot {
	t.Helper()
	smokeSet := make(map[string]struct{}, len(smokeTests))
	for _, smoke := range smokeTests {
		smokeSet[smoke] = struct{}{}
	}
	var roots []compositionRoot
	for _, packagePath := range compositionTestPackages {
		root := discoverPackageTestRoot(t, backendRoot, packagePath)
		root.SmokeTest = compositionTestSmokeByPackage[packagePath]
		if _, exists := smokeSet[root.SmokeTest]; !exists {
			t.Errorf("test root %q smoke test %q is absent from smoke_tests", root.ID, root.SmokeTest)
		}
		roots = append(roots, root)
	}
	return roots
}

func discoverPackageTestRoot(t *testing.T, backendRoot, packagePath string) compositionRoot {
	t.Helper()
	directory := filepath.Join(backendRoot, filepath.FromSlash(packagePath))
	entries, err := os.ReadDir(directory)
	if err != nil {
		t.Fatalf("read test-root package %s: %v", packagePath, err)
	}
	for _, entry := range entries {
		if entry.IsDir() || !strings.HasSuffix(entry.Name(), "_test.go") {
			continue
		}
		file := filepath.ToSlash(filepath.Join(packagePath, entry.Name()))
		exists, parseErr := sourceDeclarationExists(filepath.Join(backendRoot, filepath.FromSlash(file)), "TestMain")
		if parseErr != nil {
			t.Fatalf("inspect test root %s: %v", file, parseErr)
		}
		if exists {
			return compositionRoot{
				ID: strings.ReplaceAll(packagePath, "/", "-") + "-tests", Kind: "test-binary",
				File: file, Declaration: "TestMain", Tables: []string{},
			}
		}
	}
	t.Fatalf("package %s has no TestMain composition root", packagePath)
	return compositionRoot{}
}

func assertSmokeTestsExist(t *testing.T, backendRoot string, smokeTests []string) {
	t.Helper()
	for _, smoke := range smokeTests {
		parts := strings.SplitN(smoke, "#", 2)
		if len(parts) != 2 {
			t.Errorf("invalid smoke test reference %q; want file#TestName", smoke)
			continue
		}
		path := filepath.Join(backendRoot, filepath.FromSlash(parts[0]))
		exists, err := sourceDeclarationExists(path, parts[1])
		if err != nil {
			t.Errorf("inspect smoke test %q: %v", smoke, err)
			continue
		}
		if !exists {
			t.Errorf("smoke test %q is not declared", smoke)
		}
	}
}

func assertRootsExist(t *testing.T, backendRoot string, roots []compositionRoot) {
	t.Helper()
	seen := make(map[string]struct{}, len(roots))
	for _, root := range roots {
		if root.ID == "" || root.Kind == "" || root.File == "" || root.Declaration == "" || root.SmokeTest == "" || root.Tables == nil {
			t.Errorf("incomplete production root: %#v", root)
			continue
		}
		if _, duplicate := seen[root.ID]; duplicate {
			t.Errorf("duplicate production root ID %q", root.ID)
		}
		seen[root.ID] = struct{}{}
		path := filepath.Join(backendRoot, filepath.FromSlash(root.File))
		exists, err := sourceDeclarationExists(path, root.Declaration)
		if err != nil {
			t.Errorf("inspect production root %q: %v", root.ID, err)
			continue
		}
		if !exists {
			t.Errorf("production root %q declaration %q is absent from %s", root.ID, root.Declaration, root.File)
		}
	}
}

func sourceDeclarationExists(path, name string) (bool, error) {
	file, err := parser.ParseFile(token.NewFileSet(), path, nil, 0) // #nosec G304 -- manifest-owned repository path
	if err != nil {
		return false, err
	}
	for _, declaration := range file.Decls {
		switch typed := declaration.(type) {
		case *ast.FuncDecl:
			if typed.Name.Name == name {
				return true, nil
			}
		case *ast.GenDecl:
			if generatedDeclarationContains(typed, name) {
				return true, nil
			}
		}
	}
	return false, nil
}

func generatedDeclarationContains(declaration *ast.GenDecl, name string) bool {
	for _, specification := range declaration.Specs {
		switch typed := specification.(type) {
		case *ast.TypeSpec:
			if typed.Name.Name == name {
				return true
			}
		case *ast.ValueSpec:
			for _, candidate := range typed.Names {
				if candidate.Name == name {
					return true
				}
			}
		}
	}
	return false
}

func assertJSONEqual(t *testing.T, label string, want, got any) {
	t.Helper()
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatalf("encode expected %s: %v", label, err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatalf("encode actual %s: %v", label, err)
	}
	if string(wantJSON) != string(gotJSON) {
		t.Errorf("%s differ; run go test ./test -run TestCompositionInventory -update-composition-inventory\nwant %s\ngot  %s", label, wantJSON, gotJSON)
	}
}

func writeCompositionInventory(t *testing.T, path string, inventory compositionInventory) {
	t.Helper()
	encoded, err := json.MarshalIndent(inventory, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func sortedStringSet(values map[string]struct{}) []string {
	result := make([]string, 0, len(values))
	for value := range values {
		result = append(result, value)
	}
	sort.Strings(result)
	return result
}

func (inventory compositionInventory) validateRuntimeBaseline() error {
	if inventory.RuntimeBaseline.RegisteredRouteCount <= 0 || inventory.RuntimeBaseline.ServeRootConstructionMillis <= 0 || inventory.RuntimeBaseline.BackendTestSuiteSeconds <= 0 {
		return fmt.Errorf("runtime baseline measurements must be positive")
	}
	return nil
}
