package auth

import (
	"os"
	"path/filepath"
	"regexp"
	"sort"
	"testing"
)

// TestBaseRoleSyncAcrossLayers ensures ValidBaseRoles, the DB CHECK constraint,
// and the frontend BASE_ROLE_LABELS all define the same set of base roles.
// If this test fails, someone added a role in one place but not all three.
// See also: role.go comment on ValidBaseRoles.
func TestBaseRoleSyncAcrossLayers(t *testing.T) {
	t.Parallel()

	projectRoot, err := findProjectRoot()
	if err != nil {
		t.Skipf("Cannot locate project root: %v", err)
	}

	validRoles := ValidBaseRoles()
	goRoles := make([]string, len(validRoles))
	copy(goRoles, validRoles)
	sort.Strings(goRoles)

	t.Run("Go ValidBaseRoles matches DB CHECK constraint", func(t *testing.T) {
		migrationPath := filepath.Join(projectRoot, "backend/database/migrations/001015031_roles_base_role.go")
		content, err := os.ReadFile(migrationPath)
		if err != nil {
			t.Skipf("Cannot read migration file: %v", err)
		}

		dbRoles := extractCheckConstraintRoles(string(content))
		if dbRoles == nil {
			t.Fatal("Could not parse base_role CHECK constraint from migration file")
		}
		sort.Strings(dbRoles)

		if !equal(goRoles, dbRoles) {
			t.Errorf("ValidBaseRoles %v != DB CHECK constraint %v", goRoles, dbRoles)
		}
	})

	t.Run("Go ValidBaseRoles matches frontend BASE_ROLE_LABELS", func(t *testing.T) {
		frontendPath := filepath.Join(projectRoot, "frontend/src/lib/auth-helpers.ts")
		content, err := os.ReadFile(frontendPath)
		if err != nil {
			t.Skipf("Cannot read frontend file: %v", err)
		}

		feRoles := extractBaseRoleLabelKeys(string(content))
		if feRoles == nil {
			t.Fatal("Could not parse BASE_ROLE_LABELS keys from auth-helpers.ts")
		}
		sort.Strings(feRoles)

		if !equal(goRoles, feRoles) {
			t.Errorf("ValidBaseRoles %v != frontend BASE_ROLE_LABELS %v", goRoles, feRoles)
		}
	})
}

// extractCheckConstraintRoles parses roles from: base_role IN ('admin', 'user', 'guardian')
func extractCheckConstraintRoles(source string) []string {
	re := regexp.MustCompile(`base_role\s+IN\s*\(([^)]+)\)`)
	match := re.FindStringSubmatch(source)
	if len(match) < 2 {
		return nil
	}
	valueRe := regexp.MustCompile(`'([^']+)'`)
	matches := valueRe.FindAllStringSubmatch(match[1], -1)
	var roles []string
	for _, m := range matches {
		roles = append(roles, m[1])
	}
	return roles
}

// extractBaseRoleLabelKeys parses keys from: export const BASE_ROLE_LABELS = { admin: ..., user: ..., }
func extractBaseRoleLabelKeys(source string) []string {
	// Match the object block after BASE_ROLE_LABELS
	blockRe := regexp.MustCompile(`BASE_ROLE_LABELS[^{]*\{([^}]+)\}`)
	match := blockRe.FindStringSubmatch(source)
	if len(match) < 2 {
		return nil
	}
	// Extract keys (unquoted identifiers before colons)
	keyRe := regexp.MustCompile(`(\w+)\s*:`)
	matches := keyRe.FindAllStringSubmatch(match[1], -1)
	var roles []string
	for _, m := range matches {
		roles = append(roles, m[1])
	}
	return roles
}

func equal(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}

// findProjectRoot is duplicated here (from test/helpers.go) to avoid importing
// the testpkg package which pulls in database dependencies. This is a pure
// filesystem helper that doesn't need any external packages.
func findProjectRoot() (string, error) {
	dir, err := os.Getwd()
	if err != nil {
		return "", err
	}
	for {
		if _, err := os.Stat(filepath.Join(dir, "go.mod")); err == nil {
			return filepath.Dir(dir), nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return "", os.ErrNotExist
		}
		dir = parent
	}
}
