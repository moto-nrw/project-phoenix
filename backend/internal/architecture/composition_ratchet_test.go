package architecture

import (
	"path/filepath"
	"strings"
	"testing"
)

func TestCompositionSurfaceRatchet(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct {
		name, path, base, candidate, want string
	}{
		{"service field", "services/factory.go", "type Factory struct { Old int }", "type Factory struct { Old, New int }", "production|field|services|Factory.New"},
		{"repository field", "database/repositories/factory.go", "type Factory struct {}", "type Factory struct { New int }", "production|field|database/repositories|Factory.New"},
		{"API embedded pointer", "api/api.go", "type API struct {}", "type API struct { *Dependency }", "production|field|api|API.Dependency"},
		{"embedded value", "services/factory.go", "type Factory struct {}", "type Factory struct { Dependency }", "production|field|services|Factory.Dependency"},
		{"removed field", "services/factory.go", "type Factory struct { Old int }", "type Factory struct {}", ""},
		{"unchanged embedded", "api/api.go", "type API struct { *Dependency }", "type API struct { *Dependency }", ""},
		{"pointer setter", "services/scheduler/worker.go", "type Worker struct { limit int }", "type Worker struct { limit int }; func (w *Worker) SetLimit(n int) { w.limit = n }", "production|setter|services/scheduler|Worker.SetLimit"},
		{"renamed setter", "modules/delivery/worker.go", "type Worker struct { limit int }; func (w *Worker) SetLimit(n int) { w.limit = n }", "type Worker struct { limit int }; func (w *Worker) Configure(n int) { w.limit = n }", "production|setter|modules/delivery|Worker.Configure"},
		{"value receiver", "services/scheduler/worker.go", "type Worker struct { limits map[string]int }", "type Worker struct { limits map[string]int }; func (w Worker) Configure(n int) { w.limits[\"retry\"] = n }", "production|setter|services/scheduler|Worker.Configure"},
		{"removed setter", "modules/delivery/worker.go", "type Worker struct { limit int }; func (w *Worker) Configure(n int) { w.limit = n }", "type Worker struct { limit int }", ""},
		{"constructor", "modules/delivery/worker.go", "type Worker struct { limit int }", "type Worker struct { limit int }; func New(n int) *Worker { return &Worker{limit:n} }", ""},
		{"query", "services/scheduler/worker.go", "type Worker struct { limit int }", "type Worker struct { limit int }; func (w *Worker) Limit(n int) int { return w.limit + n }", ""},
		{"test-only field", "api/api_test.go", "type API struct {}", "type API struct { New int }", "internal_test|field|api|API.New"},
		{"test-only setter", "services/scheduler/worker_test.go", "type Worker struct { limit int }", "type Worker struct { limit int }; func (w *Worker) Configure(n int) { w.limit = n }", "internal_test|setter|services/scheduler|Worker.Configure"},
		{"unchanged test helper", "services/scheduler/worker_test.go", "type Worker struct { limit int }; func (w *Worker) Configure(n int) { w.limit = n }", "type Worker struct { limit int }; func (w *Worker) Configure(n int) { w.limit = n }", ""},
		{"dependency alias", "services/service.go", "type Port interface{ Run() }; type Service struct { port Port }", "type Port interface{ Run() }; type Service struct { port Port }; func (s *Service) Wire(p Port) { dependency := p; s.port = dependency }", "production|setter|services|Service.Wire"},
		{"receiver alias", "services/service.go", "type Service struct { callback func() }", "type Service struct { callback func() }; func (s *Service) Wire(f func()) { target := s; target.callback = f }", "production|setter|services|Service.Wire"},
		{"constructed dependency", "services/service.go", "type Service struct { callback func() }", "type Service struct { callback func() }; func (s *Service) Wire() { s.callback = func() {} }", "production|setter|services|Service.Wire"},
		{"package wiring", "services/service.go", "type Service struct { callback func() }", "type Service struct { callback func() }; func Wire(s *Service, f func()) { s.callback = f }", "production|setter|services|Wire"},
		{"dependency bundle", "services/service.go", "type Deps struct { callback func() }; type Service struct { deps *Deps }", "type Deps struct { callback func() }; type Service struct { deps *Deps }; func (s *Service) Wire(d *Deps) { s.deps = d }", "production|setter|services|Service.Wire"},
		{"ordinary domain state", "modules/example/internal/domain/state.go", "type State struct { status string }", "type State struct { status string }; func (s *State) Advance(next string) { s.status = next }", ""},
		{"capability result state", "modules/example/service.go", "type Result struct { count int }; type Service struct { last Result }", "type Result struct { count int }; type Service struct { last Result }; func (s *Service) RecordResult(r Result) { s.last = r }", ""},
		{"scheduler task state", "services/scheduler/task.go", "type ScheduledTask struct { Running bool }", "type ScheduledTask struct { Running bool }; func (s *ScheduledTask) MarkRunning() { s.Running = true }", ""},
		{"aliased aggregate", "services/factory.go", "type Fields struct {}; type Factory = Fields", "type Fields struct { New int }; type Factory = Fields", "production|field|services|Factory.New"},
		{"whole receiver", "services/service.go", "type Service struct { callback func() }", "type Service struct { callback func() }; func (s *Service) Wire(f func()) { *s = Service{callback: f} }", "production|setter|services|Service.Wire"},
		{"aliased scalar configuration", "services/scheduler/worker.go", "type Worker struct { limit int }", "type RetryLimit = int; type Worker struct { limit RetryLimit }; func (w *Worker) Configure(n int) { w.limit = n }", "production|setter|services/scheduler|Worker.Configure"},
		{"defined scalar configuration", "modules/delivery/worker.go", "type Worker struct { limit int }", "type RetryLimit int; type Worker struct { limit RetryLimit }; func (w *Worker) Configure(n RetryLimit) { w.limit = n }", "production|setter|modules/delivery|Worker.Configure"},
		{"embedded dependency", "services/service.go", "type Port interface { Run() }; type Service struct { Port }", "type Port interface { Run() }; type Service struct { Port }; func (s *Service) Wire(p Port) { s.Port = p }", "production|setter|services|Service.Wire"},
		{"promoted dependency", "services/service.go", "type Port interface { Run() }; type Deps struct { port Port }; type Service struct { *Deps }", "type Port interface { Run() }; type Deps struct { port Port }; type Service struct { *Deps }; func (s *Service) Wire(p Port) { s.port = p }", "production|setter|services|Service.Wire"},
		{"promoted result state", "services/service.go", "type Result struct { count int }; type Service struct { Result }", "type Result struct { count int }; type Service struct { Result }; func (s *Service) Record(n int) { s.count = n }", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			path := filepath.Join(repo, tt.path)
			writeFile(t, path, "package fixture\n"+tt.base+"\n")
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "composition base")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, path, "package fixture\n"+tt.candidate+"\n")
			err := CompareCompositionSurface(repo, base)
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

func TestCompositionSurfaceCannotReintroduceRemovedDebt(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	path := filepath.Join(repo, "services", "factory.go")
	original := "package services\ntype Factory struct { Old int }; func (f *Factory) Configure(n int) { f.Old = n }\n"
	writeFile(t, path, original)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "existing debt")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, "package services\ntype Factory struct {}\n")
	if err := CompareCompositionSurface(repo, base); err != nil {
		t.Fatal(err)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "remove debt")
	base = strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, original)
	err := CompareCompositionSurface(repo, base)
	if err == nil || !strings.Contains(err.Error(), "Factory.Old") || !strings.Contains(err.Error(), "Factory.Configure") {
		t.Fatalf("reintroduced field and setter must fail: %v", err)
	}
}

func TestCompositionSurfaceTestDebtCannotAuthorizeProduction(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	source := "package services\ntype Factory struct { Old int }; func (f *Factory) Configure(n int) { f.Old = n }\n"
	writeFile(t, filepath.Join(repo, "services", "factory_test.go"), source)
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "test-only debt")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(repo, "services", "factory.go"), source)
	err := CompareCompositionSurface(repo, base)
	if err == nil || !strings.Contains(err.Error(), "production|field") || !strings.Contains(err.Error(), "production|setter") {
		t.Fatalf("test-only declarations authorized production growth: %v", err)
	}
}

func TestCheckRejectsCompositionGrowthWithoutManifestChange(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2583))
	policyPath := filepath.Join(repo, "architecture", "policy.json")
	policy := mutatePolicy(t, readFile(t, policyPath), func(document map[string]any) {
		document["packages"] = append(document["packages"].([]any), map[string]any{
			"path": "services", "owner": "module", "role": "application",
			"internal_test_role": "module-internal-test", "external_test_role": "module-behavior-test",
		})
	})
	writeFile(t, policyPath, policy)
	path := filepath.Join(repo, "services", "factory.go")
	writeFile(t, path, "package services\ntype Factory struct { Old int }\n")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "composition base")
	base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, path, "package services\ntype Factory struct { Old, New int }\n")
	output, err := runArchitecture(t, "check", "--project", repo, "--baseline", "architecture/legacy.jsonl", "--base-ref", base)
	if err == nil || !strings.Contains(output, "production|field|services|Factory.New") {
		t.Fatalf("CLI accepted field growth with unchanged manifests: %v\n%s", err, output)
	}
	writeFile(t, path, "package services\ntype Factory struct {}\n")
	output, err = runArchitecture(t, "check", "--project", repo, "--baseline", "architecture/legacy.jsonl", "--base-ref", base)
	if err != nil {
		t.Fatalf("CLI rejected field removal: %v\n%s", err, output)
	}
}

func TestCompositionSurfaceResolvesCrossFileDependenciesAndTestScopes(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, path, source, want string }{
		{"imported interface", "services/wiring.go", "package services\nimport p \"example.test/architecture-fixture/modules/example/ports\"\nfunc (s *Service) Configure(p p.Port) { s.port = p }", "production|setter|services|Service.Configure->Service.port"},
		{"same package test", "services/wiring_test.go", "package services\nfunc (s *Service) ConfigureForTest() { s.port = nil }", "internal_test|setter|services|Service.ConfigureForTest->Service.port"},
		{"external package test", "services/wiring_test.go", "package services_test\ntype Port interface { Run() }; type Service struct { port Port }; func (s *Service) Configure() { s.port = nil }", "external_test|setter|services|Service.Configure->Service.port"},
		{"logger pointer", "services/wiring.go", "package services\nimport \"log/slog\"\nfunc (s *Service) Configure(l *slog.Logger) { s.logger = l }", "production|setter|services|Service.Configure->Service.logger"},
		{"external interface", "services/wiring.go", "package services\nimport \"io\"\nfunc (s *Service) Configure(r io.Reader) { s.reader = r }", "production|setter|services|Service.Configure->Service.reader"},
		{"duration configuration", "services/wiring.go", "package services\nimport clock \"time\"\nfunc (w *Worker) Configure(d clock.Duration) { w.lease = d }", "production|setter|services|Worker.Configure->Worker.lease"},
		{"timestamp state", "services/wiring.go", "package services\nimport \"time\"\nfunc (s *Service) Record(t *time.Time) { s.last = t }", ""},
		{"domain result pointer", "services/wiring.go", "package services\nimport d \"example.test/architecture-fixture/modules/example/internal/domain\"\nfunc (s *Service) Record(r *d.Result) { s.result = r }", ""},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepository(t, legacyRecord(2583))
			writeFile(t, filepath.Join(repo, "modules/example/ports/port.go"), "package ports\ntype Port interface { Run() }\n")
			writeFile(t, filepath.Join(repo, "modules/example/internal/domain/result.go"), "package domain\nimport \"time\"\ntype Result struct { At *time.Time }\n")
			writeFile(t, filepath.Join(repo, "services/service.go"), "package services\nimport (\"time\"; \"io\"; \"log/slog\"; p \"example.test/architecture-fixture/modules/example/ports\"; d \"example.test/architecture-fixture/modules/example/internal/domain\")\ntype Alias = p.Port\ntype Service struct { port Alias; reader io.Reader; logger *slog.Logger; last *time.Time; result *d.Result }\ntype LeaseDuration = time.Duration\ntype Worker struct { lease LeaseDuration }\n")
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "dependency declarations")
			base := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			writeFile(t, filepath.Join(repo, tt.path), tt.source+"\n")
			err := CompareCompositionSurface(repo, base)
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

func TestCompositionRatchetWrapperFromGitHook(t *testing.T) {
	t.Parallel()
	root := filepath.Clean(filepath.Join(packageDir(t), "..", "..", ".."))
	gitDir := strings.TrimSpace(runGit(t, root, "rev-parse", "--absolute-git-dir"))
	output, err := runArchitectureWithEnv(t, map[string]string{"GIT_DIR": gitDir, "GIT_WORK_TREE": "."}, "check")
	if err != nil || !strings.Contains(output, "composition surface passed:") {
		t.Fatalf("hook-local Git environment broke base comparison: %v\n%s", err, output)
	}
}
