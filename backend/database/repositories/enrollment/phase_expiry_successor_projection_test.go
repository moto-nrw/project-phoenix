package enrollment_test

import (
	"context"
	"testing"

	capability "github.com/moto-nrw/project-phoenix/modules/enrollment"
	enrollmentCompose "github.com/moto-nrw/project-phoenix/modules/enrollment/enrollmenttest"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	usersRepo "github.com/moto-nrw/project-phoenix/database/repositories/users"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	carePlanTest "github.com/moto-nrw/project-phoenix/modules/careplan/careplantest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryProjection_ListSnapshots_RequiresEffectiveSuccessorBooking(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	phaseRepo := enrollmentCompose.New()
	requestRepo := enrollmentCompose.New()
	childRepo := enrollmentCompose.New()
	offeringRepo := carePlanTest.NewCareOfferingRepository(t, db)
	linkRepo := enrollmentCompose.New()

	source := makeOwnerEligibilityPhase(uniquePhaseName("expiry-source"))
	source.ServiceStartDate = capability.Date(timezone.NewDate(2026, 8, 1))
	source.ServiceEndDate = capability.Date(timezone.NewDate(2027, 1, 29))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.InsertPhase(ctx, source)
	}))

	sourceRequest := makeOwnerRequest(source.ID, uniqueToken("expiry-source"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.InsertRequest(ctx, sourceRequest)
	}))
	student := testpkg.CreateTestStudent(t, db, "Expiry", "Successor", "2a")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return usersRepo.NewStudentRepository(db).SetEnrollmentWindowByID(
			ctx, student.ID, timezone.Date(source.ServiceStartDate), usersModels.StudentStatusActive,
		)
	}))
	sourceChild := makeChild(sourceRequest.ID, "Expiry", "Successor")
	sourceChild.Status = enrollmentModels.ChildStatusApproved
	sourceChild.CreatedStudentID = &student.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.InsertChild(ctx, sourceChild)
	}))
	sourceOffering := makeOffering(source.ID, uniqueOfferingName("source-care"))
	sourceOffering.AvailableDays = []string{"mon"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, sourceOffering)
	}))
	sourceValidFrom := timezone.Date(source.ServiceStartDate)
	sourceValidUntil := timezone.Date(source.ServiceEndDate).AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
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
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.InsertPhase(ctx, successor)
	}))

	listSnapshots := func() []*capability.PhaseExpirySnapshot {
		t.Helper()
		var snapshots []*capability.PhaseExpirySnapshot
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
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
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.UpdatePhase(ctx, successor)
	}))

	targetRequest := makeOwnerRequest(successor.ID, uniqueToken("expiry-target"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.InsertRequest(ctx, targetRequest)
	}))
	targetChild := makeChild(targetRequest.ID, "Expiry", "Successor")
	targetChild.Status = enrollmentModels.ChildStatusPendingRenewal
	targetChild.CreatedStudentID = &student.ID
	targetChild.RolloverSourceChildID = &sourceChild.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.InsertChild(ctx, targetChild)
	}))
	unrelatedRequest := makeOwnerRequest(successor.ID, uniqueToken("expiry-unrelated"), "other@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.InsertRequest(ctx, unrelatedRequest)
	}))
	unrelatedRejectedChild := makeChild(unrelatedRequest.ID, "Expiry", "Successor")
	unrelatedRejectedChild.Status = enrollmentModels.ChildStatusRejected
	unrelatedRejectedChild.CreatedStudentID = &student.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.InsertChild(ctx, unrelatedRejectedChild)
	}))

	pendingSuccessor := listSnapshots()
	require.Len(t, pendingSuccessor, 1)
	assert.Equal(t, 1, pendingSuccessor[0].UnresolvedChildren,
		"an unrelated rejection for the same student must not override the open rollover candidate")

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusRejected, nil, 0)
	}))
	rejectedSuccessor := listSnapshots()
	require.Len(t, rejectedSuccessor, 1)
	assert.Equal(t, 0, rejectedSuccessor[0].UnresolvedChildren)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0)
	}))
	withdrawnSuccessor := listSnapshots()
	require.Len(t, withdrawnSuccessor, 1)
	assert.Equal(t, 0, withdrawnSuccessor[0].UnresolvedChildren)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	approvedWithoutOffering := listSnapshots()
	require.Len(t, approvedWithoutOffering, 1)
	assert.Equal(t, 1, approvedWithoutOffering[0].UnresolvedChildren)

	invalidTargetOffering := makeOffering(successor.ID, uniqueOfferingName("target-invalid"))
	invalidTargetOffering.AvailableDays = []string{"mon"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, invalidTargetOffering)
	}))
	targetValidFrom := timezone.Date(successor.ServiceStartDate)
	targetValidUntil := timezone.Date(successor.ServiceEndDate).AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return linkRepo.InsertRequestChildOffering(ctx, &capability.RequestChildOffering{
			RequestChildID: targetChild.ID,
			CareOfferingID: invalidTargetOffering.ID,
			SelectedDays:   []string{"not-a-day"},
			ValidFrom:      offeringDatePointer(targetValidFrom),
			ValidUntil:     offeringDatePointer(targetValidUntil),
		})
	}))
	invalidTargetBooking := listSnapshots()
	require.Len(t, invalidTargetBooking, 1)
	assert.Equal(t, 1, invalidTargetBooking[0].UnresolvedChildren,
		"a booking without a canonical weekday must not hide the warning")

	targetOffering := makeOffering(successor.ID, uniqueOfferingName("target-care"))
	targetOffering.AvailableDays = []string{"tue"}
	targetOffering.IsActive = false
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, targetOffering)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
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
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Update(ctx, targetOffering)
	}))

	mismatchedWeekdayBooking := listSnapshots()
	require.Len(t, mismatchedWeekdayBooking, 1)
	assert.Equal(t, 1, mismatchedWeekdayBooking[0].UnresolvedChildren,
		"a successor booking on another weekday must not hide the warning")

	targetOffering.AvailableDays = []string{"mon"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Update(ctx, targetOffering)
	}))
	completedSuccessor := listSnapshots()
	require.Len(t, completedSuccessor, 1)
	assert.Equal(t, 0, completedSuccessor[0].UnresolvedChildren)

	replacementStudent := testpkg.CreateTestStudent(t, db, "Replacement", "Successor", "2a")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		_, err := base.GetDB(ctx, db).NewUpdate().
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

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if updateErr := enrollmentCompose.New().UpdateChildStatus(
			ctx,
			targetChild.ID,
			enrollmentModels.ChildStatusPendingRenewal,
			nil,
			0,
		); updateErr != nil {
			return updateErr
		}
		if updateErr := enrollmentCompose.New().UpdateChildStatus(
			ctx,
			unrelatedRejectedChild.ID,
			enrollmentModels.ChildStatusRejected,
			nil,
			0,
		); updateErr != nil {
			return updateErr
		}
		if _, updateErr := base.GetDB(ctx, db).NewUpdate().
			TableExpr(`enrollment.phases AS "phase"`).
			Set("rollover_source_phase_id = NULL").
			Set("rollover_mode = NULL").
			Where(`"phase".id = ?`, successor.ID).
			Exec(ctx); updateErr != nil {
			return updateErr
		}
		_, updateErr := base.GetDB(ctx, db).NewUpdate().
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

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return enrollmentCompose.New().UpdateChildStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	temporalFallback = listSnapshots()
	require.Len(t, temporalFallback, 1)
	require.NotNil(t, temporalFallback[0].SuccessorPhaseID)
	assert.Equal(t, successor.ID, *temporalFallback[0].SuccessorPhaseID,
		"a contiguous school-year phase must be recognized without rollover lineage")
	assert.Equal(t, 0, temporalFallback[0].UnresolvedChildren,
		"an approved booking for the same student must complete a legacy successor without child lineage")
}
