package architecture

import (
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

// runRelocations publishes only the ADR 0020 verifier's result. Source ratchets
// use this metadata command instead of duplicating the owner/role/Git-file proof
// or importing the architecture implementation across an owner seam.
func runRelocations(args []string) error {
	options, err := parseCheckOptions(append([]string{"--baseline", "architecture/legacy.jsonl"}, args...))
	if err != nil {
		return err
	}
	policy, err := LoadPolicy(options.policy)
	if err != nil {
		return err
	}
	base, _, err := LoadBasePolicyAndManifest(options.project, options.policy, options.baseline, options.baseRef)
	if err != nil {
		return err
	}
	if base.ModulePath != policy.ModulePath {
		return fmt.Errorf("relocations may not change module_path")
	}
	moves, err := relocatedPackages(options.project, options.baseRef, base, policy)
	if err != nil {
		return err
	}
	relative := make(map[string]string, len(moves))
	for from, to := range moves {
		relative[strings.TrimPrefix(from, base.ModulePath+"/")] = strings.TrimPrefix(to, policy.ModulePath+"/")
	}
	return json.NewEncoder(os.Stdout).Encode(relative)
}
