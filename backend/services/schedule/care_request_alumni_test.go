// Graduated children drop out of the care-request review surface (#405 review).
//
// Graduation is a soft delete: the guardian link and the pending request survive
// it (the hard delete it replaced cascaded them away). The parent portal already
// hides the child, but the staff side hydrated the queue through the unfiltered
// FindByIDs and authorized decisions through the unfiltered FindByID — so the
// request stayed in the queue and the sidebar badge, and could still be approved
// onto a departed child.
package schedule_test

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	scheduleModels "github.com/moto-nrw/project-phoenix/models/schedule"
	usersModels "github.com/moto-nrw/project-phoenix/models/users"
	"github.com/moto-nrw/project-phoenix/services/schedule"
)

func TestCareRequest_GraduatedChildLeavesQueueAndRefusesDecisions(t *testing.T) {
	f := newCareFixture(t)
	req := f.createPending(t, careWeekdays(
		map[string]any{"weekday": 1, "mode": "pickup", "arrival": "08:00", "pickup": "16:00"},
	))
	ctx := f.staffCtx(f.staffAccount)

	containsRequest := func(items []*schedule.CareRequestReviewItem) bool {
		for _, it := range items {
			if it.Request.ID == req.ID {
				return true
			}
		}
		return false
	}

	items, err := f.svc.ListPending(ctx)
	require.NoError(t, err)
	require.True(t, containsRequest(items), "the pending request must start out visible")

	// The child graduates (grade-transition apply).
	_, err = f.db.NewUpdate().
		TableExpr("users.students").
		Set("status = ?", string(usersModels.StudentStatusAlumnus)).
		Where("id = ?", f.chain.StudentID).
		Exec(ctx)
	require.NoError(t, err)

	items, err = f.svc.ListPending(ctx)
	require.NoError(t, err)
	assert.False(t, containsRequest(items), "a graduated child's request must leave the queue and the badge")

	// Neither direction may still act on the alumnus — same 404 the rest of the
	// child surface returns for graduates.
	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: true, ReviewedBy: f.staffAccount,
	})
	assert.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound)

	_, err = f.svc.Decide(ctx, schedule.CareRequestDecideInput{
		RequestID: req.ID, Approve: false, Reason: "child has left", ReviewedBy: f.staffAccount,
	})
	assert.ErrorIs(t, err, scheduleModels.ErrCareRequestNotFound)

	// The request is untouched, so a transition revert restores a coherent state.
	var status string
	require.NoError(t, f.db.NewSelect().
		TableExpr("schedule.care_schedule_change_requests").
		Column("status").
		Where("id = ?", req.ID).
		Scan(ctx, &status))
	assert.Equal(t, scheduleModels.CareRequestStatusPending, status)
}
