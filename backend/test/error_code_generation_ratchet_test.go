package test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Exercise the real generator CLI on an isolated registry copy, then the same
// scanners and budgets as the backend gates. No repository output is mutated.
func TestErrorCodeGenerationRatchetsAllowAdditionalCode(t *testing.T) {
	t.Parallel()
	for _, additions := range []int{1, 2000} {
		t.Run(fmt.Sprintf("%d additional codes", additions), func(t *testing.T) {
			t.Parallel()
			generateAdditionalErrorCodes(t, additions)
		})
	}
}

func generateAdditionalErrorCodes(t *testing.T, additions int) {
	t.Helper()
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
	for _, script := range []string{"scripts/generate-error-codes.mjs", "scripts/error-registry.mjs"} {
		copyErrorCodeGeneratorInput(t, filepath.Dir(backendRoot), root, script)
	}
	fragments, err := filepath.Glob(filepath.Join(backendRoot, "..", "error-registry", "*.json"))
	if err != nil {
		t.Fatal(err)
	}
	if len(fragments) == 0 {
		t.Fatal("no authored registry fragments")
	}
	if err := os.MkdirAll(filepath.Join(root, "error-registry"), 0755); err != nil {
		t.Fatal(err)
	}
	for _, fragment := range fragments {
		copyErrorCodeGeneratorInput(t, filepath.Dir(backendRoot), root, filepath.Join("error-registry", filepath.Base(fragment)))
	}
	var registry struct {
		Codes []json.RawMessage `json:"codes"`
	}
	for index := range additions {
		code := fmt.Sprintf("probe.ratchet_probe_%d", index)
		entry := fmt.Sprintf(`{"code":%q,"class":"server","parameters":[],"legacy":%q}`, code, code)
		registry.Codes = append(registry.Codes, json.RawMessage(entry))
	}
	data, err := json.Marshal(registry)
	if err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Join(root, "error-registry"), 0755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, "error-registry", "probe.json"), data, 0600); err != nil {
		t.Fatal(err)
	}
	if err := runErrorCodeFixtureGenerator(filepath.Join(root, "scripts/generate-error-codes.mjs")); err != nil {
		t.Fatal(err)
	}
	output, err := os.ReadFile(filepath.Join(root, "frontend/src/lib/error-codes/probe.generated.ts"))
	if err != nil || !strings.Contains(string(output), `"probe.ratchet_probe_0"`) {
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
	violations := moduleFileSizeViolations(sizes, nil)
	violations = append(violations, moduleFuncLenViolations(lengths, nil)...)
	violations = append(violations, moduleComplexityViolations(scores, nil)...)
	if len(violations) != 0 {
		t.Fatalf("adding a registered code must keep source ratchets green:\n%s", strings.Join(violations, "\n"))
	}
}
