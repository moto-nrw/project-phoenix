package architecture_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
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

func expectedSemanticViolationKeys() []string {
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
		"production|contracts.orm-type|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.repository-type|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.filter-map|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.internal-model|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.orm-tag|example.test/architecture-semantic/public|public.Leaky",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.Service.List",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.Service.Get",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.Service.Upsert",
		"production|contracts.generic-crud|example.test/architecture-semantic/public|public.New.List",
		"production|database.direct-access|example.test/architecture-semantic/service|github.com/uptrace/bun." + "DB.NewSelect",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.WrappedDB.NewSelect",
		"production|database.direct-access|example.test/architecture-semantic/service|github.com/uptrace/bun." + "DB.Begin",
		"production|database.direct-access|example.test/architecture-semantic/service|database/sql.Conn.ExecContext",
		"production|database.direct-access|example.test/architecture-semantic/service|database/sql.Stmt.Exec",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.SelectDB.NewSelect",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.SQLDB.ExecContext",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.PingDB.Ping",
		"production|database.direct-access|example.test/architecture-semantic/service|example.test/architecture-semantic/service.PingDB.PingContext",
		"production|tables.foreign-read|example.test/architecture-semantic/service|beta.fragment_records",
		"production|tables.foreign-read|example.test/architecture-semantic/service|beta.records",
		"production|tables.unresolved|example.test/architecture-semantic/service|service.Exec",
		"production|composition.legacy-reference|example.test/architecture-semantic/consumer|example.test/architecture-semantic/legacy.Factory",
		"production|composition.legacy-reference|example.test/architecture-semantic/consumer|example.test/architecture-semantic/legacy.NewFactory",
	}
}

func TestPolicyRejectsLegacyReferencesOutsideCompositionPackages(t *testing.T) {
	t.Parallel()

	policy, err := architecture.LoadPolicy(fixturePath(t, "semantic", "invalid", "policy.json"))
	if err != nil {
		t.Fatalf("load policy: %v", err)
	}
	for i := range policy.Packages {
		if policy.Packages[i].Path == "legacy" {
			policy.Packages[i].Owner = "alpha"
			break
		}
	}
	if err := policy.Validate(); err == nil || !strings.Contains(err.Error(), `legacy composition package "legacy" must have a composition owner and compose role`) {
		t.Fatalf("legacy package outside composition was accepted: %v", err)
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
	t.Helper()

	root := filepath.Clean(filepath.Join(packageDir(t), "..", "..", ".."))
	command := exec.Command(filepath.Join(root, "scripts", "backend-architecture.sh"), args...)
	command.Dir = root
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
	if ok && filepath.IsAbs(file) {
		return filepath.Dir(file)
	}
	dir, err := os.Getwd()
	if err != nil {
		t.Fatal("resolve test package directory")
	}
	for current := dir; ; current = filepath.Dir(current) {
		if testPackageDir(current) {
			return current
		}
		candidate := filepath.Join(current, "backend", "internal", "architecture")
		if testPackageDir(candidate) {
			return candidate
		}
		if parent := filepath.Dir(current); parent == current {
			break
		}
	}
	t.Fatal("resolve test package directory")
	return ""
}

func testPackageDir(dir string) bool {
	info, err := os.Stat(filepath.Join(dir, "testdata"))
	return err == nil && info.IsDir()
}
