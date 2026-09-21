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

	if err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, map[string]struct{}{}, created, map[string]struct{}{}, map[string]struct{}{}); err != nil {
		t.Fatalf("candidate projection was rejected: %v", err)
	}

	err := comparePolicyStrictness(base, candidate, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{}, map[string]struct{}{})
	if err == nil || !strings.Contains(err.Error(), "owner view with kind projection was added") || !strings.Contains(err.Error(), "new tenant-safe read projection grant") {
		t.Fatalf("existing package bypassed the projection guard: %v", err)
	}
}

func TestStudentProjectionReplacementRejectsOtherChanges(t *testing.T) {
	t.Parallel()
	cases := []struct {
		name   string
		change func(base, candidate *Policy)
	}{
		{"no epoch", func(b, c *Policy) { c.PolicyEpoch = b.PolicyEpoch }},
		{"old grant retained", func(_, c *Policy) {
			c.ReadProjections[0].DataObjects = append(c.ReadProjections[0].DataObjects, "users.students")
		}},
		{"no historical grant", func(b, _ *Policy) { b.ReadProjections[0].DataObjects = []string{"users.persons"} }},
		{"incomplete targets", func(_, c *Policy) { c.ReadProjections[0].DataObjects = c.ReadProjections[0].DataObjects[:3] }},
		{"extra target", func(_, c *Policy) {
			c.ReadProjections[0].DataObjects = append(c.ReadProjections[0].DataObjects, "users.privacy_consents")
		}},
		{"removed unrelated grant", func(_, c *Policy) { c.ReadProjections[0].DataObjects = c.ReadProjections[0].DataObjects[1:] }},
		{"other existing package", func(_, c *Policy) {
			c.ReadProjections[1].DataObjects = append(c.ReadProjections[1].DataObjects, "users.privacy_consents")
		}},
		{"module", func(b, c *Policy) { b.ModulePath = "example.test/other"; c.ModulePath = b.ModulePath }},
		{"package", func(b, c *Policy) {
			b.Packages[0].Path += "other"
			c.Packages[0].Path = b.Packages[0].Path
			b.ReadProjections[0].Package = b.Packages[0].Path
			c.ReadProjections[0].Package = b.Packages[0].Path
		}},
		{"projection ID", func(_, c *Policy) { c.ReadProjections[0].ID = "other" }},
		{"owner", func(_, c *Policy) { c.Packages[0].Owner = "parent-announcement-audience" }},
		{"owner kind", func(b, c *Policy) {
			for i, o := range b.Owners {
				if o.ID == "parent-message-inbox" {
					b.Owners[i].Kind = "inbound"
					c.Owners[i].Kind = "inbound"
				}
			}
		}},
		{"production role", func(_, c *Policy) { c.Packages[0].Role = "adapter" }},
		{"internal test role", func(_, c *Policy) { c.Packages[0].InternalTestRole = "adapter-test" }},
		{"external test role", func(_, c *Policy) { c.Packages[0].ExternalTestRole = "module-behavior-test" }},
		{"unsafe base", func(b, _ *Policy) { b.ReadProjections[0].TenantSafe = false }},
		{"unsafe candidate", func(_, c *Policy) { c.ReadProjections[0].TenantSafe = false }},
		{"new target table", func(b, _ *Policy) { b.DataObjects = slices.Delete(b.DataObjects, 4, 5) }},
		{"changed target owner", func(_, c *Policy) { c.DataObjects[4].WriteOwner = "people-directory" }},
		{"wrong target owner in both", func(b, c *Policy) {
			b.DataObjects[4].WriteOwner = "people-directory"
			c.DataObjects[4].WriteOwner = "people-directory"
		}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			base, candidate := studentProjectionPolicy(t), studentProjectionPolicy(t)
			candidate.PolicyEpoch++
			replaceStudentProjection(candidate, 0)
			tc.change(base, candidate)
			err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil)
			if err == nil || !strings.Contains(err.Error(), "new tenant-safe read projection grant") {
				t.Fatalf("unapproved grant change passed projection guard: %v", err)
			}
		})
	}
}

func TestStudentProjectionReplacementLifecycle(t *testing.T) {
	t.Parallel()
	base, candidate := studentProjectionPolicy(t), studentProjectionPolicy(t)
	candidate.PolicyEpoch++
	for i := range candidate.ReadProjections {
		replaceStudentProjection(candidate, i)
	}
	if err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	if err := comparePolicyStrictness(candidate, candidate, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
	replay := studentProjectionPolicy(t)
	replay.PolicyEpoch = candidate.PolicyEpoch + 1
	if err := comparePolicyStrictness(candidate, replay, nil, nil, nil, nil, nil); err == nil {
		t.Fatal("restoring compatibility grants after cutover was accepted")
	}
	// #3427 may remove the third projection before this cutover.
	base.Packages = base.Packages[:2]
	base.ReadProjections = base.ReadProjections[:2]
	candidate.Packages = candidate.Packages[:2]
	candidate.ReadProjections = candidate.ReadProjections[:2]
	if err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCheckStudentProjectionReplacementAgainstGitBase(t *testing.T) {
	t.Parallel()
	repo := t.TempDir()
	base := studentProjectionPolicy(t)
	writeFile(t, filepath.Join(repo, "go.mod"), "module github.com/moto-nrw/project-phoenix\n\ngo 1.25\n")
	for _, pkg := range base.Packages {
		writeFile(t, filepath.Join(repo, pkg.Path, "projection.go"), "package projection\n")
	}
	writeStudentProjectionPolicy(t, repo, base)
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), "")
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "architecture-test@example.test")
	runGit(t, repo, "config", "user.name", "Architecture Test")
	runGit(t, repo, "config", "commit.gpgsign", "false")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "historical student grants")
	baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	candidate := studentProjectionPolicy(t)
	candidate.PolicyEpoch++
	for i := range candidate.ReadProjections {
		replaceStudentProjection(candidate, i)
	}
	writeStudentProjectionPolicy(t, repo, candidate)
	if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
		t.Fatalf("replacement rejected: %v\n%s", err, output)
	}
	candidate.ReadProjections[0].DataObjects = append(candidate.ReadProjections[0].DataObjects, "users.privacy_consents")
	writeStudentProjectionPolicy(t, repo, candidate)
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "new tenant-safe read projection grant") {
		t.Fatalf("extra grant accepted: %v\n%s", err, output)
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

func TestStudentProjectionReplacement(t *testing.T) {
	t.Parallel()
	for _, index := range []int{0, 1, 2} {
		t.Run([]string{"inbox", "audience", "care-exit"}[index], func(t *testing.T) {
			base := studentProjectionPolicy(t)
			candidate := studentProjectionPolicy(t)
			candidate.PolicyEpoch++
			replaceStudentProjection(candidate, index)
			if err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil); err != nil {
				t.Fatalf("reviewed student projection replacement rejected: %v", err)
			}
		})
	}
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

func replaceStudentProjection(p *Policy, index int) {
	p.ReadProjections[index].DataObjects = []string{
		"users.persons", "users.student_profiles", "users.student_school_memberships", "users.student_care_profiles",
	}
}
