package enrollment_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	enrollmentModels "github.com/moto-nrw/project-phoenix/models/enrollment"
	enrollmentService "github.com/moto-nrw/project-phoenix/services/enrollment"
	testpkg "github.com/moto-nrw/project-phoenix/test"
)

// The staff queue has to say what a decision does to the child's whole booking
// picture, not just to the lines that move (#2434): a request that empties the
// list must be recognizable as a Komplett-Abmeldung, and one that keeps other
// offerings must name them.

func pendingViewForRequest(
	t *testing.T,
	svc enrollmentService.OfferingChangeRequestService,
	requestID int64,
) *enrollmentService.OfferingChangeView {
	t.Helper()
	views, err := svc.ListPending(offeringChangeAdminContext())
	require.NoError(t, err)
	for _, view := range views {
		if view.Request != nil && view.Request.ID == requestID {
			return view
		}
	}
	t.Fatal("the queue must contain the new request")
	return nil
}

func TestOfferingChangeRequestService_ListPending_MarksFullWithdrawal(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "FullWithdrawal")

	row, err := svc.Create(offeringChangeAdminContext(), enrollmentService.CreateOfferingChangeInput{
		StudentID:     fx.studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections:    nil,
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	view := pendingViewForRequest(t, svc, row.ID)
	assert.True(t, view.FullWithdrawal, "an empty offering list is a Komplett-Abmeldung")
	assert.Empty(t, view.Unchanged, "nothing stays booked after a Komplett-Abmeldung")
}

func TestOfferingChangeRequestService_ListPending_KeepsUntouchedBookingsOutOfTheWarning(t *testing.T) {
	env, cleanup := setupDecisionTest(t)
	defer cleanup()
	svc := newOfferingChangeServiceForTest(t, env)
	fx := setupOfferingChangeFixture(t, env, "KeepsOther")
	// A second child that holds both offerings, so dropping one leaves one.
	_, _, studentID := submitApprovedAdjustmentChild(
		t, env, "offer-change-keeps-other@example.com", "OfferChangeKeepsOther",
		[]*enrollmentModels.CareOffering{fx.oldOffering, fx.newOffering},
	)

	row, err := svc.Create(offeringChangeAdminContext(), enrollmentService.CreateOfferingChangeInput{
		StudentID:     studentID,
		AccountID:     env.creatorID,
		EffectiveFrom: fx.switchDate,
		Selections: []enrollmentService.OfferingChangeSelection{
			{OfferingID: fx.oldOffering.ID, SelectedDays: []string{"mon"}},
		},
	})
	require.NoError(t, err)
	t.Cleanup(func() { testpkg.CleanupTableRecords(t, env.db, "enrollment.offering_change_requests", row.ID) })

	view := pendingViewForRequest(t, svc, row.ID)
	assert.False(t, view.FullWithdrawal, "one offering stays booked, so this is no Komplett-Abmeldung")
	require.Len(t, view.Unchanged, 1)
	assert.Equal(t, fx.oldOffering.ID, view.Unchanged[0].OfferingID)
	assert.Equal(t, []string{"mon"}, view.Unchanged[0].NewDays)
	for _, entry := range view.Diff {
		assert.NotEqual(t, fx.oldOffering.ID, entry.OfferingID,
			"an untouched booking does not belong in the changed lines")
	}
}
