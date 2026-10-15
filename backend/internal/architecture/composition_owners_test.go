package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

func ownerFixturePolicy(owners map[string]string) *Policy {
	policy := &Policy{Owners: []Owner{
		{ID: rootCompositionOwner, Kind: "composition"},
		{ID: "alpha", Kind: "domain"},
		{ID: "beta", Kind: "domain"},
		{ID: "shared-kernel", Kind: "kernel"},
	}}
	for _, path := range []string{"api", "cmd", "modules/alpha", "modules/beta", "modules/gamma", "sharedkernel"} {
		owner := owners[path]
		if owner == "" {
			continue
		}
		policy.Packages = append(policy.Packages, Package{Path: path, Owner: owner, Role: "compose"})
	}
	return policy
}

var compositionOwnerClassification = map[string]string{
	"api": rootCompositionOwner, "cmd": rootCompositionOwner,
	"modules/alpha": "alpha", "modules/beta": "beta", "modules/gamma": "beta",
	"sharedkernel": "shared-kernel",
}

const compositionOwnerImports = "package api\nimport (\n" +
	"alpha \"example.test/architecture-fixture/modules/alpha\"\n" +
	"beta \"example.test/architecture-fixture/modules/beta\"\n" +
	"gamma \"example.test/architecture-fixture/modules/gamma\"\n" +
	"kernel \"example.test/architecture-fixture/sharedkernel\"\n" +
	"other \"example.test/architecture-fixture/modules/other\"\n" +
	"\"sync/atomic\"\n)\n" +
	"var _ = []any{alpha.Service{}, beta.Service{}, gamma.Service{}, kernel.Date{}, other.Service{}, atomic.Bool{}}\n"

func TestCompositionSurfaceCountsRootStructsOfSeveralOwners(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, base, candidate, want string }{
		{"two owners", "type Wiring struct { A *alpha.Service; B beta.Port }", "type Wiring struct { A *alpha.Service; B beta.Port; New int }", "production|field|api|Wiring.New"},
		{"one owner", "type Wiring struct { A *alpha.Service }", "type Wiring struct { A *alpha.Service; B alpha.Port }", ""},
		{"same owner in two packages", "type Wiring struct { B beta.Port }", "type Wiring struct { B beta.Port; G gamma.Port }", ""},
		{"kernel value is shared vocabulary", "type Wiring struct { A alpha.Port }", "type Wiring struct { A alpha.Port; Day kernel.Date }", ""},
		{"scalar configuration", "type Wiring struct { A alpha.Port }", "type Wiring struct { A alpha.Port; URL string }", ""},
		{"second owner makes the whole struct count", "type Wiring struct { A alpha.Port }", "type Wiring struct { A alpha.Port; B beta.Port }", "production|field|api|Wiring.A"},
		{"containers and type arguments", "type Wiring struct { A []alpha.Port; Slot atomic.Pointer[alpha.Service] }", "type Wiring struct { A []alpha.Port; Slot atomic.Pointer[beta.Service] }", "production|field|api|Wiring.Slot"},
		{"map value", "type Wiring struct { A alpha.Port }", "type Wiring struct { A alpha.Port; B map[string]*beta.Service }", "production|field|api|Wiring.B"},
		{"root struct is followed", "type Inner struct { B beta.Port }; type Wiring struct { A alpha.Port }", "type Inner struct { B beta.Port }; type Wiring struct { A alpha.Port; Inner *Inner }", "production|field|api|Wiring.Inner"},
		{"embedded root struct is followed", "type Inner struct { B beta.Port }; type Wiring struct { A alpha.Port }", "type Inner struct { B beta.Port }; type Wiring struct { A alpha.Port; Inner }", "production|field|api|Wiring.Inner"},
		{"root alias is followed", "type Port = beta.Port; type Wiring struct { A alpha.Port }", "type Port = beta.Port; type Wiring struct { A alpha.Port; B Port }", "production|field|api|Wiring.B"},
		{"root port holds no owner value", "type Port interface { Load() beta.Service }; type Wiring struct { A alpha.Port }", "type Port interface { Load() beta.Service }; type Wiring struct { A alpha.Port; B Port }", ""},
		{"function literal holds no owner value", "type Wiring struct { A alpha.Port }", "type Wiring struct { A alpha.Port; Load func() beta.Service }", ""},
		{"unclassified package is an owner", "type Wiring struct { A alpha.Port }", "type Wiring struct { A alpha.Port; O other.Service }", "production|field|api|Wiring.O"},
		{"removed field", "type Wiring struct { A alpha.Port; B beta.Port; Old int }", "type Wiring struct { A alpha.Port; B beta.Port }", ""},
		{"new struct", "", "type Wiring struct { A alpha.Port; B beta.Port }", "production|field|api|Wiring.B"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			path := filepath.Join(repo, "api", "wiring.go")
			writeFile(t, path, compositionOwnerImports+tt.base+"\n")
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "composition base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, path, compositionOwnerImports+tt.candidate+"\n")
			policy := ownerFixturePolicy(compositionOwnerClassification)
			err := CompareCompositionSurface(repo, base, policy, policy)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

func TestCompositionSurfaceRootStructsKeepTheirScope(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	writeFile(t, filepath.Join(repo, "cmd", "root.go"), "package cmd\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "composition base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	source := strings.Replace(compositionOwnerImports, "package api", "package cmd_test", 1) + "type Wiring struct { A alpha.Port; B beta.Port }\n"
	writeFile(t, filepath.Join(repo, "cmd", "root_test.go"), source)
	policy := ownerFixturePolicy(compositionOwnerClassification)
	err := CompareCompositionSurface(repo, base, policy)
	if err == nil || !strings.Contains(err.Error(), "external_test|field|cmd|Wiring.A") {
		t.Fatalf("an external test struct of the cmd root must count: %v", err)
	}
}

func TestCompositionSurfaceReclassificationCannotHideRootGrowth(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	path := filepath.Join(repo, "api", "wiring.go")
	writeFile(t, path, compositionOwnerImports+"type Wiring struct { A alpha.Port; B beta.Port }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "composition base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, compositionOwnerImports+"type Wiring struct { A alpha.Port; B beta.Port; New int }\n")
	merged := map[string]string{}
	for path, owner := range compositionOwnerClassification {
		merged[path] = owner
	}
	merged["modules/beta"] = "alpha"
	candidate := ownerFixturePolicy(merged)
	if err := CompareCompositionSurface(repo, base, candidate); err != nil {
		t.Fatalf("one owner under the candidate alone counts nothing: %v", err)
	}
	err := CompareCompositionSurface(repo, base, ownerFixturePolicy(compositionOwnerClassification), candidate)
	if err == nil || !strings.Contains(err.Error(), "production|field|api|Wiring.New") {
		t.Fatalf("a candidate reclassification hid the base owners: %v", err)
	}
}

func TestCompositionSurfaceWithoutPolicyCountsCentralAggregatesOnly(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	path := filepath.Join(repo, "api", "wiring.go")
	writeFile(t, path, compositionOwnerImports+"type Wiring struct { A alpha.Port }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "composition base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, compositionOwnerImports+"type Wiring struct { A alpha.Port; B beta.Port }\n")
	if err := CompareCompositionSurface(repo, base); err != nil {
		t.Fatalf("no policy names a root package: %v", err)
	}
}
