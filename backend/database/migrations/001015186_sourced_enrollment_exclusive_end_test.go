// Verifies the repair installed by migration 1.15.186.
package migrations

import (
	"database/sql"
	"fmt"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/database/repositories"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRepairSourcedEnrollmentExclusiveEnds_IsTenantJoinedAndSourceScoped(t *testing.T) {
	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	ctx := testpkg.TenantContext(tenantID)

	phase := &enrollmentModels.Phase{
		Name:                      fmt.Sprintf("exclusive-end-%d", time.Now().UnixNano()),
		Kind:                      enrollmentModels.PhaseKindCustom,
		ServiceStartDate:          timezone.NewDate(2027, time.January, 4),
		ServiceEndDate:            timezone.NewDate(2027, time.January, 8),
		CareOverflowMode:          enrollmentModels.PhaseCareOverflowWaitlist,
		CareOfferingSelectionMode: enrollmentModels.PhaseCareOfferingSelectionOptional,
		AvailableSchoolClasses:    []string{},
		IsActive:                  true,
	}
	phase.SetTenantID(tenantID)
	repos := repositories.NewFactory(db)
	require.NoError(t, repos.Phase.Create(ctx, phase))
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
	`, tenantID, phase.ID,
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

	legacyEnd := phase.ServiceEndDate
	unrelatedEnd := phase.ServiceEndDate.AddDays(5)
	target := &activitiesModels.StudentEnrollment{
		StudentID: student.ID, ActivityGroupID: group.ID,
		ValidFrom: phase.ServiceStartDate, ValidUntil: &legacyEnd,
		EnrollmentRequestChildID: &childID,
	}
	manual := &activitiesModels.StudentEnrollment{
		StudentID: student.ID, ActivityGroupID: group.ID,
		ValidFrom: phase.ServiceStartDate, ValidUntil: &legacyEnd,
	}
	unrelated := &activitiesModels.StudentEnrollment{
		StudentID: student.ID, ActivityGroupID: group.ID,
		ValidFrom: phase.ServiceStartDate, ValidUntil: &unrelatedEnd,
		EnrollmentRequestChildID: &childID,
	}
	wrongStudentSource := &activitiesModels.StudentEnrollment{
		StudentID: wrongStudent.ID, ActivityGroupID: group.ID,
		ValidFrom: phase.ServiceStartDate, ValidUntil: &legacyEnd,
		// A stale id-only source reference must not let the migration rewrite a
		// different student's roster bounds.
		EnrollmentRequestChildID: &childID,
	}
	for _, row := range []*activitiesModels.StudentEnrollment{target, manual, unrelated, wrongStudentSource} {
		row.SetTenantID(tenantID)
		require.NoError(t, repos.StudentEnrollment.Create(ctx, row))
	}
	otherTenantID := testpkg.UniqueTestTenantID(t)
	testpkg.EnsureTestTenant(t, db, otherTenantID)
	otherCtx := testpkg.TenantContext(otherTenantID)
	otherStudent := testpkg.CreateTestStudentForTenant(t, db, otherTenantID, "Other", "Tenant", "3a")
	otherGroup := testpkg.CreateTestActivityGroupForTenant(t, db, otherTenantID, "Exclusive-end other tenant")
	crossTenantSource := &activitiesModels.StudentEnrollment{
		StudentID: otherStudent.ID, ActivityGroupID: otherGroup.ID,
		ValidFrom: phase.ServiceStartDate, ValidUntil: &legacyEnd,
		// The legacy FK is id-only, so this deliberately points at tenant A's
		// child. The repair's tenant joins must prevent cross-tenant mutation.
		EnrollmentRequestChildID: &childID,
	}
	crossTenantSource.SetTenantID(otherTenantID)
	require.NoError(t, repositories.NewFactory(db).StudentEnrollment.Create(otherCtx, crossTenantSource))

	tx, err := db.BeginTx(ctx, &sql.TxOptions{})
	require.NoError(t, err)
	t.Cleanup(func() { _ = tx.Rollback() })
	require.NoError(t, repairSourcedEnrollmentExclusiveEnds(ctx, tx))

	loadEnd := func(id int64) timezone.Date {
		var end timezone.Date
		require.NoError(t, tx.NewSelect().
			TableExpr(`activities.student_enrollments`).
			Column("valid_until").
			Where("id = ?", id).
			Scan(ctx, &end))
		return end
	}
	assert.Equal(t, phase.ServiceEndDate.AddDays(1), loadEnd(target.ID))
	assert.Equal(t, phase.ServiceEndDate, loadEnd(manual.ID))
	assert.Equal(t, unrelatedEnd, loadEnd(unrelated.ID))
	assert.Equal(t, phase.ServiceEndDate, loadEnd(wrongStudentSource.ID))
	assert.Equal(t, phase.ServiceEndDate, loadEnd(crossTenantSource.ID))
}
