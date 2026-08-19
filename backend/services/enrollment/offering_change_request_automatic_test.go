package enrollment_test

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

type ruleMutationOnLockCareOfferingRepo struct {
	enrollmentModels.CareOfferingRepository
	mutate  func(context.Context) error
	mutated bool
	locked  []int64
}

func (r *ruleMutationOnLockCareOfferingRepo) ListByIDsForUpdate(
	ctx context.Context,
	ids []int64,
) ([]*enrollmentModels.CareOffering, error) {
	r.locked = append([]int64(nil), ids...)
	if !r.mutated {
		r.mutated = true
		if err := r.mutate(ctx); err != nil {
			return nil, err
		}
	}
	return r.CareOfferingRepository.ListByIDsForUpdate(ctx, ids)
}

// Marking and per-request override of Mitbuchungs-Regeln in the change-request
// review (#2365, #2370). Setup shared by the tests below: the child may pick
// fx.newOffering, and a target offering is configured to be auto-added
// whenever fx.newOffering is selected.

func createAutoAddTarget(
	t *testing.T,
	env *decisionTestEnv,
	label string,
	triggerID int64,
) *enrollmentModels.CareOffering {
	t.Helper()
	offering := createAdjustmentCareOfferingWith(t, env, "Automatisch "+label, func(o *enrollmentModels.CareOffering) {
		o.SortOrder = 203
	})
	// Create writes only the offering row; the rule lives in
	// enrollment.care_offering_auto_triggers.
	require.NoError(t, env.repos.CareOffering.ReplaceAutoAddTriggers(
		testpkg.Ctx(t), offering.ID, []int64{triggerID},
	))
	offering.AutoAddTriggerOfferingIDs = []int64{triggerID}
	return offering
}

func createPendingTriggerRequest(
	t *testing.T,
	env *decisionTestEnv,
	svc enrollmentService.OfferingChangeRequestService,
	fx *offeringChangeFixture,
	days []string,
) *enrollmentModels.OfferingChangeRequest {
	t.Helper()
	ctx := offeringChangeAdminContext(t)
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{{
			OfferingID: fx.newOffering.ID, SelectedDays: days,
		}},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })
	return row
}

func TestOfferingChangeRequestService_GetForStudent_MarksAutomaticDiffEntries(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "MarkAuto")
	auto := createAutoAddTarget(t, env, "MarkAuto", fx.newOffering.ID)
	createPendingTriggerRequest(t, env, svc, fx, []string{"mon", "tue"})

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view)

	byLabel := make(map[string]enrollmentService.OfferingChangeDiffEntry, len(view.Diff))
	for _, entry := range view.Diff {
		byLabel[entry.Label] = entry
	}
	autoEntry, ok := byLabel[auto.Name]
	require.True(t, ok, "the rule-added offering must appear in the diff")
	assert.Equal(t, auto.ID, autoEntry.OfferingID)
	assert.Equal(t, []string{"mon", "tue"}, autoEntry.NewAutomaticDays)
	assert.Equal(t, []int64{fx.newOffering.ID}, autoEntry.AutoTriggerIDs)
	assert.Equal(t, []string{fx.newOffering.Name}, autoEntry.AutoTriggerNames)

	manualEntry, ok := byLabel[fx.newOffering.Name]
	require.True(t, ok)
	assert.Equal(t, fx.newOffering.ID, manualEntry.OfferingID)
	assert.Empty(t, manualEntry.NewAutomaticDays, "the parent's own pick carries no automatic share")
	assert.Empty(t, manualEntry.AutoTriggerNames)
}

// Deliberately NOT parallel: the code under test sweeps rows across tenants.
// These service-level tests call it with a plain tenant context instead of a
// tenant transaction, so RLS never narrows the query and the sweep also picks
// up the rows of every test running beside it.
func TestOfferingChangeRequestService_ListPending_MarksAutomaticDiffEntries(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "QueueAuto")
	auto := createAutoAddTarget(t, env, "QueueAuto", fx.newOffering.ID)
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon"})

	views, err := svc.ListPending(ctx)
	require.NoError(t, err)
	var view *enrollmentService.OfferingChangeView
	for _, candidate := range views {
		if candidate.Request != nil && candidate.Request.ID == row.ID {
			view = candidate
		}
	}
	require.NotNil(t, view, "the queue must contain the new request")

	found := false
	for _, entry := range view.Diff {
		if entry.OfferingID == auto.ID {
			found = true
			assert.Equal(t, []string{"mon"}, entry.NewAutomaticDays)
			assert.Equal(t, []string{fx.newOffering.Name}, entry.AutoTriggerNames)
		}
	}
	assert.True(t, found, "the queue diff must mark the rule-added offering")
}

func TestOfferingChangeRequestService_ListPending_IncludesUnchangedGrandfatheredRuleTarget(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "QueueUnchangedAuto")
	auto := createAutoAddTarget(t, env, "QueueUnchangedAuto", fx.oldOffering.ID)
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID:        fx.childID,
		CareOfferingID:        auto.ID,
		SelectedDays:          []string{"mon", "tue"},
		ManualSelectedDays:    []string{"tue"},
		AutomaticSelectedDays: []string{"mon"},
	}))
	auto.AvailabilityRule = &enrollmentModels.CareOfferingAvailabilityRule{
		Match: enrollmentModels.AvailabilityMatchAll,
		Conditions: []enrollmentModels.CareOfferingAvailabilityCondition{{
			Source:   enrollmentModels.AvailabilitySourceGradeLevel,
			Operator: enrollmentModels.AvailabilityOperatorIn,
			Value:    []int{1},
		}},
	}
	require.NoError(t, env.repos.CareOffering.Update(ctx, auto))
	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: auto.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	views, err := svc.ListPending(ctx)
	require.NoError(t, err)
	for _, view := range views {
		if view.Request == nil || view.Request.ID != row.ID {
			continue
		}
		for _, entry := range view.Diff {
			if entry.OfferingID != auto.ID {
				continue
			}
			assert.Equal(t, []string{"mon", "tue"}, entry.OldDays)
			assert.Equal(t, []string{"mon", "tue"}, entry.NewDays)
			assert.Equal(t, []string{"mon"}, entry.NewRuleDays)
			assert.Equal(t, []int64{fx.oldOffering.ID}, entry.AutoTriggerIDs)
			return
		}
	}
	t.Fatal("unchanged rule-added offering is missing from the staff review")
}

func TestOfferingChangeRequestService_Decide_ExclusionSkipsAutoTargetAndRecordsOverride(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "OptOut")
	auto := createAutoAddTarget(t, env, "OptOut", fx.newOffering.ID)
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon", "tue"})

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:               row.ID,
		Approve:                 true,
		ReviewedBy:              env.creatorID,
		ExcludedAutoOfferingIDs: []int64{auto.ID},
	}))

	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, fx.childID, fx.switchDate)
	require.NoError(t, err)
	linkedIDs := make([]int64, 0, len(links))
	for _, link := range links {
		linkedIDs = append(linkedIDs, link.CareOfferingID)
	}
	assert.Contains(t, linkedIDs, fx.newOffering.ID)
	assert.NotContains(t, linkedIDs, auto.ID, "the excluded rule target must not be booked")

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, decided.DecisionSnapshot, "the decision must freeze its diff")
	require.Len(t, decided.DecisionSnapshot.OverriddenOfferings, 1)
	assert.Equal(t, auto.ID, decided.DecisionSnapshot.OverriddenOfferings[0].OfferingID)
	assert.Equal(t, auto.Name, decided.DecisionSnapshot.OverriddenOfferings[0].Name)
	assert.NotEmpty(t, decided.DecisionSnapshot.Diff)

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	require.NotNil(t, view)
	require.NotNil(t, view.LastDecision)
	require.Len(t, view.LastDecision.OverriddenOfferings, 1)
	assert.Equal(t, auto.Name, view.LastDecision.OverriddenOfferings[0].Name)
	assert.NotEmpty(t, view.LastDecision.AppliedDiff, "the recap must read from the frozen snapshot")
}

func TestOfferingChangeRequestService_Decide_ExclusionOmitsNeverBookedTargetFromSnapshot(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "OptOutSnapshot")
	auto := createAutoAddTarget(t, env, "OptOutSnapshot", fx.newOffering.ID)
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon"})

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
		ExcludedAutoOfferingIDs: []int64{auto.ID},
	}))
	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, decided.DecisionSnapshot)
	for _, entry := range decided.DecisionSnapshot.Diff {
		assert.NotEqual(t, auto.ID, entry.OfferingID,
			"an excluded target that was never booked must not appear as removed")
	}
}

func TestOfferingChangeRequestService_Decide_SnapshotMatchesGrandfatheredAutomaticBooking(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "GrandfatheredSnapshot")
	automatic := createAutoAddTarget(t, env, "GrandfatheredSnapshot", fx.oldOffering.ID)
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID:        fx.childID,
		CareOfferingID:        automatic.ID,
		SelectedDays:          []string{"mon"},
		AutomaticSelectedDays: []string{"mon"},
	}))
	automatic.AvailabilityRule = &enrollmentModels.CareOfferingAvailabilityRule{
		Match: enrollmentModels.AvailabilityMatchAll,
		Conditions: []enrollmentModels.CareOfferingAvailabilityCondition{{
			Source:   enrollmentModels.AvailabilitySourceGradeLevel,
			Operator: enrollmentModels.AvailabilityOperatorIn,
			Value:    []int{1},
		}},
	}
	require.NoError(t, env.repos.CareOffering.Update(ctx, automatic))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"mon"}},
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
	}))
	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, fx.childID, fx.switchDate)
	require.NoError(t, err)
	applied := false
	for _, link := range links {
		if link.CareOfferingID == automatic.ID {
			applied = true
			assert.Equal(t, []string{"mon"}, link.SelectedDays)
		}
	}
	require.True(t, applied, "the applier must retain the grandfathered automatic booking")

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, decided.DecisionSnapshot)
	for _, entry := range decided.DecisionSnapshot.Diff {
		assert.NotEqual(t, automatic.ID, entry.OfferingID,
			"the snapshot must not record a retained grandfathered booking as removed")
	}
}

func TestOfferingChangeRequestService_Decide_ExclusionKeepsManualAndRequiredLunchDays(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "OptOutMixed")
	care := createAdjustmentCareOfferingWith(t, env, "Ganztag OptOutMixed", func(o *enrollmentModels.CareOffering) {
		o.CountsAsCare, o.CountsAsCareSet, o.SortOrder = true, true, 203
	})
	trigger := createAdjustmentCareOfferingWith(t, env, "Randstunde OptOutMixed", func(o *enrollmentModels.CareOffering) {
		o.CountsAsCare, o.CountsAsCareSet = false, true
		o.SortOrder = 204
	})
	lunch := createAutoAddTarget(t, env, "OptOutMixed", trigger.ID)
	lunch.IsRequired, lunch.IncludesLunch = true, true
	lunch.CountsAsCare, lunch.CountsAsCareSet = false, true
	require.NoError(t, env.repos.CareOffering.Update(ctx, lunch))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: care.ID, SelectedDays: []string{"wed"}},
			{OfferingID: trigger.ID, SelectedDays: []string{"tue"}},
			{OfferingID: lunch.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	view, err := svc.GetForStudent(ctx, fx.studentID)
	require.NoError(t, err)
	found := false
	for _, entry := range view.Diff {
		if entry.OfferingID == lunch.ID {
			found = true
			assert.Equal(t, []string{"tue"}, entry.NewRuleDays)
			assert.Equal(t, []string{"mon", "wed"}, entry.NewDaysWithoutRules)
		}
	}
	require.True(t, found, "the mixed automatic target must appear in the review diff")
	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
		ExcludedAutoOfferingIDs: []int64{lunch.ID},
	}))
	links, err := env.repos.RequestChildOffering.ListByRequestChildIDAtDate(ctx, fx.childID, fx.switchDate)
	require.NoError(t, err)
	for _, link := range links {
		if link.CareOfferingID == lunch.ID {
			assert.Equal(t, []string{"mon", "wed"}, link.SelectedDays)
			return
		}
	}
	t.Fatal("the manual and required-lunch shares must keep the target booked")
}

func TestOfferingChangeRequestService_PreviewDecision_RecomputesPartialExclusionCascade(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "PreviewPartialCascade")
	care := createAdjustmentCareOfferingWith(t, env, "Ganztag PreviewPartialCascade", func(o *enrollmentModels.CareOffering) {
		o.CountsAsCare, o.CountsAsCareSet, o.SortOrder = true, true, 203
	})
	trigger := createAdjustmentCareOfferingWith(t, env, "Randstunde PreviewPartialCascade", func(o *enrollmentModels.CareOffering) {
		o.CountsAsCare, o.CountsAsCareSet, o.SortOrder = false, true, 204
	})
	mixed := createAutoAddTarget(t, env, "PreviewPartialCascade", trigger.ID)
	mixed.IsRequired, mixed.IncludesLunch = true, true
	mixed.CountsAsCare, mixed.CountsAsCareSet, mixed.SortOrder = false, true, 205
	require.NoError(t, env.repos.CareOffering.Update(ctx, mixed))
	downstream := createAutoAddTarget(t, env, "PreviewPartialCascade downstream", mixed.ID)
	downstream.CountsAsCare, downstream.CountsAsCareSet, downstream.SortOrder = false, true, 206
	require.NoError(t, env.repos.CareOffering.Update(ctx, downstream))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: care.ID, SelectedDays: []string{"wed"}},
			{OfferingID: trigger.ID, SelectedDays: []string{"tue"}},
			{OfferingID: mixed.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	preview, err := svc.PreviewDecision(ctx, row.ID, []int64{mixed.ID})
	require.NoError(t, err)
	byID := make(map[int64][]string, len(preview))
	for _, selection := range preview {
		byID[selection.OfferingID] = selection.Days
	}
	assert.Equal(t, []string{"mon", "wed"}, byID[mixed.ID])
	assert.Equal(t, []string{"mon", "wed"}, byID[downstream.ID],
		"the downstream rule must use the partially retained trigger days")
}

func TestOfferingChangeRequestService_Decide_RejectsExclusionOfNonAutomaticOffering(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "OptOutInvalid")
	createAutoAddTarget(t, env, "OptOutInvalid", fx.newOffering.ID)
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon"})

	err := svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:               row.ID,
		Approve:                 true,
		ReviewedBy:              env.creatorID,
		ExcludedAutoOfferingIDs: []int64{fx.newOffering.ID},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid,
		"a parent-chosen offering cannot be overridden away")

	pending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, pending.Status)
}

func TestOfferingChangeRequestService_Decide_RejectsExclusionWithoutRuleDerivedDays(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "OptOutNoRuleDays")
	care := createAdjustmentCareOfferingWith(t, env, "Ganztag OptOutNoRuleDays", func(o *enrollmentModels.CareOffering) {
		o.CountsAsCare = true
		o.CountsAsCareSet = true
		o.SortOrder = 203
	})
	trigger := createAdjustmentCareOfferingWith(t, env, "Randstunde OptOutNoRuleDays", func(o *enrollmentModels.CareOffering) {
		o.SortOrder = 204
	})
	lunch := createAdjustmentCareOfferingWith(t, env, "Mittagessen OptOutNoRuleDays", func(o *enrollmentModels.CareOffering) {
		o.AvailableDays = []string{"mon"}
		o.IsRequired = true
		o.IncludesLunch = true
		o.CountsAsCare = false
		o.CountsAsCareSet = true
		o.SortOrder = 205
	})
	require.NoError(t, env.repos.CareOffering.ReplaceAutoAddTriggers(ctx, lunch.ID, []int64{trigger.ID}))

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID: fx.studentID, AccountID: env.creatorID, EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: care.ID, SelectedDays: []string{"mon"}},
			{OfferingID: trigger.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	err = svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:               row.ID,
		Approve:                 true,
		ReviewedBy:              env.creatorID,
		ExcludedAutoOfferingIDs: []int64{lunch.ID},
	})
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid,
		"a selected trigger that contributes no target days must not authorize an override")

	pending, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, pending.Status)
}

func TestOfferingChangeRequestService_Decide_RevalidatesExclusionAgainstAppliedRules(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	fx := setupOfferingChangeFixture(t, env, "FinalOverrideRules")
	auto := createAutoAddTarget(t, env, "FinalOverrideRules", fx.newOffering.ID)
	mutatingRepo := &ruleMutationOnLockCareOfferingRepo{
		CareOfferingRepository: env.repos.CareOffering,
		mutate: func(ctx context.Context) error {
			return env.repos.CareOffering.ReplaceAutoAddTriggers(ctx, auto.ID, nil)
		},
	}
	svc := newOfferingChangeServiceForTestWithCareRepo(t, env, mutatingRepo)
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon"})

	err := svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: true, ReviewedBy: env.creatorID,
		ExcludedAutoOfferingIDs: []int64{auto.ID},
	})
	require.True(t, mutatingRepo.mutated, "the rule must change before the applied materialization")
	assert.Contains(t, mutatingRepo.locked, auto.ID, "the excluded rule target must be locked")
	require.ErrorIs(t, err, enrollmentService.ErrOfferingChangeInvalid)

	pending, findErr := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, findErr)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusPending, pending.Status)
	assert.Nil(t, pending.DecisionSnapshot)
}

func TestOfferingChangeRequestService_Decide_RejectionFreezesDiffSnapshot(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "SnapReject")
	auto := createAutoAddTarget(t, env, "SnapReject", fx.newOffering.ID)
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon"})

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID:  row.ID,
		Approve:    false,
		Reason:     "Kapazität",
		ReviewedBy: env.creatorID,
	}))

	decided, err := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, err)
	require.NotNil(t, decided.DecisionSnapshot)
	assert.Empty(t, decided.DecisionSnapshot.OverriddenOfferings)
	var autoSnap *enrollmentModels.OfferingChangeSnapshotEntry
	for i := range decided.DecisionSnapshot.Diff {
		if decided.DecisionSnapshot.Diff[i].OfferingID == auto.ID {
			autoSnap = &decided.DecisionSnapshot.Diff[i]
		}
	}
	require.NotNil(t, autoSnap, "the frozen diff keeps the rule-added line")
	assert.Equal(t, []string{"mon"}, autoSnap.NewAutomaticDays)
	assert.Equal(t, []string{"mon"}, autoSnap.NewRuleDays)
	assert.Equal(t, []string{fx.newOffering.Name}, autoSnap.AutoTriggerNames)
}

func TestOfferingChangeRequestService_Decide_RejectionFallsBackToPayloadSnapshot(t *testing.T) {
	t.Parallel()

	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "RejectSnapshotFailure")
	auto := createAutoAddTarget(t, env, "RejectSnapshotFailure", fx.oldOffering.ID)
	require.NoError(t, env.repos.RequestChildOffering.Create(ctx, &enrollmentModels.RequestChildOffering{
		RequestChildID:        fx.childID,
		CareOfferingID:        auto.ID,
		SelectedDays:          []string{"mon"},
		AutomaticSelectedDays: []string{"mon"},
	}))
	row := createPendingTriggerRequest(t, env, svc, fx, []string{"mon"})
	fx.newOffering.IsActive = false
	require.NoError(t, env.repos.CareOffering.Update(ctx, fx.newOffering))

	err := svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: false, Reason: "Kapazität", ReviewedBy: env.creatorID,
	})
	require.NoError(t, err)

	decided, findErr := env.repos.OfferingChangeRequest.FindByID(ctx, row.ID)
	require.NoError(t, findErr)
	assert.Equal(t, enrollmentModels.OfferingChangeStatusRejected, decided.Status)
	require.NotNil(t, decided.DecisionSnapshot)
	require.NotEmpty(t, decided.DecisionSnapshot.Diff)
	assert.Contains(t, decided.DecisionSnapshot.Diff, enrollmentModels.OfferingChangeSnapshotEntry{
		OfferingID: fx.newOffering.ID,
		Label:      fx.newOffering.Name,
		OldState:   "not_booked",
		NewState:   "booked",
		NewDays:    []string{"mon"},
	})
	for _, entry := range decided.DecisionSnapshot.Diff {
		assert.NotEqual(t, auto.ID, entry.OfferingID,
			"the payload fallback must not report an automatic-only booking as explicitly removed")
	}
}
