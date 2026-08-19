package enrollment_test

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// TestOfferingChangeRequestService_ListHistory proves the staff history:
// a decided request comes back with the child's name, the reviewer's display
// name, the decision reason, and the FROZEN snapshot diff — while pending rows
// stay out.
func TestOfferingChangeRequestService_ListHistory(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	ctx := offeringChangeAdminContext(t)
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "History")

	row, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	// While pending, the history is empty.
	items, next, err := svc.ListHistory(ctx, time.Time{}, 0, 25)
	require.NoError(t, err)
	assert.Nil(t, next)
	for _, item := range items {
		assert.NotEqual(t, row.ID, item.Request.ID, "pending rows must never appear in the history")
	}

	require.NoError(t, svc.Decide(ctx, enrollmentService.DecideOfferingChangeInput{
		RequestID: row.ID, Approve: false, Reason: "Kapazität erschöpft", ReviewedBy: env.creatorID,
	}))

	items, next, err = svc.ListHistory(ctx, time.Time{}, 0, 25)
	require.NoError(t, err)
	assert.Nil(t, next)
	var got *enrollmentService.OfferingChangeHistoryItem
	for _, item := range items {
		if item.Request.ID == row.ID {
			got = item
		}
	}
	require.NotNil(t, got, "the decided request must appear in the history")
	assert.Equal(t, enrollmentModels.OfferingChangeStatusRejected, got.Request.Status)
	assert.NotEmpty(t, got.StudentName)
	assert.Equal(t, "Rollover Tester", got.ReviewerName)
	require.NotNil(t, got.Request.DecisionReason)
	assert.Equal(t, "Kapazität erschöpft", *got.Request.DecisionReason)
	require.NotEmpty(t, got.Diff, "the history diff comes from the frozen decision snapshot")
	labels := make([]string, 0, len(got.Diff))
	for _, entry := range got.Diff {
		labels = append(labels, entry.Label)
	}
	assert.Contains(t, labels, fx.newOffering.Name)

	withdrawn, err := svc.Create(ctx, enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.newOffering.ID, SelectedDays: []string{"tue"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", withdrawn.ID) })
	require.NoError(t, svc.Withdraw(ctx, withdrawn.ID, env.creatorID, fx.studentID))

	items, _, err = svc.ListHistory(ctx, time.Time{}, 0, 25)
	require.NoError(t, err)
	var withdrawnHistory *enrollmentService.OfferingChangeHistoryItem
	for _, item := range items {
		if item.Request.ID == withdrawn.ID {
			withdrawnHistory = item
			break
		}
	}
	require.NotNil(t, withdrawnHistory)
	assert.Empty(t, withdrawnHistory.Diff, "withdrawn rows have no decision snapshot")
	require.Equal(t, []enrollmentService.OfferingChangeRequestedItem{{
		OfferingID: fx.newOffering.ID, Name: fx.newOffering.Name, Days: []string{"tue"},
	}}, withdrawnHistory.Requested)
}
