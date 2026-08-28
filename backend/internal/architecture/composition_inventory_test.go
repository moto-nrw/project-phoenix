package architecture

import (
	"bytes"
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"sort"
	"testing"
)

var (
	updateCompositionLegacy = flag.Bool("update-composition-legacy", false, "rewrite typed legacy callers in the composition inventory")
	compositionEvidenceRoot = flag.String("composition-evidence-root", "", "optional fixed-commit backend root used only to capture immutable evidence")
)

type compositionLegacyCaller struct {
	Key       string     `json:"key"`
	Locations []Location `json:"locations"`
}

type compositionLegacyInventory struct {
	Evidence []compositionLegacyCaller
	Current  []compositionLegacyCaller
}

func TestCompositionLegacyCallerInventory(t *testing.T) {
	t.Parallel()

	backendRoot := architectureBackendRoot(t)
	policy, err := LoadPolicy(filepath.Join(backendRoot, "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := LoadGraph(backendRoot, policy)
	if err != nil {
		t.Fatal(err)
	}
	actual := compositionLegacyViolations(graph.SemanticViolations)
	manifestPath := filepath.Join(backendRoot, "architecture", "composition.json")
	if *updateCompositionLegacy {
		var evidence []compositionLegacyCaller
		if *compositionEvidenceRoot != "" {
			evidence = loadCompositionLegacyEvidence(t, filepath.Clean(*compositionEvidenceRoot))
		}
		updateCompositionLegacyCallers(t, manifestPath, actual, evidence)
		return
	}
	inventory := loadCompositionLegacyCallers(t, manifestPath)
	if len(inventory.Evidence) == 0 {
		t.Fatal("fixed-commit typed legacy caller evidence is empty")
	}
	assertCompositionLegacyEqual(t, inventory.Current, actual)
}

func loadCompositionLegacyEvidence(t *testing.T, backendRoot string) []compositionLegacyCaller {
	t.Helper()
	policy, err := LoadPolicy(filepath.Join(backendRoot, "architecture", "policy.json"))
	if err != nil {
		t.Fatal(err)
	}
	graph, err := LoadGraph(backendRoot, policy)
	if err != nil {
		t.Fatal(err)
	}
	return compositionLegacyViolations(graph.SemanticViolations)
}

func compositionLegacyViolations(violations []Violation) []compositionLegacyCaller {
	var callers []compositionLegacyCaller
	for _, violation := range violations {
		if violation.Rule == "composition.legacy-reference" {
			callers = append(callers, compositionLegacyCaller{Key: violation.Key(), Locations: violation.Locations})
		}
	}
	sort.Slice(callers, func(i, j int) bool { return callers[i].Key < callers[j].Key })
	return callers
}

func loadCompositionLegacyCallers(t *testing.T, path string) compositionLegacyInventory {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- fixed repository manifest
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	return compositionLegacyInventory{
		Evidence: decodeCompositionLegacyCallers(t, document["evidence_legacy_callers"]),
		Current:  decodeCompositionLegacyCallers(t, document["legacy_callers"]),
	}
}

func decodeCompositionLegacyCallers(t *testing.T, content []byte) []compositionLegacyCaller {
	t.Helper()
	decoder := json.NewDecoder(bytes.NewReader(content))
	decoder.DisallowUnknownFields()
	var callers []compositionLegacyCaller
	if err := decoder.Decode(&callers); err != nil {
		t.Fatalf("decode legacy composition callers: %v", err)
	}
	return callers
}

func updateCompositionLegacyCallers(t *testing.T, path string, callers, evidence []compositionLegacyCaller) {
	t.Helper()
	content, err := os.ReadFile(path) // #nosec G304 -- fixed repository manifest
	if err != nil {
		t.Fatal(err)
	}
	var document map[string]json.RawMessage
	if err := json.Unmarshal(content, &document); err != nil {
		t.Fatal(err)
	}
	document["legacy_callers"], err = json.Marshal(callers)
	if err != nil {
		t.Fatal(err)
	}
	if evidence != nil {
		document["evidence_legacy_callers"], err = json.Marshal(evidence)
		if err != nil {
			t.Fatal(err)
		}
	}
	encoded, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, append(encoded, '\n'), 0o644); err != nil {
		t.Fatal(err)
	}
}

func assertCompositionLegacyEqual(t *testing.T, want, got []compositionLegacyCaller) {
	t.Helper()
	wantJSON, err := json.MarshalIndent(want, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	gotJSON, err := json.MarshalIndent(got, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	if !bytes.Equal(wantJSON, gotJSON) {
		t.Errorf("typed legacy callers differ; run go test ./internal/architecture -run TestCompositionLegacyCallerInventory -update-composition-legacy\nwant %s\ngot  %s", wantJSON, gotJSON)
	}
}

func architectureBackendRoot(t *testing.T) string {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	return filepath.Clean(filepath.Join(workingDirectory, "..", ".."))
}
