package architecture

import (
	"encoding/json"
	"slices"
	"strings"
	"testing"
)

func toolingSchema4(t *testing.T) string {
	t.Helper()
	return mutatePolicy(t, readMaterializedPolicy(t, "../../architecture/policy.json"), func(doc map[string]any) {
		doc["schema_version"] = 4
		for _, value := range doc["owners"].([]any) {
			owner := value.(map[string]any)
			if owner["kind"] == "tool" {
				owner["kind"] = "inbound"
			}
		}
		for _, value := range doc["rules"].([]any) {
			rule := value.(map[string]any)
			if rule["source_owner_kind"] == "tool" {
				rule["source_owner_kind"] = "inbound"
				rule["id"] = "own-test.inbound.contract"
			}
		}
	})
}

func TestToolSchemaImmutableBaseUsesOrdinaryStrictness(t *testing.T) {
	t.Parallel()
	base, err := decodePolicy(strings.NewReader(toolingSchema4(t)), true)
	if err != nil {
		t.Fatal(err)
	}
	candidate := repositoryPolicy(t)
	if base.SchemaVersion != 5 || !slices.Equal(base.Owners, candidate.Owners) || !slices.Equal(base.Packages, candidate.Packages) {
		t.Fatal("wire upgrade changed owner identities, package ownership or roles")
	}
	if err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("same tooling permissions across schemas: %v", err)
	}
	for _, change := range []string{"owner kind", "package owner", "production permission"} {
		t.Run(change, func(t *testing.T) {
			t.Parallel()
			widened := repositoryPolicy(t)
			switch change {
			case "owner kind":
				for i := range widened.Owners {
					if widened.Owners[i].ID == "dev-tools" {
						widened.Owners[i].Kind = "workflow"
					}
				}
			case "package owner":
				for i := range widened.Packages {
					if widened.Packages[i].Path == "cmd/devtools" {
						widened.Packages[i].Owner = rootCompositionOwner
					}
				}
			case "production permission":
				for i := range widened.Rules {
					if widened.Rules[i].ID == "own-test.tool.contract" {
						widened.Rules[i].Scopes = append(slices.Clone(widened.Rules[i].Scopes), "production")
					}
				}
			}
			if err := comparePolicyStrictness(base, widened, nil, nil, nil, nil, nil); err == nil {
				t.Fatalf("schema upgrade bypassed %s strictness", change)
			}
		})
	}
}

func TestToolSchemaHasNoLiveInboundAlias(t *testing.T) {
	t.Parallel()
	old := toolingSchema4(t)
	if _, err := DecodePolicy(strings.NewReader(old)); err == nil {
		t.Fatal("live schema-4 candidate accepted")
	}
	for _, field := range []string{"owner", "source selector", "target selector"} {
		t.Run(field, func(t *testing.T) {
			t.Parallel()
			p := repositoryPolicy(t)
			switch field {
			case "owner":
				p.Owners = append(p.Owners, Owner{ID: "inbound-other", Kind: "inbound"})
			case "source selector":
				p.Rules = []Rule{{ID: "test.kind", Description: "test", Scopes: []string{"internal_test"}, SourceOwnerKind: "inbound", TargetOwner: "dev-tools", TargetRole: "contract"}}
			case "target selector":
				p.Rules = []Rule{{ID: "test.kind", Description: "test", Scopes: []string{"internal_test"}, SourceOwner: "dev-tools", TargetOwnerKind: "inbound", TargetRole: "contract"}}
			}
			if err := p.Validate(); err == nil {
				t.Fatalf("live inbound %s accepted", field)
			}
		})
	}
	p := repositoryPolicy(t)
	for i := range p.Rules {
		p.Rules[i].SourcePackage, p.Rules[i].TargetPackage = "", ""
	}
	current, err := json.Marshal(p)
	if err != nil {
		t.Fatal(err)
	}
	for _, version := range []string{"2", "3", "4"} {
		document := strings.Replace(string(current), `"schema_version":5`, `"schema_version":`+version, 1)
		if _, err := decodePolicy(strings.NewReader(document), true); err == nil || !strings.Contains(err.Error(), `invalid kind "tool"`) {
			t.Fatalf("schema %s tool-kind error: %v", version, err)
		}
	}
}

func TestHistoricalHTTPInboundNeverBecomesATool(t *testing.T) {
	t.Parallel()
	document := mutatePolicy(t, toolingSchema4(t), func(doc map[string]any) {
		doc["owners"] = append(doc["owners"].([]any), map[string]any{"id": "inbound-http", "kind": "inbound"})
	})
	base, err := decodePolicy(strings.NewReader(document), true)
	if err != nil {
		t.Fatal(err)
	}
	if base.SchemaVersion != 4 {
		t.Fatal("historical HTTP inventory crossed the tooling wire upgrade")
	}
	for _, owner := range base.Owners {
		if owner.Kind == "tool" {
			t.Fatal("historical HTTP or tooling owner was silently reclassified")
		}
	}
	if err := comparePolicyStrictness(base, repositoryPolicy(t), nil, nil, nil, nil, nil); err == nil {
		t.Fatal("old HTTP inventory bypassed ordinary ownership guards")
	}
}

func TestToolsHaveNoWritableDataOrDomainGraphNode(t *testing.T) {
	t.Parallel()
	p := repositoryPolicy(t)
	for _, node := range TargetProjection(p).Nodes {
		if node.Kind == "tool" {
			t.Fatalf("engineering tool entered the domain dependency graph: %s", node.ID)
		}
	}
	p.DataObjects = append(p.DataObjects, DataObject{Name: "test.records", WriteOwner: "dev-tools"})
	if err := p.Validate(); err == nil || !strings.Contains(err.Error(), "non-owning kind") {
		t.Fatalf("tool became a writable-data owner: %v", err)
	}
}

func TestImmutableBaseKeepsARoleTheCandidateRetired(t *testing.T) {
	t.Parallel()
	document := mutatePolicy(t, readMaterializedPolicy(t, "../../architecture/policy.json"), func(doc map[string]any) {
		doc["roles"] = append(doc["roles"].([]any), "retired-role")
		doc["packages"] = append(doc["packages"].([]any), map[string]any{
			"path": "retired", "owner": "legacy-composition", "role": "retired-role",
			"internal_test_role": "module-internal-test", "external_test_role": "module-behavior-test",
		})
	})
	if _, err := DecodePolicy(strings.NewReader(document)); err == nil || !strings.Contains(err.Error(), `invalid role "retired-role"`) {
		t.Fatalf("candidate kept a role the checker does not know: %v", err)
	}
	base, err := decodePolicy(strings.NewReader(document), true)
	if err != nil {
		t.Fatalf("base declaring a retired role no longer loads: %v", err)
	}
	if err := comparePolicyStrictness(base, repositoryPolicy(t), nil, nil, nil, nil, nil); err != nil {
		t.Fatalf("retiring a role and its package counted as a loosening: %v", err)
	}
}
