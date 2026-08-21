package enrollment_test

import (
	"context"
	"log/slog"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// Post-enrollment offering change requests (#1665). The service is exercised
// against the real test DB; the emitter is left nil (chat pills are covered by
// the messaging tests and are best-effort by contract).

// offeringChangeAdminContext makes the staff-side service calls explicit about
// their authority. TenantContext alone deliberately carries no JWT permissions.
func offeringChangeAdminContext(t *testing.T) context.Context {
	return context.WithValue(testpkg.Ctx(t), jwt.CtxPermissions, []string{"admin:*"})
}

func newOfferingChangeServiceForTest(
	t *testing.T,
	env *decisionTestEnv,
) enrollmentService.OfferingChangeRequestService {
	return newOfferingChangeServiceForTestWithCareRepo(t, env, env.repos.CareOffering)
}

func newOfferingChangeServiceForTestWithCareRepo(
	t *testing.T,
	env *decisionTestEnv,
	careOfferingRepo enrollmentModels.CareOfferingRepository,
) enrollmentService.OfferingChangeRequestService {
	t.Helper()
	env.settings.boolValues[configModel.KeyEnrollmentOfferingChangesEnabled] = true
	env.settings.stringValues[configModel.KeyEnrollmentOfferingChangesLeadDays] = "14"
	return enrollmentService.NewOfferingChangeRequestService(enrollmentService.OfferingChangeRequestServiceConfig{
		ChangeRepo:               env.repos.OfferingChangeRequest,
		RequestChildRepo:         env.repos.RequestChild,
		RequestRepo:              env.repos.Request,
		PhaseRepo:                env.repos.Phase,
		CareOfferingRepo:         careOfferingRepo,
		RequestChildOfferingRepo: env.repos.RequestChildOffering,
		StudentRepo:              env.repos.Student,
		PersonRepo:               env.repos.Person,
		Applier:                  changeRequestApplierForTest(t, env),
		Settings:                 env.settings,
		Logger:                   slog.Default(),
	})
}

// offeringChangeFixture sets up an approved child booked into oldOffering, plus
// a second offering it could switch to.
type offeringChangeFixture struct {
	requestID    int64
	childID      int64
	studentID    int64
	oldOffering  *enrollmentModels.CareOffering
	newOffering  *enrollmentModels.CareOffering
	oldGroupID   int64
	newGroupID   int64
	switchDate   timezone.Date
	pastSwitchAt timezone.Date
}

func setupOfferingChangeFixture(
	t *testing.T,
	env *decisionTestEnv,
	label string,
) *offeringChangeFixture {
	t.Helper()
	oldGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferChangeOld"+label)
	newGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferChangeNew"+label)
	oldOffering := createAdjustmentCareOfferingWith(t, env, "Bisher "+label, func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &oldGroup.ID
		o.SortOrder = 201
	})
	newOffering := createAdjustmentCareOfferingWith(t, env, "Neu "+label, func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &newGroup.ID
		o.SortOrder = 202
	})
	requestID, childID, studentID := submitApprovedAdjustmentChild(
		t, env, "offer-change-"+label+"@example.com", "OfferChange"+label,
		[]*enrollmentModels.CareOffering{oldOffering},
	)
	return &offeringChangeFixture{
		requestID:    requestID,
		childID:      childID,
		studentID:    studentID,
		oldOffering:  oldOffering,
		newOffering:  newOffering,
		oldGroupID:   oldGroup.ID,
		newGroupID:   newGroup.ID,
		switchDate:   env.sourcePhase.ServiceStartDate.AddDays(150),
		pastSwitchAt: timezone.TodayDate().AddDays(-1),
	}
}

func TestOfferingChangeRequestService_Create_StoresPendingRequest(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Create")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Note:          "Neuer Arbeitsbeginn",
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, row.Status)
	assert.Equal(t, fx.childID, row.RequestChildID)
	assert.Equal(t, fx.switchDate, row.EffectiveFrom)
	require.NotNil(t, row.ParentNote)
	assert.Equal(t, "Neuer Arbeitsbeginn", *row.ParentNote)

	// The read view carries the request plus a diff naming both sides.
	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view)
	labels := make([]string, 0, len(view.Diff))
	for _, entry := range view.Diff {
		labels = append(labels, entry.Label)
	}
	assert.Contains(t, labels, fx.oldOffering.Name, "dropped offering must appear in the diff")
	assert.Contains(t, labels, fx.newOffering.Name, "added offering must appear in the diff")
}

func TestOfferingChangeRequestService_Create_PayloadExcludesAutomaticOfferings(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PayloadAuto")
	automatic := createAdjustmentCareOfferingWith(t, env, "Automatisch PayloadAuto", func(o *enrollmentModels.CareOffering) {
		o.AutoAddTriggerOfferingIDs = []int64{fx.newOffering.ID}
		o.SortOrder = 203
	})

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)

	offerings, ok := row.Payload["offerings"].([]any)
	require.True(t, ok)
	require.Len(t, offerings, 1, "the payload must record only the guardian's explicit choice")

	automatic.AutoAddTriggerOfferingIDs = nil
	require.NoError(t, env.repos.CareOffering.Update(ctx, automatic))
	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, ReviewedBy: env.creatorID, Approve: true,
	}))
	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, fx.childID, fx.switchDate)
	require.NoError(t, err)
	for _, link := range links {
		assert.NotEqual(t, automatic.ID, link.CareOfferingID,
			"an offering whose automatic trigger was removed must not become a manual booking")
	}
}

func TestOfferingChangeRequestService_Create_StripsChangedCurrentAutomaticOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ChangedCurrentAuto")
	automatic := createAdjustmentCareOfferingWith(t, env, "Automatisch ChangedCurrentAuto", func(o *enrollmentModels.CareOffering) {
		o.SortOrder = 203
	})

	// This row represents an offering materialized from another selection. A
	// crafted request must not be able to change its days and persist it as a
	// manual booking.
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID:        fx.childID,
		CareOfferingID:        automatic.ID,
		SelectedDays:          []string{"mon"},
		AutomaticSelectedDays: []string{"mon"},
	}))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: automatic.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.NoError(t, err)

	offerings, ok := row.Payload["offerings"].([]any)
	require.True(t, ok)
	require.Len(t, offerings, 1, "automatic offerings must never enter the manual request payload")
	entry, ok := offerings[0].(map[string]any)
	require.True(t, ok)
	assert.EqualValues(t, fx.newOffering.ID, entry["offering_id"])
}

func TestOfferingChangeRequestService_Create_RejectsSecondPendingRequest(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Second")

	_, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	_, err = svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.ErrorIs(t, err, enrollmentModels.ErrOfferingChangeAlreadyPending)
}

func TestOfferingChangeRequestService_Create_UsesSelectionAtEffectiveDate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "EffectiveSelection")

	first, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.NoError(t, err)
	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: first.ID, Approve: true, ReviewedBy: env.creatorID,
	}))

	// The first approved switch is already the booking at this later date.
	// Repeating it must be rejected instead of being compared with today's old
	// selection and creating a no-op pending request.
	_, err = svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate.AddDays(10),
		Selections: []enrollmentService.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"},
		}},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Create_RejectsEffectiveDateInsideNoticePeriod(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Notice")

	_, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.pastSwitchAt,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Create_RejectsOfferingOutsideThePhase(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Foreign")

	_, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID + 100_000, SelectedDays: []string{"mon"}},
		},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Create_RefusedWhenSchoolDisabledIt(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Off")
	env.settings.boolValues[configModel.KeyEnrollmentOfferingChangesEnabled] = false

	_, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeDisabled)
}

func TestOfferingChangeRequestService_Decide_ApprovalAppliesTheDatedSwitch(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Approve")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	}))

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusApproved, decided.Status)
	require.NotNil(t, decided.AppliedAt)

	// The old group is capped at the switch date instead of deleted, the new one
	// starts there — the whole point of the dated path.
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	byGroup := map[int64]activitiesModels.StudentEnrollment{}
	for _, enrollment := range rows {
		byGroup[enrollment.ActivityGroupID] = enrollment
	}
	oldRow, ok := byGroup[fx.oldGroupID]
	require.True(t, ok, "the attended group must survive as history")
	require.NotNil(t, oldRow.ValidUntil)
	assert.Equal(t, fx.switchDate, *oldRow.ValidUntil)
	newRow, ok := byGroup[fx.newGroupID]
	require.True(t, ok, "the requested group must be materialized")
	assert.Equal(t, fx.switchDate, newRow.ValidFrom)

	// The queue is empty afterwards.
	pending, _, err := svc.ListPending(ctx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	assert.Empty(t, pending)
}

// A pending request outlives edits to its care period. When the period's
// service start moves past the requested date, an approval applies at the new
// period start — the queue has to name that date instead of today, or the
// office approves a switch believing it takes effect immediately.
func TestOfferingChangeRequestService_ListPending_ReportsDateClampedToThePhaseStart(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "QueueClamp")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// The period start moves behind the requested date while the request waits.
	phaseStart := timezone.TodayDate().AddDays(30)
	env.sourcePhase.ServiceStartDate = phaseStart
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))
	require.NoError(t, env.repos.OfferingChangeRequest.UpdateEffectiveFrom(ctx, row.ID, fx.pastSwitchAt))

	pending, _, err := svc.ListPending(ctx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	var queued *enrollmentService.OfferingChangeView
	for _, view := range pending {
		if view.Request != nil && view.Request.ID == row.ID {
			queued = view
		}
	}
	require.NotNil(t, queued, "the pending request must stay in the queue")
	assert.Equal(t, phaseStart, queued.Request.EffectiveFrom,
		"the queue must show the date an approval would actually apply")
}

func TestOfferingChangeRequestService_Decide_ApprovalRejectsWhenCareOfferingsAreDisabled(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "DisabledApproval")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	env.settings.boolValues[configModel.KeyEnrollmentCareOfferingsEnabled] = false
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, enrollmentService.ErrCareOfferingsDisabled)

	pending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, pending.Status)
}

func TestOfferingChangeRequestService_Decide_RejectionNeedsAReasonAndChangesNothing(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Reject")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    false,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    false,
		Reason:     "Kein Platz in der Gruppe",
		ReviewedBy: env.creatorID,
	}))

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusRejected, decided.Status)
	assert.Nil(t, decided.AppliedAt, "a rejection must not stamp applied_at")

	// The booking is untouched: still exactly the original group.
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	require.Len(t, rows, 1)
	assert.Equal(t, fx.oldGroupID, rows[0].ActivityGroupID)
}

func TestOfferingChangeRequestService_Decide_RefusesApprovalWhenOfferingIsFull(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Full")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// The offering fills up between submission and decision.
	zeroCapacity := 0
	fx.newOffering.Capacity = &zeroCapacity
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.newOffering))

	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeCapacityFull)

	// Still pending, so the office can talk to the family instead of finding a
	// request marked done that never applied.
	stillPending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, stillPending.Status)
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	require.Len(t, rows, 1, "a refused approval must not change the booking")
}

func TestOfferingChangeRequestService_Decide_AllowsLeavingOverCapacityOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "LeaveFull")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// Capacity can be reduced after enrollment. That must never trap an
	// already-booked child in the now-over-capacity offering.
	zeroCapacity := 0
	fx.oldOffering.Capacity = &zeroCapacity
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.oldOffering))

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	}))

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusApproved, decided.Status)
}

func TestOfferingChangeRequestService_Decide_AllowsRetainingOverCapacityOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "KeepFull")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	zeroCapacity := 0
	fx.oldOffering.Capacity = &zeroCapacity
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.oldOffering))

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusApproved, decided.Status)
}

func TestOfferingChangeRequestService_Withdraw_OnlyBySubmitter(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Withdraw")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	_, otherAccount := testpkg.CreateTestPersonWithAccount(t, env.db, "Andere", "Bezugsperson")
	err = svc.Withdraw(ctx, row.ID, otherAccount.ID, fx.studentID)
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeForbidden)

	require.NoError(t, svc.Withdraw(ctx, row.ID, env.creatorID, fx.studentID))
	withdrawn, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusWithdrawn, withdrawn.Status)

	// A withdrawn request can no longer be decided.
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    true,
		ReviewedBy: env.creatorID,
	})
	require.ErrorIs(t, err, enrollmentModels.ErrOfferingChangeNotPending)
}

func TestOfferingChangeRequestService_Catalog_MarksCurrentBookingAndCapacity(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Catalog")

	capacity := 3
	fx.newOffering.Capacity = &capacity
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.newOffering))

	catalog, err := svc.Catalog(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Equal(t, env.sourcePhase.ServiceEndDate, catalog.LatestEffectiveFrom)
	assert.False(t, catalog.EarliestEffectiveFrom.Before(timezone.TodayDate()),
		"a switch must not be offered for a date already gone")

	byID := map[int64]enrollmentService.OfferingChangeCatalogItem{}
	for _, item := range catalog.Items {
		byID[item.OfferingID] = item
	}
	current, ok := byID[fx.oldOffering.ID]
	require.True(t, ok)
	assert.True(t, current.Selected, "the current booking must open prefilled")
	assert.Equal(t, []string{"mon"}, current.SelectedDays)

	other, ok := byID[fx.newOffering.ID]
	require.True(t, ok)
	assert.False(t, other.Selected)
	require.NotNil(t, other.FreeSlots)
	assert.Equal(t, 3, *other.FreeSlots)
}

func TestOfferingChangeRequestService_GetForStudent_ReportsRecentDecision(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Decision")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    false,
		Reason:     "Gruppe ist voll",
		ReviewedBy: env.creatorID,
	}))

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view, "a decided request must still be reported")
	assert.Nil(t, view.Request, "no request is open anymore")
	require.NotNil(t, view.LastDecision)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusRejected, view.LastDecision.Status)
	assert.Equal(t, "Gruppe ist voll", view.LastDecision.Reason)
	assert.Equal(t, fx.switchDate, view.LastDecision.EffectiveFrom)
	// The request itself is read back from the payload: a rejection is only
	// understandable next to what was asked for.
	require.Len(t, view.LastDecision.Requested, 1)
	assert.Equal(t, fx.newOffering.Name, view.LastDecision.Requested[0].Name)
	assert.Equal(t, []string{"mon"}, view.LastDecision.Requested[0].Days)

	// Aged past the recency window it drops out, so the card does not turn into
	// a history list.
	_, err = env.db.NewRaw(`
		UPDATE enrollment.offering_change_requests
		SET reviewed_at = NOW() - INTERVAL '20 days'
		WHERE id = ?
	`, row.ID).Exec(ctx)
	require.NoError(t, err)

	view, err = svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, view, "an old decision is no longer reported")
}

func TestOfferingChangeRequestService_GetForStudent_IgnoresOwnWithdrawal(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "Silent")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Withdraw(ctx, row.ID, env.creatorID, fx.studentID))

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, view, "a guardian's own withdrawal needs no status message")
}
