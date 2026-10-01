package enrollment_test

import (
	"context"

	testutil "github.com/moto-nrw/project-phoenix/api/testutil"
	"github.com/moto-nrw/project-phoenix/database/repositories"

	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/uptrace/bun"
)

func TestPhaseExpiryProjection_ListSnapshots_RequiresEffectiveSuccessorBooking(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	phaseRepo := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)
	requestRepo := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)
	childRepo := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)
	offeringRepo := newCareOfferingFixtures(testutil.NewTestCarePlan(t, db))
	linkRepo := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant)

	source := makeOwnerEligibilityPhase(uniquePhaseName("expiry-source"))
	source.ServiceStartDate = capability.Date(timezone.NewDate(2026, 8, 1))
	source.ServiceEndDate = capability.Date(timezone.NewDate(2027, 1, 29))
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return phaseRepo.InsertPhase(ctx, source)
	}))

	sourceRequest := makeOwnerRequest(source.ID, uniqueToken("expiry-source"), "expiry@example.test")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return requestRepo.InsertRequest(ctx, sourceRequest)
	}))
	student := testpkg.CreateTestStudent(t, db, "Expiry", "Successor", "2a")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return newExpiryStudentFixture(t, db).SetEnrollmentWindowByID(
			ctx, student.ID, timezone.Date(source.ServiceStartDate), usersModels.StudentStatusActive,
		)
	}))
	sourceChild := makeChild(sourceRequest.ID, "Expiry", "Successor")
	sourceChild.Status = enrollmentModels.ChildStatusApproved
	sourceChild.CreatedStudentID = &student.ID
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return childRepo.InsertChild(ctx, sourceChild)
	}))
	sourceOffering := makeOffering(source.ID, uniqueOfferingName("source-care"))
	sourceOffering.AvailableDays = []string{"mon"}
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return offeringRepo.Create(ctx, sourceOffering)
	}))
	sourceValidFrom := timezone.Date(source.ServiceStartDate)
	sourceValidUntil := timezone.Date(source.ServiceEndDate).AddDays(1)
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return linkRepo.InsertRequestChildOffering(ctx, &capability.RequestChildOffering{
			RequestChildID: sourceChild.ID,
			CareOfferingID: sourceOffering.ID,
			ValidFrom:      offeringDatePointer(sourceValidFrom),
			ValidUntil:     offeringDatePointer(sourceValidUntil),
		})
	}))

	successor := makeOwnerEligibilityPhase(uniquePhaseName("expiry-successor"))
	successor.ServiceStartDate = capability.Date(timezone.NewDate(2027, 2, 2))
	successor.ServiceEndDate = capability.Date(timezone.NewDate(2027, 7, 31))
	successor.RolloverSourcePhaseID = &source.ID
	rolloverMode := enrollmentModels.PhaseRolloverModeOptIn
	successor.RolloverMode = &rolloverMode
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return phaseRepo.InsertPhase(ctx, successor)
	}))

	listSnapshots := func() []*capability.PhaseExpirySnapshot {
		t.Helper()
		var snapshots []*capability.PhaseExpirySnapshot
		require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
			var listErr error
			snapshots, listErr = newPhaseExpiryProjection(t, db).ListSnapshots(
				ctx,
				timezone.NewDate(2027, 1, 2),
				timezone.NewDate(2027, 2, 1),
			)
			return listErr
		}))
		return snapshots
	}

	emptySuccessor := listSnapshots()
	require.Len(t, emptySuccessor, 1)
	require.NotNil(t, emptySuccessor[0].SuccessorPhaseID)
	assert.Equal(t, successor.ID, *emptySuccessor[0].SuccessorPhaseID)
	assert.Equal(t, 1, emptySuccessor[0].UnresolvedChildren)

	successor.ServiceStartDate = capability.Date(timezone.NewDate(2027, 1, 30))
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return phaseRepo.UpdatePhase(ctx, successor)
	}))

	targetRequest := makeOwnerRequest(successor.ID, uniqueToken("expiry-target"), "expiry@example.test")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return requestRepo.InsertRequest(ctx, targetRequest)
	}))
	targetChild := makeChild(targetRequest.ID, "Expiry", "Successor")
	targetChild.Status = enrollmentModels.ChildStatusPendingRenewal
	targetChild.CreatedStudentID = &student.ID
	targetChild.RolloverSourceChildID = &sourceChild.ID
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return childRepo.InsertChild(ctx, targetChild)
	}))
	unrelatedRequest := makeOwnerRequest(successor.ID, uniqueToken("expiry-unrelated"), "other@example.test")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return requestRepo.InsertRequest(ctx, unrelatedRequest)
	}))
	unrelatedRejectedChild := makeChild(unrelatedRequest.ID, "Expiry", "Successor")
	unrelatedRejectedChild.Status = enrollmentModels.ChildStatusRejected
	unrelatedRejectedChild.CreatedStudentID = &student.ID
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return childRepo.InsertChild(ctx, unrelatedRejectedChild)
	}))

	pendingSuccessor := listSnapshots()
	require.Len(t, pendingSuccessor, 1)
	assert.Equal(t, 1, pendingSuccessor[0].UnresolvedChildren,
		"an unrelated rejection for the same student must not override the open rollover candidate")

	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusRejected, nil, 0)
	}))
	rejectedSuccessor := listSnapshots()
	require.Len(t, rejectedSuccessor, 1)
	assert.Equal(t, 0, rejectedSuccessor[0].UnresolvedChildren)

	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0)
	}))
	withdrawnSuccessor := listSnapshots()
	require.Len(t, withdrawnSuccessor, 1)
	assert.Equal(t, 0, withdrawnSuccessor[0].UnresolvedChildren)

	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	approvedWithoutOffering := listSnapshots()
	require.Len(t, approvedWithoutOffering, 1)
	assert.Equal(t, 1, approvedWithoutOffering[0].UnresolvedChildren)

	invalidTargetOffering := makeOffering(successor.ID, uniqueOfferingName("target-invalid"))
	invalidTargetOffering.AvailableDays = []string{"mon"}
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return offeringRepo.Create(ctx, invalidTargetOffering)
	}))
	targetValidFrom := timezone.Date(successor.ServiceStartDate)
	targetValidUntil := timezone.Date(successor.ServiceEndDate).AddDays(1)
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		// Represent historical invalid data without weakening owner-command validation.
		_, err := tx.NewRaw(`INSERT INTO enrollment.care_offering_bookings
			(tenant_id, request_child_id, care_offering_id, manual_selected_days, valid_from, valid_until)
			VALUES (?, ?, ?, '["not-a-day"]'::jsonb, ?, ?)`,
			tenantID, targetChild.ID, invalidTargetOffering.ID, targetValidFrom, targetValidUntil).Exec(ctx)
		return err
	}))
	invalidTargetBooking := listSnapshots()
	require.Len(t, invalidTargetBooking, 1)
	assert.Equal(t, 1, invalidTargetBooking[0].UnresolvedChildren,
		"a booking without a canonical weekday must not hide the warning")

	targetOffering := makeOffering(successor.ID, uniqueOfferingName("target-care"))
	targetOffering.AvailableDays = []string{"tue"}
	targetOffering.IsActive = false
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return offeringRepo.Create(ctx, targetOffering)
	}))
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return linkRepo.InsertRequestChildOffering(ctx, &capability.RequestChildOffering{
			RequestChildID: targetChild.ID,
			CareOfferingID: targetOffering.ID,
			ValidFrom:      offeringDatePointer(targetValidFrom),
			ValidUntil:     offeringDatePointer(targetValidUntil),
		})
	}))
	inactiveTargetBooking := listSnapshots()
	require.Len(t, inactiveTargetBooking, 1)
	assert.Equal(t, 1, inactiveTargetBooking[0].UnresolvedChildren,
		"an inactive target offering must not hide the warning")
	targetOffering.IsActive = true
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return offeringRepo.Update(ctx, targetOffering)
	}))

	mismatchedWeekdayBooking := listSnapshots()
	require.Len(t, mismatchedWeekdayBooking, 1)
	assert.Equal(t, 1, mismatchedWeekdayBooking[0].UnresolvedChildren,
		"a successor booking on another weekday must not hide the warning")

	targetOffering.AvailableDays = []string{"mon"}
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return offeringRepo.Update(ctx, targetOffering)
	}))
	completedSuccessor := listSnapshots()
	require.Len(t, completedSuccessor, 1)
	assert.Equal(t, 0, completedSuccessor[0].UnresolvedChildren)

	replacementStudent := testpkg.CreateTestStudent(t, db, "Replacement", "Successor", "2a")
	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		_, err := tx.NewUpdate().
			TableExpr(`enrollment.request_children AS "request_child"`).
			Set("created_student_id = ?", replacementStudent.ID).
			Where(`"request_child".id = ?`, targetChild.ID).
			Exec(ctx)
		return err
	}))
	lineageWithReplacementStudent := listSnapshots()
	require.Len(t, lineageWithReplacementStudent, 1)
	assert.Equal(t, 0, lineageWithReplacementStudent[0].UnresolvedChildren,
		"an approved rollover child must complete the warning despite a replacement student ID")

	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		if updateErr := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(
			ctx,
			targetChild.ID,
			enrollmentModels.ChildStatusPendingRenewal,
			nil,
			0,
		); updateErr != nil {
			return updateErr
		}
		if updateErr := repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(
			ctx,
			unrelatedRejectedChild.ID,
			enrollmentModels.ChildStatusRejected,
			nil,
			0,
		); updateErr != nil {
			return updateErr
		}
		if _, updateErr := tx.NewUpdate().
			TableExpr(`enrollment.phases AS "phase"`).
			Set("rollover_source_phase_id = NULL").
			Set("rollover_mode = NULL").
			Where(`"phase".id = ?`, successor.ID).
			Exec(ctx); updateErr != nil {
			return updateErr
		}
		_, updateErr := tx.NewUpdate().
			TableExpr(`enrollment.request_children AS "request_child"`).
			Set("created_student_id = ?", student.ID).
			Set("rollover_source_child_id = NULL").
			Where(`"request_child".id = ?`, targetChild.ID).
			Exec(ctx)
		return updateErr
	}))

	temporalFallback := listSnapshots()
	require.Len(t, temporalFallback, 1)
	assert.Equal(t, 1, temporalFallback[0].UnresolvedChildren,
		"a pending legacy successor must take precedence over an unrelated rejection")

	require.NoError(t, testpkg.WithTenantTx(t, testpkg.Ctx(t), db, tenantID, func(ctx context.Context, tx bun.Tx) error {
		return repositories.NewEnrollmentBookingFixture(testpkg.WithinCurrentTenant).UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	temporalFallback = listSnapshots()
	require.Len(t, temporalFallback, 1)
	require.NotNil(t, temporalFallback[0].SuccessorPhaseID)
	assert.Equal(t, successor.ID, *temporalFallback[0].SuccessorPhaseID,
		"a contiguous school-year phase must be recognized without rollover lineage")
	assert.Equal(t, 0, temporalFallback[0].UnresolvedChildren,
		"an approved booking for the same student must complete a legacy successor without child lineage")
}
