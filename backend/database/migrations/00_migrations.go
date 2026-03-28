package migrations

import (
	"fmt"
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"strconv"
	"strings"

	"github.com/uptrace/bun/migrate"
)

// Migrations is the shared migrations registry
var Migrations = migrate.NewMigrations()

// MigrationRegistry keeps track of all registered migrations with their metadata
var MigrationRegistry = make(map[string]*Migration)

// compareVersionsSemantic compares two "X.Y.Z" version strings numerically per segment.
// Returns true if a < b (a should be ordered before b).
// Lexicographic comparison would incorrectly order "1.12.7" < "1.8.3" because "1" < "8".
func compareVersionsSemantic(a, b string) bool {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

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
		if numA != numB {
			return numA < numB
		}
	}
	return false
}

// RegisteredMigrations returns all registered migrations sorted by version
func RegisteredMigrations() []*Migration {
	migrations := make([]*Migration, 0, len(MigrationRegistry))
	for _, m := range MigrationRegistry {
		migrations = append(migrations, m)
	}

	// Sort migrations by version semantically (numeric per segment, not lexicographic)
	sort.Slice(migrations, func(i, j int) bool {
		return compareVersionsSemantic(migrations[i].Version, migrations[j].Version)
	})

	return migrations
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

// DetectVersionCollisions scans migration source files for duplicate version registrations.
// Since MigrationRegistry is a map[string]*Migration, duplicate versions in init() functions
// silently overwrite each other — the map hides the collision. This function reads the actual
// source files to detect collisions that the map cannot reveal.
//
// If migrationsDir does not exist (e.g., in Docker where only the compiled binary is present),
// the check is skipped gracefully and returns nil.
func DetectVersionCollisions(migrationsDir string) error {
	// Match version constants like: someVersion = "1.15.20"
	// Migration files use named constants (not inline strings in struct fields),
	// so we match the const declaration pattern.
	versionPattern := regexp.MustCompile(`Version\s*=\s*"([^"]+)"`)

	entries, err := os.ReadDir(migrationsDir)
	if err != nil {
		if os.IsNotExist(err) {
			// In Docker, source files don't exist — only the compiled binary.
			// Skip the check gracefully.
			return nil
		}
		return fmt.Errorf("failed to read migrations directory %s: %w", migrationsDir, err)
	}

	versionFiles := make(map[string][]string)

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// Migration files start with 000 or 001
		if !strings.HasPrefix(name, "000") && !strings.HasPrefix(name, "001") {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, name))
		if err != nil {
			return fmt.Errorf("failed to read %s: %w", name, err)
		}

		matches := versionPattern.FindAllSubmatch(content, -1)
		for _, match := range matches {
			version := string(match[1])
			versionFiles[version] = append(versionFiles[version], name)
		}
	}

	var collisions []string
	for version, files := range versionFiles {
		if len(files) > 1 {
			collisions = append(collisions, fmt.Sprintf(
				"  version %s is registered by %d files: %s",
				version, len(files), strings.Join(files, ", "),
			))
		}
	}

	if len(collisions) > 0 {
		sort.Strings(collisions)
		return fmt.Errorf("migration version collisions detected:\n%s\n"+
			"each migration must have a unique version number, "+
			"duplicate versions cause silent overwrites in MigrationRegistry (a Go map)",
			strings.Join(collisions, "\n"))
	}

	return nil
}
