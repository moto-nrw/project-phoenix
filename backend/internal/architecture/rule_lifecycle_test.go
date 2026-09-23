package architecture

import (
	"strings"
	"testing"
)

func TestRuleIssueFixtures(t *testing.T) {
	t.Parallel()
	if _, err := LoadPolicy(fixturePath(t, "rule-issue-valid.json")); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadPolicy(fixturePath(t, "rule-issue-empty.json")); err == nil {
		t.Fatal("empty cleanup issue accepted")
	}
}

func TestExternalRuleLivenessAcrossScopes(t *testing.T) {
	t.Parallel()
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest, ScopeExternalTest} {
		t.Run(string(scope), func(t *testing.T) {
			t.Parallel()
			policy, err := LoadPolicy(fixturePath(t, "rules-live.json"))
			if err != nil {
				t.Fatal(err)
			}
			policy.Rules[0].TargetRole = ""
			policy.Rules[0].SameOwner = false
			policy.Rules[0].TargetClass = "stdlib"
			policy.ExternalClasses = []string{"stdlib"}
			policy.ExternalPackages = []ExternalPackage{{Path: "fmt", Class: "stdlib"}}
			if err := policy.Validate(); err != nil {
				t.Fatal(err)
			}
			graph := &Graph{Packages: []string{"example.test/architecture-fixture/linuxonly", "example.test/architecture-fixture/source", "example.test/architecture-fixture/target"}, Edges: []Edge{{Scope: scope, Source: "example.test/architecture-fixture/source", Target: "fmt"}}}
			if got := Check(policy, graph); len(got) != 0 {
				t.Fatalf("live external rule: %+v", got)
			}
			graph.Edges = nil
			got := Check(policy, graph)
			if len(got) != 2 || got[0].Rule != "external.stale" || got[1].Rule != "rules.stale" {
				t.Fatalf("stale external rule: %+v", got)
			}
		})
	}
}

func TestCheckRuleLivenessAcrossScopes(t *testing.T) {
	t.Parallel()
	for _, scope := range []Scope{ScopeProduction, ScopeInternalTest, ScopeExternalTest} {
		t.Run(string(scope), func(t *testing.T) {
			t.Parallel()
			policy, err := LoadPolicy(fixturePath(t, "rules-live.json"))
			if err != nil {
				t.Fatal(err)
			}
			graph := &Graph{Packages: []string{"example.test/architecture-fixture/linuxonly", "example.test/architecture-fixture/source", "example.test/architecture-fixture/target"}, Edges: []Edge{{Scope: scope, Source: "example.test/architecture-fixture/source", Target: "example.test/architecture-fixture/target"}}}
			if got := Check(policy, graph); len(got) != 0 {
				t.Fatalf("live rule: %+v", got)
			}
			graph.Edges = nil
			got := Check(policy, graph)
			if len(got) != 1 || got[0].Rule != "rules.stale" || got[0].Source != "application-domain" || got[0].Target != "application-domain" {
				t.Fatalf("unused rule: %+v", got)
			}
		})
	}
}

func TestCheckStaleRuleFixture(t *testing.T) {
	t.Parallel()
	output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "rules-stale.json"))
	if err == nil || !strings.Contains(output, "|rules.stale|unused-domain|unused-domain") {
		t.Fatalf("stale fixture: %v\n%s", err, output)
	}
}

func TestRuleIssueValidation(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name, issue string
		valid       bool
	}{
		{"ordinary", "", true},
		{"temporary", "https://github.com/moto-nrw/project-phoenix/issues/2736", true},
		{"query", "https://github.com/moto-nrw/project-phoenix/issues/2736?x=1", false},
		{"fragment", "https://github.com/moto-nrw/project-phoenix/issues/2736#x", false},
		{"userinfo", "https://user@github.com/moto-nrw/project-phoenix/issues/2736", false},
		{"pull request", "https://github.com/moto-nrw/project-phoenix/pull/2736", false},
	} {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			document := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-allowed.json")), func(doc map[string]any) {
				doc["schema_version"] = 3
				if tc.issue != "" {
					doc["rules"].([]any)[0].(map[string]any)["issue"] = tc.issue
				}
			})
			_, err := DecodePolicy(strings.NewReader(document))
			if (err == nil) != tc.valid {
				t.Fatalf("DecodePolicy error = %v, valid = %v", err, tc.valid)
			}
			if !tc.valid {
				_, want := ParseGitHubIssue(tc.issue)
				if !strings.Contains(err.Error(), want.Error()) {
					t.Fatalf("error = %v, want %v", err, want)
				}
			}
		})
	}
}

func TestRuleIssueDoesNotChangePermission(t *testing.T) {
	t.Parallel()
	base, err := LoadPolicy(fixturePath(t, "vertical-allowed.json"))
	if err != nil {
		t.Fatal(err)
	}
	candidate, err := LoadPolicy(fixturePath(t, "vertical-allowed.json"))
	if err != nil {
		t.Fatal(err)
	}
	base.Rules[0].Issue = "https://github.com/moto-nrw/project-phoenix/issues/2725"
	candidate.Rules[0].Issue = "https://github.com/moto-nrw/project-phoenix/issues/2736"
	if err := comparePolicyStrictness(base, candidate, nil, nil, nil, nil, nil); err != nil {
		t.Fatal(err)
	}
}

func TestCheckSchema2BaseTransition(t *testing.T) {
	t.Parallel()
	old := strings.Replace(readFile(t, fixturePath(t, "vertical-allowed.json")), `"schema_version": 3`, `"schema_version": 2`, 1)
	repo, base := ratchetRepositoryWithPolicy(t, "", old)
	writeFile(t, repo+"/architecture/policy.json", readFile(t, fixturePath(t, "vertical-allowed.json")))
	if output, err := runRepositoryCheck(t, repo, base); err != nil {
		t.Fatalf("base transition: %v\n%s", err, output)
	}
	if _, err := DecodePolicy(strings.NewReader(old)); err == nil {
		t.Fatal("schema-2 candidate accepted")
	}
}

func TestStaleRulesCannotBecomeDebt(t *testing.T) {
	t.Parallel()
	entry := LegacyEntry{Violation: Violation{Scope: ScopeProduction, Rule: "rules.stale", Source: "example.test/source", Target: "example.test/target"}, Issue: "https://github.com/moto-nrw/project-phoenix/issues/2736"}
	if err := entry.Validate(); err == nil || !strings.Contains(err.Error(), "cannot be legacy debt") {
		t.Fatalf("stale debt accepted: %v", err)
	}
}

func TestTemporaryRuleRequiresNonemptyIssue(t *testing.T) {
	t.Parallel()
	for _, value := range []any{"", nil, " ", 2736} {
		document := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-allowed.json")), func(doc map[string]any) { doc["rules"].([]any)[0].(map[string]any)["issue"] = value })
		if _, err := DecodePolicy(strings.NewReader(document)); err == nil {
			t.Fatalf("accepted issue %#v", value)
		}
	}
	document := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-allowed.json")), func(doc map[string]any) { doc["rules"].([]any)[0].(map[string]any)["surprise"] = true })
	if _, err := DecodePolicy(strings.NewReader(document)); err == nil || !strings.Contains(err.Error(), `unknown field "surprise"`) {
		t.Fatalf("unknown rule field: %v", err)
	}
}
