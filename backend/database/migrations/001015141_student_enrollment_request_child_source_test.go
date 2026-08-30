package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestStudentEnrollmentRequestChildSourceDoesNotBackfillAmbiguousManualRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.CleanupTenantTestData(t, db, tenantID)

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Manual", "Roster", "2a")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "ManualRoster")
	phaseID := insertRequestChildSourcePhase(t, db, tenantID)
	requestID := insertRequestChildSourceRequest(t, db, tenantID, phaseID)
	childID := insertRequestChildSourceChild(t, db, tenantID, requestID, student.ID)
	offeringID := insertRequestChildSourceOffering(t, db, tenantID, phaseID, group.ID)
	insertRequestChildSourceOfferingLink(t, db, tenantID, childID, offeringID)
	enrollmentID := insertRequestChildSourceManualEnrollment(t, db, tenantID, student.ID, group.ID)

	require.NoError(t, studentEnrollmentRequestChildSourceUp(ctx, db))

	var sourceID *int64
	require.NoError(t, db.NewRaw(`
		SELECT enrollment_request_child_id
		FROM activities.student_enrollments
		WHERE id = ?
	`, enrollmentID).Scan(ctx, &sourceID))
	require.Nil(t, sourceID)
}

func TestStudentEnrollmentRequestChildSourceBackfillsUnambiguousApprovalRows(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	ctx := context.Background()
	tenantID := time.Now().UnixNano()
	testpkg.EnsureTestTenant(t, db, tenantID)
	testpkg.CleanupTenantTestData(t, db, tenantID)

	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Legacy", "Roster", "2a")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "LegacyRoster")
	phaseID := insertRequestChildSourcePhase(t, db, tenantID)
	requestID := insertRequestChildSourceRequest(t, db, tenantID, phaseID)
	childID := insertRequestChildSourceChild(t, db, tenantID, requestID, student.ID)
	offeringID := insertRequestChildSourceOffering(t, db, tenantID, phaseID, group.ID)
	insertRequestChildSourceOfferingLink(t, db, tenantID, childID, offeringID)
	enrollmentID := insertRequestChildSourceManualEnrollment(t, db, tenantID, student.ID, group.ID)
	_, err := db.NewRaw(`
		UPDATE enrollment.request_children
		SET reviewed_at = NOW() + INTERVAL '1 minute'
		WHERE id = ?
	`, childID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, studentEnrollmentRequestChildSourceUp(ctx, db))

	var sourceID *int64
	require.NoError(t, db.NewRaw(`
		SELECT enrollment_request_child_id
		FROM activities.student_enrollments
		WHERE id = ?
	`, enrollmentID).Scan(ctx, &sourceID))
	require.NotNil(t, sourceID)
	require.Equal(t, childID, *sourceID)
}

func insertRequestChildSourcePhase(t *testing.T, db *testpkg.DB, tenantID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.phases (
			tenant_id, name, kind, service_start_date, service_end_date,
			care_overflow_mode, care_offering_selection_mode, is_active
		)
		VALUES (?, ?, 'school_year', '2026-09-01', '2027-07-31', 'waitlist', 'optional', TRUE)
		RETURNING id
	`, tenantID, "Manual Backfill Guard").Scan(context.Background(), &id))
	return id
}

func insertRequestChildSourceRequest(t *testing.T, db *testpkg.DB, tenantID, phaseID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests (
			tenant_id, phase_id, guardian_first_name, guardian_last_name,
			guardian_email, consent_flags, custom_data, status_token
		)
		VALUES (?, ?, 'Eva', 'Roster', ?, '{}'::jsonb, '{}'::jsonb, ?)
		RETURNING id
	`, tenantID, phaseID, "manual-roster@example.test", fmt.Sprintf("manual-roster-token-%d", tenantID)).Scan(context.Background(), &id))
	return id
}

func insertRequestChildSourceChild(t *testing.T, db *testpkg.DB, tenantID, requestID, studentID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.request_children (
			tenant_id, request_id, first_name, last_name, date_of_birth,
			custom_data, status, activation_mode, created_student_id, sort_order
		)
		VALUES (?, ?, 'Manual', 'Roster', '2018-01-02', '{}'::jsonb, 'approved', 'scheduled', ?, 0)
		RETURNING id
	`, tenantID, requestID, studentID).Scan(context.Background(), &id))
	return id
}

func insertRequestChildSourceOffering(t *testing.T, db *testpkg.DB, tenantID, phaseID, groupID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.care_offerings (
			tenant_id, phase_id, activity_group_id, name, days_of_week_mode,
			available_days, is_active, sort_order
		)
		VALUES (?, ?, ?, 'Ganztag', 'parent_choice', '["mon"]'::jsonb, TRUE, 1)
		RETURNING id
	`, tenantID, phaseID, groupID).Scan(context.Background(), &id))
	return id
}

func insertRequestChildSourceOfferingLink(t *testing.T, db *testpkg.DB, tenantID, childID, offeringID int64) {
	t.Helper()
	_, err := db.NewRaw(`
		INSERT INTO enrollment.request_child_offerings (
			tenant_id, request_child_id, care_offering_id, selected_days
		)
		VALUES (?, ?, ?, '["mon"]'::jsonb)
	`, tenantID, childID, offeringID).Exec(context.Background())
	require.NoError(t, err)
}

func insertRequestChildSourceManualEnrollment(t *testing.T, db *testpkg.DB, tenantID, studentID, groupID int64) int64 {
	t.Helper()
	var id int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO activities.student_enrollments (
			tenant_id, student_id, activity_group_id, valid_from, valid_until
		)
		VALUES (?, ?, ?, '2026-09-01', '2027-07-31')
		RETURNING id
	`, tenantID, studentID, groupID).Scan(context.Background(), &id))
	return id
}
