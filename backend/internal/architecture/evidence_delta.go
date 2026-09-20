package architecture

import (
	"errors"
	"fmt"
	"path/filepath"
	"slices"
	"strings"
)

func validateEvidenceDelta(root, ref string, tickets map[string]*migrationTicket) error {
	project := filepath.Join(root, "backend")
	basePolicy, base, err := LoadBasePolicyAndManifest(project, "architecture/policy.json", "architecture/legacy.jsonl", ref)
	if err != nil {
		return err
	}
	policy, err := LoadPolicy(filepath.Join(project, "architecture/policy.json"))
	if err != nil {
		return err
	}
	candidate, err := LoadLegacyManifest(filepath.Join(project, "architecture/legacy.jsonl"))
	if err != nil {
		return err
	}
	moves, err := relocatedPackages(project, ref, basePolicy, policy)
	if err != nil {
		return err
	}
	normalized, err := relocateManifest(base, moves)
	if err != nil {
		return err
	}
	previous, err := previousEvidenceClaims(root, ref)
	if err != nil {
		return err
	}
	return reportEvidenceDelta(base, normalized, candidate, moves, previous, tickets)
}

func reportEvidenceDelta(base, normalized, candidate *LegacyManifest, moves map[string]string, previous map[string]map[string]bool, tickets map[string]*migrationTicket) error {
	printEvidenceKeys("raw removed", missingEvidenceKeys(base, candidate))
	printEvidenceKeys("raw added", missingEvidenceKeys(candidate, base))
	var relocated []string
	for _, entry := range base.Entries {
		key := relocateEvidenceKey(entry.Key(), moves)
		if key != entry.Key() {
			relocated = append(relocated, entry.Key()+" -> "+key)
		}
	}
	printEvidenceKeys("verified relocation mappings (not automatically retirements)", relocated)
	removed := missingEvidenceKeys(normalized, candidate)
	coverage := make(map[string][]string)
	var problems []error
	paths := make([]string, 0, len(tickets))
	for path := range tickets {
		paths = append(paths, path)
	}
	slices.Sort(paths)
	for _, path := range paths {
		ticket := tickets[path]
		name := filepath.Base(path)
		var pending, historical, retired []string
		for _, declared := range *ticket.ExactRatchetKeys {
			key := relocateEvidenceKey(declared, moves)
			_, wasDebt := normalized.byKey[key]
			_, isDebt := candidate.byKey[key]
			if !wasDebt && !isDebt && !previous[name][declared] {
				problems = append(problems, fmt.Errorf("%s: unsupported new historical claim %s", name, declared))
				continue
			}
			switch {
			case isDebt:
				pending = append(pending, key)
			case wasDebt:
				retired = append(retired, key)
				if ticket.hasHistoricalGaps() {
					fmt.Printf("%s: historical gaps cannot cover new removal %s\n", name, key)
				} else {
					coverage[key] = append(coverage[key], name)
				}
			default:
				historical = append(historical, declared)
			}
		}
		printEvidenceKeys(name+" pending", pending)
		printEvidenceKeys(name+" already absent", historical)
		printEvidenceKeys(name+" removed in diff", retired)
	}
	for _, key := range removed {
		if len(coverage[key]) == 0 {
			problems = append(problems, fmt.Errorf("unclaimed removal (complete non-template evidence required): %s", key))
		}
	}
	fmt.Printf("debt retirements: %d; covered: %d\n", len(removed), len(coverage))
	return errors.Join(problems...)
}

func relocateEvidenceKey(key string, moves map[string]string) string {
	parts := strings.Split(key, "|")
	for _, index := range []int{2, 3} {
		if target, exists := moves[parts[index]]; exists {
			parts[index] = target
		}
	}
	return strings.Join(parts, "|")
}

func missingEvidenceKeys(left, right *LegacyManifest) []string {
	var keys []string
	for key := range left.byKey {
		if _, exists := right.byKey[key]; !exists {
			keys = append(keys, key)
		}
	}
	slices.Sort(keys)
	return keys
}

func printEvidenceKeys(label string, keys []string) {
	slices.Sort(keys)
	fmt.Printf("%s: %d\n", label, len(keys))
	for _, key := range keys {
		fmt.Printf("  %s\n", key)
	}
}
