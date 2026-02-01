package migrations_test

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/database/migrations"
)

// TestNoDuplicateMigrationVersions ensures that all migrations have unique semantic versions.
// This catches the case where two migration files accidentally use the same Version constant.
//
// The MigrationRegistry is a map that silently overwrites duplicate keys, so we compare
// the count of registered migrations against the count of files that call MustRegister.
func TestNoDuplicateMigrationVersions(t *testing.T) {
	// Count files that register migrations (have MustRegister call)
	migrationsDir := "."
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	mustRegisterPattern := regexp.MustCompile(`Migrations\.MustRegister\(`)
	filesWithMustRegister := 0

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}
		// Skip the helper file that defines the registry
		if file.Name() == "00_migrations.go" {
			continue
		}

		content, err := os.ReadFile(filepath.Join(migrationsDir, file.Name()))
		if err != nil {
			t.Fatalf("failed to read file %s: %v", file.Name(), err)
		}

		if mustRegisterPattern.Match(content) {
			filesWithMustRegister++
		}
	}

	registeredCount := len(migrations.MigrationRegistry)

	if filesWithMustRegister != registeredCount {
		t.Errorf(
			"Migration version collision detected!\n"+
				"  Files with MustRegister: %d\n"+
				"  Entries in MigrationRegistry: %d\n"+
				"  Missing: %d migrations (silently overwritten due to duplicate versions)\n\n"+
				"To find duplicates, run:\n"+
				"  grep -h 'Version.*=' backend/database/migrations/*.go | sort | uniq -d",
			filesWithMustRegister,
			registeredCount,
			filesWithMustRegister-registeredCount,
		)
	}
}

// TestNoDuplicateFilePrefixes ensures that all migration files have unique numeric prefixes.
// The prefix (e.g., "001006017" in "001006017_some_name.go") determines execution order.
// Duplicate prefixes can cause unpredictable migration ordering.
//
// LEGACY EXCEPTION: Some prefixes were duplicated before this test existed and have already
// been applied to production databases. We cannot rename them without breaking deployments.
// New duplicates are still blocked - only the legacy ones are allowed.
func TestNoDuplicateFilePrefixes(t *testing.T) {
	// Legacy duplicate prefixes that already exist in production bun_migrations table.
	// These cannot be renamed without causing migrations to run again.
	// DO NOT ADD NEW ENTRIES HERE - fix the duplication instead!
	legacyAllowedDuplicates := map[string]bool{
		"001006017": true, // add_position_to_invitation_tokens + audit_data_imports
		"001006018": true, // add_pickup_status_to_students + import_performance_indexes
		"001006021": true, // restrict_substitution_permissions_to_admin (conflicts with version 1.7.1)
		"001007006": true, // grade_transitions + guardian_phone_numbers
		"001007007": true, // grade_transition_permissions + remove_guardian_contact_constraint
	}

	migrationsDir := "."
	files, err := os.ReadDir(migrationsDir)
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	// Pattern to extract numeric prefix from filename
	prefixPattern := regexp.MustCompile(`^(\d+)_`)
	prefixToFiles := make(map[string][]string)

	for _, file := range files {
		if file.IsDir() || !strings.HasSuffix(file.Name(), ".go") {
			continue
		}
		if strings.HasSuffix(file.Name(), "_test.go") {
			continue
		}

		matches := prefixPattern.FindStringSubmatch(file.Name())
		if len(matches) >= 2 {
			prefix := matches[1]
			prefixToFiles[prefix] = append(prefixToFiles[prefix], file.Name())
		}
	}

	// Find NEW duplicates (excluding legacy allowed ones)
	var newDuplicates []string
	for prefix, fileList := range prefixToFiles {
		if len(fileList) > 1 && !legacyAllowedDuplicates[prefix] {
			newDuplicates = append(newDuplicates, prefix+": "+strings.Join(fileList, ", "))
		}
	}

	if len(newDuplicates) > 0 {
		t.Errorf(
			"NEW duplicate migration file prefixes detected!\n"+
				"Each migration file must have a unique numeric prefix.\n\n"+
				"Duplicates found:\n  %s\n\n"+
				"Fix by assigning unique prefixes to each migration file.\n"+
				"Note: Some legacy duplicates are allowed (see legacyAllowedDuplicates in test).",
			strings.Join(newDuplicates, "\n  "),
		)
	}
}

// TestMigrationsSortedSemantically verifies that migrations are sorted by semantic version,
// not lexicographically. This ensures 1.10.0 comes after 1.9.0, not before.
func TestMigrationsSortedSemantically(t *testing.T) {
	migs := migrations.RegisteredMigrations()

	for i := 0; i < len(migs)-1; i++ {
		current := migs[i].Version
		next := migs[i+1].Version

		// Ensure numeric ordering is correct
		if compareVersionsForTest(current, next) > 0 {
			t.Errorf(
				"Migrations not sorted semantically: %s should come before %s",
				current, next,
			)
		}
	}
}

// compareVersionsForTest compares two semantic version strings for testing.
func compareVersionsForTest(a, b string) int {
	partsA := strings.Split(a, ".")
	partsB := strings.Split(b, ".")

	maxLen := len(partsA)
	if len(partsB) > maxLen {
		maxLen = len(partsB)
	}

	for i := 0; i < maxLen; i++ {
		var numA, numB int
		if i < len(partsA) {
			numA = parseIntSimple(partsA[i])
		}
		if i < len(partsB) {
			numB = parseIntSimple(partsB[i])
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

func parseIntSimple(s string) int {
	var n int
	for _, c := range s {
		if c >= '0' && c <= '9' {
			n = n*10 + int(c-'0')
		} else {
			break
		}
	}
	return n
}
