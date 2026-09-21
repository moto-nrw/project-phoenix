package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/modules/careplan/carerequests"
	"github.com/moto-nrw/project-phoenix/modules/careplan/internal/ports"
	"github.com/stretchr/testify/require"
)

type failingReviewPickupRead struct {
	ScheduleReviewSchedules
	err   error
	calls int
}

func (r *failingReviewPickupRead) ListPickupSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	r.calls++
	return nil, r.err
}

func TestListPendingFallsBackToRequestedWhenPickupPlanReadFails(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	reads := &failingReviewPickupRead{err: errors.New("pickup plan unavailable")}
	query := NewScheduleReviews(ScheduleReviewDependencies{
		Requests: pendingPickupReview{}, Schedules: reads,
		People: pendingPickupReview{},
		Scope:  func(context.Context) (ports.ReviewScope, error) { return ports.ReviewScope{SchoolWide: true}, nil },
		Today:  func() careplan.Date { return "2026-09-20" },
	})
	items, _, err := query.ListPending(ctx, careplan.RequestQueueFilter{})
	require.NoError(t, err)
	require.Equal(t, 1, reads.calls)
	require.Len(t, items, 1)
	require.Equal(t, []carerequests.DiffEntry{{Label: "Montag · Abholzeit", New: "16:00", Weekday: 1, CareKind: carerequests.KindPickup}}, items[0].Diff)
}

type pendingPickupReview struct {
	careplan.CareScheduleRequestQuery
	ports.ScheduleReviewDirectory
}

func (pendingPickupReview) ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	return []careplan.CareScheduleChangeRequest{{RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`)}}, nil
}

func (pendingPickupReview) FindStudents(context.Context, []int64) (map[int64]ports.ReviewStudent, error) {
	var student ports.ReviewStudent
	return map[int64]ports.ReviewStudent{student.ID: student}, nil
}

func (pendingPickupReview) PersonNames(context.Context, []int64) (map[int64]ports.PersonName, error) {
	return nil, nil
}
