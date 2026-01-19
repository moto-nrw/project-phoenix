// Package test provides test utilities and hermetic test verification.
package test

import (
	"bufio"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// TestHermeticTestPatterns verifies that test files follow hermetic testing patterns.
// This test scans all *_test.go files in the backend directory and checks for:
// 1. Hardcoded small integer IDs (int64(1), int64(2), etc.) that indicate non-hermetic tests
// 2. Test files with DB operations that don't use SetupTestDB
//
// Hermetic tests should:
// - Create their own test data using fixtures
// - Never rely on hardcoded IDs that may not exist in the database
// - Clean up after themselves
// - Be runnable in any order and in parallel
func TestHermeticTestPatterns(t *testing.T) {
	// Find the backend root directory
	backendRoot, err := findBackendRoot()
	if err != nil {
		t.Skipf("Could not find backend root: %v", err)
		return
	}

	t.Run("no_hardcoded_integer_ids", func(t *testing.T) {
		violations := checkHardcodedIDs(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d hardcoded ID violation(s):\n\n%s\n\n"+
				"Fix: Use test fixtures instead of hardcoded IDs.\n"+
				"Example:\n"+
				"  // Before (non-hermetic):\n"+
				"  result, err := repo.FindByID(ctx, int64(1))\n\n"+
				"  // After (hermetic):\n"+
				"  student := testpkg.CreateTestStudent(t, db, \"First\", \"Last\", \"1a\")\n"+
				"  defer testpkg.CleanupTableRecords(t, db, \"users.students\", student.ID)\n"+
				"  result, err := repo.FindByID(ctx, student.ID)",
				len(violations), strings.Join(violations, "\n"))
		}
	})

	t.Run("db_operations_use_setup_test_db", func(t *testing.T) {
		violations := checkMissingSetupTestDB(t, backendRoot)
		if len(violations) > 0 {
			t.Errorf("Found %d test file(s) with DB operations missing SetupTestDB:\n\n%s\n\n"+
				"Fix: Add SetupTestDB to initialize the test database.\n"+
				"Example:\n"+
				"  func TestExample(t *testing.T) {\n"+
				"      db := testpkg.SetupTestDB(t)\n"+
				"      defer func() { _ = db.Close() }()\n"+
				"      // ... test code\n"+
				"  }",
				len(violations), strings.Join(violations, "\n"))
		}
	})
}

// findBackendRoot walks up the directory tree to find the backend root.
func findBackendRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}

	for {
		// Check if this looks like the backend root
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return dir, nil
		}

		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}

	return "", os.ErrNotExist
}

// checkHardcodedIDs scans test files for hardcoded small integer IDs.
func checkHardcodedIDs(t *testing.T, root string) []string {
	t.Helper()

	var violations []string

	// Pattern matches int64(1) through int64(9)
	hardcodedIDPattern := regexp.MustCompile(`int64\([1-9]\)`)

	// Patterns that indicate legitimate uses (not IDs)
	// Note: Some patterns use simple substring matching, others use word boundaries
	legitimatePatterns := []string{
		"//",             // Comments
		"i :=",           // Loop variables
		"i =",            // Loop variables
		"offset",         // Pagination
		"limit",          // Pagination
		"page",           // Pagination
		"Weekday",        // Day of week
		"weekday",        // Day of week
		"day",            // Time-related
		"hour",           // Time-related
		"minute",         // Time-related
		"second",         // Time-related
		"duration",       // Time-related
		"timeout",        // Time-related
		"retry",          // Retry counts
		"max",            // Limits
		"min",            // Limits
		"size",           // Sizes
		"len",            // Lengths
		"cap",            // Capacities
		"index",          // Array indices
		"999999",         // Non-existent ID patterns (intentional)
		"GreaterOrEqual", // Assertions checking >= 1
		"LessOrEqual",    // Assertions checking <= n
		"Greater",        // Assertions checking > n
		"Less",           // Assertions checking < n
		"func()",         // Inline functions creating pointers
		"return &id",     // Pointer helpers in model tests
	}

	// Patterns that require word boundary matching to avoid false negatives.
	// For example, "count" should NOT match "AccountID" (Acc-ount-ID).
	wordBoundaryPatterns := []*regexp.Regexp{
		regexp.MustCompile(`\bcount\b`), // Counts (word boundary to avoid matching "AccountID")
	}

	// Files to skip (mock tests, model unit tests without DB)
	skipPatterns := []string{
		"_internal_test.go",                   // Internal tests often use mocks
		"_mock_test.go",                       // Mock tests
		"models/",                             // Model unit tests don't hit DB
		"invitation_service_test.go",          // Uses mocks
		"password_reset_integration_test.go",  // Uses mocks (sqlmock + stubs)
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil // Skip files we can't access
		}

		// Only check test files
		if info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip this verification test itself
		if strings.Contains(path, "hermetic_verification_test.go") {
			return nil
		}

		// Skip files matching skip patterns (mocks, model unit tests, etc.)
		for _, pattern := range skipPatterns {
			if strings.Contains(path, pattern) {
				return nil
			}
		}

		file, err := os.Open(path)
		if err != nil {
			return nil
		}
		defer func() { _ = file.Close() }()

		scanner := bufio.NewScanner(file)
		lineNum := 0

		for scanner.Scan() {
			lineNum++
			line := scanner.Text()

			// Check if line contains hardcoded ID pattern
			if !hardcodedIDPattern.MatchString(line) {
				continue
			}

			// Check if it's a legitimate use
			isLegitimate := false
			for _, pattern := range legitimatePatterns {
				if strings.Contains(line, pattern) {
					isLegitimate = true
					break
				}
			}

			// Also check word-boundary patterns if not already flagged as legitimate
			if !isLegitimate {
				for _, pattern := range wordBoundaryPatterns {
					if pattern.MatchString(line) {
						isLegitimate = true
						break
					}
				}
			}

			if !isLegitimate {
				relPath, _ := filepath.Rel(root, path)
				violations = append(violations,
					formatViolation(relPath, lineNum, strings.TrimSpace(line)))
			}
		}

		return nil
	})

	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	return violations
}

// checkMissingSetupTestDB finds test files with DB operations that don't use SetupTestDB.
func checkMissingSetupTestDB(t *testing.T, root string) []string {
	t.Helper()

	var violations []string

	// Patterns indicating DB operations
	dbPatterns := []string{
		"bun.DB",
		".NewSelect()",
		".NewInsert()",
		".NewUpdate()",
		".NewDelete()",
		"repositories.",
	}

	// Patterns indicating SetupTestDB or SetupAPITest usage
	setupPatterns := []string{
		"SetupTestDB",
		"setupTestDB",
		"SetupAPITest",
		"setupAPITest",
	}

	// Patterns indicating mock-based testing (legitimate alternative)
	mockPatterns := []string{
		"sqlmock",
		"Mock",
		"mock",
		"Stub",
		"stub",
		"fake",
		"Fake",
	}

	err := filepath.Walk(root, func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return nil
		}

		if info.IsDir() || !strings.HasSuffix(path, "_test.go") {
			return nil
		}

		// Skip this verification test
		if strings.Contains(path, "hermetic_verification_test.go") {
			return nil
		}

		content, err := os.ReadFile(path)
		if err != nil {
			return nil
		}

		contentStr := string(content)

		// Check if file has DB operations
		hasDBOps := false
		for _, pattern := range dbPatterns {
			if strings.Contains(contentStr, pattern) {
				hasDBOps = true
				break
			}
		}

		if !hasDBOps {
			return nil
		}

		// Check if file uses SetupTestDB
		usesSetup := false
		for _, pattern := range setupPatterns {
			if strings.Contains(contentStr, pattern) {
				usesSetup = true
				break
			}
		}

		// Check if file uses mocks
		usesMocks := false
		for _, pattern := range mockPatterns {
			if strings.Contains(contentStr, pattern) {
				usesMocks = true
				break
			}
		}

		// Flag files with DB ops that don't use SetupTestDB and aren't mock-based
		if hasDBOps && !usesSetup && !usesMocks {
			relPath, _ := filepath.Rel(root, path)
			violations = append(violations, "  - "+relPath)
		}

		return nil
	})

	if err != nil {
		t.Logf("Warning: error walking directory: %v", err)
	}

	return violations
}

// formatViolation formats a violation message for display.
func formatViolation(file string, line int, content string) string {
	return "  " + file + ":" + itoa(line) + "\n    " + content
}

// itoa converts an int to string without importing strconv.
func itoa(i int) string {
	if i == 0 {
		return "0"
	}

	var b [20]byte
	pos := len(b)
	neg := i < 0
	if neg {
		i = -i
	}

	for i > 0 {
		pos--
		b[pos] = byte('0' + i%10)
		i /= 10
	}

	if neg {
		pos--
		b[pos] = '-'
	}

	return string(b[pos:])
}
