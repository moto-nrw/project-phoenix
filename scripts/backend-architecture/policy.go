package main

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	"github.com/moto-nrw/project-phoenix/architecture/artifacts"
)

// Export uses the same complete storage reader as every policy consumer.
func policyExport(args []string, project string) error {
	if len(args) != 1 {
		return fmt.Errorf("policy-export takes no arguments")
	}
	data, err := artifacts.ReadPolicy(filepath.Join(project, "architecture", "policy.json"))
	if err != nil {
		return err
	}
	var document any
	if err := json.Unmarshal(data, &document); err != nil {
		return err
	}
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(document)
}
