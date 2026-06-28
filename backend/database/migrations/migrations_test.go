package migrations

import (
	"context"
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestNoDuplicateMigrationVersions(t *testing.T) {
	// This test scans migration source files to detect version collisions.
	// MigrationRegistry is a map[string]*Migration — if two init() functions
	// register the same version key, the second silently overwrites the first.
	// This causes migrations to be skipped without any error.

	// Match version constants like: someVersion = "1.15.20"
	// Migration files use named constants (not inline strings), so we match
	// the const declaration pattern rather than struct field assignment.
	versionPattern := regexp.MustCompile(`Version\s*=\s*"([^"]+)"`)
	filePrefixPattern := regexp.MustCompile(`^(\d{1,14})_`)

	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatalf("failed to read migrations directory: %v", err)
	}

	versionFiles := make(map[string][]string)
	bunNameFiles := make(map[string][]string)
	migrationFileCount := 0

	for _, entry := range entries {
		name := entry.Name()
		if entry.IsDir() || !strings.HasSuffix(name, ".go") || strings.HasSuffix(name, "_test.go") {
			continue
		}
		// Migration files start with 000 or 001 (schema setup + domain migrations)
		if !strings.HasPrefix(name, "000") && !strings.HasPrefix(name, "001") {
			continue
		}

		if match := filePrefixPattern.FindStringSubmatch(name); match != nil {
			bunNameFiles[match[1]] = append(bunNameFiles[match[1]], name)
		}

		content, err := os.ReadFile(filepath.Join(".", name))
		if err != nil {
			t.Fatalf("failed to read %s: %v", name, err)
		}

		// Only count files that actually register in MigrationRegistry
		if !strings.Contains(string(content), "MigrationRegistry.Register(") && !strings.Contains(string(content), "MigrationRegistry[") {
			continue
		}
		migrationFileCount++

		matches := versionPattern.FindAllSubmatch(content, -1)
		for _, match := range matches {
			version := string(match[1])
			versionFiles[version] = append(versionFiles[version], name)
		}
	}

	// Check for duplicate versions in source files
	for version, files := range versionFiles {
		if len(files) > 1 {
			t.Errorf("VERSION COLLISION: version %s is registered by %d files: %s\n"+
				"Only one will actually run — the others are silently lost.\n"+
				"Each migration MUST have a unique version number.",
				version, len(files), strings.Join(files, ", "))
		}
	}

	for bunName, files := range bunNameFiles {
		if len(files) <= 1 {
			continue
		}

		t.Errorf("BUN NAME COLLISION: filename prefix %s is shared by %d files: %s\n"+
			"Bun identifies Go migrations by this numeric filename prefix, so new collisions are unsafe.",
			bunName, len(files), strings.Join(files, ", "))
	}

	// Belt-and-suspenders: verify file count matches registry count
	registryCount := len(MigrationRegistry)
	if registryCount != migrationFileCount {
		t.Errorf("Migration file count (%d) != MigrationRegistry entries (%d). "+
			"%d migration(s) were silently overwritten due to version collisions.",
			migrationFileCount, registryCount, migrationFileCount-registryCount)
	}
}

func TestScheduleTimeframesAreMigratedToTimezoneFreeClockTimes(t *testing.T) {
	content, err := os.ReadFile("001015050_timeframes_use_time_without_timezone.go")
	if err != nil {
		t.Fatalf("failed to read timeframe clock migration: %v", err)
	}
	src := string(content)
	for _, required := range []string{
		"ALTER TABLE schedule.timeframes",
		"start_time TYPE TIME WITHOUT TIME ZONE",
		"end_time TYPE TIME WITHOUT TIME ZONE",
		"start_time AT TIME ZONE 'UTC'",
		"end_time AT TIME ZONE 'UTC'",
	} {
		if !strings.Contains(src, required) {
			t.Fatalf("timeframe clock migration must contain %q", required)
		}
	}
}

func TestOperatorRefreshTokenMigrationUpDown(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	t.Cleanup(func() { _ = db.Close() })
	ctx := context.Background()

	if err := downOperatorRefreshTokens(ctx, db); err != nil {
		t.Fatalf("pre-clean operator refresh token migration: %v", err)
	}
	t.Cleanup(func() {
		_ = upOperatorRefreshTokens(context.Background(), db)
	})

	if err := upOperatorRefreshTokens(ctx, db); err != nil {
		t.Fatalf("run operator refresh token migration: %v", err)
	}
	if !relationExists(t, db, "platform.operator_refresh_tokens") {
		t.Fatal("operator refresh token table was not created")
	}
	if !relationExists(t, db, "platform.idx_operator_refresh_tokens_operator_id") {
		t.Fatal("operator refresh token operator_id index was not created")
	}

	if err := downOperatorRefreshTokens(ctx, db); err != nil {
		t.Fatalf("rollback operator refresh token migration: %v", err)
	}
	if relationExists(t, db, "platform.operator_refresh_tokens") {
		t.Fatal("operator refresh token table still exists after rollback")
	}
}

func relationExists(t *testing.T, db *bun.DB, relation string) bool {
	t.Helper()
	var exists bool
	if err := db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, relation).Scan(context.Background(), &exists); err != nil {
		t.Fatalf("check relation %s: %v", relation, err)
	}
	return exists
}
