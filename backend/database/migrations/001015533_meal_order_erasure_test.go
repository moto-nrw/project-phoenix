package migrations

import (
	"fmt"
	"os"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// #3873: erasing a school removes its meal orders, registration changes,
// caterer invitations and supplies, and keeps the caterer business, its
// accounts and the supply of another school.
func TestMealOrderErasureRemovesTheSchoolsOrdersAndKeepsTheCaterer(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	schoolID := testpkg.Tenant(t)
	otherSchool, _ := testpkg.CreateTestTenant(t, db)
	var catererID int64
	require.NoError(t, db.NewRaw(`INSERT INTO auth.caterers (name) VALUES ('Erasure Caterer') RETURNING id`).Scan(t.Context(), &catererID))
	erasureExec(t, db, `INSERT INTO auth.caterer_schools (caterer_id, tenant_id) VALUES (?, ?), (?, ?)`, catererID, schoolID, catererID, otherSchool)
	erasureExec(t, db, `INSERT INTO schedule.meal_orders (tenant_id, period_start, period_end, deadline_at) VALUES (?, '2026-11-02', '2026-11-02', '2026-11-02 09:00+01')`, schoolID)

	var preview []byte
	operator := testpkg.CreateTestOperator(t, db)
	require.NoError(t, db.NewRaw(`SELECT platform.school_erasure_preview(?, ?)::text`, schoolID, operator.ID).Scan(t.Context(), &preview))
	assert.Contains(t, string(preview), `"auth.caterer_schools": 1`)
	assert.Contains(t, string(preview), `"schedule.meal_orders": 1`)

	orderID := beginErasure(t, db, schoolID)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, schoolID, orderID).Status)
	var remaining struct{ Caterers, Supplies, Orders int }
	require.NoError(t, db.NewRaw(`SELECT
		(SELECT count(*) FROM auth.caterers WHERE id = ?) AS caterers,
		(SELECT count(*) FROM auth.caterer_schools WHERE caterer_id = ?) AS supplies,
		(SELECT count(*) FROM schedule.meal_orders WHERE tenant_id = ?) AS orders`, catererID, catererID, schoolID).Scan(t.Context(), &remaining))
	assert.Equal(t, struct{ Caterers, Supplies, Orders int }{Caterers: 1, Supplies: 1, Orders: 0}, remaining)
}

// Down removes exactly the meal ordering tables from the installed plan and
// Up adds them again.
func TestMealOrderErasureRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	planNames := func() []string {
		t.Helper()
		names := make([]string, 0)
		for _, entry := range readErasurePlan(t, db) {
			names = append(names, entry.Name)
		}
		return names
	}
	withMeals := planNames()
	require.Contains(t, withMeals, "schedule.meal_orders")
	require.NoError(t, mealOrderErasureDown(t.Context(), db))
	without := planNames()
	assert.Len(t, without, len(withMeals)-8)
	assert.NotContains(t, without, "auth.caterer_schools")
	require.NoError(t, mealOrderErasureUp(t.Context(), db))
	assert.Equal(t, withMeals, planNames())
}

// mealOrderErasureTables are the eight school-scoped tables 1.15.533 adds to
// development's school erasure functions.
var mealOrderErasureTables = regexp.MustCompile(`\b(schedule\.(meal_order_changes|meal_order_change_counters|meal_order_lines|meal_order_days|meal_orders|meal_registration_changes)|auth\.caterer_(invitations|schools))\b`)

var schoolErasureFingerprint = regexp.MustCompile(`<> '[0-9a-f]{64}' THEN`)

// withoutMealOrderErasureTables removes every line naming a meal table (with
// the UNION ALL joining it to the residual check) and blanks the FK
// fingerprint, which the meal tables' foreign keys change by design.
func withoutMealOrderErasureTables(sql string) string {
	kept := make([]string, 0)
	for _, line := range strings.Split(sql, "\n") {
		if mealOrderErasureTables.MatchString(line) {
			if n := len(kept); n > 0 && strings.TrimSpace(kept[n-1]) == "UNION ALL" {
				kept = kept[:n-1]
			}
			continue
		}
		kept = append(kept, line)
	}
	return schoolErasureFingerprint.ReplaceAllString(strings.Join(kept, "\n"), "<> 'fingerprint' THEN")
}

// The erasure functions 1.15.533 installs are static copies (new migrations
// take static SQL only). Development keeps its current definition in the
// school_erasure_{plan,preflight,residual,delete,preview}.go constants and
// every erasure migration reinstalls them, so 1.15.533 running last would
// silently drop a table development added after the copies were taken. Each
// copy must therefore equal development's current function with exactly the
// meal tables added: when this fails, rebuild the copy from development's
// constant, recompute the fingerprint and depend on the new migration.
func TestMealOrderErasureCopiesTrackDevelopmentsErasureFunctions(t *testing.T) {
	t.Parallel()
	for _, pair := range []struct{ name, development, meal string }{
		{"plan", schoolErasurePlanSQL, mealOrderErasurePlanSQL},
		{"preflight", schoolErasurePreflightSQL, mealOrderErasurePreflightSQL},
		{"residual", schoolErasureResidualSQL, mealOrderErasureResidualSQL},
		{"delete", schoolErasureDeleteSQL, mealOrderErasureDeleteSQL},
		{"preview", schoolErasurePreviewSQL, mealOrderErasurePreviewSQL},
	} {
		require.Empty(t, mealOrderErasureTables.FindAllString(pair.development, -1), "%s: development's function names no meal table", pair.name)
		names := map[string]bool{}
		for _, name := range mealOrderErasureTables.FindAllString(pair.meal, -1) {
			names[name] = true
		}
		assert.Len(t, names, 8, "%s: the copy names all eight meal tables", pair.name)
		assert.Equal(t, withoutMealOrderErasureTables(pair.development), withoutMealOrderErasureTables(pair.meal),
			"%s: the 1.15.533 copy drifted from development's current erasure function", pair.name)
	}
}

var schoolErasureInstall = regexp.MustCompile(`\binstallSchoolErasure(Functions|Plan|Preflight|Residual|Delete|Preview)?\(ctx`)

// 1.15.533 must run after the newest development migration that reinstalls
// the school erasure functions, or that migration overwrites the meal
// tables (and a newer one would need its function rebuilt here first).
func TestMealOrderErasureRunsAfterEveryErasureReinstall(t *testing.T) {
	t.Parallel()
	entries, err := os.ReadDir(".")
	require.NoError(t, err)
	newest := ""
	for _, entry := range entries {
		name := entry.Name()
		if len(name) < 10 || name[9] != '_' || strings.HasSuffix(name, "_test.go") || strings.HasPrefix(name, "001015533_") {
			continue
		}
		source, err := os.ReadFile(name)
		require.NoError(t, err)
		if !schoolErasureInstall.Match(source) {
			continue
		}
		parts := make([]any, 0, 3)
		for _, segment := range []string{name[0:3], name[3:6], name[6:9]} {
			number, err := strconv.Atoi(segment)
			require.NoError(t, err, name)
			parts = append(parts, number)
		}
		if version := fmt.Sprintf("%d.%d.%d", parts...); newest == "" || compareVersionsSemantic(newest, version) {
			newest = version
		}
	}
	require.NotEmpty(t, newest)
	assert.True(t, compareVersionsSemantic(newest, mealOrderErasureVersion),
		"%s reinstalls the school erasure after %s: rebuild the meal copies on top of it", newest, mealOrderErasureVersion)
	assert.True(t, slices.Contains(MigrationRegistry[mealOrderErasureVersion].DependsOn, newest),
		"%s must depend on %s, the newest migration installing development's erasure functions", mealOrderErasureVersion, newest)
}
