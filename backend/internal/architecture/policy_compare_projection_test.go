package architecture

import (
	"encoding/json"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

func TestCandidateProjectionRequiresNewPackage(t *testing.T) {
	t.Parallel()

	base := &Policy{ModulePath: "example.test/project", Owners: []Owner{{ID: "module", Kind: "domain"}}}
	candidate := &Policy{
		ModulePath: "example.test/project",
		Owners:     []Owner{{ID: "module", Kind: "domain"}, {ID: "view", Kind: "projection"}},
		Packages: []Package{{
			Path: "projection", Owner: "view", Role: "postgres",
			InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test",
		}},
		ReadProjections: []ReadProjection{{
			ID: "view", Package: "projection", DataObjects: []string{"module.records"}, TenantSafe: true,
		}},
	}
	created := map[string]struct{}{"example.test/project/projection": {}}

	if err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, created, map[string]struct{}{}, map[string]struct{}{}, nil); err != nil {
		t.Fatalf("candidate projection was rejected: %v", err)
	}

	err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, nil)
	if err == nil || !strings.Contains(err.Error(), "owner view with kind projection was added") || !strings.Contains(err.Error(), "new tenant-safe read projection grant") {
		t.Fatalf("existing package bypassed the projection guard: %v", err)
	}
}

func writeStudentProjectionPolicy(t *testing.T, repo string, policy *Policy) {
	t.Helper()
	if err := policy.Validate(); err != nil {
		t.Fatal(err)
	}
	encoded, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), string(encoded))
}

// Keep the historical grant explicit: this fixture must not follow the
// repository policy's cutover and thereby lose the migration's base evidence.
func studentProjectionPolicy(t *testing.T) *Policy {
	t.Helper()
	p, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	p.PolicyEpoch = 15
	// #3427 retired the care-exit projection owner from the repository policy.
	if !slices.ContainsFunc(p.Owners, func(owner Owner) bool { return owner.ID == "care-exit-view" }) {
		p.Owners = append(p.Owners, Owner{ID: "care-exit-view", Kind: "projection"})
	}
	p.Packages = nil
	p.ReadProjections = nil
	p.Rules = []Rule{}
	p.LegacyComposition = []LegacyReference{}
	p.Relocations = nil
	p.ExternalPackages = []ExternalPackage{}
	p.DataObjects = []DataObject{
		{Name: "users.students", WriteOwner: "people-directory"},
		{Name: "users.persons", WriteOwner: "people-directory"},
		{Name: "users.student_profiles", WriteOwner: "people-directory"},
		{Name: "users.student_school_memberships", WriteOwner: "school-membership"},
		{Name: "users.student_care_profiles", WriteOwner: "care-plan"},
		{Name: "users.privacy_consents", WriteOwner: "student-presence"},
	}
	for _, tuple := range []struct{ id, path string }{
		{"parent-message-inbox", "modules/communication/internal/adapters/parentinbox"},
		{"parent-announcement-audience", "modules/communication/internal/adapters/parentaudience"},
		{"care-exit-view", "modules/careplan/legacy/careexitview"},
	} {
		p.Packages = append(p.Packages, Package{Path: tuple.path, Owner: tuple.id, Role: "postgres",
			InternalTestRole: "module-internal-test", ExternalTestRole: "adapter-test"})
		p.ReadProjections = append(p.ReadProjections, ReadProjection{ID: tuple.id, Package: tuple.path,
			DataObjects: []string{"users.persons", "users.students"}, TenantSafe: true})
	}
	return p
}
