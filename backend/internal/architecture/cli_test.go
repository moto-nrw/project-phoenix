package architecture_test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/internal/architecture"
)

func TestCheckRejectsUnknownPolicyFields(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "unknown-field.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, `unknown field "surprise"`) {
		t.Fatalf("check error does not identify the unknown field:\n%s", output)
	}
}

func TestCheckRejectsUnsupportedSchemaVersion(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "unsupported-schema.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "schema_version must be 2, got 3") {
		t.Fatalf("check error does not identify the unsupported schema:\n%s", output)
	}
}

func TestCheckReportsForbiddenProductionImport(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "forbidden-edge.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	want := "production|imports.forbidden|example.test/architecture-fixture/source|example.test/architecture-fixture/target"
	if !strings.Contains(output, want) {
		t.Fatalf("check did not report the canonical import violation %q:\n%s", want, output)
	}
}

func TestCheckReportsUnclassifiedFirstPartyPackage(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "unclassified-package.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	want := "production|packages.unclassified|example.test/architecture-fixture/target|example.test/architecture-fixture/target"
	if !strings.Contains(output, want) {
		t.Fatalf("check did not report the unclassified package %q:\n%s", want, output)
	}
	if !strings.Contains(output, "  at target/target.go:1 (package target)") {
		t.Fatalf("unclassified package has no source evidence:\n%s", output)
	}
}

func TestProjectionIncludesUnclassifiedPackageLocation(t *testing.T) {
	t.Parallel()

	project := fixturePath(t, "valid")
	policy := fixturePath(t, "unclassified-package.json")
	bundle := loadDiagramBundle(t, project, policy)
	key := "production|packages.unclassified|example.test/architecture-fixture/target|example.test/architecture-fixture/target"
	want := []architecture.Location{{File: "target/target.go", Line: 1, Declaration: "package target"}}
	if got := projectedViolation(t, bundle.Migration, key).Locations; !slices.Equal(got, want) {
		t.Fatalf("projected package locations = %#v, want %#v", got, want)
	}
}

func TestCheckRejectsDuplicatePackageClassification(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "duplicate-package.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, `package "target" is classified more than once`) {
		t.Fatalf("check error does not identify the duplicate classification:\n%s", output)
	}
}

func TestCheckRejectsMissingOrConflictingTableOwners(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  string
		message string
	}{
		{name: "missing owner", policy: "missing-table-owner.json", message: `data object "fixture.records" has no write owner`},
		{name: "conflicting owners", policy: "conflicting-table-owners.json", message: `data object "fixture.records" has conflicting write owners "source" and "target"`},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, tc.policy))
			if err == nil {
				t.Fatalf("check unexpectedly succeeded:\n%s", output)
			}
			if !strings.Contains(output, tc.message) {
				t.Fatalf("check error does not contain %q:\n%s", tc.message, output)
			}
		})
	}
}

func TestCheckRejectsInvalidReadProjection(t *testing.T) {
	t.Parallel()

	tests := []struct{ policy, message string }{
		{policy: "unsafe-read-projection.json", message: `read projection "fixture-view" must be explicitly tenant-safe`},
		{policy: "invalid-read-projection-role.json", message: `read projection "fixture-view" package "target" must have role "adapter" or "postgres"`},
	}
	for _, tt := range tests {
		output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, tt.policy))
		if err == nil {
			t.Fatalf("check unexpectedly succeeded:\n%s", output)
		}
		if !strings.Contains(output, tt.message) {
			t.Fatalf("check error does not contain %q:\n%s", tt.message, output)
		}
	}
}

func TestCheckReportsUnknownExternalPackage(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "external", "app"), "--policy", fixturePath(t, "external", "policy.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	want := "production|external.unclassified|example.test/architecture-external-app/source|example.test/architecture-vendor"
	if !strings.Contains(output, want) {
		t.Fatalf("check did not report the unknown external package %q:\n%s", want, output)
	}
}

func TestCheckRejectsInvalidPolicyValues(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name    string
		policy  string
		message string
	}{
		{name: "build context", policy: "invalid-build.json", message: `build.goos must be "linux"`},
		{name: "shared kernel", policy: "invalid-shared-kernel.json", message: `shared_kernel_types must be exactly [CorrelationID Date TenantID WallClock]`},
		{name: "owner kind", policy: "invalid-owner-kind.json", message: `owner "source" has invalid kind "misc"`},
		{name: "role", policy: "invalid-role.json", message: `package "source" has invalid production role "helper"`},
		{name: "scope", policy: "invalid-scope.json", message: `rule "source-uses-target" has invalid scope "all"`},
		{name: "external class", policy: "invalid-external-class.json", message: `external package "example.test/architecture-fixture-extra" has unknown class "unknown"`},
	}

	for _, tt := range tests {
		tc := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, tc.policy))
			if err == nil {
				t.Fatalf("check unexpectedly succeeded:\n%s", output)
			}
			if !strings.Contains(output, tc.message) {
				t.Fatalf("check error does not contain %q:\n%s", tc.message, output)
			}
		})
	}
}

func TestExplainNamesTheRuleThatAllowsAnEdge(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t,
		"explain",
		"--policy", fixturePath(t, "allowed-edge.json"),
		"--scope", "production",
		"--source", "example.test/architecture-fixture/source",
		"--target", "example.test/architecture-fixture/target",
	)
	if err != nil {
		t.Fatalf("explain failed: %v\n%s", err, output)
	}
	for _, want := range []string{"ALLOWED", "source-uses-target", "The fixture source may call its target."} {
		if !strings.Contains(output, want) {
			t.Fatalf("explain output does not contain %q:\n%s", want, output)
		}
	}
}

func TestCheckUsesLinuxAMD64WithCGODisabled(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "allowed-edge.json"))
	if err != nil {
		t.Fatalf("check did not use the policy build context: %v\n%s", err, output)
	}
}

func TestCheckEvaluatesProductionInternalAndExternalTestsSeparately(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "scopes"), "--policy", fixturePath(t, "scopes", "policy.json"))
	if err != nil {
		t.Fatalf("scope-aware check failed: %v\n%s", err, output)
	}
}

func TestCheckReportsEveryImportLocationInEachScope(t *testing.T) {
	t.Parallel()

	project := fixturePath(t, "scopes")
	policy := forbiddenScopePolicy(t)
	output := stableFailingCheck(t, "check", "--project", project, "--policy", policy)
	assertContainsAll(t, output,
		"production|imports.forbidden|example.test/architecture-scopes/source|example.test/architecture-scopes/production",
		"  at source/another.go:3 (import example.test/architecture-scopes/production)",
		"  at source/source.go:3 (import example.test/architecture-scopes/production)",
		"internal_test|imports.forbidden|example.test/architecture-scopes/source|example.test/architecture-scopes/internal-target",
		"  at source/another_test.go:6 (import example.test/architecture-scopes/internal-target)",
		"  at source/source_test.go:6 (import example.test/architecture-scopes/internal-target)",
		"external_test|imports.forbidden|example.test/architecture-scopes/source|example.test/architecture-scopes/external-target",
		"  at source/another_external_test.go:6 (import example.test/architecture-scopes/external-target)",
		"  at source/source_external_test.go:6 (import example.test/architecture-scopes/external-target)",
	)
	assertNoProjectPath(t, output, project)
	bundle := loadDiagramBundle(t, project, policy)
	if bundle.SchemaVersion != 2 || bundle.Migration.SchemaVersion != 2 {
		t.Fatalf("location-aware projection schema versions = bundle %d, migration %d", bundle.SchemaVersion, bundle.Migration.SchemaVersion)
	}
	key := "production|imports.forbidden|example.test/architecture-scopes/source|example.test/architecture-scopes/production"
	want := []architecture.Location{
		{File: "source/another.go", Line: 3, Declaration: "import example.test/architecture-scopes/production"},
		{File: "source/source.go", Line: 3, Declaration: "import example.test/architecture-scopes/production"},
	}
	if got := projectedViolation(t, bundle.Migration, key).Locations; !slices.Equal(got, want) {
		t.Fatalf("projected import locations = %#v", got)
	}
}

func TestLoadGraphIncludesInternalAndExternalTestImports(t *testing.T) {
	t.Parallel()

	policy, err := architecture.LoadPolicy(fixturePath(t, "scopes", "policy.json"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	graph, err := architecture.LoadGraph(fixturePath(t, "scopes"), policy)
	if err != nil {
		t.Fatalf("load graph: %v", err)
	}

	want := map[architecture.Edge]struct{}{
		{Scope: architecture.ScopeProduction, Source: "example.test/architecture-scopes/source", Target: "example.test/architecture-scopes/production"}:        {},
		{Scope: architecture.ScopeInternalTest, Source: "example.test/architecture-scopes/source", Target: "example.test/architecture-scopes/internal-target"}: {},
		{Scope: architecture.ScopeInternalTest, Source: "example.test/architecture-scopes/source", Target: "testing"}:                                          {},
		{Scope: architecture.ScopeExternalTest, Source: "example.test/architecture-scopes/source", Target: "example.test/architecture-scopes/external-target"}: {},
		{Scope: architecture.ScopeExternalTest, Source: "example.test/architecture-scopes/source", Target: "example.test/architecture-scopes/source"}:          {},
		{Scope: architecture.ScopeExternalTest, Source: "example.test/architecture-scopes/source", Target: "testing"}:                                          {},
	}
	for _, edge := range graph.Edges {
		delete(want, edge)
	}
	if len(want) != 0 {
		t.Fatalf("graph is missing test import edges: %v", want)
	}
}

func TestCheckRejectsLogicallyOverlappingRules(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "overlapping-rules.json"))
	if err == nil {
		t.Fatalf("check unexpectedly succeeded:\n%s", output)
	}
	want := `rule "overlapping-kind-edge" overlaps rule "exact-edge" for scope "production"`
	if !strings.Contains(output, want) {
		t.Fatalf("check did not report the overlapping rules %q:\n%s", want, output)
	}
}

func TestCheckCoversAllowedAndForbiddenVerticalEdges(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "vertical-allowed.json"))
	if err != nil {
		t.Fatalf("allowed vertical edge failed: %v\n%s", err, output)
	}

	output, err = runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "vertical-forbidden.json"))
	if err == nil {
		t.Fatalf("forbidden vertical edge unexpectedly succeeded:\n%s", output)
	}
	if !strings.Contains(output, "production|imports.forbidden|example.test/architecture-fixture/source|example.test/architecture-fixture/target") {
		t.Fatalf("forbidden vertical edge was not reported:\n%s", output)
	}
}

func TestDiagramWritesDeterministicPolicyAndMigrationProjections(t *testing.T) {
	t.Parallel()

	outputDirectory := t.TempDir()
	output, err := runArchitecture(t,
		"diagram",
		"--project", fixturePath(t, "projection"),
		"--policy", fixturePath(t, "projection", "policy.json"),
		"--baseline", fixturePath(t, "projection", "legacy.jsonl"),
		"--output", outputDirectory,
	)
	if err != nil {
		t.Fatalf("diagram failed: %v\n%s", err, output)
	}
	if !strings.Contains(output, outputDirectory) {
		t.Fatalf("diagram did not print its temporary output directory: %s", output)
	}

	for _, name := range []string{"target.svg", "migration.svg"} {
		assertValidSVGFile(t, filepath.Join(outputDirectory, name))
	}
	for _, name := range []string{"architecture.json", "go-arch-lint.yml"} {
		if contents := readFile(t, filepath.Join(outputDirectory, name)); contents == "" {
			t.Fatalf("%s is empty", name)
		}
	}
	var bundle struct {
		Target    architecture.Projection `json:"target"`
		Migration architecture.Projection `json:"migration"`
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outputDirectory, "architecture.json"))), &bundle); err != nil {
		t.Fatalf("decode architecture.json: %v", err)
	}
	if got := projectionEdgeSummaries(bundle.Migration); !slices.Equal(got, []string{"alpha->beta:allowed", "alpha->gamma:new", "beta->gamma:legacy"}) {
		t.Fatalf("CLI migration statuses = %v", got)
	}
}

func TestDependenciesWritesFocusedSVGJSONAndGodaQuery(t *testing.T) {
	t.Parallel()

	outputDirectory := t.TempDir()
	output, err := runArchitecture(t,
		"dependencies",
		"--project", fixturePath(t, "projection"),
		"--policy", fixturePath(t, "projection", "policy.json"),
		"--baseline", fixturePath(t, "projection", "legacy.jsonl"),
		"--focus", "module:beta",
		"--output", outputDirectory,
	)
	if err != nil {
		t.Fatalf("dependencies failed: %v\n%s", err, output)
	}
	assertValidSVGFile(t, filepath.Join(outputDirectory, "dependencies.svg"))
	if query := readFile(t, filepath.Join(outputDirectory, "dependencies.goda")); !strings.Contains(query, "cgo_enabled=0(goarch=amd64(goos=linux") {
		t.Fatalf("Goda query does not pin the policy build context: %s", query)
	}
	var projection architecture.Projection
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(outputDirectory, "dependencies.json"))), &projection); err != nil {
		t.Fatalf("decode dependencies.json: %v", err)
	}
	if projection.Focus != "module:beta" {
		t.Fatalf("focused machine projection has focus %q", projection.Focus)
	}
}

func TestDependenciesRejectsUnknownAndAmbiguousFocus(t *testing.T) {
	t.Parallel()

	for _, tt := range []struct{ focus, want string }{
		{focus: "missing", want: "unknown focus"},
		{focus: "beta", want: "ambiguous focus"},
	} {
		output, err := runArchitecture(t,
			"dependencies",
			"--project", fixturePath(t, "projection"),
			"--policy", fixturePath(t, "projection", "policy.json"),
			"--focus", tt.focus,
			"--output", t.TempDir(),
		)
		if err == nil || !strings.Contains(output, tt.want) {
			t.Errorf("focus %q error = %v, output = %s, want %q", tt.focus, err, output, tt.want)
		}
	}
}

func TestDiagramRejectsCommittedOutputLocations(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t,
		"diagram",
		"--project", fixturePath(t, "projection"),
		"--policy", fixturePath(t, "projection", "policy.json"),
		"--output", fixturePath(t, "projection", "generated"),
	)
	if err == nil || !strings.Contains(output, "must be inside the system temporary directory") {
		t.Fatalf("repository output was not rejected: %v\n%s", err, output)
	}
}

func assertValidSVGFile(t *testing.T, path string) {
	t.Helper()
	assertValidSVG(t, []byte(readFile(t, path)))
}

func TestDiagramRejectsTemporarySymlinkOutsideTemporaryRoot(t *testing.T) {
	t.Parallel()

	link := filepath.Join(t.TempDir(), "outside")
	if err := os.Symlink(fixturePath(t, "projection"), link); err != nil {
		t.Fatalf("create output symlink: %v", err)
	}
	output, err := runArchitecture(t,
		"diagram",
		"--project", fixturePath(t, "projection"),
		"--policy", fixturePath(t, "projection", "policy.json"),
		"--output", link,
	)
	if err == nil || !strings.Contains(output, "must resolve inside the system temporary directory") {
		t.Fatalf("symlink output outside temporary root was not rejected: %v\n%s", err, output)
	}
}

func TestDiagramRejectsExistingArtifactSymlink(t *testing.T) {
	t.Parallel()

	outputDirectory := t.TempDir()
	target := filepath.Join(t.TempDir(), "target.svg")
	if err := os.WriteFile(target, []byte("unchanged"), 0o600); err != nil {
		t.Fatalf("write symlink target: %v", err)
	}
	if err := os.Symlink(target, filepath.Join(outputDirectory, "target.svg")); err != nil {
		t.Fatalf("create artifact symlink: %v", err)
	}
	output, err := runArchitecture(t,
		"diagram",
		"--project", fixturePath(t, "projection"),
		"--policy", fixturePath(t, "projection", "policy.json"),
		"--output", outputDirectory,
	)
	if err == nil || !strings.Contains(output, "write generated architecture artifact target.svg") {
		t.Fatalf("artifact symlink was not rejected: %v\n%s", err, output)
	}
	if got := readFile(t, target); got != "unchanged" {
		t.Fatalf("artifact symlink target was overwritten: %q", got)
	}
}

func TestCheckAllowsOwnedTableAccessThroughPostgresAdapter(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "semantic", "valid"), "--policy", fixturePath(t, "semantic", "valid", "policy.json"))
	if err != nil {
		t.Fatalf("owned table access failed: %v\n%s", err, output)
	}
}

func TestCheckReportsSemanticArchitectureViolations(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "semantic", "invalid"), "--policy", fixturePath(t, "semantic", "invalid", "policy.json"))
	if err == nil {
		t.Fatalf("semantic violations unexpectedly succeeded:\n%s", output)
	}

	for _, want := range expectedSemanticViolationKeys() {
		if !strings.Contains(output, want) {
			t.Errorf("check did not report semantic violation %q:\n%s", want, output)
		}
	}
	if strings.Contains(output, "tables.foreign-read|example.test/architecture-semantic/projection|beta.records") {
		t.Fatalf("tenant-safe projection read was rejected:\n%s", output)
	}
	if strings.Contains(output, "database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.MixedDB.Close") {
		t.Fatalf("unrelated interface method was reported as database access:\n%s", output)
	}
}

func TestCheckReportsDeterministicSemanticLocations(t *testing.T) {
	t.Parallel()

	project := fixturePath(t, "semantic", "invalid")
	policy := fixturePath(t, "semantic", "invalid", "policy.json")
	output := stableFailingCheck(t, "check", "--project", project, "--policy", policy)
	assertContainsAll(t, output,
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.records",
		"  at foreign/duplicate.go:6 (ReadAgain)",
		"  at foreign/foreign.go:10 (ReadAndWrite)",
		"production|database.direct-access|example.test/architecture-semantic/service|github.com/uptrace/bun."+"DB.NewSelect",
		"  at service/service.go:12 (Read)",
		"production|database.direct-access|example.test/architecture-semantic/service|database/sql.Conn.ExecContext",
		"  at service/service.go:34 (SQLAccess)",
		"production|contracts.orm-tag|example.test/architecture-semantic/public|public.Leaky",
		"  at public/public.go:10 (public.Leaky)",
		"production|composition.legacy-reference|example.test/architecture-semantic/consumer|example.test/architecture-semantic/legacy.Factory",
		"  at consumer/consumer.go:5 (Build)",
	)
	assertNoProjectPath(t, output, project)
	bundle := loadDiagramBundle(t, project, policy)
	for _, violation := range bundle.Migration.Violations {
		if isSemanticRule(violation.Rule) && len(violation.Locations) == 0 {
			t.Errorf("semantic violation %q has no machine-readable location", violation.Key)
		}
	}
	key := "production|contracts.orm-tag|example.test/architecture-semantic/public|public.Leaky"
	want := []architecture.Location{{File: "public/public.go", Line: 10, Declaration: "public.Leaky"}}
	if got := projectedViolation(t, bundle.Migration, key).Locations; !slices.Equal(got, want) {
		t.Fatalf("projected contract locations = %#v, want %#v", got, want)
	}
}

func isSemanticRule(rule string) bool {
	return strings.HasPrefix(rule, "tables.") || strings.HasPrefix(rule, "contracts.") ||
		strings.HasPrefix(rule, "database.") || strings.HasPrefix(rule, "composition.")
}

type architectureBundle struct {
	SchemaVersion int                     `json:"schema_version"`
	Migration     architecture.Projection `json:"migration"`
}

func forbiddenScopePolicy(t *testing.T) string {
	t.Helper()
	policy := mutatePolicy(t, readFile(t, fixturePath(t, "scopes", "policy.json")), func(document map[string]any) {
		var rules []any
		for _, value := range document["rules"].([]any) {
			rule := value.(map[string]any)
			if !slices.Contains([]string{"production-edge", "internal-test-edge", "external-test-edge"}, rule["id"].(string)) {
				rules = append(rules, rule)
			}
		}
		document["rules"] = rules
	})
	path := filepath.Join(t.TempDir(), "policy.json")
	writeFile(t, path, policy)
	return path
}

func stableFailingCheck(t *testing.T, args ...string) string {
	t.Helper()
	first, err := runArchitecture(t, args...)
	if err == nil {
		t.Fatalf("architecture check unexpectedly succeeded:\n%s", first)
	}
	second, secondErr := runArchitecture(t, args...)
	if secondErr == nil {
		t.Fatalf("repeated architecture check unexpectedly succeeded:\n%s", second)
	}
	if first != second {
		t.Fatalf("identical sources produced unstable output:\nFIRST:\n%s\nSECOND:\n%s", first, second)
	}
	return first
}

func assertContainsAll(t *testing.T, output string, expected ...string) {
	t.Helper()
	for _, want := range expected {
		if !strings.Contains(output, want) {
			t.Errorf("check output does not contain %q:\n%s", want, output)
		}
	}
}

func assertNoProjectPath(t *testing.T, output, project string) {
	t.Helper()
	if strings.Contains(output, project) {
		t.Fatalf("check output contains an absolute project path:\n%s", output)
	}
}

func loadDiagramBundle(t *testing.T, project, policy string) architectureBundle {
	t.Helper()
	outputDirectory := t.TempDir()
	output, err := runArchitecture(t, "diagram", "--project", project, "--policy", policy, "--output", outputDirectory)
	if err != nil {
		t.Fatalf("diagram failed: %v\n%s", err, output)
	}
	var bundle architectureBundle
	path := filepath.Join(outputDirectory, "architecture.json")
	if err := json.Unmarshal([]byte(readFile(t, path)), &bundle); err != nil {
		t.Fatalf("decode architecture.json: %v", err)
	}
	return bundle
}

func projectedViolation(t *testing.T, projection architecture.Projection, key string) architecture.ProjectionViolation {
	t.Helper()
	for _, violation := range projection.Violations {
		if violation.Key == key {
			return violation
		}
	}
	t.Fatalf("machine projection omits violation %q", key)
	return architecture.ProjectionViolation{}
}

func TestLineDirectivesDoNotReplacePhysicalSemanticLocations(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t,
		"check",
		"--project", fixturePath(t, "unresolved-location"),
		"--policy", fixturePath(t, "unresolved-location", "policy.json"),
	)
	if err == nil {
		t.Fatalf("semantic violation unexpectedly succeeded:\n%s", output)
	}
	for _, want := range []string{"database.direct-access", "  at source/source.go:7 (Read)"} {
		if !strings.Contains(output, want) {
			t.Fatalf("physical semantic location does not contain %q:\n%s", want, output)
		}
	}
}

func expectedSemanticViolationKeys() []string {
	return slices.Concat(
		expectedTableViolationKeys(),
		expectedContractViolationKeys(),
		expectedDatabaseViolationKeys(),
		expectedCompositionViolationKeys(),
	)
}

func expectedTableViolationKeys() []string {
	return []string{
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.comma_source",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.comma_table_expr",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.delete_source",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.fragment_records",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.joined_records",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.merge_source",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.records",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.qualified_records",
		"production|tables.foreign-read|example.test/architecture-semantic/foreign|beta.table_expr_join",
		"production|tables.foreign-write|example.test/architecture-semantic/foreign|beta.hidden_records",
		"production|tables.foreign-write|example.test/architecture-semantic/foreign|beta.merged_records",
		"production|tables.foreign-write|example.test/architecture-semantic/foreign|beta.records",
		"production|tables.foreign-write|example.test/architecture-semantic/foreign|beta.truncated_records",
		"production|tables.foreign-write|example.test/architecture-semantic/foreign|beta.truncate_query_records",
		"production|tables.foreign-write|example.test/architecture-semantic/foreign|beta.later_truncated_records",
		"production|tables.unresolved|example.test/architecture-semantic/foreign|foreign.Exec",
		"production|tables.unresolved|example.test/architecture-semantic/foreign|foreign.ColumnExpr",
		"production|tables.unresolved|example.test/architecture-semantic/foreign|foreign.Join",
		"production|tables.foreign-write|example.test/architecture-semantic/projection|beta.records",
		"production|tables.unclassified|example.test/architecture-semantic/unknown|ghost.records",
		"production|tables.unresolved|example.test/architecture-semantic/dynamic|dynamic.TableExpr",
		"production|tables.unresolved|example.test/architecture-semantic/dynamic|dynamic.Where",
		"production|tables.unresolved|example.test/architecture-semantic/dynamic|dynamic.ColumnExpr",
		"production|tables.unresolved|example.test/architecture-semantic/dynamic|dynamic.OrderExpr",
		"production|tables.unresolved|example.test/architecture-semantic/dynamic|dynamic.Exec",
		"production|tables.unresolved|example.test/architecture-semantic/dynamic|dynamic.ExecContext",
		"production|tables.foreign-read|example.test/architecture-semantic/service|beta.fragment_records",
		"production|tables.foreign-read|example.test/architecture-semantic/service|beta.records",
		"production|tables.unresolved|example.test/architecture-semantic/service|service.Exec",
	}
}

func expectedContractViolationKeys() []string {
	return []string{
		"production|contracts.orm-type|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.repository-type|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.filter-map|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.internal-model|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.orm-tag|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.Service.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.Service.Get",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.Service.Upsert",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.New.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.NewRecursive.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.NewAnonymous.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.NewImported.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.NewShared.First.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.NewShared.Second.List",
	}
}

func expectedDatabaseViolationKeys() []string {
	return []string{
		"production|database.direct-access|example.test/architecture-semantic/service|github.com/uptrace/bun." + "DB.NewSelect",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.WrappedDB.NewSelect",
		"production|database.direct-access|example.test/architecture-semantic/service|github.com/uptrace/bun." + "DB.Begin",
		"production|database.direct-access|example.test/architecture-semantic/service|database/sql.Conn.ExecContext",
		"production|database.direct-access|example.test/architecture-semantic/service|database/sql.Stmt.Exec",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.SelectDB.NewSelect",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.SQLDB.ExecContext",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.PingDB.Ping",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.PingDB.PingContext",
	}
}

func expectedCompositionViolationKeys() []string {
	return []string{
		"production|composition.legacy-reference|example.test/architecture-semantic/consumer|example.test/architecture-semantic/legacy.Factory",
		"production|composition.legacy-reference|example.test/architecture-semantic/consumer|example.test/architecture-semantic/legacy.NewFactory",
	}
}

func TestPolicyRejectsLegacyReferencesOutsideCompositionPackages(t *testing.T) {
	t.Parallel()

	tests := map[string]func(*architecture.Package){
		"domain owner":     func(pkg *architecture.Package) { pkg.Owner = "alpha" },
		"non-compose role": func(pkg *architecture.Package) { pkg.Role = "application" },
	}
	for name, mutate := range tests {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			policy, err := architecture.LoadPolicy(fixturePath(t, "semantic", "invalid", "policy.json"))
			if err != nil {
				t.Fatalf("load policy: %v", err)
			}
			for i := range policy.Packages {
				if policy.Packages[i].Path == "legacy" {
					mutate(&policy.Packages[i])
					break
				}
			}
			if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), `legacy composition package "legacy" must have a composition owner and compose role`) {
				t.Fatalf("legacy package outside composition was accepted: %v", err)
			}
		})
	}
}

func TestPolicyRejectsLegacyPackageAliasesWithDuplicateSymbols(t *testing.T) {
	t.Parallel()

	policy, err := architecture.LoadPolicy(fixturePath(t, "semantic", "invalid", "policy.json"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	policy.LegacyComposition = append(policy.LegacyComposition, architecture.LegacyReference{
		Package: "/legacy",
		Symbols: []string{"Factory"},
	})
	if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), `legacy composition symbol "example.test/architecture-semantic/legacy.Factory" is declared more than once`) {
		t.Fatalf("duplicate legacy symbol package aliases were accepted: %v", err)
	}
}

func TestPolicyRejectsProjectionPackageAliasesWithDuplicateGrants(t *testing.T) {
	t.Parallel()

	policy, err := architecture.LoadPolicy(fixturePath(t, "semantic", "invalid", "policy.json"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	policy.ReadProjections = append(policy.ReadProjections, architecture.ReadProjection{
		ID:          "duplicate-projection",
		Package:     "/projection",
		DataObjects: []string{"beta.records"},
		TenantSafe:  true,
	})
	if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), `both grant package "/projection" access to data object "beta.records"`) {
		t.Fatalf("projection package aliases were accepted: %v", err)
	}
}

func TestCanonicalPolicyClassifiesTheEntireBackend(t *testing.T) {
	t.Parallel()

	output, err := runArchitecture(t, "check")
	if err == nil {
		t.Fatal("canonical check unexpectedly has no target-architecture violations")
	}
	if !strings.Contains(output, "|imports.forbidden|") {
		t.Fatalf("canonical check did not evaluate real import edges:\n%s", output)
	}
	for _, forbidden := range []string{"|packages.unclassified|", "|packages.stale|", "|external.unclassified|", "|external.stale|", "|policy.rules-overlap|"} {
		if strings.Contains(output, forbidden) {
			t.Fatalf("canonical policy contains classification failure %q:\n%s", forbidden, output)
		}
	}
}

func runArchitecture(t *testing.T, args ...string) (string, error) {
	return runArchitectureWithEnv(t, nil, args...)
}

func runArchitectureWithEnv(t *testing.T, environment map[string]string, args ...string) (string, error) {
	t.Helper()

	root := filepath.Clean(filepath.Join(packageDir(t), "..", "..", ".."))
	command := exec.Command(filepath.Join(root, "scripts", "backend-architecture.sh"), args...)
	command.Dir = root
	command.Env = os.Environ()
	for key, value := range environment {
		prefix := key + "="
		filtered := command.Env[:0]
		for _, variable := range command.Env {
			if !strings.HasPrefix(variable, prefix) {
				filtered = append(filtered, variable)
			}
		}
		command.Env = append(filtered, prefix+value)
	}
	output, err := command.CombinedOutput()
	return string(output), err
}

func fixturePath(t *testing.T, parts ...string) string {
	t.Helper()
	return filepath.Join(append([]string{packageDir(t), "testdata"}, parts...)...)
}

func packageDir(t *testing.T) string {
	t.Helper()
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("resolve test package directory")
	}
	if filepath.IsAbs(file) {
		if _, err := os.Stat(file); err == nil {
			return filepath.Dir(file)
		}
	}

	wd, err := os.Getwd()
	if err != nil {
		t.Fatalf("resolve working directory: %v", err)
	}
	moduleRoot := wd
	for {
		if _, err := os.Stat(filepath.Join(moduleRoot, "go.mod")); err == nil {
			return filepath.Join(moduleRoot, "internal", "architecture")
		}
		parent := filepath.Dir(moduleRoot)
		if parent == moduleRoot {
			t.Fatalf("resolve module root from %s", wd)
		}
		moduleRoot = parent
	}
}
