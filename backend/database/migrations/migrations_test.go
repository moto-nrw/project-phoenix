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
func TestNoDuplicateFilePrefixes(t *testing.T) {
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

	// Find duplicates
	var duplicates []string
	for prefix, fileList := range prefixToFiles {
		if len(fileList) > 1 {
			duplicates = append(duplicates, prefix+": "+strings.Join(fileList, ", "))
		}
	}

	if len(duplicates) > 0 {
		t.Errorf(
			"Duplicate migration file prefixes detected!\n"+
				"Each migration file must have a unique numeric prefix.\n\n"+
				"Duplicates found:\n  %s\n\n"+
				"Fix by assigning unique prefixes to each migration file.",
			strings.Join(duplicates, "\n  "),
		)
	}
}
