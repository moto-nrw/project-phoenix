package compose

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

func TestScheduleHistoryCursorUsesLastDatabaseRowBeforeScopeFiltering(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	visible := testpkg.CreateTestStudent(t, db, "Visible", "History", "1a")
	hidden := testpkg.CreateTestStudent(t, db, "Hidden", "History", "1a")
	account := testpkg.CreateTestAccount(t, db, "parent")
	group := testpkg.CreateTestEducationGroup(t, db, "HistoryScope")
	var visibleID int64
	for _, id := range []int64{visible.ID, hidden.ID} {
		request, err := module.CreateCareScheduleRequest(ctx, careplan.CareScheduleChangeRequest{
			StudentID: id, SubmittedBy: account.ID, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`),
		})
		require.NoError(t, err)
		require.NoError(t, module.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: request.ID, Status: "rejected", ReviewedBy: &account.ID}))
		if id == visible.ID {
			visibleID = request.ID
		}
	}
	directory := scheduleReviewDirectoryFixture{reviewDirectoryFixture{students: map[int64]ReviewStudent{
		visible.ID: {ID: visible.ID, PersonID: visible.PersonID, GroupID: &group.ID},
		hidden.ID:  {ID: hidden.ID, PersonID: hidden.PersonID},
	}}}
	query, err := NewScheduleReviews(db, func(Observation) {}, ScheduleReviewDependencies{
		People: directory, Bookings: scheduleReviewBookingsFixture{}, Classes: scheduleReviewClassesFixture{},
		Scope:                 func(context.Context) (ReviewScope, error) { return ReviewScope{GroupIDs: []int64{group.ID}}, nil },
		BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil }, Today: func() careplan.Date { return "2026-09-20" },
	})
	require.NoError(t, err)
	items, cursor, err := query.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, items)
	require.NotNil(t, cursor, "a hidden page must still lead to the older visible row")
	items, next, err := query.ListHistory(ctx, careplan.RequestQueueFilter{Limit: 1, BeforeInstant: cursor.UpdatedAt, BeforeID: cursor.ID})
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, items, 1)
	require.Equal(t, visibleID, items[0].Request.ID)
}
