package architecture

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

func TestCommandEffectsDomainWorkflowDirectionCannotUseOrdinaryLabels(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ effect, classification, adr string }{
		{"mutation", "leaf", "0067"},
		{"lock", "leaf", "0067"},
		{"query", "query", ""},
		{"query", "leaf", "0067"},
		{"mutation", "workflow", "0067"},
	} {
		t.Run(tc.effect+"/"+tc.classification, func(t *testing.T) {
			t.Parallel()
			project, policy := workflowDirectionFixture(t)
			reviewGapFixture(t, project, policy, tc.effect, tc.classification, tc.adr)
			err := RunCLI([]string{"check", "--project", project, "--baseline", "architecture/legacy.jsonl"}, CLIDependencies{})
			if err == nil || !strings.Contains(err.Error(), "domain-to-workflow") {
				t.Fatalf("CLI check = %v, want domain-to-workflow rejection", err)
			}
		})
	}
}

func TestCommandEffectsDomainWorkflowExceptionsNeedFreshApproval(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ effect, classification, adr string }{
		{"query", "debt", "0083"}, {"mutation", "debt", "0083"}, {"lock", "debt", "0083"}, {"query", "E2", "0067"},
	} {
		t.Run(tc.effect+"/"+tc.classification, func(t *testing.T) {
			t.Parallel()
			project, policy := workflowDirectionFixture(t)
			base := commitGapFixture(t, project)
			reviewGapFixture(t, project, policy, tc.effect, tc.classification, tc.adr)
			requireGapBaseFailure(t, project, base, "new exception needs approval_adr")
			manifest, err := loadCommandEffects(project)
			if err != nil {
				t.Fatal(err)
			}
			for i := range manifest.Entries {
				manifest.Entries[i].ApprovalADR = "0094"
			}
			blob, err := marshalCommandEffects(manifest)
			if err != nil {
				t.Fatal(err)
			}
			if err := artifacts.WriteInventory(filepath.Join(project, commandEffectInventory), artifacts.CommandEffects, blob); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(project, "docs", "adr", "0094-fixture.md"), "# Reviewed runtime-direction debt\n")
			if err := RunCLI([]string{"check", "--project", project, "--baseline", "architecture/legacy.jsonl", "--base-ref", base}, CLIDependencies{}); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func workflowDirectionFixture(t *testing.T) (string, *Policy) {
	t.Helper()
	project, policy := commandEffectGateFixture(t)
	for i := range policy.Owners {
		if policy.Owners[i].ID == "foreign" {
			policy.Owners[i].Kind = "workflow"
		}
	}
	blob, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(project, "architecture", "policy.json"), string(blob))
	return project, policy
}

func TestCommandEffectsHTTPWorkflowEntryIsNotDomainCallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ effect, classification, adr string }{
		{"query", "query", ""}, {"mutation", "workflow", "0067"},
	} {
		t.Run(tc.effect, func(t *testing.T) {
			t.Parallel()
			project, policy := workflowDirectionFixture(t)
			policy.Roles = append(policy.Roles, "http")
			for i := range policy.Packages {
				if policy.Packages[i].Owner == "enrollment" {
					policy.Packages[i].Role = "http"
				}
			}
			for i := range policy.Rules {
				if policy.Rules[i].TargetOwner == "enrollment" {
					policy.Rules[i].TargetRole = "http"
				}
			}
			blob, err := json.Marshal(policy)
			if err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(project, "architecture", "policy.json"), string(blob))
			reviewGapFixture(t, project, policy, tc.effect, tc.classification, tc.adr)
			if err := RunCLI([]string{"check", "--project", project, "--baseline", "architecture/legacy.jsonl"}, CLIDependencies{}); err != nil {
				t.Fatalf("HTTP workflow entry rejected as domain callback: %v", err)
			}
		})
	}
}
