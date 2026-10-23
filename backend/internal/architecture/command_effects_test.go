package architecture

import (
	"path/filepath"
	"strings"
	"testing"
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
	writeFile(t, filepath.Join(project, "architecture", "command-effects.json"), `{"schema_version":1,"entries":[]}`)
	return project, &Policy{ModulePath: "example.test/effects", Build: Build{GOOS: "linux", GOARCH: "amd64"},
		Owners:   []Owner{{ID: "enrollment", Kind: "domain"}, {ID: "foreign", Kind: "domain"}, {ID: "root-composition", Kind: "composition"}},
		Packages: []Package{{Path: "enrollment", Owner: "enrollment", Role: "application"}, {Path: "foreign", Owner: "foreign", Role: "public"}, {Path: "roots", Owner: "root-composition", Role: "compose"}},
	}
}

func TestCommandEffectsAllowReviewedQueryAndException(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, effect, classification, adr string }{
		{"query", "query", "query", ""}, {"exception", "mutation", "E1", "0067"},
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
			for _, violation := range Check(policy, graph) {
				if strings.HasPrefix(violation.Rule, "effects.") {
					t.Fatalf("reviewed %s rejected: %+v", tc.name, violation)
				}
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
	runGit(t, project, "-c", "user.name=Fixture", "-c", "user.email=fixture@example.test", "commit", "--quiet", "-m", "base")
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
func (b base) Record() { command := b.target.Record; command() }
type binding struct { base }
func Build() { var port enrollment.Writer = binding{}; enrollment.Run(port) }
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
	t.Fatal("embedded interface, promoted implementation or method value hid a command")
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
