package cmd

import (
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/delivery/objects"
	tenancyCompose "github.com/moto-nrw/project-phoenix/modules/organizationtenancy/compose"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

func demoSchoolExists(t *testing.T, db *bun.DB, schoolID int64) bool {
	t.Helper()
	var n int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM platform.schools WHERE id = ?`, schoolID).Scan(t.Context(), &n))
	return n == 1
}

// The demo process erases a retired demo school from the database only after
// its uploads are gone (#5559): both steps run as phoenix_demo through the
// owner compositions the dev tools binary binds. A school whose files could
// not be erased keeps its database rows, so the file phase can still find
// it; the next run erases the files and then, in the same run, the rows.
func TestDemoProcessErasesRetiredDemoSchoolsFromTheDatabaseAfterTheirFiles(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := t.Context()
	kinds := demoFileKinds(t)
	uploads := t.TempDir()

	retired, _ := testpkg.CreateTestTenant(t, db)
	young, _ := testpkg.CreateTestTenant(t, db)
	for _, id := range []int64{retired, young} {
		testpkg.CreateTestStudentForTenant(t, db, id, "Demo", "Kind", "1a")
		seedDemoUploads(t, uploads, kinds, id)
	}
	orderDemoSchools(t, db, retired, young)
	hideDemoSchool(t, db, retired, "91 days")
	hideDemoSchool(t, db, young, "89 days")

	demoDB := testpkg.DemoRoleDatabase(t, db)
	seam := &storageSeam{Local: objects.NewLocal(uploads), failRemoval: kinds[0].Kind}
	erasure, err := tenancyCompose.NewDemoSchoolErasure(demoDB, seam)
	require.NoError(t, err)

	// The file phase fails for both: no marker, so the database stays.
	_, _, err = erasure.EraseRetiredDemoSchoolFiles(ctx)
	require.Error(t, err)
	erased, _, err := erasure.EraseRetiredDemoSchoolDatabases(ctx)
	require.NoError(t, err)
	assert.Zero(t, erased)
	assert.True(t, demoSchoolExists(t, db, retired), "the file phase still needs the school")

	seam.failRemoval = ""
	schools, _, err := erasure.EraseRetiredDemoSchoolFiles(ctx)
	require.NoError(t, err)
	assert.Equal(t, 2, schools)
	erased, _, err = erasure.EraseRetiredDemoSchoolDatabases(ctx)
	require.NoError(t, err)
	assert.Equal(t, 1, erased)

	assert.False(t, demoSchoolExists(t, db, retired), "the school retired 91 days ago is gone")
	assert.Empty(t, schoolPaths(t, uploads, retired))
	assert.True(t, demoSchoolExists(t, db, young), "a school retired 89 days ago stays")
	var students int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM users.student_profiles WHERE tenant_id = ?`, young).Scan(ctx, &students))
	assert.Equal(t, 1, students)

	// A second run finds nothing left to do.
	erased, _, err = erasure.EraseRetiredDemoSchoolDatabases(ctx)
	require.NoError(t, err)
	assert.Zero(t, erased)
}

// Only the dev tools binary, which ships in the demo image alone (ADR 0060),
// reaches the database erasure of demo schools: the server binary of the
// production image has no caller, and the database function is granted to
// phoenix_demo only (migration 1.15.844).
func TestOnlyTheDevToolsReachTheDemoSchoolDatabaseErasure(t *testing.T) {
	t.Parallel()
	var callers []string
	require.NoError(t, filepath.WalkDir("..", func(path string, entry fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if entry.IsDir() && strings.HasPrefix(entry.Name(), ".") && path != ".." {
			return filepath.SkipDir
		}
		if entry.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		source, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		text := string(source)
		if strings.Contains(text, "NewDemoSchoolErasure(") || strings.Contains(text, "erase_retired_demo_school(") {
			rel, err := filepath.Rel("..", path)
			callers = append(callers, filepath.ToSlash(rel))
			return err
		}
		return nil
	}))
	sort.Strings(callers)
	assert.Equal(t, []string{
		"cmd/devtools/demo_runtime.go",
		"database/migrations/001015844_demo_school_database_erasure.go",
		"modules/organizationtenancy/compose/erasure.go",
		"modules/organizationtenancy/internal/adapters/postgres/demo_databases.go",
		"modules/organizationtenancy/internal/application/demo_databases.go",
	}, callers)
}
