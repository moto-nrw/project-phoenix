package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckReviewedWorkflowRegistration(t *testing.T) {
	t.Parallel()
	for _, reviewed := range []bool{false, true} {
		name := "unchanged epoch"
		if reviewed {
			name = "reviewed epoch"
		}
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := ratchetRepository(t, legacyRecord(2583))
			policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
				if reviewed {
					document["policy_epoch"] = document["policy_epoch"].(float64) + 1
				}
				document["roles"] = append(document["roles"].([]any), "public", "workflow-decision-test", "workflow-integration-test")
				document["owners"] = append(document["owners"].([]any), map[string]any{"id": "offboarding", "kind": "workflow"})
				document["packages"] = append(document["packages"].([]any), map[string]any{
					"path": "offboarding", "owner": "offboarding", "role": "public",
					"internal_test_role": "workflow-decision-test", "external_test_role": "workflow-integration-test",
				})
			})
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
			writeFile(t, filepath.Join(repo, "offboarding", "offboarding.go"), "package offboarding\n")
			runGit(t, repo, "add", ".")
			output, err := runRepositoryCheck(t, repo, baseRef)
			if reviewed {
				if err != nil {
					t.Fatalf("reviewed candidate-only workflow rejected: %v\n%s", err, output)
				}
			} else if err == nil || !strings.Contains(output, "owner offboarding with kind workflow was added") {
				t.Fatalf("unreviewed workflow was not rejected: %v\n%s", err, output)
			}
		})
	}
}

func TestCheckReviewedWorkflowPreservesExistingGuards(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		want string
	}{
		{"adopt package", "owner offboarding with kind workflow was added"},
		{"own data", "non-owning kind"},
		{"expand existing imports", "newly allows production module/application -> module/domain"},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef := ratchetRepository(t, legacyRecord(2583))
			policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
				document["policy_epoch"] = document["policy_epoch"].(float64) + 1
				document["roles"] = append(document["roles"].([]any), "public", "workflow-decision-test", "workflow-integration-test")
				document["owners"] = append(document["owners"].([]any), map[string]any{"id": "offboarding", "kind": "workflow"})
				document["packages"] = append(document["packages"].([]any), map[string]any{
					"path": "offboarding", "owner": "offboarding", "role": "public",
					"internal_test_role": "workflow-decision-test", "external_test_role": "workflow-integration-test",
				})
				switch scenario.name {
				case "adopt package":
					for _, item := range document["packages"].([]any) {
						pkg := item.(map[string]any)
						if pkg["path"] == "source" {
							pkg["owner"] = "offboarding"
						}
					}
				case "own data":
					document["data_objects"] = []any{map[string]any{"name": "staff.records", "write_owner": "offboarding"}}
				case "expand existing imports":
					document["rules"] = append(document["rules"].([]any), map[string]any{
						"id": "expanded", "description": "Forbidden existing permission expansion.", "scopes": []string{"production"},
						"source_owner": "module", "source_role": "application", "target_owner": "module", "target_role": "domain",
					})
				}
			})
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
			writeFile(t, filepath.Join(repo, "offboarding", "offboarding.go"), "package offboarding\n")
			runGit(t, repo, "add", ".")
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, scenario.want) {
				t.Fatalf("existing guard was not preserved: %v\n%s", err, output)
			}
		})
	}
}
