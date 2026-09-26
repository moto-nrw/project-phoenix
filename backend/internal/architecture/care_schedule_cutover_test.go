package architecture

import (
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

// The historical permission is explicit: this fixture must keep exercising
// the retired adapter after the repository's own policy has removed it.
func careScheduleCutoverPolicy(t *testing.T) *Policy {
	t.Helper()
	p, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	p.PolicyEpoch = 17
	for _, retired := range []string{"inbound-schedules", "inbound-students"} {
		if !slices.ContainsFunc(p.Owners, func(owner Owner) bool { return owner.ID == retired }) {
			p.Owners = append(p.Owners, Owner{ID: retired, Kind: "inbound"})
		}
	}
	p.Packages = []Package{
		{Path: "api/students", Owner: "inbound-students", Role: "http", InternalTestRole: "adapter-test", ExternalTestRole: "adapter-test"},
		{Path: "modules/careplan/legacy/careschedule", Owner: "inbound-schedules", Role: "adapter", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan", Owner: "care-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/internal/application", Owner: "care-plan", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/compose", Owner: "care-plan", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/internal/ports", Owner: "care-plan", Role: "port", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/carerequests", Owner: "care-plan", Role: "contract", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/careplan/careplantest", Owner: "care-plan", Role: "test-support", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/mealplan", Owner: "meal-plan", Role: "public", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "internal/timezone", Owner: "legacy-shared", Role: "domain", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
		{Path: "sharedkernel/calendar", Owner: "shared-kernel", Role: "contract", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"},
	}
	p.Rules = []Rule{
		{ID: "students.legacy-schedules", Description: "Historical care-schedule consumer.", Scopes: []string{"production"}, SourceOwner: "inbound-students", SourceRole: "http", TargetOwner: "inbound-schedules", TargetRole: "adapter"},
		{ID: "schedules.legacy-calendar", Description: "Historical date wrapper.", Scopes: []string{"production"}, SourceOwner: "inbound-schedules", SourceRole: "adapter", TargetOwner: "legacy-shared", TargetRole: "domain"},
		{ID: "date-wrapper.calendar", Description: "Canonical date contract.", Scopes: []string{"production"}, SourceOwner: "legacy-shared", SourceRole: "domain", TargetOwner: "shared-kernel", TargetRole: "contract"},
	}
	p.ReadProjections = []ReadProjection{}
	p.DataObjects = []DataObject{}
	p.LegacyComposition = []LegacyReference{}
	p.Relocations = nil
	p.ExternalPackages = []ExternalPackage{}
	return p
}

func TestCareScheduleCutoverNativePublicReplacements(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, owner, role, target string
		scope                     Scope
	}{
		{"capacity error", "timetable-activities", "compose", "student-presence", ScopeProduction},
		{"timetable internal errors", "inbound-timetable", "module-internal-test", "timetable-activities", ScopeInternalTest},
		{"timetable behavior errors", "inbound-timetable", "module-behavior-test", "timetable-activities", ScopeExternalTest},
		{"enrollment timetable contracts", "enrollment", "module-behavior-test", "timetable-activities", ScopeExternalTest},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			base := careScheduleCutoverPolicy(t)
			base.Rules = append(base.Rules, Rule{ID: "historical.native-contract", Scopes: []string{string(tc.scope)}, SourceOwner: tc.owner, SourceRole: tc.role, TargetOwner: "inbound-schedules", TargetRole: "adapter"})
			candidate := *base
			candidate.PolicyEpoch++
			candidate.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool { return p.Owner == "inbound-schedules" })
			source := Package{Owner: tc.owner, Role: tc.role, InternalTestRole: tc.role, ExternalTestRole: tc.role}
			target := Package{Owner: tc.target, Role: "public"}
			if !careScheduleCutoverPermission(base, &candidate, tc.scope, source, target) {
				t.Fatal("native public replacement rejected")
			}
			for _, scope := range allScopes() {
				if scope != tc.scope && careScheduleCutoverPermission(base, &candidate, scope, source, target) {
					t.Fatalf("borrowed permission from %s into %s", tc.scope, scope)
				}
			}
			for _, role := range []string{"compose", "application", "port", "postgres", "test-support"} {
				other := target
				other.Role = role
				if careScheduleNativePublicReplacement(tc.scope, source, other) {
					t.Fatalf("replacement exposed %s", role)
				}
			}
			other := target
			other.Owner = "meal-plan"
			if careScheduleCutoverPermission(base, &candidate, tc.scope, source, other) {
				t.Fatal("replacement granted unrelated owner")
			}
			withoutPermission := *base
			withoutPermission.Rules = base.Rules[:len(base.Rules)-1]
			if careScheduleCutoverPermission(&withoutPermission, &candidate, tc.scope, source, target) {
				t.Fatal("replacement did not require historical permission")
			}
			candidate.PolicyEpoch = base.PolicyEpoch
			if careScheduleCutoverPermission(base, &candidate, tc.scope, source, target) {
				t.Fatal("replacement did not require reviewed epoch")
			}
			candidate.PolicyEpoch++
			candidate.Packages = base.Packages
			if careScheduleCutoverPermission(base, &candidate, tc.scope, source, target) {
				t.Fatal("replacement permitted incomplete retirement")
			}
		})
	}
}

func TestCareScheduleCutoverNativeArrivalFixtureConstruction(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		owner, role string
		scope       Scope
	}{
		{"inbound-timetable", "adapter-test", ScopeInternalTest},
		{"enrollment", "module-behavior-test", ScopeExternalTest},
	} {
		for _, owner := range []string{"timetable-activities"} {
			t.Run(tc.owner+"/"+owner, func(t *testing.T) {
				t.Parallel()
				base := careScheduleCutoverPolicy(t)
				base.Rules = append(base.Rules, Rule{ID: "historical.arrival-fixture", Scopes: []string{string(tc.scope)}, SourceOwner: tc.owner, SourceRole: tc.role, TargetOwner: "inbound-schedules", TargetRole: "adapter"})
				candidate := *base
				candidate.PolicyEpoch++
				candidate.Packages = slices.DeleteFunc(slices.Clone(base.Packages), func(p Package) bool { return p.Owner == "inbound-schedules" })
				source := Package{Owner: tc.owner, Role: tc.role, InternalTestRole: tc.role, ExternalTestRole: tc.role}
				target := Package{Owner: owner, Role: "compose"}
				if !careScheduleCutoverPermission(base, &candidate, tc.scope, source, target) {
					t.Fatal("native arrival fixture construction rejected")
				}
				for _, scope := range allScopes() {
					if scope != tc.scope && careScheduleCutoverPermission(base, &candidate, scope, source, target) {
						t.Fatalf("fixture permission leaked into %s", scope)
					}
				}
				for _, role := range []string{"application", "port", "postgres", "test-support"} {
					other := target
					other.Role = role
					if careScheduleCutoverPermission(base, &candidate, tc.scope, source, other) {
						t.Fatalf("fixture exposed %s", role)
					}
				}
				other := target
				other.Owner = "legacy-composition"
				if careScheduleCutoverPermission(base, &candidate, tc.scope, source, other) {
					t.Fatal("fixture granted root composition and its repository graph")
				}
				other.Owner = "meal-plan"
				if careScheduleCutoverPermission(base, &candidate, tc.scope, source, other) {
					t.Fatal("fixture granted unrelated composition")
				}
				candidate.PolicyEpoch = base.PolicyEpoch
				if careScheduleCutoverPermission(base, &candidate, tc.scope, source, target) {
					t.Fatal("fixture did not require reviewed epoch")
				}
				candidate.PolicyEpoch++
				base.Rules = base.Rules[:len(base.Rules)-1]
				if careScheduleCutoverPermission(base, &candidate, tc.scope, source, target) {
					t.Fatal("fixture did not require historical permission")
				}
			})
		}
	}
}

func TestCheckCareSchedulePublicCutover(t *testing.T) {
	t.Parallel()
	repo, baseRef, candidate := careScheduleCutoverRepository(t)
	writeStudentProjectionPolicy(t, repo, candidate)
	if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
		t.Fatalf("retired care-schedule consumer cannot use its owner: %v\n%s", err, output)
	}
}

func careScheduleCutoverRepository(t *testing.T, testScopes ...Scope) (string, string, *Policy) {
	t.Helper()
	repo := t.TempDir()
	base := careScheduleCutoverPolicy(t)
	for _, scope := range testScopes {
		base.Rules = append(base.Rules, Rule{ID: "students.tests.legacy", Description: "Historical fixture construction.", Scopes: []string{string(scope)}, SourceOwner: "inbound-students", SourceRole: "adapter-test", TargetOwner: "inbound-schedules", TargetRole: "adapter"})
		testPackage := "students"
		if scope == ScopeExternalTest {
			testPackage += "_test"
		}
		writeFile(t, filepath.Join(repo, "api/students/value_test.go"), "package "+testPackage+"\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule\"\nvar TestValue = capability.Value\n")
	}
	writeFile(t, filepath.Join(repo, "go.mod"), "module github.com/moto-nrw/project-phoenix\n\ngo 1.25\n")
	for _, pkg := range base.Packages {
		writeFile(t, filepath.Join(repo, pkg.Path, "value.go"), "package capability\nconst Value = 1\n")
	}
	writeFile(t, filepath.Join(repo, "api/students/value.go"), "package students\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan/legacy/careschedule\"\nvar Value = capability.Value\n")
	writeFile(t, filepath.Join(repo, "internal/timezone/value.go"), "package timezone\nimport calendar \"github.com/moto-nrw/project-phoenix/sharedkernel/calendar\"\nconst Value = calendar.Value\n")
	writeFile(t, filepath.Join(repo, "modules/careplan/legacy/careschedule/value.go"), "package capability\nimport timezone \"github.com/moto-nrw/project-phoenix/internal/timezone\"\nconst Value = timezone.Value\n")
	writeStudentProjectionPolicy(t, repo, base)
	writeFile(t, filepath.Join(repo, "architecture/legacy.jsonl"), "")
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "architecture-test@example.test")
	runGit(t, repo, "config", "user.name", "Architecture Test")
	runGit(t, repo, "config", "commit.gpgsign", "false")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "historical care-schedule consumer")
	baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))

	candidate := careScheduleCutoverPolicy(t)
	candidate.PolicyEpoch++
	candidate.Packages = slices.DeleteFunc(candidate.Packages, func(p Package) bool { return p.Owner == "inbound-schedules" })
	candidate.Rules[0].TargetOwner = "care-plan"
	candidate.Rules[0].TargetRole = "public"
	candidate.Rules = slices.Delete(candidate.Rules, 1, 2)
	for _, scope := range testScopes {
		candidate.Rules = append(candidate.Rules, Rule{ID: "students.tests.native", Description: "Native fixture construction.", Scopes: []string{string(scope)}, SourceOwner: "inbound-students", SourceRole: "adapter-test", TargetOwner: "care-plan", TargetRole: "compose"})
		testPackage := "students"
		if scope == ScopeExternalTest {
			testPackage += "_test"
		}
		writeFile(t, filepath.Join(repo, "api/students/value_test.go"), "package "+testPackage+"\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan/compose\"\nvar TestValue = capability.Value\n")
	}
	if err := os.Remove(filepath.Join(repo, "modules/careplan/legacy/careschedule/value.go")); err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(repo, "api/students/value.go"), "package students\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan\"\nvar Value = capability.Value\n")
	return repo, baseRef, candidate
}

func TestCheckCareScheduleCutoverReplacesTestConstruction(t *testing.T) {
	t.Parallel()
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		t.Run(string(scope), func(t *testing.T) {
			t.Parallel()
			repo, baseRef, candidate := careScheduleCutoverRepository(t, scope)
			writeStudentProjectionPolicy(t, repo, candidate)
			if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
				t.Fatalf("native fixture construction rejected: %v\n%s", err, output)
			}
		})
	}
}

func TestCheckCareScheduleCutoverUsesCanonicalCalendar(t *testing.T) {
	t.Parallel()
	repo, baseRef, candidate := careScheduleCutoverRepository(t)
	candidate.Rules = append(candidate.Rules, Rule{ID: "care-plan.public.calendar", Description: "Canonical calendar replaces the retired date wrapper.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "public", TargetOwner: "shared-kernel", TargetRole: "contract"})
	writeFile(t, filepath.Join(repo, "modules/careplan/value.go"), "package capability\nimport calendar \"github.com/moto-nrw/project-phoenix/sharedkernel/calendar\"\nconst Value = calendar.Value\n")
	writeStudentProjectionPolicy(t, repo, candidate)
	if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
		t.Fatalf("canonical calendar replacement rejected: %v\n%s", err, output)
	}
}

func TestCheckCareScheduleCutoverNativeContractComposition(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, sourcePath, sourceRole, targetPath, targetRole string
	}{
		{"compose contract", "modules/careplan/compose", "compose", "modules/careplan/carerequests", "contract"},
		{"application contract", "modules/careplan/internal/application", "application", "modules/careplan/carerequests", "contract"},
		{"port public", "modules/careplan/internal/ports", "port", "modules/careplan", "public"},
		{"port contract", "modules/careplan/internal/ports", "port", "modules/careplan/carerequests", "contract"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef, candidate := careScheduleCutoverRepository(t)
			candidate.Rules = append(candidate.Rules, Rule{ID: "care-plan.native.contract", Description: "Native implementation uses its owner's public contract.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: tc.sourceRole, TargetOwner: "care-plan", TargetRole: tc.targetRole})
			writeFile(t, filepath.Join(repo, tc.sourcePath, "value.go"), "package capability\nimport contract \"github.com/moto-nrw/project-phoenix/"+tc.targetPath+"\"\nconst Value = contract.Value\n")
			writeStudentProjectionPolicy(t, repo, candidate)
			if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
				t.Fatalf("native owner contract rejected: %v\n%s", err, output)
			}
		})
	}
}

func TestCheckCareScheduleCutoverComposeUsesCanonicalCalendar(t *testing.T) {
	t.Parallel()
	repo, baseRef, candidate := careScheduleCutoverRepository(t)
	candidate.Rules = append(candidate.Rules, Rule{ID: "care-plan.compose.calendar", Description: "Composition binds the canonical date contracts.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "compose", TargetOwner: "shared-kernel", TargetRole: "contract"})
	writeFile(t, filepath.Join(repo, "modules/careplan/compose/value.go"), "package capability\nimport calendar \"github.com/moto-nrw/project-phoenix/sharedkernel/calendar\"\nconst Value = calendar.Value\n")
	writeStudentProjectionPolicy(t, repo, candidate)
	if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
		t.Fatalf("composition calendar replacement rejected: %v\n%s", err, output)
	}
}

func TestCheckCareScheduleCalendarCutoverRejectsExtraKernelAccess(t *testing.T) {
	t.Parallel()
	repo, baseRef, candidate := careScheduleCutoverRepository(t)
	candidate.Packages = append(candidate.Packages, Package{Path: "sharedkernel/extra", Owner: "shared-kernel", Role: "contract", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"})
	candidate.Rules = append(candidate.Rules, Rule{ID: "care-plan.public.calendar", Description: "Must not grant another kernel package.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "public", TargetOwner: "shared-kernel", TargetRole: "contract"})
	writeFile(t, filepath.Join(repo, "sharedkernel/extra/value.go"), "package extra\nconst Value = 2\n")
	writeFile(t, filepath.Join(repo, "modules/careplan/value.go"), "package capability\nimport calendar \"github.com/moto-nrw/project-phoenix/sharedkernel/calendar\"\nconst Value = calendar.Value\n")
	writeStudentProjectionPolicy(t, repo, candidate)
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "architecture policy loosening") {
		t.Fatalf("calendar exception granted another kernel contract: %v\n%s", err, output)
	}
}

func TestCheckCareScheduleCutoverCannotBeReused(t *testing.T) {
	t.Parallel()
	repo, baseRef, candidate := careScheduleCutoverRepository(t)
	writeStudentProjectionPolicy(t, repo, candidate)
	if output, err := runRepositoryCheck(t, repo, baseRef); err != nil {
		t.Fatalf("initial cutover rejected: %v\n%s", err, output)
	}
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "retire care-schedule adapter")
	baseRef = strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	candidate.PolicyEpoch++
	candidate.Rules = append(candidate.Rules, Rule{ID: "care-plan.application.calendar", Description: "A later epoch cannot reuse the retired adapter's permission.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "application", TargetOwner: "shared-kernel", TargetRole: "contract"})
	writeFile(t, filepath.Join(repo, "modules/careplan/internal/application/value.go"), "package application\nimport calendar \"github.com/moto-nrw/project-phoenix/sharedkernel/calendar\"\nconst Value = calendar.Value\n")
	writeStudentProjectionPolicy(t, repo, candidate)
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "architecture policy loosening") {
		t.Fatalf("later epoch reused the cutover exception: %v\n%s", err, output)
	}
}

func TestCheckCareScheduleCutoverRejectsIncompleteRetirement(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name   string
		change func(*testing.T, string, *Policy)
		want   string
	}{
		{"no reviewed epoch", func(_ *testing.T, _ string, p *Policy) { p.PolicyEpoch-- }, "architecture policy loosening"},
		{"retained source", func(t *testing.T, repo string, _ *Policy) {
			writeFile(t, filepath.Join(repo, "modules/careplan/legacy/careschedule/value.go"), "package capability\nconst Value = 1\n")
		}, "packages.unclassified"},
		{"relocated adapter", func(t *testing.T, repo string, p *Policy) {
			p.Packages = append(p.Packages, Package{Path: "retained-schedules", Owner: "inbound-schedules", Role: "adapter", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"})
			writeFile(t, filepath.Join(repo, "retained-schedules/value.go"), "package capability\nconst Value = 1\n")
		}, "architecture policy loosening"},
		{"unrelated module", func(t *testing.T, repo string, p *Policy) {
			p.Rules[0].TargetOwner = "meal-plan"
			writeFile(t, filepath.Join(repo, "api/students/value.go"), "package students\nimport capability \"github.com/moto-nrw/project-phoenix/modules/mealplan\"\nvar Value = capability.Value\n")
		}, "architecture policy loosening"},
		{"private implementation", func(t *testing.T, repo string, p *Policy) {
			p.Rules[0].TargetRole = "application"
			p.Packages = append(p.Packages, Package{Path: "modules/careplan/implementation", Owner: "care-plan", Role: "application", InternalTestRole: "module-internal-test", ExternalTestRole: "module-behavior-test"})
			writeFile(t, filepath.Join(repo, "modules/careplan/implementation/value.go"), "package capability\nconst Value = 1\n")
			writeFile(t, filepath.Join(repo, "api/students/value.go"), "package students\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan/implementation\"\nvar Value = capability.Value\n")
		}, "architecture policy loosening"},
		{"owner-kind wildcard", func(_ *testing.T, _ string, p *Policy) {
			p.Rules[0].SourceOwner = ""
			p.Rules[0].SourceOwnerKind = "inbound"
		}, "architecture policy loosening"},
		{"borrowed test scope", func(t *testing.T, repo string, p *Policy) {
			p.Rules = append(p.Rules, Rule{ID: "students.tests.owner", Description: "Unapproved test scope.", Scopes: []string{"external_test"}, SourceOwner: "inbound-students", SourceRole: "adapter-test", TargetOwner: "care-plan", TargetRole: "public"})
			writeFile(t, filepath.Join(repo, "api/students/value_test.go"), "package students_test\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan\"\nvar Value = capability.Value\n")
		}, "architecture policy loosening"},
		{"borrowed test construction", func(t *testing.T, repo string, p *Policy) {
			p.Rules = append(p.Rules, Rule{ID: "students.tests.compose", Description: "Production permission cannot authorize test construction.", Scopes: []string{"external_test"}, SourceOwner: "inbound-students", SourceRole: "adapter-test", TargetOwner: "care-plan", TargetRole: "compose"})
			writeFile(t, filepath.Join(repo, "api/students/value_test.go"), "package students_test\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan/compose\"\nvar TestValue = capability.Value\n")
		}, "architecture policy loosening"},
		{"production construction", func(t *testing.T, repo string, p *Policy) {
			p.Rules[0].TargetRole = "compose"
			writeFile(t, filepath.Join(repo, "api/students/value.go"), "package students\nimport capability \"github.com/moto-nrw/project-phoenix/modules/careplan/compose\"\nvar Value = capability.Value\n")
		}, "architecture policy loosening"},
		{"native composition private access", func(t *testing.T, repo string, p *Policy) {
			p.Rules = append(p.Rules, Rule{ID: "care-plan.compose.private", Description: "Private implementation access is not a contract cutover.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "compose", TargetOwner: "care-plan", TargetRole: "application"})
			writeFile(t, filepath.Join(repo, "modules/careplan/compose/value.go"), "package capability\nimport impl \"github.com/moto-nrw/project-phoenix/modules/careplan/internal/application\"\nconst Value = impl.Value\n")
		}, "architecture policy loosening"},
		{"native contract borrowed test scope", func(t *testing.T, repo string, p *Policy) {
			p.Rules = append(p.Rules, Rule{ID: "care-plan.tests.contract", Description: "Production composition does not authorize a new test permission.", Scopes: []string{"external_test"}, SourceOwner: "care-plan", SourceRole: "module-behavior-test", TargetOwner: "care-plan", TargetRole: "contract"})
			writeFile(t, filepath.Join(repo, "modules/careplan/value_test.go"), "package capability_test\nimport contract \"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests\"\nvar Value = contract.Value\n")
		}, "architecture policy loosening"},
		{"native test support contract", func(t *testing.T, repo string, p *Policy) {
			p.Rules = append(p.Rules, Rule{ID: "care-plan.testsupport.contract", Description: "Fixture placement cannot widen production-scope support permissions.", Scopes: []string{"production"}, SourceOwner: "care-plan", SourceRole: "test-support", TargetOwner: "care-plan", TargetRole: "contract"})
			writeFile(t, filepath.Join(repo, "modules/careplan/careplantest/value.go"), "package capability\nimport contract \"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests\"\nconst Value = contract.Value\n")
		}, "architecture policy loosening"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			repo, baseRef, candidate := careScheduleCutoverRepository(t)
			tc.change(t, repo, candidate)
			writeStudentProjectionPolicy(t, repo, candidate)
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, tc.want) {
				t.Fatalf("unsafe cutover accepted or rejected for the wrong reason: %v\n%s", err, output)
			}
		})
	}
}
