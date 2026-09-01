package enrollment_test

import (
	"context"
	"log/slog"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/auth/jwt"
	"github.com/moto-nrw/project-phoenix/internal/timezone"
	activitiesModels "github.com/moto-nrw/project-phoenix/models/activities"
	modelBase "github.com/moto-nrw/project-phoenix/models/base"
	configModel "github.com/moto-nrw/project-phoenix/models/config"
	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	userModels "github.com/moto-nrw/project-phoenix/models/users"
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
		CareWithdrawalRepo:       env.repos.CareWithdrawal,
		ImpactRepo:               env.repos.OfferingChangeImpact,
		Applier:                  changeRequestApplierForTest(t, env),
		Settings:                 env.settings,
		Logger:                   slog.Default(),
		Today:                    func() timezone.Date { return decisionTestToday },
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
		pastSwitchAt: timezone.NewDate(2026, 8, 24).AddDays(-1),
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

func TestOfferingChangeRequestService_Decide_RefusesApprovalAfterPlannedCareEnd(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "CareEnd")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	_, err = env.db.NewUpdate().TableExpr("users.students").
		Set("enrolled_until = ?", fx.switchDate.AddDays(-1)).Where("id = ?", fx.studentID).Exec(ctx)
	require.NoError(t, err)

	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)
}

func TestOfferingChangeRequestService_Decide_CapsRebookingAtPlannedCareEnd(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "CareEndCap")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)
	_, err = env.db.NewUpdate().TableExpr("users.students").
		Set("enrolled_until = ?", fx.switchDate).Where("id = ?", fx.studentID).Exec(ctx)
	require.NoError(t, err)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))
	exclusiveEnd := fx.switchDate.AddDays(1)
	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, fx.childID, fx.switchDate)
	require.NoError(t, err)
	require.NotEmpty(t, links)
	for _, link := range links {
		require.NotNil(t, link.ValidUntil)
		assert.False(t, link.ValidUntil.After(exclusiveEnd), "source booking must not outlive the child care interval")
	}
	for _, enrollment := range listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID) {
		if enrollment.ActivityGroupID != fx.newGroupID {
			continue
		}
		require.NotNil(t, enrollment.ValidUntil)
		assert.False(t, enrollment.ValidUntil.After(exclusiveEnd), "derived booking must not be recreated past care end")
	}
}

func TestOfferingChangeRequestService_Decide_RebookingObsoletesWithdrawalWithoutGap(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RebookWithdrawal")
	studentID := fx.studentID
	actorID := env.creatorID
	repo := env.repos.CareWithdrawal
	require.NoError(t, repo.UpsertPending(ctx, &userModels.CareWithdrawalCompletion{
		StudentID: &studentID, FirstBookinglessDay: fx.switchDate,
		Trigger:               userModels.CareWithdrawalTriggerDirectSchool,
		WithdrawalConfirmedBy: &actorID, WithdrawalConfirmedRole: "admin",
		WithdrawalConfirmedAt: time.Now(),
	}))
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}}},
	})
	require.NoError(t, err)

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))
	pending, err := pendingWithdrawalForStudent(ctx, repo, fx.studentID)
	require.NoError(t, err)
	assert.Nil(t, pending, "approved parent rebooking must remove the stale complete-withdrawal task")
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
	phaseStart := timezone.NewDate(2026, 8, 24).AddDays(30)
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
	assert.Equal(t, fx.pastSwitchAt, queued.RequestedEffectiveFrom,
		"the queue must keep the date the family asked for, so the card can name it")
}

// Ein Datum, das die OGS selbst gewählt hat, überschreibt das der Eltern in der
// Anfragezeile. Ohne die Zeile im Änderungsprotokoll bliebe von deren Wunsch
// nichts übrig (#2484).
func TestOfferingChangeRequestService_Decide_LogsTheDateTheFamilyAskedFor(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "LogWish")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := fx.switchDate.AddDays(7)
	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	}))

	adjustments, err := env.decision.ListOfferingAdjustments(ctx, fx.requestID, fx.childID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.Contains(t, adjustments[0].Reason,
		"gültig ab "+confirmed.Format("02.01.2006"))
	assert.Contains(t, adjustments[0].Reason,
		"Wunschdatum der Eltern war "+fx.switchDate.Format("02.01.2006"))
}

// Bleibt es beim Wunsch der Eltern, hat das Protokoll nichts zu ergänzen.
func TestOfferingChangeRequestService_Decide_LogsNoWishWhenTheDateIsKept(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "LogNoWish")

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
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &fx.switchDate,
	}))

	adjustments, err := env.decision.ListOfferingAdjustments(ctx, fx.requestID, fx.childID)
	require.NoError(t, err)
	require.Len(t, adjustments, 1)
	assert.NotContains(t, adjustments[0].Reason, "Wunschdatum der Eltern")
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
	assert.False(t, catalog.EarliestEffectiveFrom.Before(timezone.NewDate(2026, 8, 24)),
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

func TestOfferingChangeRequestService_Decide_AppliesTheConfirmedDate(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ConfirmDate")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := fx.switchDate.AddDays(7)
	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	}))

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusApproved, decided.Status)
	assert.Equal(t, confirmed, decided.EffectiveFrom,
		"the request must record the date the office confirmed, not the parents' wish")

	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	byGroup := map[int64]activitiesModels.StudentEnrollment{}
	for _, enrollment := range rows {
		byGroup[enrollment.ActivityGroupID] = enrollment
	}
	oldRow, ok := byGroup[fx.oldGroupID]
	require.True(t, ok, "the attended group must survive as history")
	require.NotNil(t, oldRow.ValidUntil)
	assert.Equal(t, confirmed, *oldRow.ValidUntil)
	newRow, ok := byGroup[fx.newGroupID]
	require.True(t, ok, "the requested group must be materialized")
	assert.Equal(t, confirmed, newRow.ValidFrom)
}

// A confirmed date before today would rewrite weeks that already happened.
// Refused outright: a silent shift to today would hand the office a different
// switch than the one it confirmed.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateBeforeToday(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PastDate")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := timezone.NewDate(2026, 8, 24).AddDays(-1)
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeDateOutOfRange)

	pending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, pending.Status,
		"a refused date leaves the request open")
}

// The switch cannot start after the care period it belongs to has ended.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateAfterTheCarePeriod(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "AfterPeriod")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := env.sourcePhase.ServiceEndDate.AddDays(1)
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeDateOutOfRange)

	pending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, pending.Status)
}

// Nothing takes effect on a rejection, so a date on one is a caller mistake.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateOnARejection(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RejectDate")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := fx.switchDate
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       false,
		Reason:        "Kein Platz",
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)
}

// The card bounds its date field from the queue, so staff cannot pick a date
// the approval would refuse.
func TestOfferingChangeRequestService_ListPending_ReportsTheSelectableDateRange(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "DateRange")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	pending, _, err := svc.ListPending(ctx, modelBase.RequestQueueFilters{})
	require.NoError(t, err)
	var queued *enrollmentService.OfferingChangeView
	for _, view := range pending {
		if view.Request != nil && view.Request.ID == row.ID {
			queued = view
		}
	}
	require.NotNil(t, queued)
	earliest := env.sourcePhase.ServiceStartDate
	if today := decisionTestToday; earliest.Before(today) {
		earliest = today
	}
	assert.Equal(t, earliest, queued.EarliestEffectiveFrom,
		"the office may switch from today, but never before the care period starts")
	assert.Equal(t, env.sourcePhase.ServiceEndDate, queued.LatestEffectiveFrom)
}

// The preview is what the office decides from. A date the approval would refuse
// must fail here too, or the card offers a switch that cannot happen.
func TestOfferingChangeRequestService_PreviewDecision_RefusesADateOutOfRange(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewRange")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	confirmed := timezone.NewDate(2026, 8, 24).AddDays(-1)
	_, err = svc.PreviewDecision(ctx, row.ID, nil, &confirmed)
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeDateOutOfRange)
}

type planningOccurrenceOpts struct {
	status       string
	spontaneous  bool
	materialized bool
	unplanned    bool
	notScheduled bool
}

func createPlanningOccurrence(
	t *testing.T,
	env *decisionTestEnv,
	groupID, studentID, roomID, periodID int64,
	date timezone.Date,
	title string,
	opts planningOccurrenceOpts,
) {
	t.Helper()
	status := opts.status
	if status == "" {
		status = scheduleModels.InstanceStatusPlanned
	}
	var calendarPeriodID *int64
	if opts.materialized {
		calendarPeriodID = &periodID
	}
	instance := testpkg.CreateTestActivityInstance(t, env.db, date, roomID, testpkg.ActivityInstanceOpts{
		ActivityGroupID:  &groupID,
		CalendarPeriodID: calendarPeriodID,
		Title:            title,
		Status:           status,
		IsSpontaneous:    opts.spontaneous,
	})
	attendance := testpkg.CreateTestInstanceStudent(t, env.db, instance.ID, studentID, "", testpkg.InstanceStudentOpts{
		NotScheduled: opts.notScheduled,
	})
	if opts.unplanned {
		attendance.IsUnplanned = true
		require.NoError(t, env.repos.InstanceStudent.Update(testpkg.Ctx(t), attendance))
	}
}

func TestOfferingChangeRequestService_PreviewDecision_ReportsOnlyUncoveredManualPlanning(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = true
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewManualPlanning")
	fx.newOffering.DaysOfWeekMode = enrollmentModels.DaysOfWeekModeFixed
	fx.newOffering.AvailableDays = []string{"mon"}
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.newOffering))
	fx.oldOffering.DaysOfWeekMode = enrollmentModels.DaysOfWeekModeFixed
	fx.oldOffering.AvailableDays = []string{"mon", "wed", "thu"}
	fx.oldOffering.CountsAsCare = true
	fx.oldOffering.CountsAsCareSet = true
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.oldOffering))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID},
		},
	})
	require.NoError(t, err)

	manualGroup := testpkg.CreateTestActivityGroup(t, env.db, "ManualPlanningPreview")
	manualGroup.Type = activitiesModels.GroupTypeCare
	manualGroup.IsTemplate = true
	manualGroup.TargetGroupType = activitiesModels.TargetGroupTypeAngebot
	require.NoError(t, env.repos.ActivityGroup.Update(ctx, manualGroup))

	sourcedGroup := testpkg.CreateTestActivityGroup(t, env.db, "OfferingPlanningPreview")
	sourcedGroup.Type = activitiesModels.GroupTypeCare
	sourcedGroup.IsTemplate = true
	sourcedGroup.TargetGroupType = activitiesModels.TargetGroupTypeAngebot
	sourcedGroup.SourceCareOfferingIDs = []int64{fx.newOffering.ID}
	require.NoError(t, env.repos.ActivityGroup.Update(ctx, sourcedGroup))

	legacySourcedGroup, err := env.repos.ActivityGroup.FindByID(ctx, fx.oldGroupID)
	require.NoError(t, err)
	legacySourcedGroup.Type = activitiesModels.GroupTypeCare
	legacySourcedGroup.IsTemplate = true
	legacySourcedGroup.TargetGroupType = activitiesModels.TargetGroupTypeNone
	require.NoError(t, env.repos.ActivityGroup.Update(ctx, legacySourcedGroup))
	parentChoiceGroup := testpkg.CreateTestActivityGroup(t, env.db, "EmptyParentChoicePlanning")
	parentChoiceGroup.Type = activitiesModels.GroupTypeCare
	parentChoiceGroup.IsTemplate = true
	require.NoError(t, env.repos.ActivityGroup.Update(ctx, parentChoiceGroup))
	parentChoiceOffering := createAdjustmentCareOfferingWith(t, env, "Leere Tagesauswahl", func(o *enrollmentModels.CareOffering) {
		o.ActivityGroupID = &parentChoiceGroup.ID
		o.CountsAsCare, o.CountsAsCareSet = true, true
	})
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID: fx.childID,
		CareOfferingID: parentChoiceOffering.ID,
		SelectedDays:   []string{},
	}))

	period := testpkg.CreateTestCalendarPeriod(
		t, env.db, "Manual planning preview "+t.Name(), env.sourcePhase.ServiceStartDate, env.sourcePhase.ServiceEndDate,
	)
	room := testpkg.CreateTestRoom(t, env.db, "Manual planning preview")
	dateFor := func(weekday time.Weekday) timezone.Date {
		date := fx.switchDate
		for date.Weekday() != weekday {
			date = date.AddDays(1)
		}
		return date
	}
	_, err = env.db.NewRaw(`
		UPDATE enrollment.request_child_offerings
		SET valid_until = ?
		WHERE tenant_id = ? AND request_child_id = ? AND care_offering_id = ?
	`, dateFor(time.Thursday), testpkg.Tenant(t), fx.childID, fx.oldOffering.ID).Exec(ctx)
	require.NoError(t, err)
	materialized := planningOccurrenceOpts{materialized: true}
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Monday), "Montags gedeckt", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Dienstags ohne Buchung", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday).AddDays(7), "Dienstags erneut", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Wednesday), "Mittwochs ohne Buchung", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		fx.switchDate.AddDays(-1), "Vor dem Wechsel", materialized)
	createPlanningOccurrence(t, env, sourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Wird automatisch abgeglichen", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Monday), "Von alter Buchung gedeckt", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Alte Angebots-Verknüpfung", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Wednesday), "Abgelaufene Angebots-Verknüpfung", materialized)
	createPlanningOccurrence(t, env, legacySourcedGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday), "Nicht zählendes Angebot", materialized)
	createPlanningOccurrence(t, env, parentChoiceGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Tuesday), "Leere Tagesauswahl", materialized)
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Friday), "Nicht gebucht", planningOccurrenceOpts{materialized: true, notScheduled: true})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday), "Spontan", planningOccurrenceOpts{materialized: true, spontaneous: true})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday).AddDays(7), "Nicht materialisiert", planningOccurrenceOpts{})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday).AddDays(14), "Abgesagt", planningOccurrenceOpts{
			materialized: true,
			status:       scheduleModels.InstanceStatusCancelled,
		})
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		dateFor(time.Thursday).AddDays(21), "Ungeplant", planningOccurrenceOpts{materialized: true, unplanned: true})

	preview, err := svc.PreviewDecision(ctx, row.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, preview.ManualPlanningConflicts, 3)
	conflictsByGroupID := make(map[int64]enrollmentService.ManualPlanningConflict, len(preview.ManualPlanningConflicts))
	for _, item := range preview.ManualPlanningConflicts {
		conflictsByGroupID[item.ActivityGroupID] = item
	}
	conflict := conflictsByGroupID[manualGroup.ID]
	assert.Equal(t, manualGroup.ID, conflict.ActivityGroupID)
	assert.Equal(t, manualGroup.Name, conflict.ActivityGroupName)
	assert.Equal(t, []string{"tue", "wed"}, conflict.Days)
	assert.Equal(t, dateFor(time.Tuesday), conflict.FirstDate)
	assert.Equal(t, 3, conflict.OccurrenceCount)
	legacyConflict := conflictsByGroupID[legacySourcedGroup.ID]
	assert.Equal(t, legacySourcedGroup.ID, legacyConflict.ActivityGroupID)
	assert.Equal(t, []string{"tue", "wed", "thu"}, legacyConflict.Days)
	assert.Equal(t, dateFor(time.Tuesday), legacyConflict.FirstDate)
	assert.Equal(t, 3, legacyConflict.OccurrenceCount)
	parentChoiceConflict := conflictsByGroupID[parentChoiceGroup.ID]
	assert.Equal(t, parentChoiceGroup.ID, parentChoiceConflict.ActivityGroupID)
	assert.Equal(t, []string{"tue"}, parentChoiceConflict.Days)
	assert.Equal(t, dateFor(time.Tuesday), parentChoiceConflict.FirstDate)
	assert.Equal(t, 1, parentChoiceConflict.OccurrenceCount)
	assert.True(t, preview.ArrivalExpectationsFollowBookings)
	laterEffectiveFrom := dateFor(time.Thursday).AddDays(22)
	laterPreview, err := svc.PreviewDecision(ctx, row.ID, nil, &laterEffectiveFrom)
	require.NoError(t, err)
	assert.Empty(t, laterPreview.ManualPlanningConflicts,
		"the chosen effective date must bound the planning consequences")
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = false
	preview, err = svc.PreviewDecision(ctx, row.ID, nil, nil)
	require.NoError(t, err)
	assert.False(t, preview.ArrivalExpectationsFollowBookings)

	otherTenantCtx := testpkg.TenantContext(testpkg.Tenant(t) + 1)
	foreignRows, err := env.repos.OfferingChangeImpact.ListManualPlanningOccurrences(
		otherTenantCtx, fx.studentID, fx.switchDate, env.sourcePhase.ServiceEndDate,
	)
	require.NoError(t, err)
	assert.Empty(t, foreignRows, "the projection must not leak planning rows across tenants")
}

func TestOfferingChangeRequestService_PreviewDecision_ReportsManualPlanningForFullWithdrawal(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	env.settings.boolValues[configModel.KeyEnrollmentBookingsAuthoritative] = false
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewFullWithdrawal")
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
	})
	require.NoError(t, err)

	manualGroup := testpkg.CreateTestActivityGroup(t, env.db, "ManualFullWithdrawal")
	manualGroup.Type, manualGroup.IsTemplate = activitiesModels.GroupTypeCare, true
	require.NoError(t, env.repos.ActivityGroup.Update(ctx, manualGroup))
	period := testpkg.CreateTestCalendarPeriod(
		t, env.db, "Manual full withdrawal "+t.Name(), env.sourcePhase.ServiceStartDate, env.sourcePhase.ServiceEndDate,
	)
	room := testpkg.CreateTestRoom(t, env.db, "Manual full withdrawal")
	createPlanningOccurrence(t, env, manualGroup.ID, fx.studentID, room.ID, period.ID,
		fx.switchDate, "Bleibt manuell geplant", planningOccurrenceOpts{materialized: true})

	preview, err := svc.PreviewDecision(ctx, row.ID, nil, nil)
	require.NoError(t, err)
	require.Len(t, preview.ManualPlanningConflicts, 1)
	assert.Equal(t, manualGroup.ID, preview.ManualPlanningConflicts[0].ActivityGroupID)
	assert.Equal(t, 1, preview.ManualPlanningConflicts[0].OccurrenceCount)
}

// The confirmed date is checked against the same capacity the parents' date
// would be: the office must not be able to move a switch onto a day the
// offering has no room on.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateWithoutCapacity(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "ConfirmedFull")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	zeroCapacity := 0
	fx.newOffering.Capacity = &zeroCapacity
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.newOffering))

	confirmed := fx.switchDate.AddDays(7)
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeCapacityFull)

	stillPending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, stillPending.Status)
	rows := listStudentEnrollmentRowsForDecisionTest(t, env, fx.studentID)
	require.Len(t, rows, 1, "a refused approval must not change the booking")
}

// A care period that starts in the future is the lower bound, not today: a
// switch cannot begin before the period it belongs to.
func TestOfferingChangeRequestService_Decide_RefusesAConfirmedDateBeforeTheCarePeriod(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "BeforePeriod")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)

	// The period start moves into the future while the request waits.
	phaseStart := timezone.NewDate(2026, 8, 24).AddDays(30)
	env.sourcePhase.ServiceStartDate = phaseStart
	require.NoError(t, env.repos.Phase.Update(ctx, env.sourcePhase))

	// Tomorrow is after today but still before the period begins.
	confirmed := timezone.NewDate(2026, 8, 24).AddDays(1)
	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:     row.ID,
		Approve:       true,
		ReviewedBy:    env.creatorID,
		EffectiveFrom: &confirmed,
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeDateOutOfRange)

	stillPending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, stillPending.Status)
}
