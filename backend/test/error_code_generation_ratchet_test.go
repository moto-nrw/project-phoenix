package test

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the real generator CLI on an isolated registry copy, then the same
// scanners and budgets as the backend gates. No repository output is mutated.
func TestErrorCodeGenerationRatchetsAllowAdditionalCode(t *testing.T) {
	t.Parallel()
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Fatal(err)
	}
	root, err := filepath.EvalSymlinks(t.TempDir())
	if err != nil {
		t.Fatal(err)
	}
	for _, dir := range []string{"scripts", "backend/api/common", "frontend/src/lib"} {
		if err := os.MkdirAll(filepath.Join(root, dir), 0755); err != nil {
			t.Fatal(err)
		}
	}
	copyErrorCodeGeneratorInput(t, filepath.Dir(backendRoot), root, "scripts/generate-error-codes.mjs")
	data, err := os.ReadFile(filepath.Join(backendRoot, "..", "error-registry.json"))
	if err != nil {
		t.Fatal(err)
	}
	var registry struct {
		Codes []json.RawMessage `json:"codes"`
	}
	if err := json.Unmarshal(data, &registry); err != nil {
		t.Fatal(err)
	}
	registry.Codes = append(registry.Codes, json.RawMessage(`{"code":"general.ratchet_probe","class":"server","parameters":[],"legacy":"general.ratchet_probe"}`))
	data, err = json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "error-registry.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	cmd := exec.Command("node", filepath.Join(root, "scripts/generate-error-codes.mjs"))
	if output, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("generate error codes: %v\n%s", err, output)
	}
	output, err := os.ReadFile(filepath.Join(root, "frontend/src/lib/error-codes.generated.ts"))
	if err != nil || !strings.Contains(string(output), `"general.ratchet_probe"`) {
		t.Fatalf("generator did not emit the additional code: %v", err)
	}
	assertGeneratedErrorCodeRatchets(t, filepath.Join(root, "backend"))
}

func copyErrorCodeGeneratorInput(t *testing.T, source, destination, rel string) {
	t.Helper()
	data, err := os.ReadFile(filepath.Join(source, rel))
	if err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(destination, rel), data, 0600); err != nil {
		t.Fatal(err)
	}
}

func assertGeneratedErrorCodeRatchets(t *testing.T, root string) {
	t.Helper()
	// Keep any current generated-file allowance so the red run demonstrates
	// growth of the frozen baseline, rather than merely its existing size.
	budget := make(map[string]int)
	for path, size := range moduleFileSizeAllowlist {
		if strings.HasPrefix(path, "api/common/error_codes") {
			budget[path] = size
		}
	}
	sizes, err := moduleFileSizeScan(root)
	if err != nil {
		t.Fatal(err)
	}
	lengths, err := moduleFuncLenScan(root)
	if err != nil {
		t.Fatal(err)
	}
	scores, err := moduleComplexityScan(root)
	if err != nil {
		t.Fatal(err)
	}
	violations := moduleFileSizeViolations(sizes, budget)
	violations = append(violations, moduleFuncLenViolations(lengths, nil)...)
	violations = append(violations, moduleComplexityViolations(scores, nil)...)
	if len(violations) != 0 {
		t.Fatalf("adding a registered code must keep source ratchets green:\n%s", strings.Join(violations, "\n"))
	}
}
