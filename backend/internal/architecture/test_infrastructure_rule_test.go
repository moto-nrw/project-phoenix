package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

// testInfrastructureFixture returns the vertical fixture with database/sql
// classified as orm-sql and the source package's internal test importing it, so
// the import is a real edge in the fixed build context (ADR 0014, #3215).
func testInfrastructureFixture(t *testing.T, class string) string {
	t.Helper()
	return mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
		document["external_classes"] = []any{"orm-sql", "test", "utility"}
		document["external_packages"] = []any{
			map[string]any{"path": "database/sql", "class": class},
			map[string]any{"path": "testing", "class": "test"},
		}
		// Mirror the real policy: every test scope may use the test framework.
		document["rules"] = []any{map[string]any{
			"id": "external.test", "description": "test",
			"scopes": []any{"internal_test", "external_test"}, "target_class": "test",
		}}
	})
}

func writeSourceInternalSQLTest(t *testing.T, repo string) {
	t.Helper()
	writeFile(t, filepath.Join(repo, "source", "source_internal_test.go"),
		"package source\n\nimport (\n\t\"database/sql\"\n\t\"testing\"\n)\n\nfunc TestSQL(t *testing.T) { _ = sql.ErrNoRows }\n")
}

// testInfrastructureBaseline lists the fixture's forbidden edges in canonical
// key order: the internal-test import of database/sql sorts before the
// production edge the vertical fixture already carries.
func testInfrastructureBaseline() string {
	return "{\"scope\":\"internal_test\",\"rule\":\"imports.forbidden\",\"source\":\"example.test/architecture-fixture/source\",\"target\":\"database/sql\",\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/2583\"}\n" +
		legacyRecord(2583)
}

func TestCheckReviewedTestInfrastructureRule(t *testing.T) {
	t.Parallel()
	for _, reviewed := range []bool{false, true} {
		name := "unchanged epoch"
		if reviewed {
			name = "reviewed epoch"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			basePolicy := testInfrastructureFixture(t, "orm-sql")
			repo, _ := ratchetRepositoryWithPolicy(t, testInfrastructureBaseline(), basePolicy)
			writeSourceInternalSQLTest(t, repo)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "import")
			baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

			policy := mutatePolicy(t, basePolicy, func(document map[string]any) {
				if reviewed {
					document["policy_epoch"] = document["policy_epoch"].(float64) + 1
				}
				document["rules"] = append(document["rules"].([]any), map[string]any{
					"id": "external.orm-sql.module-internal-test", "description": "test",
					"scopes": []any{"internal_test"}, "source_role": "module-internal-test", "target_class": "orm-sql",
				})
			})
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
			writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2583))
			runGit(t, repo, "add", ".")
			output, err := runRepositoryCheck(t, repo, baseRef)
			if reviewed {
				if err != nil {
					t.Fatalf("reviewed test-infrastructure rule rejected: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(output, "rule external.orm-sql.module-internal-test newly allows internal_test") {
				t.Fatalf("unreviewed test-infrastructure rule was not rejected: %v\n%s", err, output)
			}
		})
	}
}

func TestCheckReviewedTestInfrastructureRulePreservesGuards(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name  string
		class string
		rule  map[string]any
		want  string
	}{
		{
			name:  "owner-specific grant",
			class: "orm-sql",
			rule: map[string]any{"id": "module.internal-test.orm", "description": "test", "scopes": []any{"internal_test"},
				"source_owner": "module", "source_role": "module-internal-test", "target_class": "orm-sql"},
			want: "rule module.internal-test.orm newly allows internal_test",
		},
		{
			name:  "production scope on a test role",
			class: "orm-sql",
			rule: map[string]any{"id": "external.orm-sql.module-internal-test", "description": "test", "scopes": []any{"production", "internal_test"},
				"source_role": "module-internal-test", "target_class": "orm-sql"},
			want: "newly allows production",
		},
		{
			name:  "non-infrastructure class",
			class: "utility",
			rule: map[string]any{"id": "external.utility.module-internal-test", "description": "test", "scopes": []any{"internal_test"},
				"source_role": "module-internal-test", "target_class": "utility"},
			want: "rule external.utility.module-internal-test newly allows internal_test",
		},
		{
			name:  "first-party target",
			class: "orm-sql",
			rule: map[string]any{"id": "external.first-party.module-internal-test", "description": "test", "scopes": []any{"internal_test"},
				"source_role": "module-internal-test", "target_owner": "module", "target_role": "domain"},
			want: "rule external.first-party.module-internal-test newly allows internal_test",
		},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			basePolicy := testInfrastructureFixture(t, scenario.class)
			repo, _ := ratchetRepositoryWithPolicy(t, testInfrastructureBaseline(), basePolicy)
			writeSourceInternalSQLTest(t, repo)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "import")
			baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

			policy := mutatePolicy(t, basePolicy, func(document map[string]any) {
				document["policy_epoch"] = document["policy_epoch"].(float64) + 1
				document["rules"] = append(document["rules"].([]any), scenario.rule)
			})
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
			runGit(t, repo, "add", ".")
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, scenario.want) {
				t.Fatalf("reviewed epoch admitted %q: %v\n%s", scenario.name, err, output)
			}
		})
	}
}

func TestReviewedTestInfrastructureRuleShape(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		rule Rule
		want bool
	}{
		{"adapter-test orm-sql", Rule{SourceRole: "adapter-test", TargetClass: "orm-sql", Scopes: []string{"internal_test", "external_test"}}, true},
		{"e2e-test http-router", Rule{SourceRole: "e2e-test", TargetClass: "http-router", Scopes: []string{"external_test"}}, true},
		{"test-support test in production", Rule{SourceRole: "test-support", TargetClass: "test", Scopes: []string{"production"}}, true},
		{"test role in production", Rule{SourceRole: "adapter-test", TargetClass: "orm-sql", Scopes: []string{"production"}}, false},
		{"production role", Rule{SourceRole: "application", TargetClass: "orm-sql", Scopes: []string{"production"}}, false},
		{"owner-specific", Rule{SourceOwner: "module", SourceRole: "adapter-test", TargetClass: "orm-sql", Scopes: []string{"internal_test"}}, false},
		{"owner-kind", Rule{SourceOwnerKind: "domain", SourceRole: "adapter-test", TargetClass: "orm-sql", Scopes: []string{"internal_test"}}, false},
		{"other class", Rule{SourceRole: "adapter-test", TargetClass: "crypto", Scopes: []string{"internal_test"}}, false},
		{"first-party target", Rule{SourceRole: "adapter-test", TargetOwner: "module", TargetRole: "domain", Scopes: []string{"internal_test"}}, false},
		{"same owner", Rule{SourceRole: "adapter-test", TargetClass: "orm-sql", SameOwner: true, Scopes: []string{"internal_test"}}, false},
		{"missing role", Rule{TargetClass: "orm-sql", Scopes: []string{"internal_test"}}, false},
	} {
		if got := reviewedTestInfrastructureRule(scenario.rule); got != scenario.want {
			t.Errorf("%s: reviewedTestInfrastructureRule = %v, want %v", scenario.name, got, scenario.want)
		}
	}
}
