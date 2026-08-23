package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/database/repositories/base"
	enrollmentRepo "github.com/moto-nrw/project-phoenix/database/repositories/enrollment"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

func TestPhaseExpiryRepository_ListSnapshots_RequiresEffectiveSuccessorBooking(t *testing.T) {
	t.Parallel()

	db := testpkg.SetupTestDB(t)
	tenantID := testpkg.Tenant(t)
	testpkg.EnsureTestTenant(t, db, tenantID)
	phaseRepo := enrollmentRepo.NewPhaseRepository(db)
	requestRepo := enrollmentRepo.NewRequestRepository(db)
	childRepo := enrollmentRepo.NewRequestChildRepository(db)
	offeringRepo := enrollmentRepo.NewCareOfferingRepository(db)
	linkRepo := enrollmentRepo.NewRequestChildOfferingRepository(db)

	source := makeValidPhase(uniquePhaseName("expiry-source"))
	source.ServiceStartDate = timezone.NewDate(2026, 8, 1)
	source.ServiceEndDate = timezone.NewDate(2027, 1, 29)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, source)
	}))

	sourceRequest := makeRequest(source.ID, uniqueToken("expiry-source"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.Create(ctx, sourceRequest)
	}))
	student := testpkg.CreateTestStudent(t, db, "Expiry", "Successor", "2a")
	sourceChild := makeChild(sourceRequest.ID, "Expiry", "Successor")
	sourceChild.Status = enrollmentModels.ChildStatusApproved
	sourceChild.CreatedStudentID = &student.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.Create(ctx, sourceChild)
	}))
	sourceOffering := makeOffering(source.ID, uniqueOfferingName("source-care"))
	sourceOffering.AvailableDays = []string{"mon"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, sourceOffering)
	}))
	sourceValidFrom := source.ServiceStartDate
	sourceValidUntil := source.ServiceEndDate.AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return linkRepo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: sourceChild.ID,
			CareOfferingID: sourceOffering.ID,
			ValidFrom:      &sourceValidFrom,
			ValidUntil:     &sourceValidUntil,
		})
	}))

	successor := makeValidPhase(uniquePhaseName("expiry-successor"))
	successor.ServiceStartDate = timezone.NewDate(2027, 2, 2)
	successor.ServiceEndDate = timezone.NewDate(2027, 7, 31)
	successor.RolloverSourcePhaseID = &source.ID
	rolloverMode := enrollmentModels.PhaseRolloverModeOptIn
	successor.RolloverMode = &rolloverMode
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Create(ctx, successor)
	}))

	listSnapshots := func() []*enrollmentModels.PhaseExpirySnapshot {
		t.Helper()
		var snapshots []*enrollmentModels.PhaseExpirySnapshot
		require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
			var listErr error
			snapshots, listErr = enrollmentRepo.NewPhaseExpiryRepository(db).ListSnapshots(
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

	successor.ServiceStartDate = timezone.NewDate(2027, 1, 30)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return phaseRepo.Update(ctx, successor)
	}))

	targetRequest := makeRequest(successor.ID, uniqueToken("expiry-target"), "expiry@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.Create(ctx, targetRequest)
	}))
	targetChild := makeChild(targetRequest.ID, "Expiry", "Successor")
	targetChild.Status = enrollmentModels.ChildStatusPendingRenewal
	targetChild.CreatedStudentID = &student.ID
	targetChild.RolloverSourceChildID = &sourceChild.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.Create(ctx, targetChild)
	}))
	unrelatedRequest := makeRequest(successor.ID, uniqueToken("expiry-unrelated"), "other@example.test")
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return requestRepo.Create(ctx, unrelatedRequest)
	}))
	unrelatedRejectedChild := makeChild(unrelatedRequest.ID, "Expiry", "Successor")
	unrelatedRejectedChild.Status = enrollmentModels.ChildStatusRejected
	unrelatedRejectedChild.CreatedStudentID = &student.ID
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.Create(ctx, unrelatedRejectedChild)
	}))

	pendingSuccessor := listSnapshots()
	require.Len(t, pendingSuccessor, 1)
	assert.Equal(t, 1, pendingSuccessor[0].UnresolvedChildren,
		"an unrelated rejection for the same student must not override the open rollover candidate")

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.UpdateStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusRejected, nil, 0)
	}))
	rejectedSuccessor := listSnapshots()
	require.Len(t, rejectedSuccessor, 1)
	assert.Equal(t, 0, rejectedSuccessor[0].UnresolvedChildren)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.UpdateStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusWithdrawn, nil, 0)
	}))
	withdrawnSuccessor := listSnapshots()
	require.Len(t, withdrawnSuccessor, 1)
	assert.Equal(t, 0, withdrawnSuccessor[0].UnresolvedChildren)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return childRepo.UpdateStatus(ctx, targetChild.ID, enrollmentModels.ChildStatusApproved, nil, 0)
	}))
	approvedWithoutOffering := listSnapshots()
	require.Len(t, approvedWithoutOffering, 1)
	assert.Equal(t, 1, approvedWithoutOffering[0].UnresolvedChildren)

	invalidTargetOffering := makeOffering(successor.ID, uniqueOfferingName("target-invalid"))
	invalidTargetOffering.AvailableDays = []string{"mon"}
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, invalidTargetOffering)
	}))
	targetValidFrom := successor.ServiceStartDate
	targetValidUntil := successor.ServiceEndDate.AddDays(1)
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return linkRepo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: targetChild.ID,
			CareOfferingID: invalidTargetOffering.ID,
			SelectedDays:   []string{"not-a-day"},
			ValidFrom:      &targetValidFrom,
			ValidUntil:     &targetValidUntil,
		})
	}))
	invalidTargetBooking := listSnapshots()
	require.Len(t, invalidTargetBooking, 1)
	assert.Equal(t, 1, invalidTargetBooking[0].UnresolvedChildren,
		"a booking without a canonical weekday must not hide the warning")

	targetOffering := makeOffering(successor.ID, uniqueOfferingName("target-care"))
	targetOffering.AvailableDays = []string{"mon"}
	targetOffering.IsActive = false
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return offeringRepo.Create(ctx, targetOffering)
	}))
	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		return linkRepo.Create(ctx, &enrollmentModels.RequestChildOffering{
			RequestChildID: targetChild.ID,
			CareOfferingID: targetOffering.ID,
			ValidFrom:      &targetValidFrom,
			ValidUntil:     &targetValidUntil,
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

	completedSuccessor := listSnapshots()
	require.Len(t, completedSuccessor, 1)
	assert.Equal(t, 0, completedSuccessor[0].UnresolvedChildren)

	require.NoError(t, runInTenantTx(t, db, tenantID, func(ctx context.Context) error {
		if updateErr := childRepo.UpdateStatus(
			ctx,
			unrelatedRejectedChild.ID,
			enrollmentModels.ChildStatusPendingRenewal,
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
			Set("rollover_source_child_id = NULL").
			Where(`"request_child".id = ?`, targetChild.ID).
			Exec(ctx)
		return updateErr
	}))

	temporalFallback := listSnapshots()
	require.Len(t, temporalFallback, 1)
	require.NotNil(t, temporalFallback[0].SuccessorPhaseID)
	assert.Equal(t, successor.ID, *temporalFallback[0].SuccessorPhaseID,
		"a contiguous school-year phase must be recognized without rollover lineage")
	assert.Equal(t, 0, temporalFallback[0].UnresolvedChildren,
		"an approved booking for the same student must complete a legacy successor without child lineage")
}
