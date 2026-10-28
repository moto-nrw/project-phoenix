package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// The public loader/check boundary must see runtime edges hidden by root glue.
func TestCommandEffectsRejectUninventoriedRootBoundWrite(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	var found bool
	for _, violation := range Check(policy, graph) {
		if violation.Rule == "effects.uninventoried" && strings.Contains(violation.Target, "Ledger.Record") {
			found = true
		}
	}
	if !found {
		t.Fatal("new Enrollment port → foreign owner command passed without inventory classification")
	}
}

func commandEffectFixture(t *testing.T) (string, *Policy) {
	t.Helper()
	project := t.TempDir()
	writeFile(t, filepath.Join(project, "go.mod"), "module example.test/effects\n\ngo 1.25\n")
	writeFile(t, filepath.Join(project, "enrollment", "port.go"), `package enrollment
type Writer interface { Record() }
func Run(writer Writer) { writer.Record() }
`)
	writeFile(t, filepath.Join(project, "foreign", "ledger.go"), `package foreign
type Ledger struct{}
func (Ledger) Record() {}
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding struct { target foreign.Ledger }
func (b binding) Record() { b.target.Record() }
func Build() { enrollment.Run(binding{}) }
`)
	writeFile(t, filepath.Join(project, "architecture", "command-effects.json"), `{
  "schema_version": 1,
  "entries": []
}
`)
	return project, &Policy{ModulePath: "example.test/effects", Build: Build{GOOS: "linux", GOARCH: "amd64"},
		Owners:   []Owner{{ID: "enrollment", Kind: "domain"}, {ID: "foreign", Kind: "domain"}, {ID: "root-composition", Kind: "composition"}},
		Packages: []Package{{Path: "enrollment", Owner: "enrollment", Role: "application"}, {Path: "foreign", Owner: "foreign", Role: "public"}, {Path: "roots", Owner: "root-composition", Role: "compose"}},
		Rules: []Rule{
			{ID: "root-enrollment", SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "enrollment", TargetRole: "application", Scopes: []string{"production"}},
			{ID: "root-foreign", SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "foreign", TargetRole: "public", Scopes: []string{"production"}},
		},
	}
}

func TestCommandEffectsAllowReviewedQueryAndException(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, effect, classification, adr string }{
		{"query", "query", "query", ""}, {"exception", "mutation", "E1", "0067"}, {"notice", "notice", "leaf", "0067"}, {"known debt", "mutation", "debt", "0083"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectFixture(t)
			writeFile(t, filepath.Join(project, commandEffectInventory), `{"schema_version":1,"entries":[{
    "port":"example.test/effects/enrollment.Writer.Record",
    "binding":"example.test/effects/roots.binding.Record",
    "target":"example.test/effects/foreign.Ledger.Record","target_owner":"foreign",
    "effect":"`+tc.effect+`","classification":"`+tc.classification+`","adr":"`+tc.adr+`","reason":"Reviewed fixture protocol: caller unit, target guards."}]}`)
			if err := GenerateCommandEffects(project, policy); err != nil {
				t.Fatal(err)
			}
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			if violations := Check(policy, graph); len(violations) != 0 {
				t.Fatalf("reviewed %s rejected: %+v", tc.name, violations)
			}
		})
	}
}

func TestCommandEffectsReportUnresolvedRootBinding(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import "example.test/effects/enrollment"
type dynamic interface { Record() }
type binding struct { target dynamic }
func (b binding) Record() { b.target.Record() }
func Build() { enrollment.Run(binding{}) }
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, violation := range Check(policy, graph) {
		if violation.Rule == "effects.unresolved" && strings.Contains(violation.Target, "dynamic.Record") {
			return
		}
	}
	t.Fatal("unresolved interface binding was silently ignored or treated as query")
}

func TestCommandEffectsCannotExpandExceptionAtPRBase(t *testing.T) {
	t.Parallel()
	project, _ := commandEffectFixture(t)
	runGit(t, project, "init", "--quiet")
	runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "add", ".")
	runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
	base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(project, commandEffectInventory), `{"schema_version":1,"entries":[{
 "port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Ledger.Record","target_owner":"foreign",
 "effect":"mutation","classification":"E1","adr":"0067","reason":"Existing E1 cannot authorize a new protocol."}]}`)
	if err := CompareCommandEffects(project, base); err == nil || !strings.Contains(err.Error(), "exception") {
		t.Fatalf("exception grew without new ADR: %v", err)
	}
}

func TestCommandEffectsFollowEmbeddedPortsPromotedMethodsAndMethodValues(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{
		"command := b.target.Record",
		"var command = b.target.Record",
		"var command func() = b.target.Record",
		"var command, other = b.target.Record, b.target.Record; other()",
	} {
		t.Run(declaration, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectFixture(t)
			writeFile(t, filepath.Join(project, "enrollment", "port.go"), `package enrollment
type Base interface { Record() }
type Writer interface { Base }
func Run(writer Writer) { command := writer.Record; command() }
`)
			writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type base struct { target foreign.Ledger }
func (b base) Record() { `+declaration+`; command() }
type binding struct { base }
func Build() { var port enrollment.Writer = binding{}; enrollment.Run(port) }
`)
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			found := false
			for _, v := range Check(policy, graph) {
				if v.Rule == "effects.unresolved" {
					t.Fatalf("statically bound method value reported dynamic: %+v", v)
				}
				if v.Rule == "effects.uninventoried" && strings.Contains(v.Source, "Writer.Record") && strings.Contains(v.Target, "Ledger.Record") {
					found = true
				}
			}
			if !found {
				t.Fatal("embedded interface, promoted implementation or method value hid a command")
			}
		})
	}
}

func TestCommandEffectsKeepDynamicCallableDeclarationsUnresolved(t *testing.T) {
	t.Parallel()
	for _, declaration := range []string{
		"var command = b.command",
		"var command = b.command; command = b.target.Record",
		"var command = b.target.Record; command = b.command",
	} {
		t.Run(declaration, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectFixture(t)
			writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding struct { target foreign.Ledger; command func() }
func (b binding) Record() { `+declaration+`; command() }
func Build() { enrollment.Run(binding{}) }
`)
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, violation := range Check(policy, graph) {
				if violation.Rule == "effects.unresolved" && strings.Contains(violation.Target, "unresolved:callable:") {
					return
				}
			}
			t.Fatal("dynamic callable declaration or reassignment was treated as static")
		})
	}
}

func TestCommandEffectsFollowConsumerAliasToForeignContract(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "port.go"), `package enrollment
import "example.test/effects/foreign"
type Writer = foreign.Commands
func Run(writer Writer) { writer.Record() }
`)
	writeFile(t, filepath.Join(project, "foreign", "ledger.go"), `package foreign
type Commands interface { Record() }
type Ledger struct{}
func (Ledger) Record() {}
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.uninventoried" && strings.Contains(v.Source, "foreign.Commands.Record") && strings.Contains(v.Target, "Ledger.Record") {
			return
		}
	}
	t.Fatal("consumer alias to native foreign contract hid a command")
}

func TestCommandEffectsDiagnoseFunctionValuedAdapterField(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import "example.test/effects/enrollment"
type binding struct { command func() }
func (b binding) Record() { b.command() }
func Build() { enrollment.Run(binding{}) }
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.unresolved" && strings.Contains(v.Target, "function:") {
			return
		}
	}
	t.Fatal("function-valued binding was silently ignored")
}

func TestCommandEffectsDoNotHideChangesBehindTheSameTargets(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(project, "roots", "binding.go")
	source := readFile(t, filename)
	writeFile(t, filename, strings.Replace(source, "b.target.Record()", "b.target.Record(); b.target.Record()", 1))
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.changed" {
			return
		}
	}
	t.Fatal("changed adapter body with identical command targets retained its old evidence")
}

func TestCommandEffectsRejectUnreviewedClassifications(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, effect, classification, adr string }{
		{"unknown effect", "unknown", "query", ""}, {"unnamed exception", "mutation", "exception", "0067"},
		{"missing ADR", "mutation", "E1", ""}, {"unresolved query", "query", "query", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectFixture(t)
			target := "example.test/effects/foreign.Ledger.Record"
			if tc.name == "unresolved query" {
				writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import "example.test/effects/enrollment"
type dynamic interface { Record() }
type binding struct { target dynamic }
func (b binding) Record() { b.target.Record() }
func Build() { enrollment.Run(binding{}) }
`)
				target = "unresolved:example.test/effects/roots.dynamic.Record"
			}
			targetOwner := "foreign"
			if tc.name == "unresolved query" {
				targetOwner = ""
			}
			writeFile(t, filepath.Join(project, commandEffectInventory), `{"schema_version":1,"entries":[{
    "port":"example.test/effects/enrollment.Writer.Record","binding":"example.test/effects/roots.binding.Record",
    "target":"`+target+`","target_owner":"`+targetOwner+`","effect":"`+tc.effect+`","classification":"`+tc.classification+`","adr":"`+tc.adr+`","reason":"Fixture review evidence."}]}`)
			if err := GenerateCommandEffects(project, policy); err != nil {
				t.Fatal(err)
			}
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range Check(policy, graph) {
				if v.Rule == "effects.classification" {
					return
				}
			}
			t.Fatal("unreviewed effect classification passed")
		})
	}
}

func TestCommandEffectsCompareChangedEffectsAndEvidenceWithBase(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, effect, classification, digest, reason, approval, want string }{
		{"downgrade", "query", "query", "before", "before", "", "effect/classification changed"},
		{"unreviewed body", "mutation", "leaf", "after", "before", "", "update exact review evidence"},
		{"reviewed body", "mutation", "leaf", "after", "reviewed after change", "", ""},
		{"new exception ADR", "mutation", "E1", "before", "reviewed coordinator", "0089", ""},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, _ := commandEffectFixture(t)
			filename := filepath.Join(project, commandEffectInventory)
			manifest := func(effect, classification, digest, reason, approval string) string {
				return `{"schema_version":1,"entries":[{"port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Ledger.Record","target_owner":"foreign","binding_digest":"` + digest + `","effect":"` + effect + `","classification":"` + classification + `","adr":"0067","reason":"` + reason + `","approval_adr":"` + approval + `"}]}`
			}
			writeFile(t, filename, manifest("mutation", "leaf", "before", "before", ""))
			runGit(t, project, "init", "--quiet")
			runGit(t, project, "add", ".")
			runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
			base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
			writeFile(t, filename, manifest(tc.effect, tc.classification, tc.digest, tc.reason, tc.approval))
			if tc.approval != "" {
				writeFile(t, filepath.Join(project, "docs", "adr", "0089-fixture.md"), "# Reviewed boundary\n")
			}
			err := CompareCommandEffects(project, base)
			if tc.want == "" {
				if err != nil {
					t.Fatal(err)
				}
				return
			}
			if err == nil || !strings.Contains(err.Error(), tc.want) {
				t.Fatalf("base comparison = %v, want %s", err, tc.want)
			}
		})
	}
}

func TestCommandEffectsFollowConsumerInterfaceForwarding(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "port.go"), `package enrollment
type Base interface { Record() }
type Writer interface { Record() }
func Run(writer Writer) { writer.Record() }
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
func forward(base enrollment.Base) enrollment.Writer { return base }
func Build() { enrollment.Run(forward(foreign.Ledger{})) }
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.uninventoried" && strings.Contains(v.Source, "Writer.Record") && strings.Contains(v.Target, "Ledger.Record") {
			return
		}
	}
	t.Fatal("consumer-owned interface forwarding hid the outer port's command edge")
}

func TestCommandEffectsDiagnoseCallableReceiverAdapter(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding func()
func (b binding) Record() { b() }
func Build() { enrollment.Run(binding(foreign.Ledger{}.Record)) }
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.unresolved" && strings.Contains(v.Target, "callable:") {
			return
		}
	}
	t.Fatal("callable receiver hid a dynamic command behind an inventoried interface")
}

func TestCommandEffectsBootstrapIsBoundToNewImplementationDecision(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                              string
		markerAtBase, addMarker, baseHasRegister, allowed bool
	}{
		{"missing decision", false, false, false, false},
		{"new addendum", false, true, false, true},
		{"existing addendum is not new", true, false, false, false},
		{"existing register cannot reuse addendum", false, true, true, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, _ := commandEffectFixture(t)
			filename := filepath.Join(project, commandEffectInventory)
			if !tc.baseHasRegister {
				if err := os.Remove(filename); err != nil {
					t.Fatal(err)
				}
			}
			document := "# Existing ADR 0083\n"
			if tc.markerAtBase {
				document += "\n## Implementation addendum 2026-10-23 (#4739)\n"
			}
			writeFile(t, filepath.Join(project, commandEffectDecision), document)
			runGit(t, project, "init", "--quiet")
			runGit(t, project, "add", ".")
			runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
			base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
			if tc.addMarker {
				writeFile(t, filepath.Join(project, commandEffectDecision), document+"\n## Implementation addendum 2026-10-23 (#4739)\n")
			}
			writeFile(t, filename, `{"schema_version":1,"entries":[{"port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Ledger.Record","target_owner":"foreign","effect":"mutation","classification":"E1","adr":"0067","approval_adr":"0083","reason":"Reviewed initial E1 boundary."}]}`)
			err := CompareCommandEffects(project, base)
			if (err == nil) != tc.allowed {
				t.Fatalf("bootstrap allowed=%t, error=%v", tc.allowed, err)
			}
		})
	}
}

func TestCommandEffectsCoverEveryPolicyDomainSource(t *testing.T) {
	t.Parallel()
	for _, owner := range []string{"care-plan", "people-directory", "identity-access", "new-domain"} {
		t.Run(owner, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectFixture(t)
			policy.Owners[0].ID = owner
			policy.Packages[0].Owner = owner
			policy.Rules[0].TargetOwner = owner
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			for _, v := range Check(policy, graph) {
				if v.Rule == "effects.uninventoried" && strings.Contains(v.Target, "Ledger.Record") {
					return
				}
			}
			t.Fatalf("new %s cross-owner command escaped inventory", owner)
		})
	}
}

func TestCommandEffectsCheckRejectsNoncanonicalRegister(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(project, commandEffectInventory)
	var document any
	if err := json.Unmarshal([]byte(readFile(t, filename)), &document); err != nil {
		t.Fatal(err)
	}
	blob, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteInventory(filename, artifacts.CommandEffects, blob); err != nil {
		t.Fatal(err)
	}
	_, err = LoadGraph(project, policy)
	if err == nil || !strings.Contains(err.Error(), "scripts/backend-architecture.sh effects") {
		t.Fatalf("noncanonical register passed or omitted generator command: %v", err)
	}
	// The documented generator must repair formatting without discarding labels.
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadGraph(project, policy); err != nil {
		t.Fatal(err)
	}
}

func TestCommandEffectsRejectConflictingEffectsForOneTargetMethod(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	filename := filepath.Join(project, "roots", "binding.go")
	source := readFile(t, filename)
	source = strings.Replace(source, "func Build()", "type other struct { binding }; func Build()", 1)
	source = strings.Replace(source, "enrollment.Run(binding{})", "enrollment.Run(binding{}); enrollment.Run(other{})", 1)
	writeFile(t, filename, source)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	filename = filepath.Join(project, commandEffectInventory)
	var document struct {
		SchemaVersion int              `json:"schema_version"`
		Entries       []map[string]any `json:"entries"`
	}
	if err := json.Unmarshal([]byte(readMaterializedEffects(t, filename)), &document); err != nil {
		t.Fatal(err)
	}
	if len(document.Entries) != 2 {
		t.Fatalf("fixture needs two bindings, got %d", len(document.Entries))
	}
	for i, entry := range document.Entries {
		entry["reason"] = "Reviewed fixture command in caller transaction."
		entry["effect"], entry["classification"], entry["adr"] = "mutation", "leaf", "0067"
		if i == 1 {
			entry["effect"], entry["classification"] = "query", "query"
		}
	}
	// The generator restores canonical field ordering while retaining review labels.
	blob, err := json.Marshal(document)
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteInventory(filename, artifacts.CommandEffects, blob); err != nil {
		t.Fatal(err)
	}
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.inconsistent" {
			return
		}
	}
	t.Fatal("one target method was simultaneously approved as query and mutation")
}

func TestCommandEffectsDoNotTreatExternalErrorImplementationsAsBindings(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "foreign", "error.go"), `package foreign
type Failure struct{}
func (Failure) Error() string { return "failed" }
func Fail() error { return Failure{} }
`)
	filename := filepath.Join(project, "roots", "binding.go")
	source := readFile(t, filename)
	source = strings.Replace(source, "b.target.Record()", "b.target.Record(); err := foreign.Fail(); _ = err.Error()", 1)
	writeFile(t, filename, source)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if strings.Contains(v.Target, "Failure.Error") {
			t.Fatalf("external error interface pulled unrelated implementers into port: %+v", v)
		}
	}
	if !strings.Contains(readMaterializedEffects(t, filepath.Join(project, commandEffectInventory)), "Ledger.Record") {
		t.Fatal("real command disappeared with error noise")
	}
}

func TestCommandEffectsDoNotInventCrossOwnerEdgesForLocalInterfaceForwarding(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "local.go"), `package enrollment
type Local interface { Own() }
type Wider interface { Own(); More() }
func Narrow(w Wider) Local { return w }
`)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if strings.Contains(v.Source, "Local.Own") {
			t.Fatalf("local interface forwarding invented a cross-owner unresolved binding: %+v", v)
		}
	}
}

func TestCommandEffectsDomainExpansionApprovalCannotBeReused(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name                                                                   string
		priorMarker, priorOwner, existingChange, marker, allowed, fakeApproval bool
	}{
		{name: "new domain slice", marker: true, allowed: true},
		{name: "without addendum"},
		{name: "old addendum", priorMarker: true, marker: true},
		{name: "already covered owner", priorOwner: true, marker: true},
		{name: "existing exception change", existingChange: true, marker: true},
		{name: "pseudo ADR is not approval", existingChange: true, marker: true, fakeApproval: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, _ := commandEffectFixture(t)
			filename := filepath.Join(project, commandEffectInventory)
			entry := `{"source_owner":"care-plan","port":"care.Port.Record","binding":"root.Binding.Record","target":"foreign.Ledger.Record","target_owner":"foreign","binding_digest":"before","effect":"mutation","classification":"E2","adr":"0067","approval_adr":"0083","reason":"Existing ADR 0067 E2 protocol, newly inventoried."}`
			baseline := `{"schema_version":1,"entries":[]}`
			if tc.priorOwner || tc.existingChange {
				baseline = `{"schema_version":1,"entries":[` + entry + `]}`
			}
			writeFile(t, filename, baseline)
			marker := "\n## Domain-source expansion 2026-10-23 (#4739)\n"
			document := "# Existing ADR 0083\n"
			if tc.priorMarker {
				document += marker
			}
			writeFile(t, filepath.Join(project, commandEffectDecision), document)
			runGit(t, project, "init", "--quiet")
			runGit(t, project, "add", ".")
			runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
			base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
			if tc.marker {
				writeFile(t, filepath.Join(project, commandEffectDecision), document+marker)
			}
			candidate := entry
			if tc.priorOwner {
				candidate = strings.Replace(candidate, "care.Port.Record", "care.Other.Record", 1)
			}
			if tc.existingChange {
				candidate = strings.Replace(candidate, "before", "after", 1)
				candidate = strings.Replace(candidate, "Existing ADR", "Revised ADR", 1)
			}
			if tc.fakeApproval {
				candidate = strings.Replace(candidate, `"approval_adr":"0083"`, `"approval_adr":"0083:domains"`, 1)
			}
			writeFile(t, filename, `{"schema_version":1,"entries":[`+candidate+`]}`)
			err := CompareCommandEffects(project, base)
			if (err == nil) != tc.allowed {
				t.Fatalf("domain expansion allowed=%t, error=%v", tc.allowed, err)
			}
		})
	}
}

func TestCommandEffectsResolveWorkflowBehindForeignInterface(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	policy.Owners = append(policy.Owners, Owner{ID: "workflow", Kind: "workflow"})
	policy.Packages = append(policy.Packages, Package{Path: "workflow", Owner: "workflow", Role: "public"})
	writeFile(t, filepath.Join(project, "foreign", "port.go"), `package foreign
type Commands interface { Record() }
`)
	writeFile(t, filepath.Join(project, "workflow", "work.go"), `package workflow
type Work struct{}
func (Work) Record() {}
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
 "example.test/effects/workflow"
)
func supplied() foreign.Commands { return workflow.Work{} }
func Build() { enrollment.Run(supplied()) }
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.uninventoried" && strings.Contains(v.Source, "enrollment.Writer.Record") && strings.Contains(v.Target, "workflow.Work.Record") {
			return
		}
	}
	t.Fatal("foreign interface hid runtime workflow provider")
}

func TestCommandEffectsResolveForwardedTupleReturn(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	policy.Owners = append(policy.Owners, Owner{ID: "workflow", Kind: "workflow"})
	policy.Packages = append(policy.Packages, Package{Path: "workflow", Owner: "workflow", Role: "public"})
	writeFile(t, filepath.Join(project, "foreign", "port.go"), `package foreign
type Commands interface { Record() }
`)
	writeFile(t, filepath.Join(project, "workflow", "work.go"), `package workflow
type Work struct{}
func (Work) Record() {}
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
 "example.test/effects/workflow"
)
func makeWork() (workflow.Work, error) { return workflow.Work{}, nil }
func supplied() (foreign.Commands, error) { return makeWork() }
func Build() { port, _ := supplied(); enrollment.Run(port) }
`)
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.uninventoried" && strings.Contains(v.Source, "enrollment.Writer.Record") && strings.Contains(v.Target, "workflow.Work.Record") {
			return
		}
	}
	t.Fatal("foreign interface hid runtime workflow provider")
}

func TestCommandEffectsDomainExpansionReviewsExistingPortProvidersNotNewPorts(t *testing.T) {
	t.Parallel()
	for _, newPort := range []bool{false, true} {
		t.Run(map[bool]string{false: "newly resolved provider", true: "new port is not covered"}[newPort], func(t *testing.T) {
			t.Parallel()
			project, _ := commandEffectFixture(t)
			filename := filepath.Join(project, commandEffectInventory)
			entry := `{"port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Contract.Record","target_owner":"foreign","binding_digest":"before","effect":"mutation","classification":"E1","adr":"0067","approval_adr":"0083","reason":"Existing E1 reviewed protocol."}`
			writeFile(t, filename, `{"schema_version":1,"entries":[`+entry+`]}`)
			writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n")
			runGit(t, project, "init", "--quiet")
			runGit(t, project, "add", ".")
			runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
			base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
			writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n\n## Domain-source expansion 2026-10-23 (#4739)\n")
			provider := strings.Replace(entry, "foreign.Contract.Record", "foreign.Provider.Record", 1)
			if newPort {
				provider = strings.Replace(provider, "enrollment.Writer.Record", "enrollment.NewWriter.Record", 1)
			}
			writeFile(t, filename, `{"schema_version":1,"entries":[`+entry+`,`+provider+`]}`)
			err := CompareCommandEffects(project, base)
			if (err == nil) == newPort {
				t.Fatalf("newPort=%t: %v", newPort, err)
			}
		})
	}
}

func TestCommandEffectsKeepCalledHelperBehaviorInFingerprint(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	filename := filepath.Join(project, "roots", "binding.go")
	source := strings.Replace(readFile(t, filename), "b.target.Record()", "record(b.target)", 1)
	source += "\nfunc record(target foreign.Ledger) { target.Record() }\n"
	writeFile(t, filename, source)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	// The adapter and terminal target set stay unchanged; only its called helper changes.
	writeFile(t, filename, strings.Replace(source, "target.Record()", "target.Record(); target.Record()", 1))
	graph, err := LoadGraph(project, policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range Check(policy, graph) {
		if v.Rule == "effects.changed" && strings.Contains(v.Target, "Ledger.Record") {
			return
		}
	}
	t.Fatal("helper behavior changed without invalidating the outer binding evidence")
}

func TestCommandEffectsKeepSeparateConsumersOfOneForeignContract(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	writeFile(t, filepath.Join(project, "foreign", "port.go"), "package foreign\ntype Commands interface { Record() }\n")
	writeFile(t, filepath.Join(project, "enrollment", "port.go"), `package enrollment
import "example.test/effects/foreign"
type Writer = foreign.Commands
func Run(writer Writer) { writer.Record() }
`)
	writeFile(t, filepath.Join(project, "other", "port.go"), `package other
import "example.test/effects/foreign"
type Writer = foreign.Commands
func Run(writer Writer) { writer.Record() }
`)
	policy.Owners = append(policy.Owners, Owner{ID: "other", Kind: "domain"})
	policy.Packages = append(policy.Packages, Package{Path: "other", Owner: "other", Role: "application"})
	filename := filepath.Join(project, "roots", "binding.go")
	source := strings.Replace(readFile(t, filename), `"example.test/effects/enrollment"`, `"example.test/effects/enrollment"
 "example.test/effects/other"`, 1)
	source = strings.Replace(source, "enrollment.Run(binding{})", "enrollment.Run(binding{}); other.Run(binding{})", 1)
	writeFile(t, filename, source)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	var document struct {
		Entries []struct {
			SourceOwner string `json:"source_owner"`
			Port        string `json:"port"`
		} `json:"entries"`
	}
	if err := json.Unmarshal([]byte(readMaterializedEffects(t, filepath.Join(project, commandEffectInventory))), &document); err != nil {
		t.Fatal(err)
	}
	sources := map[string]bool{}
	for _, entry := range document.Entries {
		if strings.Contains(entry.Port, "foreign.Commands.Record") {
			sources[entry.SourceOwner] = true
		}
	}
	if !sources["enrollment"] || !sources["other"] {
		t.Fatalf("consumers of the same native contract collapsed: %v", sources)
	}
}

func TestCommandEffectsRequireRegisterOnlyWhenThereIsABindingSurface(t *testing.T) {
	t.Parallel()
	for _, wired := range []bool{false, true} {
		t.Run(map[bool]string{false: "no foreign binding", true: "foreign binding requires register"}[wired], func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectFixture(t)
			if err := os.Remove(filepath.Join(project, commandEffectInventory)); err != nil {
				t.Fatal(err)
			}
			if !wired {
				writeFile(t, filepath.Join(project, "enrollment", "port.go"), "package enrollment\n")
				writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nfunc Build() {}\n")
			}
			_, err := LoadGraph(project, policy)
			if (err != nil) != wired {
				t.Fatalf("wired=%t, load error=%v", wired, err)
			}
		})
	}
}

func TestCommandEffectsDomainExpansionReviewsNewUnresolvedEvidenceNotNewPorts(t *testing.T) {
	t.Parallel()
	for _, newPort := range []bool{false, true} {
		t.Run(map[bool]string{false: "newly resolved provider", true: "new port is not covered"}[newPort], func(t *testing.T) {
			t.Parallel()
			project, _ := commandEffectFixture(t)
			filename := filepath.Join(project, commandEffectInventory)
			entry := `{"port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Contract.Record","target_owner":"foreign","binding_digest":"before","effect":"mutation","classification":"E1","adr":"0067","approval_adr":"0083","reason":"Existing E1 reviewed protocol."}`
			writeFile(t, filename, `{"schema_version":1,"entries":[`+entry+`]}`)
			writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n")
			runGit(t, project, "init", "--quiet")
			runGit(t, project, "add", ".")
			runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
			base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
			writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n\n## Domain-source expansion 2026-10-23 (#4739)\n")
			provider := strings.Replace(entry, "foreign.Contract.Record", "unresolved:cycle:foreign.Contract.Record", 1)
			provider = strings.Replace(provider, `"target_owner":"foreign"`, `"target_owner":""`, 1)
			provider = strings.Replace(provider, `"effect":"mutation"`, `"effect":"unresolved"`, 1)
			provider = strings.Replace(provider, `"classification":"E1"`, `"classification":"unresolved"`, 1)
			provider = strings.Replace(provider, `"adr":"0067"`, `"adr":"0083"`, 1)
			if newPort {
				provider = strings.Replace(provider, "enrollment.Writer.Record", "enrollment.NewWriter.Record", 1)
			}
			writeFile(t, filename, `{"schema_version":1,"entries":[`+entry+`,`+provider+`]}`)
			err := CompareCommandEffects(project, base)
			if (err == nil) == newPort {
				t.Fatalf("newPort=%t: %v", newPort, err)
			}
		})
	}
}

func TestCommandEffectsDomainExpansionKeepsAllReviewedEffectsForABinding(t *testing.T) {
	t.Parallel()
	for _, newPort := range []bool{false, true} {
		t.Run(map[bool]string{false: "newly resolved provider", true: "new port is not covered"}[newPort], func(t *testing.T) {
			t.Parallel()
			project, _ := commandEffectFixture(t)
			filename := filepath.Join(project, commandEffectInventory)
			entry := `{"port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Contract.Record","target_owner":"foreign","binding_digest":"before","effect":"mutation","classification":"E1","adr":"0067","approval_adr":"0083","reason":"Existing E1 reviewed protocol."}`
			other := strings.Replace(entry, "foreign.Contract.Record", "foreign.Query.Read", 1)
			other = strings.Replace(other, `"effect":"mutation"`, `"effect":"query"`, 1)
			other = strings.Replace(other, `"classification":"E1"`, `"classification":"query"`, 1)
			writeFile(t, filename, `{"schema_version":1,"entries":[`+entry+`,`+other+`]}`)
			writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n")
			runGit(t, project, "init", "--quiet")
			runGit(t, project, "add", ".")
			runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
			base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
			writeFile(t, filepath.Join(project, commandEffectDecision), "# Existing ADR 0083\n\n## Domain-source expansion 2026-10-23 (#4739)\n")
			provider := strings.Replace(entry, "foreign.Contract.Record", "foreign.Provider.Record", 1)
			if newPort {
				provider = strings.Replace(provider, "enrollment.Writer.Record", "enrollment.NewWriter.Record", 1)
			}
			writeFile(t, filename, `{"schema_version":1,"entries":[`+entry+`,`+other+`,`+provider+`]}`)
			err := CompareCommandEffects(project, base)
			if (err == nil) == newPort {
				t.Fatalf("newPort=%t: %v", newPort, err)
			}
		})
	}
}

func TestCommandEffectsCanonicalFormRequiresExplicitCurrentSourceOwner(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectFixture(t)
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	filename := filepath.Join(project, "architecture", "command-effects", "reviews", "enrollment.json")
	original := readFile(t, filename)
	omitted := strings.Replace(original, "      \"source_owner\": \"enrollment\",\n", "", 1)
	if omitted == original {
		t.Fatal("fixture did not remove a source owner")
	}
	writeFile(t, filename, omitted)
	_, err := LoadGraph(project, policy)
	if err == nil || !strings.Contains(err.Error(), "source_owner") {
		t.Fatalf("omitted generator source_owner was accepted: %v", err)
	}
}

func TestCommandEffectsKnownDebtStillNeedsNewApproval(t *testing.T) {
	t.Parallel()
	project, _ := commandEffectFixture(t)
	runGit(t, project, "init", "--quiet")
	runGit(t, project, "add", ".")
	runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "-c", "commit.gpgsign=false", "commit", "--quiet", "-m", "base")
	base := strings.TrimSpace(runGit(t, project, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(project, commandEffectInventory), `{"schema_version":1,"entries":[{
 "port":"enrollment.Writer.Record","binding":"roots.binding.Record","target":"foreign.Ledger.Record","target_owner":"foreign",
 "effect":"mutation","classification":"debt","adr":"0083","reason":"Known coordinator debt does not itself grant a new exception."}]}`)
	err := CompareCommandEffects(project, base)
	if err == nil || !strings.Contains(err.Error(), "new exception needs approval_adr") {
		t.Fatalf("new debt passed without approval: %v", err)
	}
}
