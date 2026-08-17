package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

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
		testpkg.TenantContext(1), offering.ID, []int64{triggerID},
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
	ctx := offeringChangeAdminContext()
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
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext()
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

func TestOfferingChangeRequestService_ListPending_MarksAutomaticDiffEntries(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext()
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

func TestOfferingChangeRequestService_Decide_ExclusionSkipsAutoTargetAndRecordsOverride(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext()
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

func TestOfferingChangeRequestService_Decide_RejectsExclusionOfNonAutomaticOffering(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext()
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

func TestOfferingChangeRequestService_Decide_RejectionFreezesDiffSnapshot(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext()
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
	assert.Equal(t, []string{fx.newOffering.Name}, autoSnap.AutoTriggerNames)
}
