package architecture_test

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

const legacyKey = "production|imports.forbidden|example.test/architecture-fixture/source|example.test/architecture-fixture/target"

func TestCheckRequiresExactLegacyBaseline(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		manifest string
		want     string
	}{
		{name: "exact", manifest: legacyRecord(2583), want: "1 legacy violation(s) remain"},
		{name: "additional tuple", manifest: "", want: "new violations (1)"},
		{name: "stale tuple", manifest: legacyRecordWithTarget(2583, "example.test/architecture-fixture/stale"), want: "stale legacy entries (1)"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := writeManifest(t, tt.manifest)
			output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "vertical-forbidden.json"), "--baseline", manifest)
			if tt.name == "exact" && err != nil {
				t.Fatalf("exact baseline failed: %v\n%s", err, output)
			}
			if tt.name != "exact" && err == nil {
				t.Fatalf("mismatched baseline succeeded:\n%s", output)
			}
			if !strings.Contains(output, tt.want) {
				t.Fatalf("output does not contain %q:\n%s", tt.want, output)
			}
		})
	}
}

func TestCheckRejectsInvalidLegacyManifests(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, manifest, want string }{
		{name: "wildcard", manifest: legacyRecordWithTarget(2583, "example.test/architecture-fixture/*"), want: "wildcard"},
		{name: "layer-wide source", manifest: strings.Replace(legacyRecord(2583), "example.test/architecture-fixture/source", "services", 1), want: "not a package family or layer"},
		{name: "duplicate", manifest: legacyRecord(2583) + legacyRecord(2583), want: "duplicate canonical key"},
		{name: "unsorted", manifest: legacyRecordWithTarget(2583, "z") + legacyRecordWithTarget(2583, "a"), want: "sorted by canonical key"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			manifest := writeManifest(t, tt.manifest)
			output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "vertical-forbidden.json"), "--baseline", manifest)
			if err == nil || !strings.Contains(output, tt.want) {
				t.Fatalf("invalid manifest did not fail with %q: %v\n%s", tt.want, err, output)
			}
		})
	}
}

func TestCheckRejectsCandidateBaselineGrowthAndIssueReassignment(t *testing.T) {
	t.Parallel()

	tests := []struct{ name, base, candidate, want string }{
		{name: "tuple absent from base", base: "", candidate: legacyRecord(2583), want: "absent from the base baseline"},
		{name: "issue reassigned", base: legacyRecord(2583), candidate: legacyRecord(2584), want: "changed migration issue"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, baseRef := ratchetRepository(t, tt.base)
			writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), tt.candidate)
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, tt.want) {
				t.Fatalf("base comparison did not fail with %q: %v\n%s", tt.want, err, output)
			}
		})
	}
}

func TestCheckRejectsPolicyLooseningAgainstBase(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), readFile(t, fixturePath(t, "vertical-allowed.json")))

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "architecture policy loosening") || !strings.Contains(output, "stale legacy entries") || !strings.Contains(output, legacyKey) {
		t.Fatalf("policy loosening was not explained separately: %v\n%s", err, output)
	}
}

func TestCheckRejectsMutableBaseReference(t *testing.T) {
	t.Parallel()

	repo, _ := ratchetRepository(t, legacyRecord(2583))
	output, err := runRepositoryCheck(t, repo, "HEAD")
	if err == nil || !strings.Contains(output, "full immutable 40-character commit SHA") {
		t.Fatalf("mutable base reference was accepted: %v\n%s", err, output)
	}
}

func TestCheckResolvesRelativeProjectPathsForBaseComparison(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	output, err := runRepositoryCheck(t, repo, baseRef)
	if err != nil || !strings.Contains(output, "1 legacy violation(s) remain") {
		t.Fatalf("relative project path failed base comparison: %v\n%s", err, output)
	}
}

func TestCheckRejectsClassificationAndOwnerLoosening(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name, basePolicy string
	}{
		{name: "role classification", basePolicy: policyWithSourceClassification(t, "module", "domain")},
		{name: "owner classification", basePolicy: policyWithSourceClassification(t, "other", "application")},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			repo, baseRef := ratchetRepositoryWithPolicy(t, legacyRecord(2583), tt.basePolicy)
			writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), "")
			writeFile(t, filepath.Join(repo, "architecture", "policy.json"), readFile(t, fixturePath(t, "vertical-allowed.json")))
			output, err := runRepositoryCheck(t, repo, baseRef)
			if err == nil || !strings.Contains(output, "architecture policy loosening") || !strings.Contains(output, legacyKey) {
				t.Fatalf("classification loosening was not rejected: %v\n%s", err, output)
			}
		})
	}
}

func TestCheckRejectsSemanticClassificationBypassWithoutChangingImports(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	policy := policyWithSourceClassification(t, "module", "migration")
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "disables semantic checks") {
		t.Fatalf("semantic classification bypass was accepted: %v\n%s", err, output)
	}
}

func TestCheckRejectsTestScopeClassificationBypass(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
		document["roles"] = append(document["roles"].([]any), "migration")
		for _, value := range document["packages"].([]any) {
			pkg := value.(map[string]any)
			if pkg["path"] == "source" {
				pkg["internal_test_role"] = "migration"
			}
		}
	})
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "changed internal_test role") {
		t.Fatalf("test-scope classification bypass was accepted: %v\n%s", err, output)
	}
}

func TestCheckRejectsNewPackageClassifications(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	policy := mutatePolicy(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), func(document map[string]any) {
		document["packages"] = append(document["packages"].([]any), map[string]any{
			"path": "new-package", "owner": "module", "role": "domain", "internal_test_role": "module-internal-test", "external_test_role": "module-behavior-test",
		})
		document["external_classes"] = append(document["external_classes"].([]any), "stdlib")
		document["external_packages"] = append(document["external_packages"].([]any), map[string]any{"path": "example.com/new", "class": "stdlib"})
	})
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "package example.test/architecture-fixture/new-package was newly classified") || !strings.Contains(output, "external package example.com/new was newly classified") {
		t.Fatalf("new package classifications were accepted: %v\n%s", err, output)
	}
}

func TestCheckRejectsOwnerReclassificationWithoutChangingImports(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	policy := policyWithSourceClassificationFrom(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), "other", "application")
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "changed owner from module to other") {
		t.Fatalf("owner reclassification was accepted: %v\n%s", err, output)
	}
}

func TestCheckRejectsDormantBroadRule(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	policy := policyWithRule(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), map[string]any{
		"id":                "platform-domain",
		"description":       "Platform application code may use its domain.",
		"scopes":            []string{"production"},
		"source_owner_kind": "platform",
		"source_role":       "application",
		"target_role":       "domain",
		"same_owner":        true,
	})
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "rule platform-domain newly allows") || !strings.Contains(output, "remaining legacy violations: 1") {
		t.Fatalf("dormant broad rule was accepted: %v\n%s", err, output)
	}
}

func TestCheckRejectsNewDataObjectOwnership(t *testing.T) {
	t.Parallel()

	repo, baseRef := ratchetRepository(t, legacyRecord(2583))
	policy := policyWithDataObject(t, readFile(t, fixturePath(t, "vertical-forbidden.json")), "ghost.records", "module")
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)

	output, err := runRepositoryCheck(t, repo, baseRef)
	if err == nil || !strings.Contains(output, "data object ghost.records was newly assigned to owner module") {
		t.Fatalf("new data object ownership was accepted: %v\n%s", err, output)
	}
}

func TestCheckHasNoApprovalOrRebaselineSwitch(t *testing.T) {
	t.Parallel()

	for _, flag := range []string{"--init", "--approve", "--rebaseline"} {
		output, err := runArchitecture(t, "check", flag)
		if err == nil || !strings.Contains(output, "flag provided but not defined") {
			t.Fatalf("forbidden switch %s was accepted: %v\n%s", flag, err, output)
		}
	}
}

func TestAuditIssuesRejectsClosedDebtIssue(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/repos/moto-nrw/project-phoenix/issues/2583" {
			http.NotFound(w, r)
			return
		}
		_, _ = w.Write([]byte(`{"state":"closed","html_url":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`))
	}))
	defer server.Close()

	manifest := writeManifest(t, legacyRecord(2583))
	output, err := runArchitecture(t, "audit-issues", "--baseline", manifest, "--api-url", server.URL)
	if err == nil || !strings.Contains(output, "is closed") {
		t.Fatalf("closed migration issue was accepted: %v\n%s", err, output)
	}
}

func TestAuditIssuesAcceptsOneOpenIssueForMultipleEntries(t *testing.T) {
	t.Parallel()

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"state":"open","html_url":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`))
	}))
	defer server.Close()
	manifest := legacyRecordWithTarget(2583, "example.test/architecture-fixture/another") + legacyRecord(2583)

	output, err := runArchitecture(t, "audit-issues", "--baseline", writeManifest(t, manifest), "--api-url", server.URL)
	if err != nil || !strings.Contains(output, "1 open migration issue(s) cover 2 legacy violation(s)") {
		t.Fatalf("open migration issue audit failed: %v\n%s", err, output)
	}
}

func TestAuditIssuesDoesNotForwardGitHubTokenToCustomAPI(t *testing.T) {
	t.Parallel()

	authorization := make(chan string, 1)
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authorization <- r.Header.Get("Authorization")
		_, _ = w.Write([]byte(`{"state":"open","html_url":"https://github.com/moto-nrw/project-phoenix/issues/2583"}`))
	}))
	defer server.Close()

	output, err := runArchitectureWithEnv(t, map[string]string{"GITHUB_TOKEN": "top-secret"}, "audit-issues", "--baseline", writeManifest(t, legacyRecord(2583)), "--api-url", server.URL)
	if err != nil {
		t.Fatalf("issue audit failed: %v\n%s", err, output)
	}
	if got := <-authorization; got != "" {
		t.Fatalf("GitHub token leaked to custom API endpoint: %q", got)
	}
}

func TestIssueAuditNetworkFailureDoesNotAffectDeterministicCheck(t *testing.T) {
	t.Parallel()

	manifest := writeManifest(t, legacyRecord(2583))
	output, err := runArchitecture(t, "audit-issues", "--baseline", manifest, "--api-url", "http://127.0.0.1:1")
	if err == nil || !strings.Contains(output, "audit migration issue") {
		t.Fatalf("network failure was not isolated to issue audit: %v\n%s", err, output)
	}
	output, err = runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "vertical-forbidden.json"), "--baseline", manifest)
	if err != nil || !strings.Contains(output, "1 legacy violation(s) remain") {
		t.Fatalf("deterministic check changed after audit failure: %v\n%s", err, output)
	}
}

func TestCheckRejectsNonCanonicalIssueURLs(t *testing.T) {
	t.Parallel()

	for _, issue := range []string{
		"https://github.com/moto-nrw/project-phoenix/issues/2583/",
		"https://user@github.com/moto-nrw/project-phoenix/issues/2583",
		"https://github.com/moto-nrw/project-phoenix/pulls/2583",
		"https://github.com/moto-nrw/project-phoenix/issues/02583",
	} {
		t.Run(issue, func(t *testing.T) {
			manifest := strings.Replace(legacyRecord(2583), "https://github.com/moto-nrw/project-phoenix/issues/2583", issue, 1)
			output, err := runArchitecture(t, "check", "--project", fixturePath(t, "valid"), "--policy", fixturePath(t, "vertical-forbidden.json"), "--baseline", writeManifest(t, manifest))
			if err == nil || !strings.Contains(output, "migration issue") {
				t.Fatalf("non-canonical issue URL was accepted: %v\n%s", err, output)
			}
		})
	}
}

func legacyRecord(issue int) string {
	return legacyRecordWithTarget(issue, "example.test/architecture-fixture/target")
}

func legacyRecordWithTarget(issue int, target string) string {
	return fmt.Sprintf("{\"scope\":\"production\",\"rule\":\"imports.forbidden\",\"source\":\"example.test/architecture-fixture/source\",\"target\":%q,\"issue\":\"https://github.com/moto-nrw/project-phoenix/issues/%d\"}\n", target, issue)
}

func writeManifest(t *testing.T, contents string) string {
	t.Helper()
	path := filepath.Join(t.TempDir(), "legacy.jsonl")
	writeFile(t, path, contents)
	return path
}

func ratchetRepository(t *testing.T, baseline string) (string, string) {
	t.Helper()
	return ratchetRepositoryWithPolicy(t, baseline, readFile(t, fixturePath(t, "vertical-forbidden.json")))
}

func ratchetRepositoryWithPolicy(t *testing.T, baseline, policy string) (string, string) {
	t.Helper()
	repo := t.TempDir()
	if err := os.CopyFS(repo, os.DirFS(fixturePath(t, "valid"))); err != nil {
		t.Fatalf("copy project fixture: %v", err)
	}
	writeFile(t, filepath.Join(repo, "architecture", "policy.json"), policy)
	writeFile(t, filepath.Join(repo, "architecture", "legacy.jsonl"), baseline)
	runGit(t, repo, "init", "-q")
	runGit(t, repo, "config", "user.email", "architecture-test@example.test")
	runGit(t, repo, "config", "user.name", "Architecture Test")
	runGit(t, repo, "config", "commit.gpgsign", "false")
	runGit(t, repo, "add", ".")
	runGit(t, repo, "commit", "-qm", "base")
	return repo, strings.TrimSpace(runGit(t, repo, "rev-parse", "HEAD"))
}

func policyWithSourceClassification(t *testing.T, owner, role string) string {
	t.Helper()
	return policyWithSourceClassificationFrom(t, readFile(t, fixturePath(t, "vertical-allowed.json")), owner, role)
}

func policyWithSourceClassificationFrom(t *testing.T, policy, owner, role string) string {
	t.Helper()
	return mutatePolicy(t, policy, func(document map[string]any) {
		if owner == "other" {
			document["owners"] = append(document["owners"].([]any), map[string]any{"id": "other", "kind": "domain"})
		}
		if !containsJSONValue(document["roles"].([]any), role) {
			document["roles"] = append(document["roles"].([]any), role)
		}
		for _, value := range document["packages"].([]any) {
			pkg := value.(map[string]any)
			if pkg["path"] == "source" {
				pkg["owner"], pkg["role"] = owner, role
			}
		}
	})
}

func containsJSONValue(values []any, want string) bool {
	for _, value := range values {
		if value == want {
			return true
		}
	}
	return false
}

func policyWithRule(t *testing.T, policy string, rule map[string]any) string {
	t.Helper()
	return mutatePolicy(t, policy, func(document map[string]any) {
		document["rules"] = append(document["rules"].([]any), rule)
	})
}

func policyWithDataObject(t *testing.T, policy, name, owner string) string {
	t.Helper()
	return mutatePolicy(t, policy, func(document map[string]any) {
		document["data_objects"] = append(document["data_objects"].([]any), map[string]any{"name": name, "write_owner": owner})
	})
}

func mutatePolicy(t *testing.T, policy string, mutate func(map[string]any)) string {
	t.Helper()
	var document map[string]any
	if err := json.Unmarshal([]byte(policy), &document); err != nil {
		t.Fatalf("decode policy: %v", err)
	}
	mutate(document)
	encoded, err := json.Marshal(document)
	if err != nil {
		t.Fatalf("encode policy: %v", err)
	}
	return string(encoded)
}

func runRepositoryCheck(t *testing.T, repo, baseRef string) (string, error) {
	t.Helper()
	backendDir := filepath.Clean(filepath.Join(packageDir(t), "..", ".."))
	relativeRepo, err := filepath.Rel(backendDir, repo)
	if err != nil {
		t.Fatalf("resolve relative repository path: %v", err)
	}
	return runArchitecture(t, "check", "--project", relativeRepo, "--policy", "architecture/policy.json", "--baseline", "architecture/legacy.jsonl", "--base-ref", baseRef)
}

func runGit(t *testing.T, dir string, args ...string) string {
	t.Helper()
	command := exec.Command("git", append([]string{"-C", dir}, args...)...)
	output, err := command.CombinedOutput()
	if err != nil {
		t.Fatalf("git %v: %v\n%s", args, err, output)
	}
	return string(output)
}

func readFile(t *testing.T, path string) string {
	t.Helper()
	contents, err := os.ReadFile(path)
	if err != nil {
		t.Fatalf("read %s: %v", path, err)
	}
	return string(contents)
}

func writeFile(t *testing.T, path, contents string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatalf("create parent for %s: %v", path, err)
	}
	if err := os.WriteFile(path, []byte(contents), 0o644); err != nil {
		t.Fatalf("write %s: %v", path, err)
	}
}
