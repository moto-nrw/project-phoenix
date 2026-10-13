package architecture

import (
	"path/filepath"
	"testing"
)

// repositoryFactoryCutoverPolicies returns a base that classifies the root
// legacy composition exactly and a reviewed candidate one epoch later.
func repositoryFactoryCutoverPolicies(t *testing.T) (*Policy, *Policy) {
	t.Helper()
	base, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.PolicyEpoch = 35
	retained := repositoryFactoryPoint
	retained.Path = repositoryFactoryRetainedPath
	base.Packages = []Package{
		retained,
		{Path: "modules/auditlog/compose", Owner: "audit-platform", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
		{Path: "modules/settings/compose", Owner: "settings-platform", Role: "compose", InternalTestRole: "workflow-integration-test", ExternalTestRole: "module-behavior-test"},
	}
	base.Rules = []Rule{}
	base.ReadProjections = []ReadProjection{}
	base.DataObjects = []DataObject{}
	base.LegacyComposition = []LegacyReference{}
	base.Relocations = nil
	base.ExternalPackages = []ExternalPackage{}
	candidate := *base
	candidate.PolicyEpoch++
	candidate.Packages = append([]Package{}, base.Packages...)
	return base, &candidate
}

func repositoryFactoryCutoverPoint(owner, role string) Package {
	return Package{Owner: owner, Role: role, InternalTestRole: role, ExternalTestRole: role}
}

func TestRepositoryFactoryCutoverGrantsTheTwoCompositionPermissions(t *testing.T) {
	t.Parallel()
	base, candidate := repositoryFactoryCutoverPolicies(t)
	legacy := repositoryFactoryPoint
	auditCompose := repositoryFactoryCutoverPoint("audit-platform", "compose")
	settingsCompose := repositoryFactoryCutoverPoint("settings-platform", "compose")
	settingsPostgres := repositoryFactoryCutoverPoint("settings-platform", "postgres")
	if !repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, legacy, auditCompose) {
		t.Fatal("legacy composition cannot reach the Audit composition")
	}
	if !repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, settingsCompose, settingsPostgres) {
		t.Fatal("Settings composition cannot construct its Postgres adapter")
	}
	for _, scope := range []Scope{ScopeInternalTest, ScopeExternalTest} {
		if repositoryFactoryCutoverPermission(base, candidate, scope, legacy, auditCompose) ||
			repositoryFactoryCutoverPermission(base, candidate, scope, settingsCompose, settingsPostgres) {
			t.Fatalf("production permission lent to %s", scope)
		}
	}
}

func TestRepositoryFactoryCutoverRefusesOtherSources(t *testing.T) {
	t.Parallel()
	base, candidate := repositoryFactoryCutoverPolicies(t)
	auditCompose := repositoryFactoryCutoverPoint("audit-platform", "compose")
	settingsPostgres := repositoryFactoryCutoverPoint("settings-platform", "postgres")
	for _, source := range []Package{
		repositoryFactoryCutoverPoint("root-composition", "compose"),
		repositoryFactoryCutoverPoint("root-composition", "cli"),
		repositoryFactoryCutoverPoint("care-plan", "compose"),
		repositoryFactoryCutoverPoint("legacy-composition", "application"),
	} {
		if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, source, auditCompose) {
			t.Fatalf("%s/%s reached audit-platform/compose", source.Owner, source.Role)
		}
		if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, source, settingsPostgres) {
			t.Fatalf("%s/%s reached settings-platform/postgres", source.Owner, source.Role)
		}
	}
	if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, repositoryFactoryCutoverPoint("audit-platform", "compose"), settingsPostgres) {
		t.Fatal("a foreign composition reached settings-platform/postgres")
	}
}

func TestRepositoryFactoryCutoverRefusesOtherTargets(t *testing.T) {
	t.Parallel()
	base, candidate := repositoryFactoryCutoverPolicies(t)
	legacy := repositoryFactoryPoint
	for _, role := range []string{"public", "domain", "application", "postgres"} {
		if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, legacy, repositoryFactoryCutoverPoint("audit-platform", role)) {
			t.Fatalf("legacy composition reached audit-platform/%s", role)
		}
	}
	settingsCompose := repositoryFactoryCutoverPoint("settings-platform", "compose")
	for _, role := range []string{"application", "domain"} {
		if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, settingsCompose, repositoryFactoryCutoverPoint("settings-platform", role)) {
			t.Fatalf("Settings composition reached settings-platform/%s through the exception", role)
		}
	}
	if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, legacy, repositoryFactoryCutoverPoint("settings-platform", "postgres")) {
		t.Fatal("legacy composition reached settings-platform/postgres")
	}
}

func TestRepositoryFactoryCutoverRequiresAnchor(t *testing.T) {
	t.Parallel()
	legacy := repositoryFactoryPoint
	auditCompose := repositoryFactoryCutoverPoint("audit-platform", "compose")
	settingsCompose := repositoryFactoryCutoverPoint("settings-platform", "compose")
	settingsPostgres := repositoryFactoryCutoverPoint("settings-platform", "postgres")
	cases := map[string]func(base, candidate *Policy){
		"same epoch": func(_, candidate *Policy) { candidate.PolicyEpoch-- },
		"retained package missing": func(base, _ *Policy) {
			base.Packages = base.Packages[1:]
		},
		"retained package reclassified": func(base, _ *Policy) {
			base.Packages = append([]Package{}, base.Packages...)
			base.Packages[0].ExternalTestRole = "adapter-test"
		},
		"foreign module": func(_, candidate *Policy) { candidate.ModulePath = "example.com/other" },
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			t.Parallel()
			base, candidate := repositoryFactoryCutoverPolicies(t)
			mutate(base, candidate)
			if repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, legacy, auditCompose) ||
				repositoryFactoryCutoverPermission(base, candidate, ScopeProduction, settingsCompose, settingsPostgres) {
				t.Fatal("exception granted without its anchor")
			}
		})
	}
}
