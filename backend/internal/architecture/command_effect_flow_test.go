package architecture

import (
	"encoding/json"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// fixtureEffectTargets returns the targets discovered for the fixture's ports.
func fixtureEffectTargets(t *testing.T, project string, policy *Policy) []string {
	t.Helper()
	loaded, err := loadTypedPackages(project, policy.Build)
	if err != nil {
		t.Fatal(err)
	}
	unique := map[string]bool{}
	for _, site := range discoverCommandEffects(policy, loaded) {
		unique[site.SourceOwner+": "+strings.TrimPrefix(site.Target, "example.test/effects/")] = true
	}
	targets := []string{}
	for target := range unique {
		targets = append(targets, target)
	}
	sort.Strings(targets)
	return targets
}

func requireFixtureTargets(t *testing.T, project string, policy *Policy, want ...string) {
	t.Helper()
	got := fixtureEffectTargets(t, project, policy)
	sort.Strings(want)
	if strings.Join(got, "\n") != strings.Join(want, "\n") {
		t.Fatalf("targets:\n  %s\nwant:\n  %s", strings.Join(got, "\n  "), strings.Join(want, "\n  "))
	}
}

func TestCommandEffectsResolveUnexportedRootHelperMethods(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding struct { target foreign.Ledger }
func (b binding) Record() { b.write() }
func (b binding) write() { b.target.Record() }
func Build() { enrollment.Run(binding{}) }
`)
	requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
}

func TestCommandEffectsResolveRootCallables(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, declarations, body, wiring string }{
		{"closure field", "type binding struct { command func() }", "b.command()", "enrollment.Run(binding{command: func() { foreign.Ledger{}.Record() }})"},
		{"named callable conversion", "type Command func(); type binding struct { command func() }", "b.command()", "enrollment.Run(binding{command: Command(foreign.Ledger{}.Record)})"},
		{"method value field", "type binding struct { command func() }", "b.command()", "enrollment.Run(binding{command: foreign.Ledger{}.Record})"},
		{"constructor parameter", "type binding struct { command func() }; func makeBinding(command func()) binding { return binding{command: command} }", "b.command()", "enrollment.Run(makeBinding(func() { foreign.Ledger{}.Record() }))"},
		{"callable variable", "type binding struct { command func() }", "command := b.command; command()", "enrollment.Run(binding{command: foreign.Ledger{}.Record})"},
		{"root helper", "type binding struct { command func() }; func helper() { foreign.Ledger{}.Record() }", "b.command()", "enrollment.Run(binding{command: helper})"},
		{"container element", "type binding struct { command func() }", "b.command()", "commands := []func(){foreign.Ledger{}.Record}; enrollment.Run(binding{command: commands[0]})"},
		{"enclosing parameter", "type binding struct { command func() }; func wire(command func()) { enrollment.Run(binding{command: func() { command() }}) }", "b.command()", "wire(foreign.Ledger{}.Record)"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\n"+tc.declarations+"\nfunc (b binding) Record() { "+tc.body+" }\nfunc Build() { "+tc.wiring+" }\n")
			requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
		})
	}
}

func TestCommandEffectsKeepUntraceableCallablesUnresolved(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type binding struct { command func() }
func (b binding) Record() { b.command() }
// Build's caller lies outside the typed build: its argument is unknown.
func Build(command func()) {
 enrollment.Run(binding{command: command})
 enrollment.Run(binding{command: foreign.Ledger{}.Record})
}
`)
	requireFixtureTargets(t, project, policy,
		"enrollment: foreign.Ledger.Record",
		"enrollment: unresolved:function:example.test/effects/roots.binding.command")
}

// M3 (ADR 0094): the scan used to stop at a concrete receiver of the source
// owner, so a root closure stored in a domain-owned function field was never
// inventoried.
func TestCommandEffectsResolveRootClosuresBehindDomainOwnedFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, domain, wiring string }{
		{"constructor field", "type reads struct { list func() }; func NewReads(list func()) Writer { return reads{list: list} }; func (r reads) Record() { r.list() }",
			"enrollment.Run(enrollment.NewReads(func() { foreign.Ledger{}.Record() }))"},
		{"constructor alias", "type reads struct { list func() }; func NewReads(list func()) Writer { saved := list; return reads{list: saved} }; func (r reads) Record() { r.list() }",
			"enrollment.Run(enrollment.NewReads(foreign.Ledger{}.Record))"},
		{"forwarding helper", "type reads struct { list func() }; func NewReads(list func()) Writer { return reads{list: list} }; func (r reads) Record() { r.forward() }; func (r reads) forward() { call(r.list) }; func call(f func()) { f() }",
			"enrollment.Run(enrollment.NewReads(foreign.Ledger{}.Record))"},
		{"embedded provider", "type inner struct { list func() }; func (i inner) Record() { i.list() }; type reads struct { inner }; func NewReads(list func()) Writer { return reads{inner{list: list}} }",
			"enrollment.Run(enrollment.NewReads(func() { foreign.Ledger{}.Record() }))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			writeFile(t, filepath.Join(project, "enrollment", "reads.go"), "package enrollment\n"+tc.domain+"\n")
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\nfunc Build() { "+tc.wiring+" }\n")
			requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
		})
	}
}

func TestCommandEffectsDomainOwnedFieldClosureChangeNeedsReview(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "reads.go"), "package enrollment\ntype reads struct { list func() }\nfunc NewReads(list func()) Writer { return reads{list: list} }\nfunc (r reads) Record() { r.list() }\n")
	filename := filepath.Join(project, "roots", "binding.go")
	source := "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\nfunc Build() { enrollment.Run(enrollment.NewReads(func() { foreign.Ledger{}.Record() })) }\n"
	writeFile(t, filename, source)
	reviewGapFixture(t, project, policy, "mutation", "debt", "0083")
	base := commitGapFixture(t, project)
	writeFile(t, filename, strings.Replace(source, "func() { foreign.Ledger{}.Record() }", "func() { foreign.Ledger{}.Record() /* comment only */ }", 1))
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	if err := CompareCommandEffects(project, base); err != nil {
		t.Fatalf("comment-only edit needs no review: %v", err)
	}
	writeFile(t, filename, strings.Replace(source, "foreign.Ledger{}.Record() }", "foreign.Ledger{}.Record(); foreign.Ledger{}.Record() }", 1))
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	requireGapBaseFailure(t, project, base, "exception binding changed without a new approval_adr")
}

// Generic flow: instantiated objects are distinct go/types objects. Bodies,
// callable values and type-parameter bindings resolve through their origin.
func TestCommandEffectsResolveGenericFlow(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, declarations, wiring string }{
		{"generic adapter method", "type binding[T any] struct { target foreign.Ledger }; func (b binding[T]) Record() { b.target.Record() }",
			"enrollment.Run(binding[int]{})"},
		{"generic root helper", "type binding struct{}; func (binding) Record() { write[int]() }; func write[T any]() { foreign.Ledger{}.Record() }",
			"enrollment.Run(binding{})"},
		{"generic callable field", "type binding[T any] struct { command func() }; func (b binding[T]) Record() { b.command() }",
			"enrollment.Run(binding[int]{command: foreign.Ledger{}.Record})"},
		{"generic constructor parameter", "type binding[T any] struct { command func() }; func (b binding[T]) Record() { b.command() }; func makeBinding[T any](command func()) binding[T] { return binding[T]{command: command} }",
			"enrollment.Run(makeBinding[int](foreign.Ledger{}.Record))"},
		{"type parameter port", "type binding struct { writer enrollment.Writer }; func (b binding) Record() { b.writer.Record() }; func wrap[W enrollment.Writer](writer W) enrollment.Writer { return binding{writer: writer} }",
			"enrollment.Run(wrap(foreign.Ledger{}))"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\n"+tc.declarations+"\nfunc Build() { "+tc.wiring+" }\n")
			requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
		})
	}
}

func TestCommandEffectsForwardingCyclesAddNoDiagnostic(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
type forward struct { next enrollment.Writer }
func (f forward) Record() { f.next.Record() }
func Build() {
 enrollment.Run(forward{next: forward{next: foreign.Ledger{}}})
}
`)
	requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
}

// platformEffectFixture adds a platform owner whose outbox a root binds.
func platformEffectFixture(t *testing.T) (string, *Policy) {
	t.Helper()
	project, policy := commandEffectGateFixture(t)
	policy.Owners = append(policy.Owners, Owner{ID: "outbox", Kind: "platform"})
	policy.Packages = append(policy.Packages, Package{Path: "outbox", Owner: "outbox", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"})
	policy.Rules = append(policy.Rules, Rule{ID: "root-outbox", SourceOwner: "root-composition", SourceRole: "compose", TargetOwner: "outbox", TargetRole: "public", Scopes: []string{"production"}, Description: "The fixture root binds its platform outbox."})
	blob, err := json.Marshal(policy)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(project, "architecture", "policy.json"), string(blob))
	writeFile(t, filepath.Join(project, "outbox", "outbox.go"), "package outbox\ntype Queue struct{}\nfunc (Queue) Record() {}\n")
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
 "example.test/effects/outbox"
)
func Build() { _ = foreign.Ledger{}; enrollment.Run(outbox.Queue{}) }
`)
	return project, policy
}

func TestCommandEffectsInventoryPlatformTargets(t *testing.T) {
	t.Parallel()
	project, policy := platformEffectFixture(t)
	requireFixtureTargets(t, project, policy, "enrollment: outbox.Queue.Record")
}

func TestCommandEffectsPlatformClassificationIsReservedForPlatformProtocols(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, effect, classification, adr string
		platform, valid                   bool
	}{
		{"platform durable", "durable", "platform", "0098", true, true},
		{"platform scope", "scope", "platform", "0098", true, true},
		{"platform read", "query", "query", "", true, true},
		{"platform command as leaf", "durable", "leaf", "0067", true, false},
		{"platform read as platform", "query", "platform", "0098", true, false},
		{"platform without its ADR", "durable", "platform", "0067", true, false},
		{"domain command as platform", "mutation", "platform", "0098", false, false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			if tc.platform {
				project, policy = platformEffectFixture(t)
			}
			reviewGapFixture(t, project, policy, tc.effect, tc.classification, tc.adr)
			graph, err := LoadGraph(project, policy)
			if err != nil {
				t.Fatal(err)
			}
			violations := Check(policy, graph)
			if (len(violations) == 0) != tc.valid {
				t.Fatalf("valid=%t, violations=%v", tc.valid, violations)
			}
		})
	}
}

func TestCommandEffectsNewPlatformEntryNeedsReviewNotFreshApproval(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nfunc Build() {}\n")
	base := commitGapFixture(t, project)
	platformProject, _ := platformEffectFixture(t)
	for _, file := range []string{"architecture/policy.json", "outbox/outbox.go", "roots/binding.go"} {
		writeFile(t, filepath.Join(project, file), readFile(t, filepath.Join(platformProject, file)))
	}
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(project, "architecture", "policy.json"))), policy); err != nil {
		t.Fatal(err)
	}
	reviewGapFixture(t, project, policy, "durable", "platform", "0098")
	if err := CompareCommandEffects(project, base); err != nil {
		t.Fatalf("reviewed platform entry rejected: %v", err)
	}
	manifest, err := loadCommandEffects(project)
	if err != nil {
		t.Fatal(err)
	}
	manifest.Entries[0].Classification = "leaf"
	manifest.Entries[0].ADR = "0067"
	blob, err := marshalCommandEffects(manifest)
	if err != nil {
		t.Fatal(err)
	}
	if err := artifacts.WriteInventory(filepath.Join(project, commandEffectInventory), artifacts.CommandEffects, blob); err != nil {
		t.Fatal(err)
	}
	err = RunCLI([]string{"check", "--project", project, "--baseline", "architecture/legacy.jsonl", "--base-ref", base}, CLIDependencies{})
	if err == nil || !strings.Contains(err.Error(), "platform classification") {
		t.Fatalf("platform command relabelled as leaf = %v", err)
	}
}

func TestCommandEffectsOwnUntraceableCallablesAreNotBindings(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "reads.go"), `package enrollment
type reads struct { now, list, unset func() }
func clock() func() { return func() {} }
func NewReads(list func()) Writer { return reads{now: clock(), list: list} }
func (r reads) Record() { r.now(); r.unset(); r.list() }
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
// Build's caller lies outside the typed build: external is untraceable.
func Build(external func()) {
 enrollment.Run(enrollment.NewReads(foreign.Ledger{}.Record))
 enrollment.Run(enrollment.NewReads(external))
}
`)
	requireFixtureTargets(t, project, policy,
		"enrollment: foreign.Ledger.Record",
		"enrollment: unresolved:function:example.test/effects/enrollment.reads.list")
}

func TestCommandEffectsResolveCallableFactories(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, declarations, wiring string }{
		{"root factory", "func optional(command func()) func() { if command == nil { return func() {} }; return command }", "enrollment.Run(binding{command: optional(foreign.Ledger{}.Record)})"},
		{"tuple factory", "func factory() (func(), error) { return foreign.Ledger{}.Record, nil }", "command, _ := factory(); enrollment.Run(binding{command: command})"},
		{"named result", "func factory() (command func()) { command = foreign.Ledger{}.Record; return }", "enrollment.Run(binding{command: factory()})"},
		{"tuple forwarding", "func inner() (error, func()) { return nil, foreign.Ledger{}.Record }; func factory() (error, func()) { return inner() }", "_, command := factory(); enrollment.Run(binding{command: command})"},
		{"recursive factory", "func factory(n int) func() { if n == 0 { return foreign.Ledger{}.Record }; return factory(n-1) }", "enrollment.Run(binding{command: factory(2)})"},
		{"mutually recursive factories", "func factory(n int) func() { if n == 0 { return foreign.Ledger{}.Record }; return inner(n-1) }; func inner(n int) func() { return factory(n) }", "enrollment.Run(binding{command: factory(2)})"},
		{"foreign factory", "", "enrollment.Run(binding{command: foreign.Recorder()})"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			writeFile(t, filepath.Join(project, "foreign", "factory.go"), "package foreign\nfunc Recorder() func() { return Ledger{}.Record }\n")
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\ntype binding struct { command func() }\nfunc (b binding) Record() { b.command() }\n"+tc.declarations+"\nfunc Build() { "+tc.wiring+" }\n")
			want := "enrollment: foreign.Ledger.Record"
			if tc.name == "foreign factory" {
				want = "enrollment: foreign.Recorder"
			}
			requireFixtureTargets(t, project, policy, want)
		})
	}
}

func TestCommandEffectsPlatformFunctionsNeedAProtocolSignature(t *testing.T) {
	t.Parallel()
	project, policy := platformEffectFixture(t)
	writeFile(t, filepath.Join(project, "outbox", "runtime.go"), `package outbox
import "context"
func Within(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }
func Lock(ctx context.Context, key string) error { return nil }
func MarkRollback(ctx context.Context) {}
func FromContext(ctx context.Context) int64 { return 0 }
func Observe(operation string, err error) {}
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "context"
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
 "example.test/effects/outbox"
)
type binding struct{}
func (binding) Record() {
 ctx := context.Background()
 _ = outbox.Within(ctx, func(context.Context) error { return outbox.Lock(ctx, "k") })
 outbox.MarkRollback(ctx)
 _ = outbox.FromContext(ctx)
 outbox.Observe("record", nil)
}
func Build() { _ = foreign.Ledger{}; enrollment.Run(binding{}) }
`)
	requireFixtureTargets(t, project, policy, "enrollment: outbox.Lock", "enrollment: outbox.MarkRollback", "enrollment: outbox.Within")
}

// A shared own runner must not hand every port the closures of all others.
func TestCommandEffectsOwnRunnerArgumentsStayWithTheirCaller(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "port.go"), `package enrollment
type Writer interface { Record() }
type Reader interface { Read() }
func Run(writer Writer) { writer.Record() }
func Read(reader Reader) { reader.Read() }
`)
	writeFile(t, filepath.Join(project, "enrollment", "service.go"), `package enrollment
type service struct { write, read func() }
func NewService(write, read func()) interface{ Writer; Reader } { return service{write: write, read: read} }
func run(callback func()) { callback() }
func forward(callback func()) { run(callback) }
func (s service) Record() { run(func() { s.write() }) }
func (s service) Read() { forward(s.read) }
`)
	writeFile(t, filepath.Join(project, "foreign", "ledger.go"), "package foreign\ntype Ledger struct{}\nfunc (Ledger) Record() {}\nfunc (Ledger) Read() {}\n")
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
func Build() {
 svc := enrollment.NewService(foreign.Ledger{}.Record, foreign.Ledger{}.Read)
 enrollment.Run(svc)
 enrollment.Read(svc)
}
`)
	loaded, err := loadTypedPackages(project, policy.Build)
	if err != nil {
		t.Fatal(err)
	}
	got := map[string]bool{}
	for _, site := range discoverCommandEffects(policy, loaded) {
		got[strings.TrimPrefix(site.Port, "example.test/effects/")+" -> "+strings.TrimPrefix(site.Target, "example.test/effects/")] = true
	}
	want := map[string]bool{"enrollment.Writer.Record -> foreign.Ledger.Record": true, "enrollment.Reader.Read -> foreign.Ledger.Read": true}
	for edge := range want {
		if !got[edge] {
			t.Errorf("missing %s in %v", edge, got)
		}
	}
	for edge := range got {
		if !want[edge] && !strings.Contains(edge, "interface{") {
			t.Errorf("imprecise %s", edge)
		}
	}
}

func TestCommandEffectsResolveCallableContainerFields(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, domain, roots string }{
		{"domain slice field", "type reads struct { list []func() }; func NewReads(list ...func()) Writer { return reads{list: list} }; func (r reads) Record() { for _, f := range r.list { f() } }",
			"type unused struct{}\nfunc Build() { enrollment.Run(enrollment.NewReads(func() { foreign.Ledger{}.Record() })) }"},
		{"domain map field", "type reads struct { list map[string]func() }; func NewReads(list map[string]func()) Writer { return reads{list: list} }; func (r reads) Record() { r.list[\"write\"]() }",
			"type unused struct{}\nfunc Build() { enrollment.Run(enrollment.NewReads(map[string]func(){\"write\": foreign.Ledger{}.Record})) }"},
		{"root slice field", "",
			"type binding struct { commands []func() }\nfunc (b binding) Record() { b.commands[0]() }\nfunc Build() { enrollment.Run(binding{commands: []func(){foreign.Ledger{}.Record}}) }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			if tc.domain != "" {
				writeFile(t, filepath.Join(project, "enrollment", "reads.go"), "package enrollment\n"+tc.domain+"\n")
			}
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\n"+tc.roots+"\n")
			requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
		})
	}
}

// A same-owner interface field supplied by the root is its own port: the
// domain struct that holds it does not hide the foreign implementation.
func TestCommandEffectsSameOwnerInterfaceProviderBehindDomainField(t *testing.T) {
	t.Parallel()
	project, policy := commandEffectGateFixture(t)
	writeFile(t, filepath.Join(project, "enrollment", "reads.go"), `package enrollment
type Ledger interface { Record() }
type reads struct { ledger Ledger }
func NewReads(ledger Ledger) Writer { return reads{ledger: ledger} }
func (r reads) Record() { r.ledger.Record() }
`)
	writeFile(t, filepath.Join(project, "roots", "binding.go"), `package roots
import (
 "example.test/effects/enrollment"
 "example.test/effects/foreign"
)
func Build() { enrollment.Run(enrollment.NewReads(foreign.Ledger{})) }
`)
	requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
}

// A diagnostic that stays unresolved still fingerprints every assignment: a
// changed traceable closure beside an untraceable source needs fresh approval.
func TestCommandEffectsUnresolvedCallableEvidenceChangeNeedsApproval(t *testing.T) {
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
func Build(external func()) {
 enrollment.Run(binding{command: external})
 enrollment.Run(binding{command: func() { foreign.Ledger{}.Record() }})
}
`
	writeFile(t, filename, source)
	reviewGapFixture(t, project, policy, "mutation", "debt", "0083")
	base := commitGapFixture(t, project)
	writeFile(t, filename, strings.Replace(source, "foreign.Ledger{}.Record() }", "foreign.Ledger{}.Record(); foreign.Ledger{}.Record() }", 1))
	if err := GenerateCommandEffects(project, policy); err != nil {
		t.Fatal(err)
	}
	err := CompareCommandEffects(project, base)
	if err == nil || !strings.Contains(err.Error(), "exception binding changed without a new approval_adr: enrollment|example.test/effects/enrollment.Writer.Record|example.test/effects/roots.binding.Record|unresolved:function:") {
		t.Fatalf("changed evidence of an unresolved diagnostic = %v", err)
	}
}

// quorum round 2 (#4879): each case used to report success without either the
// foreign terminal or an unresolved diagnostic.
func TestCommandEffectsResolveForwardedAndMethodCallables(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct{ name, domain, roots string }{
		{"constructor forwarding", "type reads struct { list func() }; func NewReads(list func()) Writer { return newReads(list) }; func newReads(list func()) Writer { return reads{list: list} }; func (r reads) Record() { r.list() }",
			"func Build() { enrollment.Run(enrollment.NewReads(foreign.Ledger{}.Record)) }"},
		{"own named callable", "var hook func(); func SetHook(f func()) { hook = f }; func signal() { hook() }; type reads struct { command func() }; func NewReads() Writer { return reads{command: signal} }; func (r reads) Record() { r.command() }",
			"func Build() { enrollment.SetHook(foreign.Ledger{}.Record); enrollment.Run(enrollment.NewReads()) }"},
		{"own wrapping factory", "func wrap(f func()) func() { return func() { f() } }; type reads struct { list func() }; func NewReads(list func()) Writer { return reads{list: wrap(list)} }; func (r reads) Record() { r.list() }",
			"func Build() { enrollment.Run(enrollment.NewReads(foreign.Ledger{}.Record)) }"},
		{"root closure forwarding to own function", "func Invoke(f func()) { f() }",
			"type binding struct { command func() }\nfunc (b binding) Record() { b.command() }\nfunc make(f func()) func() { return func() { enrollment.Invoke(f) } }\nfunc Build() { enrollment.Run(binding{command: make(foreign.Ledger{}.Record)}) }"},
		{"root factory method", "",
			"type factory struct{}\nfunc (factory) Make(f func()) func() { return f }\ntype binding struct { command func() }\nfunc (b binding) Record() { b.command() }\nfunc Build() { enrollment.Run(binding{command: factory{}.Make(foreign.Ledger{}.Record)}) }"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			project, policy := commandEffectGateFixture(t)
			if tc.domain != "" {
				writeFile(t, filepath.Join(project, "enrollment", "reads.go"), "package enrollment\n"+tc.domain+"\n")
			}
			writeFile(t, filepath.Join(project, "roots", "binding.go"), "package roots\nimport (\"example.test/effects/enrollment\"; \"example.test/effects/foreign\")\n"+tc.roots+"\n")
			requireFixtureTargets(t, project, policy, "enrollment: foreign.Ledger.Record")
		})
	}
}
