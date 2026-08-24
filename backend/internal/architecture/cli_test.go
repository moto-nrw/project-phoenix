package architecture_test

import (
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
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
	if !strings.Contains(output, "schema_version must be 1, got 2") {
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
		{name: "external class", policy: "invalid-external-class.json", message: `external package "testing" has unknown class "unknown"`},
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
	if !ok {
		t.Fatal("resolve test package directory")
	}
	return filepath.Dir(file)
}
