package application

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

type batchReviewReads struct {
	careplan.CareScheduleRequestQuery
	careplan.ScheduleReviewDirectory
	ScheduleReviewSchedules
	unavailableID    int64
	batchUnavailable bool
}

func (batchReviewReads) ListCareScheduleRequests(context.Context, careplan.CareScheduleRequestFilter) ([]careplan.CareScheduleChangeRequest, error) {
	return []careplan.CareScheduleChangeRequest{
		{ID: 101, StudentID: 11, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`)},
		{ID: 102, StudentID: 22, RequestKind: "weekly_schedule", Status: "pending", Payload: json.RawMessage(`{"weekdays":[{"weekday":1,"pickup":"16:00"}]}`)},
	}, nil
}
func (batchReviewReads) FindStudents(context.Context, []int64) (map[int64]careplan.ReviewStudent, error) {
	return map[int64]careplan.ReviewStudent{11: {ID: 11}, 22: {ID: 22}}, nil
}
func (batchReviewReads) PersonNames(context.Context, []int64) (map[int64]careplan.PersonName, error) {
	return nil, nil
}
func (r batchReviewReads) ListPickupSchedules(_ context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	if r.batchUnavailable && len(filter.StudentIDs) > 1 {
		return nil, errors.New("batch pickup read unavailable")
	}
	var pickups []careplan.PickupSchedule
	for _, id := range filter.StudentIDs {
		if id == r.unavailableID {
			return nil, errors.New("child's pickup read unavailable")
		}
		hour := 12
		if id == 22 {
			hour = 13
		}
		pickups = append(pickups, careplan.PickupSchedule{StudentID: id, Weekday: 1, PickupTime: time.Date(1, 1, 1, hour, 0, 0, 0, time.UTC)})
	}
	return pickups, nil
}
func (batchReviewReads) ListArrivalSchedules(context.Context, careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	return nil, nil
}
func (batchReviewReads) ListClassArrivalTimes(context.Context, []string) (map[string]map[string]string, error) {
	return nil, nil
}
func (batchReviewReads) ListStudentDepartureModes(context.Context, []int64) (map[int64]map[string][]string, error) {
	return nil, nil
}
func (batchReviewReads) ApprovedForStudents(context.Context, []int64, careplan.Date, careplan.Date) ([]careplan.ReviewBooking, error) {
	return nil, nil
}
func TestReviewBatchPreservesPerChildFallback(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name             string
		unavailableID    int64
		batchUnavailable bool
		old              []string
	}{
		{name: "separate baselines on success", old: []string{"12:00", "13:00"}},
		{name: "second child unavailable", unavailableID: 22, old: []string{"12:00", ""}},
		{name: "first child unavailable", unavailableID: 11, old: []string{"", "13:00"}},
		{name: "batch unavailable but scalar reads succeed", batchUnavailable: true, old: []string{"12:00", "13:00"}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			f := batchReviewReads{unavailableID: tc.unavailableID, batchUnavailable: tc.batchUnavailable}
			query := NewScheduleReviews(ScheduleReviewDependencies{
				RunInTx: func(ctx context.Context, fn func(context.Context) error) error { return fn(ctx) }, Requests: f, Schedules: f, People: f, Classes: f, Bookings: f,
				Scope: func(context.Context) (careplan.ReviewScope, error) {
					return careplan.ReviewScope{SchoolWide: true}, nil
				},
				BookingsAuthoritative: func(context.Context) (bool, error) { return false, nil }, Today: func() careplan.Date { return "2026-11-05" },
			})
			items, _, err := query.ListPending(context.Background(), careplan.RequestQueueFilter{})
			require.NoError(t, err)
			require.Len(t, items, 2)
			for i, old := range tc.old {
				require.Len(t, items[i].Diff, 1)
				require.Equal(t, old, items[i].Diff[0].Old, "only the unavailable child's diff uses requested summary")
				require.Equal(t, "16:00", items[i].Diff[0].New)
			}
		})
	}
}
