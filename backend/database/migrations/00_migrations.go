package migrations

import (
	"fmt"
	"sort"
	"strconv"
	"strings"

	"github.com/uptrace/bun/migrate"
)

// Migrations is the shared migrations registry
var Migrations = migrate.NewMigrations()

// MigrationRegistry keeps track of all registered migrations with their metadata
var MigrationRegistry = make(map[string]*Migration)

// RegisteredMigrations returns all registered migrations sorted by version
func RegisteredMigrations() []*Migration {
	migrations := make([]*Migration, 0, len(MigrationRegistry))
	for _, m := range MigrationRegistry {
		migrations = append(migrations, m)
	}

	// Sort migrations by version (semantically, not lexicographically)
	// This ensures 1.10.0 comes after 1.9.0, not before
	sort.Slice(migrations, func(i, j int) bool {
		return compareVersions(migrations[i].Version, migrations[j].Version) < 0
	})

	return migrations
}

// compareVersions compares two semantic version strings.
// Returns -1 if a < b, 0 if a == b, 1 if a > b.
// Handles versions like "1.9.4" vs "1.10.0" correctly (1.9.4 < 1.10.0).
func compareVersions(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	// Compare each numeric part
	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			numA, _ = strconv.Atoi(partsA[i])
		}
		if i < len(partsB) {
			numB, _ = strconv.Atoi(partsB[i])
		}

		if numA < numB {
			return -1
		}
		if numA > numB {
			return 1
		}
	}

	return 0
}

// ValidateMigrations validates migration dependencies and ordering.
// This is a pure in-memory check against the registered migration graph — no database needed.
func ValidateMigrations() error {
	migrations := RegisteredMigrations()

	// Build a set of all migration versions
	versions := make(map[string]bool)
	for _, m := range migrations {
		versions[m.Version] = true
	}

	// Check that all dependencies exist
	for _, m := range migrations {
		for _, dep := range m.DependsOn {
			if _, exists := versions[dep]; !exists {
				return fmt.Errorf("migration %s depends on %s, but it doesn't exist", m.Version, dep)
			}
		}
	}

	// Check for circular dependencies (simple check)
	// For a more thorough check, we would need to implement a graph-based algorithm
	for _, m := range migrations {
		for _, dep := range m.DependsOn {
			for _, otherM := range migrations {
				if otherM.Version == dep {
					for _, otherDep := range otherM.DependsOn {
						if otherDep == m.Version {
							return fmt.Errorf("circular dependency detected: %s and %s depend on each other",
								m.Version, otherM.Version)
						}
					}
				}
			}
		}
	}

	// Ensure migrations are in the correct order
	for i := 0; i < len(migrations)-1; i++ {
		current := migrations[i]
		next := migrations[i+1]

		// Check if the next migration depends on the current one
		for _, dep := range next.DependsOn {
			if dep == current.Version {
				// This is fine - the next migration depends on the current one
				continue
			}
		}

		// Check if current migration depends on a later one (which would be a problem)
		for _, dep := range current.DependsOn {
			for j := i + 1; j < len(migrations); j++ {
				if migrations[j].Version == dep {
					return fmt.Errorf("migration ordering issue: %s depends on %s, but %s comes later",
						current.Version, dep, dep)
				}
			}
		}
	}

	return nil
}

// PrintMigrationPlan prints the full migration plan
func PrintMigrationPlan() {
	migrations := RegisteredMigrations()

	fmt.Println("Migration Plan:")
	fmt.Println("===============")

	for i, m := range migrations {
		deps := strings.Join(m.DependsOn, ", ")
		if deps == "" {
			deps = "none"
		}

		fmt.Printf("%d. V%s - %s (Dependencies: %s)\n", i+1, m.Version, m.Description, deps)
	}

	fmt.Println("===============")
}
