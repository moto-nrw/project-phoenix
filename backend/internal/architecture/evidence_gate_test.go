package architecture

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const evidenceTestKey = "production|imports.forbidden|example.test/architecture-fixture/source|example.test/architecture-fixture/target"

func evidenceRepository(t *testing.T) (string, string) {
	t.Helper()
	root := t.TempDir()
	dir := filepath.Join(root, "backend/architecture")
	for _, name := range []string{"migration-ticket-template.json", "checkpoint-ticket-template.json", "runtime-checkpoints.json"} {
		writeFile(t, filepath.Join(dir, name), readFile(t, filepath.Join(architectureBackendRoot(t), "architecture", name)))
	}
	writeFile(t, filepath.Join(dir, "policy.json"), readFile(t, fixturePath(t, "vertical-allowed.json")))
	writeFile(t, filepath.Join(dir, "legacy.jsonl"), legacyRecord(3415))
	runGit(t, root, "init", "-q")
	runGit(t, root, "config", "user.email", "architecture-test@example.test")
	runGit(t, root, "config", "user.name", "Architecture Test")
	runGit(t, root, "config", "commit.gpgsign", "false")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "base")
	return root, strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
}

func writeEvidenceDocument(t *testing.T, root, name string, document map[string]any) string {
	t.Helper()
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "backend/architecture", name)
	writeFile(t, path, string(data)+"\n")
	return path
}

func TestEvidenceGateRequiresClaimsAcrossMultiplePRs(t *testing.T) {
	t.Parallel()
	root, base := evidenceRepository(t)
	document := migrationFixture(t)
	document["exact_ratchet_keys"] = []any{evidenceTestKey}
	writeEvidenceDocument(t, root, "wave.json", document)
	check := func(ref string) error {
		return RunCLI([]string{"validate-ticket", "--base-ref", ref}, CLIDependencies{ProjectRoot: root})
	}
	if err := check(base); err != nil {
		t.Fatalf("pending debt must pass: %v", err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "declare scope in earlier PR")
	declared := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	writeFile(t, filepath.Join(root, "backend/architecture/legacy.jsonl"), "")
	if err := check(declared); err != nil {
		t.Fatalf("previous ticket must cover removal: %v", err)
	}
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "retire debt")
	retired := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	if err := check(retired); err != nil {
		t.Fatalf("historical declaration must remain valid: %v", err)
	}
	if err := os.Remove(filepath.Join(root, "backend/architecture/wave.json")); err != nil {
		t.Fatal(err)
	}
	if err := check(base); err == nil || !strings.Contains(err.Error(), "unclaimed removal") {
		t.Fatalf("unclaimed removal accepted: %v", err)
	}
}

func TestEvidenceGateRejectsUnsupportedClaimsAndInventoryEscape(t *testing.T) {
	t.Parallel()
	for _, scenario := range []string{"unknown key", "missing kind", "copied template", "missing source", "gap copied", "placeholder", "unknown metric field", "old schema"} {
		t.Run(scenario, func(t *testing.T) {
			t.Parallel()
			root, base := evidenceRepository(t)
			document := migrationFixture(t)
			document["exact_ratchet_keys"] = []any{}
			evidence := document["runtime_evidence"].(map[string]any)
			want := ""
			switch scenario {
			case "unknown key":
				document["exact_ratchet_keys"] = []any{strings.ReplaceAll(evidenceTestKey, "/target", "/never-existed")}
				want = "unsupported new historical claim"
			case "missing kind":
				delete(document, "ticket_kind")
				want = "ticket_kind"
			case "copied template":
				if err := json.Unmarshal([]byte(readFile(t, filepath.Join(root, "backend/architecture/migration-ticket-template.json"))), &document); err != nil {
					t.Fatal(err)
				}
				want = "unchanged template"
			case "missing source":
				evidence["sources"] = []any{"docs/absent-evidence.md"}
				want = "local evidence source"
			case "gap copied":
				evidence["latency_p95"] = map[string]any{"state": "historical_gap", "reason": "The old run did not capture percentile latency."}
				want = "historical_gap is not authorized"
			case "placeholder":
				evidence["lock_wait"] = map[string]any{"state": "not_applicable", "reason": "N/A"}
				want = "concrete reason"
			case "unknown metric field":
				evidence["lock_wait"].(map[string]any)["accepted"] = true
				want = "unknown field"
			case "old schema":
				document["schema_version"] = 2
				want = "schema_version must be 3"
			}
			writeEvidenceDocument(t, root, "wave.json", document)
			err := RunCLI([]string{"validate-ticket", "--all", "--base-ref", base}, CLIDependencies{ProjectRoot: root})
			if err == nil || !strings.Contains(err.Error(), want) {
				t.Fatalf("want %s, got %v", want, err)
			}
		})
	}
}

func TestCheckpointAcceptanceRequiresRepointingAllMigrationEvidence(t *testing.T) {
	t.Parallel()
	root, _ := evidenceRepository(t)
	document := migrationFixture(t)
	document["exact_ratchet_keys"] = []any{}
	writeEvidenceDocument(t, root, "wave.json", document)
	var registry map[string]any
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(root, "backend/architecture/runtime-checkpoints.json"))), &registry); err != nil {
		t.Fatal(err)
	}
	third := "https://github.com/moto-nrw/project-phoenix/issues/3021"
	registry["accepted"] = append(registry["accepted"].([]any), map[string]any{"issue": third, "acceptance": third + "#issuecomment-123"})
	writeEvidenceDocument(t, root, "runtime-checkpoints.json", registry)
	check := func() error { return RunCLI([]string{"validate-ticket", "--all"}, CLIDependencies{ProjectRoot: root}) }
	if err := check(); err == nil || !strings.Contains(err.Error(), "current accepted checkpoint") {
		t.Fatalf("stale references accepted: %v", err)
	}
	document["checkpoint_reference"] = third
	writeEvidenceDocument(t, root, "wave.json", document)
	if err := check(); err == nil {
		t.Fatal("stale template was not checked")
	}
	var template map[string]any
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(root, "backend/architecture/migration-ticket-template.json"))), &template); err != nil {
		t.Fatal(err)
	}
	template["checkpoint_reference"] = third
	writeEvidenceDocument(t, root, "migration-ticket-template.json", template)
	if err := check(); err != nil {
		t.Fatalf("repointed inventory rejected: %v", err)
	}
}

func TestHistoricalGapsCannotAuthorizeNewRetirements(t *testing.T) {
	t.Parallel()
	root, _ := evidenceRepository(t)
	realRoot := filepath.Dir(architectureBackendRoot(t))
	runGit(t, root, "fetch", "--no-tags", realRoot, migrationHistoryAnchor)
	var document map[string]any
	if err := json.Unmarshal([]byte(readFile(t, filepath.Join(realRoot, "backend/architecture/account-sessions-2720.json"))), &document); err != nil {
		t.Fatal(err)
	}
	writeEvidenceDocument(t, root, "account-sessions-2720.json", document)
	for _, source := range document["runtime_evidence"].(map[string]any)["sources"].([]any) {
		writeFile(t, filepath.Join(root, source.(string)), "Historical evidence fixture.\n")
	}
	parts := strings.Split(document["exact_ratchet_keys"].([]any)[0].(string), "|")
	entry := LegacyEntry{Violation: Violation{Scope: Scope(parts[0]), Rule: parts[1], Source: parts[2], Target: parts[3]}, Issue: "https://github.com/moto-nrw/project-phoenix/issues/2720"}
	data, err := json.Marshal(entry)
	if err != nil {
		t.Fatal(err)
	}
	writeFile(t, filepath.Join(root, "backend/architecture/legacy.jsonl"), string(data)+"\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "historical evidence and pending debt")
	base := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	check := func() error {
		return RunCLI([]string{"validate-ticket", "--all", "--base-ref", base}, CLIDependencies{ProjectRoot: root})
	}
	if err := check(); err != nil {
		t.Fatalf("frozen record rejected: %v", err)
	}
	raw := filepath.Join(root, "docs/runtime-checkpoints/account-sessions-2720.raw.json")
	if err := os.Remove(raw); err != nil {
		t.Fatal(err)
	}
	if err := check(); err == nil || !strings.Contains(err.Error(), "local evidence source") {
		t.Fatalf("deleted historical raw evidence accepted: %v", err)
	}
	writeFile(t, raw, "Historical evidence fixture.\n")
	writeFile(t, filepath.Join(root, "backend/architecture/legacy.jsonl"), "")
	if err := check(); err == nil || !strings.Contains(err.Error(), "unclaimed removal") {
		t.Fatalf("historical gap covered new work: %v", err)
	}
	complete := migrationFixture(t)
	complete["exact_ratchet_keys"] = []any{entry.Key()}
	writeEvidenceDocument(t, root, "account-sessions-followup.json", complete)
	if err := check(); err != nil {
		t.Fatalf("complete followup evidence rejected: %v", err)
	}
	document["owner_and_capability"] = "Unrelated new flow"
	writeEvidenceDocument(t, root, "account-sessions-2720.json", document)
	if err := check(); err == nil || !strings.Contains(err.Error(), "changed scope or provenance") {
		t.Fatalf("reused filename granted new exemption: %v", err)
	}
}

func TestEvidenceGateDistinguishesRelocationFromRetirement(t *testing.T) {
	t.Parallel()
	root, _ := evidenceRepository(t)
	backend := filepath.Join(root, "backend")
	writeFile(t, filepath.Join(backend, "target/target.go"), "package target\nconst Value = 1\n")
	runGit(t, root, "add", ".")
	runGit(t, root, "commit", "-qm", "base package")
	base := strings.TrimSpace(runGit(t, root, "rev-parse", "HEAD"))
	moveFixtureTarget(t, backend)
	writeFile(t, filepath.Join(backend, "architecture/policy.json"), relocatedPolicy(t, backend, nil))
	writeFile(t, filepath.Join(backend, "architecture/legacy.jsonl"), legacyRecordWithTarget(3415, "example.test/architecture-fixture/"+movedTargetPath))
	runGit(t, root, "add", "-A")
	check := func() error {
		return RunCLI([]string{"validate-ticket", "--base-ref", base}, CLIDependencies{ProjectRoot: root})
	}
	if err := check(); err != nil {
		t.Fatalf("relocation required a retirement claim: %v", err)
	}
	writeFile(t, filepath.Join(backend, "architecture/legacy.jsonl"), "")
	if err := check(); err == nil || !strings.Contains(err.Error(), "unclaimed removal") {
		t.Fatalf("real retirement hidden by relocation: %v", err)
	}
	document := migrationFixture(t)
	document["exact_ratchet_keys"] = []any{evidenceTestKey}
	writeEvidenceDocument(t, root, "wave.json", document)
	if err := check(); err != nil {
		t.Fatalf("old-path claim not normalized: %v", err)
	}
}

func TestEvidenceCLIReportsAdditionsAndRemovalsSeparately(t *testing.T) {
	t.Parallel()
	root, base := evidenceRepository(t)
	document := migrationFixture(t)
	document["exact_ratchet_keys"] = []any{evidenceTestKey}
	writeEvidenceDocument(t, root, "wave.json", document)
	writeFile(t, filepath.Join(root, "backend/architecture/legacy.jsonl"), legacyRecordWithTarget(3415, "example.test/architecture-fixture/linuxonly"))
	environment := append(os.Environ(), "PHOENIX_ARCHITECTURE_PROJECT="+filepath.Join(root, "backend"), "GOCACHEPROG=")
	output, err := processOutput(filepath.Join(filepath.Dir(architectureBackendRoot(t)), "scripts/backend-architecture"), environment, "go", "run", ".", "validate-ticket", "--base-ref", base)
	if err != nil {
		t.Fatalf("CLI failed: %v\n%s", err, output)
	}
	for _, want := range []string{"raw removed: 1", "raw added: 1", "debt retirements: 1; covered: 1"} {
		if !strings.Contains(string(output), want) {
			t.Fatalf("missing %q in report:\n%s", want, output)
		}
	}
}

func TestCheckpointCannotDeclareMissingRawEvidence(t *testing.T) {
	t.Parallel()
	for _, section := range []string{"summary", "run", "median", "worst"} {
		t.Run(section, func(t *testing.T) {
			t.Parallel()
			root, _ := evidenceRepository(t)
			document := decodeTicketFixture(t)
			report := document["runtime_evidence"].(map[string]any)
			checkpoint := document["checkpoint"].(map[string]any)
			if section == "run" {
				report = checkpoint["runs"].([]any)[0].(map[string]any)
			}
			if section == "median" || section == "worst" {
				report = checkpoint[section].(map[string]any)
			}
			report["sources"] = []any{"docs/missing-checkpoint.raw.json"}
			path := writeEvidenceDocument(t, root, "checkpoint.json", document)
			err := RunCLI([]string{"validate-ticket", "--ticket", path}, CLIDependencies{ProjectRoot: root})
			if err == nil || !strings.Contains(err.Error(), "local evidence source") {
				t.Fatalf("missing checkpoint source accepted: %v", err)
			}
		})
	}
}
