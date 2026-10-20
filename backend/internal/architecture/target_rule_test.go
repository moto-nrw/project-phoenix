package architecture

import (
	"slices"
	"strings"
	"testing"
)

// targetOnlyFixture decodes the fixture after mutate, so each case states its
// own deviation from a policy that is target-only as committed.
func targetOnlyFixture(t *testing.T, mutate func(map[string]any)) error {
	t.Helper()
	document := mutatePolicy(t, readFile(t, fixturePath(t, "target-only-rules.json")), func(document map[string]any) {
		document["policy_epoch"] = targetOnlyRulesEpoch
		mutate(document)
	})
	_, err := DecodePolicy(strings.NewReader(document))
	return err
}

func addFixtureRule(rule map[string]any) func(map[string]any) {
	return func(document map[string]any) {
		if _, ok := rule["description"]; !ok {
			rule["description"] = "Fixture rule."
		}
		if _, ok := rule["scopes"]; !ok {
			rule["scopes"] = []any{"production"}
		}
		document["rules"] = append(document["rules"].([]any), rule)
	}
}

func TestTargetOnlyFixtureIsValid(t *testing.T) {
	t.Parallel()
	if err := targetOnlyFixture(t, func(map[string]any) {}); err != nil {
		t.Fatalf("target-only fixture rejected: %v", err)
	}
}

func TestTargetOnlyRuleRejectsForeignPrivateRoles(t *testing.T) {
	t.Parallel()
	for _, role := range []string{"application", "domain", "adapter", "postgres", "cli", "legacy-model"} {
		t.Run(role, func(t *testing.T) {
			t.Parallel()
			err := targetOnlyFixture(t, addFixtureRule(map[string]any{
				"id": "alpha.application.beta-" + role, "source_owner": "alpha", "source_role": "application",
				"target_owner": "beta", "target_role": role,
			}))
			if err == nil || !strings.Contains(err.Error(), "private role is never target") {
				t.Fatalf("rule into beta/%s accepted: %v", role, err)
			}
		})
	}
	for _, scenario := range []struct {
		name string
		rule map[string]any
	}{
		{"owner-agnostic source", map[string]any{"id": "any.beta-application", "source_role": "compose", "target_owner": "beta", "target_role": "application"}},
		{"source by kind", map[string]any{"id": "domain.beta-application", "source_owner_kind": "domain", "source_role": "compose", "target_owner": "beta", "target_role": "application"}},
		{"target by kind", map[string]any{"id": "alpha.domain-application", "source_owner": "alpha", "source_role": "compose", "target_owner_kind": "domain", "target_role": "application"}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if err := targetOnlyFixture(t, addFixtureRule(scenario.rule)); err == nil || !strings.Contains(err.Error(), "private role is never target") {
				t.Fatalf("selector reaching another owner's application accepted: %v", err)
			}
		})
	}
}

func TestTargetOnlyRuleKeepsOwnPrivateRoles(t *testing.T) {
	t.Parallel()
	for _, rule := range []map[string]any{
		{"id": "beta.http.own-application", "source_owner": "beta", "source_role": "http", "target_owner": "beta", "target_role": "application"},
		{"id": "kind.compose.own-adapter", "source_owner_kind": "domain", "source_role": "compose", "target_role": "adapter", "same_owner": true},
		{"id": "beta.public.own-legacy-model", "source_owner": "beta", "source_role": "public", "target_owner": "beta", "target_role": "legacy-model"},
		{"id": "alpha.application.beta-contract", "source_owner": "alpha", "source_role": "application", "target_owner": "beta", "target_role": "contract"},
	} {
		if err := targetOnlyFixture(t, addFixtureRule(rule)); err != nil {
			t.Fatalf("%s rejected: %v", rule["id"], err)
		}
	}
}

func TestTargetOnlyRuleKeepsModelsPrivateToTheirOwner(t *testing.T) {
	t.Parallel()
	err := targetOnlyFixture(t, addFixtureRule(map[string]any{
		"id": "alpha.application.beta-models", "source_owner": "alpha", "source_role": "application", "target_owner": "beta", "target_role": "legacy-model",
	}))
	if err == nil || !strings.Contains(err.Error(), "legacy-model role of beta") {
		t.Fatalf("foreign rule onto beta's models accepted: %v", err)
	}
	for name, mutate := range map[string]func(map[string]any){
		"models package in the domain role": func(document map[string]any) {
			setFixturePackageRole(document, "models/beta", "domain")
		},
		"legacy-model outside models": func(document map[string]any) {
			setFixturePackageRole(document, "beta/application", "legacy-model")
		},
	} {
		if err := targetOnlyFixture(t, mutate); err == nil || !strings.Contains(err.Error(), "legacy-model") {
			t.Fatalf("%s accepted: %v", name, err)
		}
	}
}

func setFixturePackageRole(document map[string]any, path, role string) {
	for _, raw := range document["packages"].([]any) {
		if pkg := raw.(map[string]any); pkg["path"] == path {
			pkg["role"] = role
		}
	}
}

func TestTargetOnlyRuleKeepsPlatformsOffDomainOwners(t *testing.T) {
	t.Parallel()
	for _, target := range []string{"alpha", "board", "flow"} {
		t.Run(target, func(t *testing.T) {
			t.Parallel()
			err := targetOnlyFixture(t, addFixtureRule(map[string]any{
				"id": "runtime.application." + target + "-public", "source_owner": "runtime", "source_role": "application",
				"target_owner": target, "target_role": "public",
			}))
			if err == nil || !strings.Contains(err.Error(), "a platform depends on platforms") {
				t.Fatalf("platform rule into %s accepted: %v", target, err)
			}
		})
	}
	err := targetOnlyFixture(t, addFixtureRule(map[string]any{
		"id": "test.domain-public", "source_role": "module-behavior-test", "target_owner_kind": "domain", "target_role": "public",
		"scopes": []any{"external_test"},
	}))
	if err == nil || !strings.Contains(err.Error(), "a platform depends on platforms") {
		t.Fatalf("owner-agnostic rule that selects platform sources accepted: %v", err)
	}
	for _, rule := range []map[string]any{
		{"id": "domain.test.domain-public", "source_owner_kind": "domain", "source_role": "module-behavior-test", "target_owner_kind": "domain", "target_role": "public", "scopes": []any{"external_test"}},
		{"id": "runtime.application.clock-contract", "source_owner": "runtime", "source_role": "application", "target_owner": "clock", "target_role": "contract"},
		{"id": "alpha.application.runtime-public", "source_owner": "alpha", "source_role": "application", "target_owner": "runtime", "target_role": "public"},
	} {
		if err := targetOnlyFixture(t, addFixtureRule(rule)); err != nil {
			t.Fatalf("%s rejected: %v", rule["id"], err)
		}
	}
}

func TestTargetOnlyRuleRejectsTransitionMarkersAndIssues(t *testing.T) {
	t.Parallel()
	for _, description := range []string{
		"Compatibility permission: convert it to exact debt once the package exists at a base SHA.",
		"Convert to exact debt when the carrier lands.",
		"Compatibility binding of the retained coordinator.",
		"The live-group compatibility adapters return the public capability.",
		"Temporary reach of the retained composition.",
		"The retained implementation joins the tenant transaction until #2675.",
		"Owner HTTP adapter, tracked by #2706.",
	} {
		err := targetOnlyFixture(t, func(document map[string]any) {
			document["rules"].([]any)[0].(map[string]any)["description"] = description
		})
		if err == nil || !strings.Contains(err.Error(), "describes itself as temporary") {
			t.Fatalf("description %q accepted: %v", description, err)
		}
	}
	err := targetOnlyFixture(t, func(document map[string]any) {
		document["rules"].([]any)[0].(map[string]any)["issue"] = "https://github.com/moto-nrw/project-phoenix/issues/2729"
	})
	if err == nil || !strings.Contains(err.Error(), "carries issue") {
		t.Fatalf("rule issue accepted: %v", err)
	}
}

func TestTargetOnlyRuleAppliesFromItsEpoch(t *testing.T) {
	t.Parallel()
	older := func(document map[string]any) { document["policy_epoch"] = targetOnlyRulesEpoch - 1 }
	for name, mutate := range map[string]func(map[string]any){
		"foreign application":  addFixtureRule(map[string]any{"id": "alpha.application.beta-application", "source_owner": "alpha", "source_role": "application", "target_owner": "beta", "target_role": "application"}),
		"platform into domain": addFixtureRule(map[string]any{"id": "runtime.application.alpha-public", "source_owner": "runtime", "source_role": "application", "target_owner": "alpha", "target_role": "public"}),
		"issue": func(document map[string]any) {
			document["rules"].([]any)[0].(map[string]any)["issue"] = "https://github.com/moto-nrw/project-phoenix/issues/2729"
		},
		"marker": func(document map[string]any) {
			document["rules"].([]any)[0].(map[string]any)["description"] = "Compatibility permission until #2675."
		},
		"models in the domain role": func(document map[string]any) { setFixturePackageRole(document, "models/beta", "domain") },
	} {
		err := targetOnlyFixture(t, func(document map[string]any) {
			older(document)
			// An older base has no legacy-model role yet.
			setFixturePackageRole(document, "models/beta", "domain")
			document["rules"] = slices.DeleteFunc(document["rules"].([]any), func(raw any) bool {
				return raw.(map[string]any)["target_role"] == "legacy-model"
			})
			mutate(document)
		})
		if err != nil {
			t.Fatalf("%s rejected in an epoch before %d: %v", name, targetOnlyRulesEpoch, err)
		}
	}
}

func TestRuleTransitionMarkersAreAFixedList(t *testing.T) {
	t.Parallel()
	want := []string{"convert to exact debt", "compatibility permission", "compatibility binding", "compatibility adapter", "temporary", "until #", "tracked by #"}
	var got []string
	for _, marker := range TransitionMarkers {
		got = append(got, marker.Name)
	}
	if !slices.Equal(got, want) {
		t.Fatalf("transition markers = %v, want %v", got, want)
	}
	for _, scenario := range []struct {
		description string
		want        []string
	}{
		{"Compatibility permission: convert it to exact debt once the package exists at a base SHA.", []string{"convert to exact debt", "compatibility permission"}},
		{"Compatibility bindings of the coordinator.", []string{"compatibility binding"}},
		{"TEMPORARILY reached until #12.", []string{"temporary", "until #"}},
		{"Owner HTTP adapter, tracked by #2706.", []string{"tracked by #"}},
		{"The contemporary roster is compatible with the adapter.", nil},
		{"Kept until the carrier lands; the tracker issue is #2706.", nil},
		{"Converts the row. Exact debt stays in legacy.jsonl.", nil},
	} {
		if got := RuleTransitionMarkers(scenario.description); !slices.Equal(got, scenario.want) {
			t.Errorf("%q: markers = %v, want %v", scenario.description, got, scenario.want)
		}
	}
}

func TestPolicyEpochMayNotDecrease(t *testing.T) {
	t.Parallel()
	candidate := repositoryPolicy(t)
	base := *candidate
	base.PolicyEpoch++
	err := comparePolicyStrictness(&base, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil)
	if err == nil || !strings.Contains(err.Error(), "policy_epoch decreased") {
		t.Fatalf("lower epoch accepted: %v", err)
	}
	if err := comparePolicyStrictness(candidate, candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil); err != nil {
		t.Fatalf("unchanged epoch rejected: %v", err)
	}
}

func TestRepositoryPolicyStatesOnlyTheTarget(t *testing.T) {
	t.Parallel()
	policy := repositoryPolicy(t)
	if policy.PolicyEpoch < targetOnlyRulesEpoch {
		t.Fatalf("repository policy epoch %d predates the target-only rule epoch %d", policy.PolicyEpoch, targetOnlyRulesEpoch)
	}
	for _, pkg := range policy.Packages {
		if pkg.Path == "modules/identityaccess/sessiontoken" && pkg.Role != "http" {
			t.Fatalf("the session token runtime has role %q, want http (ADR 0057 point 3)", pkg.Role)
		}
	}
}

func TestTargetOnlyRuleKeepsCompositionGlueInTheRoot(t *testing.T) {
	t.Parallel()
	for _, scenario := range []struct {
		name string
		rule map[string]any
	}{
		{"domain composition into a foreign compose", map[string]any{"id": "alpha.compose.beta-compose", "source_owner": "alpha", "source_role": "compose", "target_owner": "beta", "target_role": "compose"}},
		{"domain composition into a foreign http", map[string]any{"id": "alpha.compose.beta-http", "source_owner": "alpha", "source_role": "compose", "target_owner": "beta", "target_role": "http"}},
		{"workflow composition", map[string]any{"id": "flow.compose.beta-compose", "source_owner": "flow", "source_role": "compose", "target_owner": "beta", "target_role": "compose"}},
		{"projection composition", map[string]any{"id": "board.compose.beta-http", "source_owner": "board", "source_role": "compose", "target_owner": "beta", "target_role": "http"}},
		{"platform composition", map[string]any{"id": "runtime.compose.clock-compose", "source_owner": "runtime", "source_role": "compose", "target_owner": "clock", "target_role": "compose"}},
		{"composition into a tool owner", map[string]any{"id": "alpha.compose.tool-http", "source_owner": "alpha", "source_role": "compose", "target_owner": "tools", "target_role": "http"}},
		{"any role of a domain", map[string]any{"id": "alpha.any.beta-compose", "source_owner": "alpha", "target_owner": "beta", "target_role": "compose"}},
		{"compositions by kind", map[string]any{"id": "domain.compose.domain-compose", "source_owner_kind": "domain", "source_role": "compose", "target_owner_kind": "domain", "target_role": "compose"}},
		{"composition into a root in production", map[string]any{"id": "alpha.compose.root-compose", "source_owner": "alpha", "source_role": "compose", "target_owner": "root", "target_role": "compose"}},
		{"any role into a root in production and test", map[string]any{"id": "alpha.any.root-compose", "source_owner": "alpha", "target_owner": "root", "target_role": "compose", "scopes": []any{"production", "internal_test"}}},
	} {
		t.Run(scenario.name, func(t *testing.T) {
			t.Parallel()
			if err := targetOnlyFixture(t, addFixtureRule(scenario.rule)); err == nil || !strings.Contains(err.Error(), "cross-owner glue belongs in the root") {
				t.Fatalf("composition glue accepted: %v", err)
			}
		})
	}
	for _, rule := range []map[string]any{
		{"id": "alpha.compose.own-http", "source_owner": "alpha", "source_role": "compose", "target_owner": "alpha", "target_role": "http"},
		{"id": "alpha.compose.beta-contract", "source_owner": "alpha", "source_role": "compose", "target_owner": "beta", "target_role": "contract"},
		{"id": "root.compose.beta-compose", "source_owner": "root", "source_role": "compose", "target_owner": "beta", "target_role": "compose"},
		{"id": "tools.compose.beta-http", "source_owner": "tools", "source_role": "compose", "target_owner": "beta", "target_role": "http"},
		{"id": "alpha.test.root-compose", "source_owner": "alpha", "target_owner": "root", "target_role": "compose", "scopes": []any{"internal_test"}},
	} {
		if err := targetOnlyFixture(t, addFixtureRule(rule)); err != nil {
			t.Fatalf("%s rejected: %v", rule["id"], err)
		}
	}
}

// The root exception covers the standing test reach only: a production rule
// from an owner composition into a root is rejected by the loader, and the
// comparison reports it as a loosening against the repository policy.
func TestCompositionIntoARootIsRejectedInProduction(t *testing.T) {
	t.Parallel()
	base := repositoryPolicy(t)
	for _, scopes := range [][]string{{"production"}, {"production", "internal_test"}} {
		candidate := *base
		candidate.PolicyEpoch++
		rule := Rule{ID: "facilities.compose.root-compose", Description: "Facilities composition names the root.", Scopes: scopes,
			SourceOwner: "facilities", SourceRole: "compose", TargetOwner: rootCompositionOwner, TargetRole: "compose"}
		candidate.Rules = append(slices.Clone(base.Rules), rule)
		if err := candidate.Validate(); err == nil || !strings.Contains(err.Error(), "cross-owner glue belongs in the root") {
			t.Fatalf("%v: validation accepted an owner composition importing a root: %v", scopes, err)
		}
		err := comparePolicyStrictness(base, &candidate, noCreated(), noCreated(), noCreated(), noCreated(), noCreated(), nil)
		if err == nil || !strings.Contains(err.Error(), "rule facilities.compose.root-compose newly allows production") {
			t.Fatalf("%v: comparison accepted an owner composition importing a root: %v", scopes, err)
		}
	}
}
