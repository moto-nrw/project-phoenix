package compose

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/require"
)

type scheduleReviewDirectoryFixture struct{ reviewDirectoryFixture }

func (scheduleReviewDirectoryFixture) DepartureModes(context.Context, []int64) (map[int64]map[string][]string, error) {
	return map[int64]map[string][]string{}, nil
}

type scheduleReviewBookingsFixture struct{}

func (scheduleReviewBookingsFixture) ApprovedForStudents(context.Context, []int64, careplan.Date, careplan.Date) ([]ReviewBooking, error) {
	return nil, nil
}

type scheduleReviewClassesFixture struct{}

func (scheduleReviewClassesFixture) ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, error) {
	return map[string]map[string]string{"1a": {"mon": "12:15"}}, nil
}

type failingPickupReviewBlocks struct{ calls int }

func (f *failingPickupReviewBlocks) PreviewPickupBlocks(context.Context, PickupReviewImpact) ([]PickupReviewBlock, error) {
	f.calls++
	return nil, errors.New("preview unavailable")
}

func TestScheduleNativeQueueDiffScopeHistoryAndPreviewFailure(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := buildModule(t, db)
	ctx := testpkg.Ctx(t)
	student := testpkg.CreateTestStudent(t, db, "Care", "Review", "1a")
	hidden := testpkg.CreateTestStudent(t, db, "Hidden", "Review", "1a")
	staff := testpkg.CreateTestStaff(t, db, "Care", "Reviewer")
	account := testpkg.CreateTestAccount(t, db, "parent")
	group := testpkg.CreateTestEducationGroup(t, db, "CareReviewScope")
	_, err := module.CreateArrivalSchedule(ctx, careplan.ArrivalSchedule{StudentID: student.ID, Weekday: 1, CreatedBy: staff.ID})
	require.NoError(t, err)
	_, err = module.CreatePickupSchedule(ctx, careplan.PickupSchedule{StudentID: student.ID, Weekday: 1, PickupTime: time.Date(1, 1, 1, 15, 0, 0, 0, time.UTC), CreatedBy: staff.ID})
	require.NoError(t, err)
	create := func(id int64, kind, payload string) careplan.CareScheduleChangeRequest {
		row, createErr := module.CreateCareScheduleRequest(ctx, careplan.CareScheduleChangeRequest{StudentID: id, SubmittedBy: account.ID, RequestKind: kind, Payload: json.RawMessage(payload), Status: "pending"})
		require.NoError(t, createErr)
		return row
	}
	weekly := create(student.ID, "weekly_schedule", `{"weekdays":[{"weekday":1,"arrival":"12:30","pickup":"14:30"}]}`)
	create(hidden.ID, "weekly_schedule", `{"weekdays":[]}`)
	directory := scheduleReviewDirectoryFixture{reviewDirectoryFixture{
		students: map[int64]ReviewStudent{student.ID: {ID: student.ID, PersonID: student.PersonID, GroupID: &group.ID, SchoolClass: "1a"}, hidden.ID: {ID: hidden.ID, PersonID: hidden.PersonID}},
		names:    map[int64]PersonName{student.PersonID: {FirstName: "Care", LastName: "Review"}},
	}}
	blocks := &failingPickupReviewBlocks{}
	deps := ScheduleReviewDependencies{People: directory, Bookings: scheduleReviewBookingsFixture{}, Classes: scheduleReviewClassesFixture{},
		Scope:                 func(context.Context) (ReviewScope, error) { return ReviewScope{GroupIDs: []int64{group.ID}}, nil },
		BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil }, Today: func() careplan.Date { return "2026-03-30" }, Blocks: blocks,
	}
	queue, err := NewScheduleReviews(db, func(Observation) {}, deps)
	require.NoError(t, err)
	items, cursor, err := queue.ListPending(ctx, careplan.RequestQueueFilter{Limit: 1})
	require.NoError(t, err)
	require.Empty(t, items)
	require.NotNil(t, cursor)
	items, next, err := queue.ListPending(ctx, careplan.RequestQueueFilter{Limit: 1, BeforeID: cursor.ID, BeforeInstant: cursor.UpdatedAt})
	require.NoError(t, err)
	require.Nil(t, next)
	require.Len(t, items, 1)
	require.Equal(t, weekly.ID, items[0].Request.ID)
	require.Len(t, items[0].Diff, 2)
	require.Equal(t, "12:15", items[0].Diff[0].Old)
	require.Equal(t, "15:00", items[0].Diff[1].Old)
	create(student.ID, "pickup_change", `{"date":"2026-03-30","pickup_time":"14:00","reason":"  Termin  "}`)
	items, _, err = queue.ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.Equal(t, "Termin", *items[0].Reason)
	require.False(t, items[0].ImpactAvailable)
	require.Equal(t, "15:00", items[0].Diff[0].Old, "a preview failure must not erase the independently resolved diff")
	require.Equal(t, 1, blocks.calls)
	ended := directory.students[student.ID]
	ended.EnrolledUntil = "2026-03-29"
	directory.students[student.ID] = ended
	items, _, err = queue.ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Empty(t, items)
	items, _, err = queue.ListPending(ctx, careplan.RequestQueueFilter{UrgentDate: "2026-03-29"})
	require.NoError(t, err)
	require.Len(t, items, 2)
	require.NoError(t, module.DecideCareScheduleRequest(ctx, careplan.CareScheduleRequestDecision{ID: weekly.ID, Status: "approved", ReviewedBy: &account.ID}))
	history, _, err := queue.ListHistory(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Len(t, history, 1)
	require.Equal(t, "Unbekannt", history[0].ReviewerName)
	require.Len(t, history[0].Requested, 2)
	require.Empty(t, history[0].Requested[0].Old)
	ended.Alumnus = true
	directory.students[student.ID] = ended
	history, _, err = queue.ListHistory(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Empty(t, history)
	deps.BookingsAuthoritative = nil
	_, err = NewScheduleReviews(db, func(Observation) {}, deps)
	require.Error(t, err)
}
