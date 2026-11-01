package migrations

import (
	"context"
	"testing"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/uptrace/bun"
)

// TestParentConferenceBookingConstraints pins the double-booking guard of
// 1.15.720 (#4801): a slot holds one child, a child one slot per day.
func TestParentConferenceBookingConstraints(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	conferenceID, school := insertParentConference(t, db)
	first := testpkg.CreateTestStudentForTenant(t, db, school, "Erste", "Familie", "1a")
	second := testpkg.CreateTestStudentForTenant(t, db, school, "Zweite", "Familie", "1a")

	book := func(studentID int64, minute int) error {
		_, err := db.ExecContext(context.Background(), `INSERT INTO calendar.parent_conference_bookings (tenant_id, conference_id, student_id, slot_start_minute)
			VALUES (?, ?, ?, ?)`, school, conferenceID, studentID, minute)
		return err
	}
	require.NoError(t, book(first.ID, 840))
	assert.ErrorContains(t, book(second.ID, 840), "uq_calendar_parent_conference_bookings_slot", "a slot holds one child")
	assert.ErrorContains(t, book(first.ID, 855), "uq_calendar_parent_conference_bookings_student", "a child holds one slot per day")
	require.NoError(t, book(second.ID, 855))

	_, err := db.ExecContext(context.Background(), `INSERT INTO calendar.parent_conferences
		(tenant_id, organizer_staff_id, school_class, conference_date, start_minute, end_minute, slot_minutes)
		SELECT tenant_id, organizer_staff_id, school_class, conference_date, 900, 840, 15 FROM calendar.parent_conferences WHERE id = ?`, conferenceID)
	assert.Error(t, err, "a day ends after it starts")
}

// TestParentConferencesMigrationRoundTrip rolls 1.15.720 back and forward:
// the down drops the tables, the booking permission and the booked
// appointments, and the up restores the erasure plan with the tables.
func TestParentConferencesMigrationRoundTrip(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupIsolatedTestDB(t)
	ctx := context.Background()
	insertParentConference(t, db)

	// The wage types (1.15.722, #4807) and the school vacations (1.15.721,
	// #5345) extend this erasure; they go first.
	require.NoError(t, payrollWageTypesDown(ctx, db))
	require.NoError(t, schoolVacationsDown(ctx, db))
	require.NoError(t, parentConferencesDown(ctx, db))
	var exists bool
	require.NoError(t, db.NewRaw(`SELECT to_regclass('calendar.parent_conferences') IS NOT NULL`).Scan(ctx, &exists))
	assert.False(t, exists)
	for _, entry := range readErasurePlan(t, db) {
		assert.NotContains(t, entry.Name, "parent_conference")
	}
	require.NoError(t, db.NewRaw(`SELECT to_regprocedure('platform.school_erasure_freeze_child_keys(bigint)') IS NOT NULL`).Scan(ctx, &exists))
	assert.False(t, exists, "the rollback drops the key freeze helper")

	require.NoError(t, parentConferencesUp(ctx, db))
	names := map[string]bool{}
	for _, entry := range readErasurePlan(t, db) {
		names[entry.Name] = true
	}
	assert.True(t, names["calendar.parent_conferences"])
	assert.True(t, names["calendar.parent_conference_bookings"])
	require.NoError(t, schoolVacationsUp(ctx, db))
	require.NoError(t, payrollWageTypesUp(ctx, db))

	// Like school_erasure_assert_empty, only the erasure itself calls the
	// key freeze: no application role may execute it directly.
	var exposed bool
	require.NoError(t, db.NewRaw(`SELECT bool_or(has_function_privilege(role, 'platform.school_erasure_freeze_child_keys(bigint)', 'EXECUTE'))
		FROM unnest(ARRAY['public','phoenix_auth','phoenix_tenant','phoenix_admin','phoenix_demo','phoenix_school_erasure']) AS role`).Scan(ctx, &exposed))
	assert.False(t, exposed)
}

func insertParentConference(t *testing.T, db *bun.DB) (int64, int64) {
	t.Helper()
	school := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, school)
	staff, _ := testpkg.CreateTestStaffWithAccountForTenant(t, db, school, "Lena", "Lehrerin")
	var id int64
	require.NoError(t, db.NewRaw(`INSERT INTO calendar.parent_conferences
		(tenant_id, organizer_staff_id, school_class, conference_date, start_minute, end_minute, slot_minutes)
		VALUES (?, ?, '1a', '2026-11-12', 840, 960, 15) RETURNING id`, school, staff.ID).Scan(context.Background(), &id))
	return id, school
}
