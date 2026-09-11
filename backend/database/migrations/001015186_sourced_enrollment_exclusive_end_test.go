// Verifies the repair installed by migration 1.15.186.
package migrations

import (
	"context"
	"fmt"
	"testing"
	"time"

	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairSourcedEnrollmentExclusiveEnds_IsTenantJoinedAndSourceScoped(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := context.Background()

	const serviceStart = "2027-01-04"
	const serviceEnd = "2027-01-08"
	var phaseID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.phases
			(tenant_id, name, kind, service_start_date, service_end_date,
			 care_overflow_mode, care_offering_selection_mode, available_school_classes, is_active)
		VALUES (?, ?, 'custom', ?, ?, 'waitlist', 'optional', '[]'::jsonb, TRUE)
		RETURNING id
	`, tenantID, fmt.Sprintf("exclusive-end-%d", time.Now().UnixNano()), serviceStart, serviceEnd).Scan(ctx, &phaseID))
	student := testpkg.CreateTestStudentForTenant(t, db, tenantID, "End", "Boundary", "3a")
	wrongStudent := testpkg.CreateTestStudentForTenant(t, db, tenantID, "Wrong", "Student", "3a")
	group := testpkg.CreateTestActivityGroupForTenant(t, db, tenantID, "Exclusive-end group")

	var requestID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.requests
			(tenant_id, phase_id, guardian_first_name, guardian_last_name,
			 guardian_email, consent_flags, custom_data, status_token, submitted_at)
		VALUES (?, ?, 'End', 'Boundary', ?, '{}'::jsonb, '{}'::jsonb, ?, NOW())
		RETURNING id
	`, tenantID, phaseID,
		fmt.Sprintf("exclusive-end-%d@example.test", time.Now().UnixNano()),
		fmt.Sprintf("exclusive-end-%d", time.Now().UnixNano()),
	).Scan(ctx, &requestID))

	var childID int64
	require.NoError(t, db.NewRaw(`
		INSERT INTO enrollment.request_children
			(tenant_id, request_id, first_name, last_name, date_of_birth,
			 status, activation_mode, created_student_id, sort_order, custom_data)
		VALUES (?, ?, 'End', 'Boundary', '2018-04-15',
			'approved', 'scheduled', ?, 0, '{}'::jsonb)
		RETURNING id
	`, tenantID, requestID, student.ID).Scan(ctx, &childID))

	insertEnrollment := func(scopeID, studentID, groupID int64, validUntil string, sourceID *int64) int64 {
		var id int64
		require.NoError(t, db.NewRaw(`
			INSERT INTO activities.student_enrollments
				(tenant_id, student_id, activity_group_id, valid_from, valid_until, enrollment_request_child_id)
			VALUES (?, ?, ?, ?, ?, ?)
			RETURNING id
		`, scopeID, studentID, groupID, serviceStart, validUntil, sourceID).Scan(ctx, &id))
		return id
	}
	targetID := insertEnrollment(tenantID, student.ID, group.ID, serviceEnd, &childID)
	manualID := insertEnrollment(tenantID, student.ID, group.ID, serviceEnd, nil)
	unrelatedID := insertEnrollment(tenantID, student.ID, group.ID, "2027-01-13", &childID)
	wrongStudentSourceID := insertEnrollment(tenantID, wrongStudent.ID, group.ID, serviceEnd, &childID)
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Other", "Tenant", "3a")
	otherGroup := testpkg.CreateTestActivityGroupForTenant(t, db, otherTenantID, "Exclusive-end other tenant")
	// The legacy FK is id-only, so this deliberately points at tenant A's child.
	crossTenantSourceID := insertEnrollment(otherTenantID, otherStudent.ID, otherGroup.ID, serviceEnd, &childID)

	tx, err := db.BeginTx(ctx, &testpkg.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	require.NoError(t, repairSourcedEnrollmentExclusiveEnds(ctx, tx))

	loadEnd := func(id int64) string {
		var end string
		require.NoError(t, tx.NewSelect().
			TableExpr(`activities.student_enrollments`).
			ColumnExpr("valid_until::text").
			Where("id = ?", id).
			Scan(ctx, &end))
		return end
	}
	assert.Equal(t, "2027-01-09", loadEnd(targetID))
	assert.Equal(t, serviceEnd, loadEnd(manualID))
	assert.Equal(t, "2027-01-13", loadEnd(unrelatedID))
	assert.Equal(t, serviceEnd, loadEnd(wrongStudentSourceID))
	assert.Equal(t, serviceEnd, loadEnd(crossTenantSourceID))
}
