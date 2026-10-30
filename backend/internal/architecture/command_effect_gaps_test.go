package architecture

import (
	"encoding/json"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
	"github.com/moto-nrw/project-phoenix/internal/gitfixture"
)

func TestCommandEffectsChangedClosureNeedsFreshApproval(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	filename := filepath.Join(project, "roots", "binding.go")
	source := `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding struct { command func() }
func (b binding) Record() { b.command() }
func Build() { enrollment.Run(binding{command: func() { foreign.Ledger{}.Record() }}) }
`
	writeFile(t, filename, source)
	reviewGapFixture(t, project, policy, "mutation", "debt", "0083")
	base := commitGapFixture(t, project)
	writeFile(t, filename, strings.Replace(source, "foreign.Ledger{}.Record()", "foreign.Ledger{}.Record(); foreign.Ledger{}.Record()", 1))
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	requireGapBaseFailure(t, project, base, "exception binding changed without a new approval_adr")
}

func TestCommandEffectsContainerOnlyBindingCannotDisappear(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	base := commitGapFixture(t, project)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
func Build() { ports := []enrollment.Writer{foreign.Ledger{}}; enrollment.Run(ports[0]) }
`)
	reviewGapFixture(t, project, policy, "mutation", "debt", "0083")
	requireGapBaseFailure(t, project, base, "new exception needs approval_adr")
}

func TestCommandEffectsNewWorkflowNeedsFreshApproval(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	base := commitGapFixture(t, project)
	reviewGapFixture(t, project, policy, "mutation", "workflow", "0067")
	requireGapBaseFailure(t, project, base, "new exception needs approval_adr")
}

func reviewGapFixture(t *testing.T, project string, policy *Policy, effect, classification, adr string) {
	t.Helper()
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	manifest, err := loadCommandEffects(project)
	if err != nil {
		t.Fatal(err)
	}
	for i := range manifest.Entries {
		entry := &manifest.Entries[i]
		entry.Effect, entry.Classification, entry.ADR = effect, classification, adr
		if strings.HasPrefix(entry.Target, "unresolved:") {
			entry.Effect, entry.Classification, entry.ADR = "unresolved", "unresolved", "0083"
		}
		entry.Reason = "Exact fixture review: supplied command retains caller scope and failure behavior."
	}
	blob, err := marshalCommandEffects(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteInventory(filepath.Join(project, commandEffectInventory), artifacts.CommandEffects, blob); err != nil {
		t.Fatal(err)
	}
}

func commitGapFixture(t *testing.T, project string) string {
	t.Helper()
	writeFile(t, filepath.Join(project, "test", "query-budgets", "owner", "list.json"), queryBudgetFragment)
	writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n")
	gitfixture.Init(t, project)
	runGit(t, project, "add", ".")
	runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
	return strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
}

func requireGapBaseFailure(t *testing.T, project, base, want string) {
	t.Helper()
	err := CompareCommandEffects(project, base)
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("base check = %v, want %q", err, want)
	}
	err = RunCLI([]string{"check", "--project", project, "--baseline", "architecture/legacy.jsonl", "--base-ref", base}, CLIDependencies{})
	if err == nil || !strings.Contains(err.Error(), want) {
		t.Fatalf("CLI check = %v, want %q", err, want)
	}
}

func TestCommandEffectsClosureEvidenceExcludesCommentsAndUnrelatedWiring(t *testing.T) {
	t.Parallel()
	for _, edit := range []string{"comment", "unrelated closure"} {
		t.Run(edit, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			filename := filepath.Join(project, "roots", "binding.go")
			source := `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding struct { command func() }
func (b binding) Record() { b.command() }
func Build() { enrollment.Run(binding{command: func() { foreign.Ledger{}.Record() }}) }
`
			writeFile(t, filename, source)
			reviewGapFixture(t, project, policy, "mutation", "debt", "0083")
			base := commitGapFixture(t, project)
			replacement := "func() { foreign.Ledger{}.Record() /* reviewed */ }"
			if edit == "unrelated closure" {
				replacement = "func() { foreign.Ledger{}.Record() }"
				source += "\nfunc unused() func() { return func() { panic(\"unused\") } }\n"
			}
			writeFile(t, filename, strings.Replace(source, "func() { foreign.Ledger{}.Record() }", replacement, 1))
			if err := GenerateCommandEffects(project, policy); err != nil {
				t.Fatal(err)
			}
			if targets := fixtureEffectTargets(t, project, policy); len(targets) != 1 || targets[0] != "enrollment: foreign.Ledger.Record" {
				t.Fatalf("closure evidence has no reviewed terminal: %v", targets)
			}
			if err := CompareCommandEffects(project, base); err != nil {
				t.Fatal(err)
			}
		})
	}
}

func TestCommandEffectsChangedCallableAssignmentsNeedFreshApproval(t *testing.T) {
	t.Parallel()
	for _, setup := range []struct{ name, declarations, body, wiring string }{
		{"field assignment", "type binding struct { command func() }", "b.command()", "b := binding{}; b.command = func() { marker() }; enrollment.Run(b)"},
		{"constructor parameter", "type binding struct { command func() }; func makeBinding(command func()) binding { return binding{command: command} }", "b.command()", "enrollment.Run(makeBinding(func() { marker() }))"},
		{"callable variable", "type binding struct { command func() }", "command := b.command; command()", "enrollment.Run(binding{command: func() { marker() }})"},
		{"tuple constructor argument", "type binding struct { command func() }; func makeBinding(_ int, command func()) binding { return binding{command: command} }; func supply() (int, func()) { return 0, func() { marker() } }", "b.command()", "enrollment.Run(makeBinding(supply()))"},
		{"variadic tuple constructor argument", "type binding struct { command func() }; func makeBinding(_ int, commands ...func()) binding { return binding{command: commands[0]} }; func supply() (int, func()) { return 0, func() { marker() } }", "b.command()", "enrollment.Run(makeBinding(supply()))"},
		{"tuple factory", "type binding struct { command func() }; func factory() (func(), error) { return func() { marker() }, nil }", "b.command()", "command, _ := factory(); enrollment.Run(binding{command: command})"},
		{"container source", "type binding struct { command func() }", "b.command()", "commands := []func(){func() { marker() }}; enrollment.Run(binding{command: commands[0]})"},
		{"called helper", "type binding struct { command func() }; func helper() { marker() }", "b.command()", "enrollment.Run(binding{command: helper})"},
	} {
		t.Run(setup.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			filename := filepath.Join(project, "roots", "binding.go")
			// The marker is a foreign write: a resolved closure keeps its
			// evidence on that terminal, an unresolved one on its diagnostic.
			source := "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\nfunc marker() { foreign.Ledger{}.Record() }\n" + setup.declarations + "\nfunc (b binding) Record() { " + setup.body + " }\nfunc Build() { " + setup.wiring + " }\n"
			writeFile(t, filename, source)
			reviewGapFixture(t, project, policy, "mutation", "debt", "0083")
			base := commitGapFixture(t, project)
			writeFile(t, filename, strings.Replace(source, "{ marker() }", "{ marker(); marker() }", 1))
			if err := GenerateCommandEffects(project, policy); err != nil {
				t.Fatal(err)
			}
			requireGapBaseFailure(t, project, base, "exception binding changed without a new approval_adr")
		})
	}
}

func TestCommandEffectsContainerAndTupleFlowExposesForeignWrites(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, declarations, wiring string }{
		{"slice", "", "ports := []enrollment.Writer{foreign.Ledger{}}; enrollment.Run(ports[0])"},
		{"map value", "", "ports := map[string]enrollment.Writer{\"write\": foreign.Ledger{}}; enrollment.Run(ports[\"write\"])"},
		{"map key", "", "ports := map[enrollment.Writer]bool{foreign.Ledger{}: true}; for port := range ports { enrollment.Run(port) }"},
		{"array", "", "ports := [1]enrollment.Writer{0: foreign.Ledger{}}; enrollment.Run(ports[0])"},
		{"nested container", "", "ports := map[string][]enrollment.Writer{\"write\": {foreign.Ledger{}}}; enrollment.Run(ports[\"write\"][0])"},
		{"named callable", "type consumer func(enrollment.Writer); var take consumer", "take(foreign.Ledger{})"},
		{"variadic", "func takePorts(ports ...enrollment.Writer) {}", "takePorts(foreign.Ledger{}, foreign.Ledger{})"},
		{"variadic expansion", "func takePorts(ports ...enrollment.Writer) {}", "ports := []enrollment.Writer{foreign.Ledger{}}; takePorts(ports...)"},
		{"tuple assignment", "func supply() (foreign.Ledger, error) { return foreign.Ledger{}, nil }", "var port enrollment.Writer; var err error; port, err = supply(); _ = err; enrollment.Run(port)"},
		{"tuple declaration", "func supply() (foreign.Ledger, error) { return foreign.Ledger{}, nil }", "var port, err = supply(); _ = err; enrollment.Run(port)"},
		{"tuple call", "func supply() (foreign.Ledger, error) { return foreign.Ledger{}, nil }; func takePorts(port enrollment.Writer, err error) {}", "takePorts(supply())"},
		{"append", "", "ports := []enrollment.Writer{}; ports = append(ports, foreign.Ledger{}); enrollment.Run(ports[0])"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\n"+tc.declarations+"\nfunc Build() { "+tc.wiring+" }\n")
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range Check(policy, graph) {
				if v.Rule == "effects.uninventoried" && strings.Contains(v.Source, "Writer.Record") && strings.Contains(v.Target, "Ledger.Record") {
					return
				}
			}
			t.Fatal("container/tuple/variadic flow hid a foreign write")
		})
	}
}

func TestCommandEffectsRecursiveContainersDoNotInventBindings(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "roots", "recursive.go"), `package roots
type Tree map[string]Tree
func copyTree(tree Tree) Tree { clone := tree; return clone }
`)
	reviewGapFixture(t, project, policy, "mutation", "leaf", "0067")
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	if violations := Check(policy, graph); len(violations) != 0 {
		t.Fatalf("recursive container invented effects: %v", violations)
	}
}

func commandEffectGateFixture(t *testing.T) (string, *Policy) {
	t.Helper()
	project, seed := commandEffectFixture(t)
	policy, err := LoadPolicy(fixturePath(t, "allowed-edge.json"))
	if err != nil {
		t.Fatal(err)
	}
	policy.ModulePath, policy.Owners, policy.Packages, policy.Rules = seed.ModulePath, seed.Owners, seed.Packages, seed.Rules
	policy.Roles = append(policy.Roles, "public", "compose")
	for i := range policy.Packages {
		policy.Packages[i].InternalTestRole = "module-internal-test"
		policy.Packages[i].ExternalTestRole = "module-behavior-test"
	}
	for i := range policy.Rules {
		policy.Rules[i].Description = "The fixture root binds its domain capability."
	}
	blob, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(project, "architecture", "policy.json"), string(blob))
	writeFile(t, filepath.Join(project, "architecture", "legacy.jsonl"), "")
	return project, policy
}
