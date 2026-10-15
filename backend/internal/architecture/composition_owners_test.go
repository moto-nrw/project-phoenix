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

// ownerFixtureRepository is a ratchet repository with the owner packages the
// root fixtures import, so the root packages type-check.
func ownerFixtureRepository(t *testing.T) string {
	t.Helper()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	for path, source := range map[string]string{
		"modules/alpha/alpha.go": "package alpha\ntype Port interface{ Run() }\ntype Service struct{}\n",
		"modules/beta/beta.go":   "package beta\ntype Port interface{ Run() }\ntype Service struct{}\n",
		"modules/gamma/gamma.go": "package gamma\ntype Port interface{ Run() }\ntype Service struct{}\n",
		"modules/other/other.go": "package other\ntype Service struct{}\n",
		"sharedkernel/kernel.go": "package sharedkernel\ntype Date struct{}\n",
	} {
		writeFile(t, filepath.Join(repo, filepath.FromSlash(path)), source)
	}
	return repo
}

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
			repo := ownerFixtureRepository(t)
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
	repo := ownerFixtureRepository(t)
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
	repo := ownerFixtureRepository(t)
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
	repo := ownerFixtureRepository(t)
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

// Review of #4095, C3 to C5: a graph of several owners counts whatever form
// the root gives it — a generic instance, wrapped owner ports, or a struct
// without a package-level type name.
func TestCompositionSurfaceCountsEveryFormOfARootGraph(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, candidate, want string }{
		{"generic instance alias", "type Graph[A, B any] struct { A A; B B }; type Wiring = Graph[alpha.Port, beta.Port]", "production|field|api|Wiring.A"},
		{"generic instance type", "type Graph[A, B any] struct { A A; B B }; type Wiring Graph[alpha.Port, beta.Port]", "production|field|api|Wiring.B"},
		{"generic declaration alone", "type A struct { P alpha.Port; Q beta.Port }; type Graph[A, B any] struct { A A; B B }", "production|field|api|A.P"},
		{"type parameters shadow package types", "type A struct { P alpha.Port; Q beta.Port }; type Graph[A any] struct { First A; Second alpha.Port }", "!production|field|api|Graph.First"},
		{"embedded owner interfaces", "type A interface { alpha.Port }; type B interface { beta.Port }; type Wiring struct { A A; B B }", "production|field|api|Wiring.A"},
		{"embedded interface literals", "type Wiring struct { A interface { alpha.Port }; B interface { beta.Port } }", "production|field|api|Wiring.B"},
		{"port that also embeds an owner port", "type P interface { alpha.Port; Extra() }; type Wiring struct { P P; B beta.Port }", "production|field|api|Wiring.P"},
		{"anonymous package variable", "var wiring = struct { A alpha.Port; B beta.Port }{}", "production|field|api|wiring.A"},
		{"typed package variable", "var wiring struct { A alpha.Port; B beta.Port }", "production|field|api|wiring.B"},
		{"local type", "func root() { type Wiring struct { A alpha.Port; B beta.Port }; _ = Wiring{} }", "production|field|api|root.Wiring.A"},
		{"local literal", "func root() { w := &struct { A alpha.Port; B beta.Port }{}; _ = w }", "production|field|api|root.w.A"},
		{"unnamed literal", "func root() any { return struct { A alpha.Port; B beta.Port }{} }", "production|field|api|root.struct1.B"},
		{"method", "type Root struct{}; func (r *Root) build() { type Wiring struct { A alpha.Port; B beta.Port }; _ = Wiring{} }", "production|field|api|Root.build.Wiring.A"},
		{"local generic instance", "type Graph[A, B any] struct { A A; B B }; func root() { type Wiring = Graph[alpha.Port, beta.Port]; _ = Wiring{} }", "production|field|api|root.Wiring.A"},
		{"anonymous struct in a named type", "type registry map[string]struct { A alpha.Port; B beta.Port }", "production|field|api|registry.A"},
		{"single-owner local graph", "func root() { type Wiring struct { A alpha.Port; B alpha.Service }; _ = Wiring{} }", ""},
		{"function type parameter", "func root[T any](t T) { w := struct { A alpha.Port; T T }{T: t}; _ = w }", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := ownerFixtureRepository(t)
			path := filepath.Join(repo, "api", "wiring.go")
			writeFile(t, path, compositionOwnerImports)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "composition base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, path, compositionOwnerImports+tt.candidate+"\n")
			policy := ownerFixturePolicy(compositionOwnerClassification)
			err := CompareCompositionSurface(repo, base, policy, policy)
			switch {
			case tt.want == "":
				if err != nil {
					t.Fatal(err)
				}
			case strings.HasPrefix(tt.want, "!"):
				if err != nil && strings.Contains(err.Error(), tt.want[1:]) {
					t.Fatalf("unexpected %q in %v", tt.want[1:], err)
				}
			default:
				if err == nil || !strings.Contains(err.Error(), tt.want) {
					t.Fatalf("want %q, got %v", tt.want, err)
				}
			}
		})
	}
}

// Review of #4095, round 2: the inventory is semantic, so neither a direct
// generic instance, nor local aliases and wrappers, nor a repeated local name,
// nor a nested struct hides a graph, and an unrelated struct moves no key.
func TestCompositionSurfaceFollowsTheTypesOfEveryCarrier(t *testing.T) {
	t.Parallel()
	const generic = "type Graph[A, B any] struct { A A; B B }\n"
	const block = "{ type Wiring struct { A alpha.Port; B beta.Port }; _ = Wiring{} }"
	const variable = "{ w := struct { A alpha.Port; B beta.Port }{}; _ = w }"
	for _, tt := range []struct{ name, base, candidate, want string }{
		{"direct generic variable", generic, generic + "var wiring Graph[alpha.Port, beta.Port]", "production|field|api|wiring.A"},
		{"direct generic literal", generic, generic + "func root() { _ = Graph[alpha.Port, beta.Port]{} }", "production|field|api|root.struct1.B"},
		{"inferred generic instance", generic + "func newGraph[A, B any](a A, b B) Graph[A, B] { return Graph[A, B]{a, b} }",
			generic + "func newGraph[A, B any](a A, b B) Graph[A, B] { return Graph[A, B]{a, b} }\nfunc root(a alpha.Port, b beta.Port) { g := newGraph(a, b); _ = g }", "production|field|api|root.g.A"},
		{"local owner aliases", "func root() {}", "func root() { type A = alpha.Port; type B = beta.Port; type Wiring struct { A A; B B }; _ = Wiring{} }", "production|field|api|root.Wiring.A"},
		{"local wrapper types", "func root() {}", "func root() { type A struct { P alpha.Port }; type B struct { P beta.Port }; type Wiring struct { A A; B B }; _ = Wiring{} }", "production|field|api|root.Wiring.B"},
		{"local alias shadows a package type", "type A = alpha.Port\nfunc root() {}", "type A = alpha.Port\nfunc root() { type A = beta.Port; type Wiring struct { X A; Y gamma.Port }; _ = Wiring{} }", ""},
		{"ports with own methods", "func root() {}", "type A interface { alpha.Port; Extra() }; type B interface { beta.Port; Extra() }; type Wiring struct { A A; B B }", "production|field|api|Wiring.A"},
		{"same-named local types", "func root() {" + block + "}", "func root() {" + block + ";" + block + "}", "production|field|api|root.Wiring#2.A"},
		{"same-named local variables", "func root() {" + variable + "}", "func root() {" + variable + ";" + variable + "}", "production|field|api|root.w#2.B"},
		{"nested growth", "var wiring = struct { Inner struct { A alpha.Port; B beta.Port } }{}", "var wiring = struct { Inner struct { A alpha.Port; B beta.Port; More alpha.Port } }{}", "production|field|api|wiring.Inner.More"},
		{"nested in a named struct", "type Wiring struct { Inner struct { A alpha.Port; B beta.Port } }", "type Wiring struct { Inner struct { A alpha.Port; B beta.Port; More alpha.Port } }", "production|field|api|Wiring.Inner.More"},
		{"unrelated struct keeps the numbering", "func root() any { return struct { A alpha.Port; B beta.Port }{} }", "func root() any { _ = struct{}{}; _ = struct{ A alpha.Port }{}; return struct { A alpha.Port; B beta.Port }{} }", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := ownerFixtureRepository(t)
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

func TestCompositionSurfaceFailsWhenARootPackageDoesNotTypeCheck(t *testing.T) {
	t.Parallel()
	repo := ownerFixtureRepository(t)
	path := filepath.Join(repo, "api", "wiring.go")
	writeFile(t, path, compositionOwnerImports)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "composition base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, compositionOwnerImports+"type Wiring struct { A alpha.Missing }\n")
	policy := ownerFixturePolicy(compositionOwnerClassification)
	if err := CompareCompositionSurface(repo, base, policy); err == nil || !strings.Contains(err.Error(), "type-check") {
		t.Fatalf("a root package without type information must fail closed: %v", err)
	}
}

// Review of #4095, round 3, S2: a call with several results types every
// variable it declares.
func TestCompositionSurfaceCountsEveryResultOfACall(t *testing.T) {
	t.Parallel()
	const generic = "type Graph[A, B any] struct { A A; B B }\nfunc pair[A, B any](a A, b B) (int, Graph[A, B]) { return 0, Graph[A, B]{a, b} }\nvar first alpha.Port\nvar second beta.Port\n"
	for _, tt := range []struct{ name, candidate, want string }{
		{"short variable declaration", "func root() { _, g := pair(first, second); _ = g }", "production|field|api|root.g.A"},
		{"variable declaration", "var _, g = pair(first, second)", "production|field|api|g.B"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo := ownerFixtureRepository(t)
			path := filepath.Join(repo, "api", "wiring.go")
			writeFile(t, path, compositionOwnerImports+generic)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "composition base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, path, compositionOwnerImports+generic+tt.candidate+"\n")
			policy := ownerFixturePolicy(compositionOwnerClassification)
			if err := CompareCompositionSurface(repo, base, policy); err == nil || !strings.Contains(err.Error(), tt.want) {
				t.Fatalf("want %q, got %v", tt.want, err)
			}
		})
	}
}

// Review of #4095, round 3, S1: an inherited GOWORK neither empties the
// archived base inventory nor makes unchanged fields look like growth.
func TestCompositionSurfaceIgnoresAnInheritedWorkspace(t *testing.T) {
	t.Parallel()
	repo := ownerFixtureRepository(t)
	writeFile(t, filepath.Join(repo, "api", "wiring.go"), compositionOwnerImports+"type Wiring struct { A alpha.Port; B beta.Port }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "composition base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	archived := t.TempDir()
	if err := extractGitTree(repo, base, ".", archived); err != nil {
		t.Fatal(err)
	}
	workspace := filepath.Join(t.TempDir(), "go.work")
	writeFile(t, workspace, "go 1.25\n\nuse "+repo+"\n")
	ownership := newCompositionOwnership(ownerFixturePolicy(compositionOwnerClassification))
	for _, directory := range []string{repo, archived} {
		inventory, err := rootStructInventoryWith(directory, ownership, []string{"GOWORK=" + workspace})
		if err != nil || inventory["production|field|api|Wiring.A"] == "" || inventory["production|field|api|Wiring.B"] == "" {
			t.Fatalf("inventory of %s under GOWORK: %v %v", directory, inventory, err)
		}
	}
}

func TestCompositionSurfaceRejectsARepositoryWorkspace(t *testing.T) {
	t.Parallel()
	repo := ownerFixtureRepository(t)
	writeFile(t, filepath.Join(repo, "api", "wiring.go"), compositionOwnerImports)
	writeFile(t, filepath.Join(repo, "go.work"), "go 1.25\n\nuse .\n")
	policy := ownerFixturePolicy(compositionOwnerClassification)
	if _, err := rootStructInventory(repo, newCompositionOwnership(policy)); err == nil || !strings.Contains(err.Error(), "declares a Go workspace") {
		t.Fatalf("a repository workspace must be reported: %v", err)
	}
}

func TestCompositionSurfaceReportsAMissingRootPackage(t *testing.T) {
	t.Parallel()
	repo := ownerFixtureRepository(t)
	writeFile(t, filepath.Join(repo, "api", "wiring.go"), "//go:build ignore\n\n"+compositionOwnerImports)
	policy := ownerFixturePolicy(compositionOwnerClassification)
	if _, err := rootStructInventory(repo, newCompositionOwnership(policy)); err == nil {
		t.Fatal("a root package that does not load must fail, not count as empty")
	}
}
