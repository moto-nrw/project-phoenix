package migrations

import (
	"context"
	"testing"

	"github.com/uptrace/bun"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// insertPayrollWageTypes gives a school one wage type, a night rule on it and
// an absence type booked onto it.
func insertPayrollWageTypes(t *testing.T, db *bun.DB, school int64) int64 {
	t.Helper()
	var wageType int64
	require.NoError(t, db.NewRaw(`INSERT INTO active.payroll_wage_types (tenant_id, name, number, unit)
		VALUES (?, 'Nachtzuschlag', '310', 'stunden') RETURNING id`, school).Scan(t.Context(), &wageType))
	erasureExec(t, db, `INSERT INTO active.payroll_surcharge_rules
		(tenant_id, kind, window_start_minute, window_end_minute, rate_type, rate_hundredths, wage_type_id)
		VALUES (?, 'nacht', 1200, 360, 'prozent', 2500, ?)`, school, wageType)
	erasureExec(t, db, `INSERT INTO active.staff_absence_types (tenant_id, name, base_type, is_active, wage_type_id)
		VALUES (?, 'Sonderurlaub', 'other', true, ?)`, school, wageType)
	return wageType
}

// TestPayrollWageTypesMigrationRoundTrip pins that 1.15.724 runs twice, rolls
// back to the 1.15.721 erasure without its tables and columns, and applies
// again; the school erasure then removes a school's wage types and rules.
func TestPayrollWageTypesMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	exists := func(relation string) bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT to_regclass(?) IS NOT NULL`, relation).Scan(ctx, &found))
		return found
	}
	column := func(table, name string) bool {
		var found bool
		require.NoError(t, db.NewRaw(`SELECT EXISTS (SELECT FROM information_schema.columns
			WHERE table_schema = 'active' AND table_name = ? AND column_name = ?)`, table, name).Scan(ctx, &found))
		return found
	}

	require.NoError(t, payrollWageTypesUp(ctx, db), "the migration is idempotent")
	require.NoError(t, payrollWageTypesDown(ctx, db))
	assert.False(t, exists("active.payroll_wage_types"))
	assert.False(t, exists("active.payroll_surcharge_rules"))
	assert.False(t, column("staff_absence_types", "wage_type_id"))
	assert.False(t, column("staff_month_balance_snapshots", "payroll_rules"))
	for _, entry := range readErasurePlan(t, db) {
		assert.NotContains(t, []string{"active.payroll_wage_types", "active.payroll_surcharge_rules"}, entry.Name)
	}
	rolledBack := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, rolledBack)
	require.Equal(t, "db_done", eraseSchoolAsCLI(t, db, rolledBack, beginErasure(t, db, rolledBack)).Status,
		"the restored erasure still works")

	require.NoError(t, payrollWageTypesUp(ctx, db))
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	insertPayrollWageTypes(t, db, school)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	kept := insertPayrollWageTypes(t, db, other)

	result := eraseSchoolAsCLI(t, db, school, beginErasure(t, db, school))
	require.Equal(t, "db_done", result.Status, "the erasure covers the wage types")
	assert.EqualValues(t, 1, result.RowCounts["active.payroll_wage_types"])
	assert.EqualValues(t, 1, result.RowCounts["active.payroll_surcharge_rules"])
	var left int
	require.NoError(t, db.NewRaw(`SELECT count(*) FROM active.payroll_wage_types WHERE id = ?`, kept).Scan(ctx, &left))
	assert.Equal(t, 1, left, "the other school keeps its wage types")
}

// TestPayrollWageTypesStayInsideTheirSchool pins the tenant references: a
// rule or an absence type of one school cannot book onto another school's
// wage type, and the table checks reject what the export cannot book.
func TestPayrollWageTypesStayInsideTheirSchool(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	other := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, other)
	testpkg.OwnTenantRows(t, db, school, other)
	foreign := insertPayrollWageTypes(t, db, other)

	_, err := db.ExecContext(ctx, `INSERT INTO active.payroll_surcharge_rules
		(tenant_id, kind, rate_type, rate_hundredths, wage_type_id) VALUES (?, 'sonntag', 'prozent', 5000, ?)`, school, foreign)
	assert.Error(t, err, "a rule books onto a wage type of its own school only")
	_, err = db.ExecContext(ctx, `INSERT INTO active.staff_absence_types (tenant_id, name, base_type, is_active, wage_type_id)
		VALUES (?, 'Kur', 'other', true, ?)`, school, foreign)
	assert.Error(t, err, "an absence type books onto a wage type of its own school only")

	own := insertPayrollWageTypes(t, db, school)
	for name, query := range map[string]string{
		"night rule without window": `INSERT INTO active.payroll_surcharge_rules (tenant_id, kind, rate_type, rate_hundredths, wage_type_id) VALUES (?, 'nacht', 'prozent', 2500, ?)`,
		"Sunday rule with window":   `INSERT INTO active.payroll_surcharge_rules (tenant_id, kind, window_start_minute, window_end_minute, rate_type, rate_hundredths, wage_type_id) VALUES (?, 'sonntag', 0, 60, 'prozent', 2500, ?)`,
		"zero rate":                 `INSERT INTO active.payroll_surcharge_rules (tenant_id, kind, rate_type, rate_hundredths, wage_type_id) VALUES (?, 'feiertag', 'betrag', 0, ?)`,
	} {
		_, err := db.ExecContext(ctx, query, school, own)
		assert.Error(t, err, name)
	}
	_, err = db.ExecContext(ctx, `INSERT INTO active.payroll_wage_types (tenant_id, name, number, unit) VALUES (?, 'Lohnart', '12345', 'stunden')`, school)
	assert.Error(t, err, "a DATEV number has at most four digits")
	_, err = db.ExecContext(ctx, `INSERT INTO active.payroll_wage_types (tenant_id, name, unit) VALUES (?, ' nachtzuschlag ', 'tage')`, school)
	assert.Error(t, err, "two active wage types of a school do not share a name")
}
