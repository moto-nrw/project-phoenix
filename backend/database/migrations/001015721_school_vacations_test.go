package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestSchoolVacationGrants pins the roles of 1.15.721 (#5345): every school
// reads the vacations, only the administrative role writes them, and the
// confirmations of planned care stay within their school.
func TestSchoolVacationGrants(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	privilege := func(role, table, action string) bool {
		var granted bool
		require.NoError(t, db.NewRaw(`SELECT has_table_privilege(?, ?, ?)`, role, table, action).Scan(ctx, &granted))
		return granted
	}
	assert.True(t, privilege("phoenix_tenant", "calendar.school_vacations", "SELECT"))
	assert.False(t, privilege("phoenix_tenant", "calendar.school_vacations", "INSERT"), "a school request never writes the vacations")
	assert.False(t, privilege("phoenix_tenant", "calendar.school_vacations", "DELETE"))
	assert.True(t, privilege("phoenix_admin", "calendar.school_vacations", "INSERT"))
	assert.True(t, privilege("phoenix_admin", "calendar.school_vacations", "DELETE"))
	assert.False(t, privilege("phoenix_auth", "calendar.school_vacations", "SELECT"))
	assert.True(t, privilege("phoenix_tenant", "calendar.school_vacation_acknowledgements", "INSERT"))
	assert.False(t, privilege("phoenix_tenant", "calendar.school_vacation_acknowledgements", "UPDATE"), "a confirmation is not edited")

	var forced bool
	require.NoError(t, db.NewRaw(`SELECT relforcerowsecurity FROM pg_class WHERE oid = 'calendar.school_vacation_acknowledgements'::regclass`).Scan(ctx, &forced))
	assert.True(t, forced, "the confirmations are school data under row-level security")
}

// TestSchoolVacationsMigrationRoundTrip rolls 1.15.721 back and forward: the
// down drops both tables and restores the erasure plan of 1.15.720, the up
// puts the confirmations back into it.
func TestSchoolVacationsMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()

	// The Nachweisarten (1.15.724, #4808) extend the erasure functions of
	// 1.15.721; they go first.
	require.NoError(t, staffQualificationTypesDown(ctx, db))
	require.NoError(t, schoolVacationsDown(ctx, db))
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('calendar.school_vacations') IS NOT NULL OR to_regclass('calendar.school_vacation_acknowledgements') IS NOT NULL`).Scan(ctx, &exists))
	assert.False(t, exists)
	for _, entry := range readErasurePlan(t, db) {
		assert.NotContains(t, entry.Name, "school_vacation")
	}

	require.NoError(t, schoolVacationsUp(ctx, db))
	names := map[string]bool{}
	for _, entry := range readErasurePlan(t, db) {
		names[entry.Name] = true
	}
	assert.True(t, names["calendar.school_vacation_acknowledgements"])
	assert.False(t, names["calendar.school_vacations"], "the vacations hold for every school and are no school data")
}

// TestSchoolVacationAcknowledgementIsUniquePerWindow keeps one confirmation
// per school and window.
func TestSchoolVacationAcknowledgementIsUniquePerWindow(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	school := testpkg.Tenant(t)
	insert := func() error {
		_, err := db.ExecContext(ctx, `INSERT INTO calendar.school_vacation_acknowledgements (tenant_id, region_code, start_date, end_date)
			VALUES (?, 'DE-NW', '2026-12-23', '2027-01-06')`, school)
		return err
	}
	require.NoError(t, insert())
	assert.ErrorContains(t, insert(), "uq_calendar_school_vacation_acknowledgements_window")
	_, err := db.ExecContext(ctx, `INSERT INTO calendar.school_vacation_acknowledgements (tenant_id, region_code, start_date, end_date)
		VALUES (?, 'DE-NW', '2027-01-06', '2026-12-23')`, school)
	assert.Error(t, err, "a window ends after it starts")
}
