package architecture

import (
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCanonicalPolicyOwnerLists(t *testing.T) {
	t.Parallel()
	policyPath := filepath.Join("..", "..", "architecture", "policy.json")
	policy, err := LoadPolicy(policyPath)
	if err != nil {
		t.Fatal(err)
	}
	slices.Reverse(policy.Owners)
	if err := policy.Validate(); err != nil {
		t.Fatalf("owner declaration order must not matter: %v", err)
	}
	for _, owner := range policy.Owners {
		if owner.Kind != "domain" && owner.Kind != "platform" {
			continue
		}
		for _, change := range []string{"remove", "rename", "reclassify"} {
			t.Run(owner.ID+"/"+change, func(t *testing.T) {
				t.Parallel()
				candidate, err := LoadPolicy(policyPath)
				if err != nil {
					t.Fatal(err)
				}
				index := slices.IndexFunc(candidate.Owners, func(item Owner) bool { return item.ID == owner.ID })
				switch change {
				case "remove":
					candidate.Owners = slices.Delete(candidate.Owners, index, index+1)
				case "rename":
					candidate.Owners[index].ID += "-renamed"
				case "reclassify":
					candidate.Owners[index].Kind = "workflow"
				}
				err = candidate.Validate()
				if err == nil || !strings.Contains(err.Error(), "canonical "+owner.Kind+" owners") {
					t.Fatalf("expected canonical %s owner rejection, got %v", owner.Kind, err)
				}
			})
		}
	}
	for _, kind := range []string{"domain", "platform"} {
		t.Run("additional-"+kind, func(t *testing.T) {
			t.Parallel()
			candidate, err := LoadPolicy(policyPath)
			if err != nil {
				t.Fatal(err)
			}
			candidate.Owners = append(candidate.Owners, Owner{ID: "unapproved-owner", Kind: kind})
			err = candidate.Validate()
			if err == nil || !strings.Contains(err.Error(), "canonical "+kind+" owners") {
				t.Fatalf("expected additional %s owner rejection, got %v", kind, err)
			}
		})
	}
}
