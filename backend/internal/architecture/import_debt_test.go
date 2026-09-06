package architecture

import (
	"bytes"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestCheckConvertsExistingAllowedImportToDebt(t *testing.T) {
	t.Parallel()
	repo, baseRef := ratchetRepositoryWithPolicy(t, "", readFile(t, fixturePath(t, "vertical-allowed.json")))
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), readFile(t, fixturePath(t, "vertical-forbidden.json")))
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2743))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err != nil || !strings.Contains(output, "1 legacy violation(s) remain") {
		t.Fatalf("existing allowed import could not become debt: %v\n%s", err, output)
	}
}

func TestCheckRejectsNewImportDuringDebtConversion(t *testing.T) {
	t.Parallel()
	repo, baseRef := ratchetRepositoryWithPolicy(t, "", readFile(t, fixturePath(t, "vertical-allowed.json")))
	writeFile(t, filepath.Join(repo, "source", "source.go"), "package source\nimport _ \"example.test/architecture-fixture/linuxonly\"\n")
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), readFile(t, fixturePath(t, "vertical-forbidden.json")))
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecordWithTarget(2743, "example.test/architecture-fixture/linuxonly"))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "absent from the base baseline") {
		t.Fatalf("candidate import was accepted as old debt: %v\n%s", err, output)
	}
}

func TestDebtConversionUsesBaseBuildAndScope(t *testing.T) {
	t.Parallel()
	for _, tt := range []struct{ name, file, source string }{
		{"internal test", "old_test.go", "package source\nimport _ \"example.test/architecture-fixture/target\"\n"},
		{"external test", "old_test.go", "package source_test\nimport _ \"example.test/architecture-fixture/target\"\n"},
		{"other OS", "old_darwin.go", "package source\nimport _ \"example.test/architecture-fixture/target\"\n"},
		{"build tag", "old.go", "//go:build custom\n\npackage source\nimport _ \"example.test/architecture-fixture/target\"\n"},
		{"cgo", "old.go", "package source\nimport \"C\"\nimport _ \"example.test/architecture-fixture/target\"\n"},
	} {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			repo, _ := ratchetRepositoryWithPolicy(t, "", readFile(t, fixturePath(t, "vertical-allowed.json")))
			original := readFile(t, filepath.Join(repo, "source", "source.go"))
			writeFile(t, filepath.Join(repo, "source", "source.go"), "package source\n")
			old := filepath.Join(repo, "source", tt.file)
			writeFile(t, old, tt.source)
			runGit(t, repo, "add", ".")
			runGit(t, repo, "commit", "-qm", "scope-specific base import")
			baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
			if err := os.Remove(old); err != nil {
				t.Fatal(err)
			}
			writeFile(t, filepath.Join(repo, "source", "source.go"), original)
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), readFile(t, fixturePath(t, "vertical-forbidden.json")))
			writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2743))
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, "absent from the base baseline") {
				t.Fatalf("inactive or test-only base import became production debt: %v\n%s", err, output)
			}
		})
	}
}

func TestConvertedDebtStaysShrinkOnly(t *testing.T) {
	t.Parallel()
	repo, _ := ratchetRepository(t, legacyRecord(2743))
	baseRef := strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), legacyRecord(2750))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "changed migration issue") {
		t.Fatalf("reassigned debt accepted: %v\n%s", err, output)
	}
	writeFile(t, filepath.Join(repo, "source", "source.go"), "package source\n")
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), "")
	output, err = runRepositoryCheck(t, repo, baseRef)
	if err != nil || !strings.Contains(output, "0 legacy violation(s) remain") {
		t.Fatalf("contract could not remove debt: %v\n%s", err, output)
	}
}

func TestConvertedImportProjectsAsDebtNotTargetPermission(t *testing.T) {
	t.Parallel()
	policy, graph, baseline := loadProjectionInputs(t)
	// Alpha -> Beta is initially allowed. Removing its rule must move that
	// exact import into debt, without hiding Beta -> Gamma's existing debt or
	// Alpha -> Gamma's new violation.
	policy.Rules = nil
	entry := LegacyEntry{Violation: Violation{Scope: ScopeProduction, Rule: "imports.forbidden", Source: policy.ModulePath + "/alpha", Target: policy.ModulePath + "/beta"}, Issue: "https://github.com/moto-nrw/project-phoenix/issues/2743"}
	baseline.Entries = append(baseline.Entries, entry)
	target := renderValidSVG(t, TargetProjection(policy))
	if bytes.Contains(target, []byte("alpha -&gt; beta")) {
		t.Fatal("compatibility edge remained in target SVG")
	}
	migration := MigrationProjection(policy, graph, baseline)
	jsonBefore, err := MarshalProjection(migration)
	if err != nil {
		t.Fatal(err)
	}
	svg := renderValidSVG(t, migration)
	for _, want := range []string{"alpha -&gt; beta (legacy)", "alpha -&gt; gamma (new)", `stroke="#d65a31"`, `stroke-dasharray="8 6"`} {
		if !bytes.Contains(svg, []byte(want)) {
			t.Errorf("migration SVG missing %s", want)
		}
	}
	graph.Edges[0], graph.Edges[len(graph.Edges)-1] = graph.Edges[len(graph.Edges)-1], graph.Edges[0]
	if !bytes.Equal(svg, renderValidSVG(t, MigrationProjection(policy, graph, baseline))) {
		t.Fatal("debt SVG depends on import order")
	}
	jsonAfter, err := MarshalProjection(MigrationProjection(policy, graph, baseline))
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(jsonBefore, jsonAfter) {
		t.Fatal("debt JSON depends on import order")
	}
}

func TestCarePlanCompatibilityHasNoTargetPermissions(t *testing.T) {
	t.Parallel()
	policy, err := LoadPolicy(filepath.Join("..", "..", "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	for _, rule := range policy.Rules {
		if (rule.SourceOwner == "care-plan" && rule.SourceRole == "adapter") || (rule.TargetOwner == "care-plan" && rule.TargetRole == "adapter") {
			t.Errorf("temporary Care Plan adapter permission %s belongs in exact debt", rule.ID)
		}
	}
	baseline, err := LoadLegacyManifest(filepath.Join("..", "..", "architecture", "legacy.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range baseline.Entries {
		if entry.Rule == "imports.forbidden" && (strings.HasSuffix(entry.Source, "/modules/careplan/legacy") || strings.HasSuffix(entry.Target, "/modules/careplan/legacy")) {
			if importAllowed(policy, Edge{Scope: entry.Scope, Source: entry.Source, Target: entry.Target}) {
				t.Errorf("debt also has target permission: %s", entry.Key())
			}
		}
	}
}
