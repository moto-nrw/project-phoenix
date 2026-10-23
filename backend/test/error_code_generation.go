package test

import (
	"fmt"
	"os/exec"
	"path/filepath"
)

// runErrorCodeFixtureGenerator is test-support tooling, not e2e application
// behavior. Keep process execution behind the fixture-support boundary, whose
// policy role permits developer tooling; callers supply an isolated script.
func runErrorCodeFixtureGenerator(script string) error {
	output, err := exec.Command("node", script).CombinedOutput() // #nosec G204 -- caller supplies a repo-local generator copied into its temporary fixture
	if err != nil {
		return fmt.Errorf("generate error-code fixture: %w\n%s", err, output)
	}
	// The Go test wrapper supplies the pinned toolchain on PATH. Check the real
	// generated directory without adding a compiler dependency to the E2E role.
	directory := filepath.Join(filepath.Dir(script), "..", "backend", "api", "common")
	output, err = exec.Command("gofmt", "-l", directory).CombinedOutput() // #nosec G204 -- isolated generated fixture directory
	if err != nil {
		return fmt.Errorf("check generated fixture formatting: %w\n%s", err, output)
	}
	if len(output) != 0 {
		return fmt.Errorf("generated source is not gofmt-clean:\n%s", output)
	}
	return nil
}
