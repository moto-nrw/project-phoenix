package test

import (
	"fmt"
	"os/exec"
)

// runErrorCodeFixtureGenerator is test-support tooling, not e2e application
// behavior. Keep process execution behind the fixture-support boundary, whose
// policy role permits developer tooling; callers supply an isolated script.
func runErrorCodeFixtureGenerator(script string) error {
	output, err := exec.Command("node", script).CombinedOutput() // #nosec G204 -- caller supplies a repo-local generator copied into its temporary fixture
	if err != nil {
		return fmt.Errorf("generate error-code fixture: %w\n%s", err, output)
	}
	return nil
}
