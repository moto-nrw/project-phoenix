package application

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"slices"
	"strings"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/require"
)

// The same port can reject a batch while individual children remain readable.
// It can also fail one child's read; that child's requested summary must not
// replace any other child's successfully loaded baseline.
type weeklyPlanReads struct {
	batchReviewReads
	batchFailure string
	childFailure string
	childID      int64
	reads        []string
}

func (r *weeklyPlanReads) read(stage string, ids []int64) error {
	r.reads = append(r.reads, fmt.Sprintf("%s:%v", stage, ids))
	if (r.batchFailure == stage && len(ids) > 1) || (r.childFailure == stage && slices.Contains(ids, r.childID)) {
		return errors.New("plan input unavailable")
	}
	return nil
}

func (r *weeklyPlanReads) ListPickupSchedules(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupSchedule, error) {
	if err := r.read("pickups", f.StudentIDs); err != nil {
		return nil, err
	}
	return r.batchReviewReads.ListPickupSchedules(ctx, f)
}

func (r *weeklyPlanReads) FindStudents(context.Context, []int64) (map[int64]careplan.ReviewStudent, error) {
	return map[int64]careplan.ReviewStudent{11: {ID: 11, SchoolClass: " 1A "}, 22: {ID: 22, SchoolClass: "2B"}}, nil
}

func (r *weeklyPlanReads) ListArrivalSchedules(_ context.Context, f careplan.StudentScheduleFilter) ([]careplan.ArrivalSchedule, error) {
	if err := r.read("arrivals", f.StudentIDs); err != nil {
		return nil, err
	}
	var rows []careplan.ArrivalSchedule
	for _, id := range f.StudentIDs {
		row := careplan.ArrivalSchedule{StudentID: id, Weekday: 1}
		if id == 11 {
			row.ExpectedArrival = time.Date(1, 1, 1, 11, 15, 0, 0, time.UTC)
		}
		rows = append(rows, row)
	}
	return rows, nil
}

func (r *weeklyPlanReads) ListClassArrivalTimes(_ context.Context, classes []string) (map[string]map[string]string, error) {
	ids := make([]int64, len(classes))
	for i, class := range classes {
		ids[i] = 11
		if strings.EqualFold(class, "2b") {
			ids[i] = 22
		}
	}
	if err := r.read("classes", ids); err != nil {
		return nil, err
	}
	return map[string]map[string]string{"1a": {"mon": "10:00"}, "2b": {"mon": "11:45"}}, nil
}

func (r *weeklyPlanReads) ListStudentDepartureModes(_ context.Context, ids []int64) (map[int64]map[string][]string, error) {
	if err := r.read("modes", ids); err != nil {
		return nil, err
	}
	return map[int64]map[string][]string{11: {"mon": {"alone"}}, 22: {"mon": {"bus"}}}, nil
}

func (r *weeklyPlanReads) ApprovedForStudents(_ context.Context, ids []int64, from, to careplan.Date) ([]careplan.ReviewBooking, error) {
	if err := r.read("bookings", ids); err != nil {
		return nil, err
	}
	var rows []careplan.ReviewBooking
	for _, id := range ids {
		rows = append(rows,
			careplan.ReviewBooking{StudentID: id, OfferingID: id, ValidFrom: from, ValidUntil: to.AddDays(1)},
			careplan.ReviewBooking{StudentID: id, OfferingID: 999}, // Unknown offering stays ignored.
		)
	}
	return rows, nil
}

func (r *weeklyPlanReads) ListCareOfferings(_ context.Context, f careplan.CareOfferingFilter) ([]careplan.CareOffering, error) {
	ids := slices.DeleteFunc(slices.Clone(f.IDs), func(id int64) bool { return id == 999 })
	if err := r.read("offerings", ids); err != nil {
		return nil, err
	}
	var rows []careplan.CareOffering
	for _, id := range ids {
		rows = append(rows, careplan.CareOffering{ID: id, IsActive: true, CountsAsCare: true, DaysOfWeekMode: "fixed", AvailableDays: []string{"mon"}, PickupTimes: map[string]string{"mon": "14:00"}})
	}
	return rows, nil
}

func weeklyPlanService(reads *weeklyPlanReads) *ScheduleReviews {
	return &ScheduleReviews{deps: ScheduleReviewDependencies{
		Requests: reads, Schedules: reads, People: reads, Classes: reads, Bookings: reads,
		Scope: func(context.Context) (careplan.ReviewScope, error) {
			return careplan.ReviewScope{SchoolWide: true}, nil
		},
		Today: func() careplan.Date { return "2026-11-05" }, Logger: slog.Default(),
		BookingsAuthoritative: func(context.Context) (bool, error) {
			if reads.batchFailure == "policy" && len(reads.reads) == 4 {
				return false, errors.New("policy read unavailable")
			}
			return true, nil
		},
	}}
}

func TestWeeklyPlanBatchMatchesScalarAndPreservesChildFallback(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	today := careplan.Date("2026-11-05")
	for _, stage := range []string{"", "pickups", "arrivals", "classes", "modes", "policy", "bookings", "offerings"} {
		for _, childID := range []int64{0, 11, 22} {
			t.Run(fmt.Sprintf("%s/child-%d", stage, childID), func(t *testing.T) {
				reads := &weeklyPlanReads{batchFailure: stage}
				if childID != 0 && stage != "policy" {
					reads.childFailure, reads.childID = stage, childID
				}
				s := weeklyPlanService(reads)
				rows, err := reads.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{})
				require.NoError(t, err)
				students, err := reads.FindStudents(ctx, nil)
				require.NoError(t, err)
				cache := s.preloadReviewPlans(ctx, rows, students, careplan.ReviewScope{SchoolWide: true}, today)
				for _, id := range []int64{11, 22} {
					got, gotErr := s.cachedPlan(ctx, cache, students[id], today)
					want, wantErr := s.plan(ctx, students[id], today, true)
					require.Equal(t, wantErr != nil, gotErr != nil)
					if gotErr == nil {
						gotPlan, err := got.weekly(today)
						require.NoError(t, err)
						wantPlan, err := want.weekly(today)
						require.NoError(t, err)
						require.Equal(t, wantPlan, gotPlan)
						require.Equal(t, map[int]string{1: fmt.Sprintf("%d:00", 11+int(id/11))}, gotPlan.PickupTimes)
					}
				}
				items, _, err := s.ListPending(ctx, careplan.RequestQueueFilter{})
				require.NoError(t, err)
				require.Len(t, items, 2)
				for i, item := range items {
					old := fmt.Sprintf("%d:00", 12+i)
					if reads.childFailure != "" && item.Request.StudentID == reads.childID {
						old = ""
					}
					require.Len(t, item.Diff, 1)
					require.Equal(t, old, item.Diff[0].Old)
					require.Equal(t, "16:00", item.Diff[0].New)
				}
			})
		}
	}
}

func TestWeeklyPlanBatchReadsOnlyReviewableWeeklyChildren(t *testing.T) {
	t.Parallel()
	reads := &weeklyPlanReads{}
	s := weeklyPlanService(reads)
	ctx := context.Background()
	rows, err := reads.ListCareScheduleRequests(ctx, careplan.CareScheduleRequestFilter{})
	require.NoError(t, err)
	students, err := reads.FindStudents(ctx, nil)
	require.NoError(t, err)
	groupID := students[11].ID
	for i, id := range []int64{33, 44, 55, 66, 77, 99} {
		rows = append(rows, careplan.CareScheduleChangeRequest{StudentID: id, RequestKind: "weekly_schedule"})
		if id != 99 {
			students[id] = careplan.ReviewStudent{ID: id, GroupID: &groupID}
		}
		switch i {
		case 0:
			student := students[id]
			student.GroupID = nil
			students[id] = student
		case 1:
			student := students[id]
			student.Alumnus = true
			students[id] = student
		case 2:
			student := students[id]
			student.EnrolledUntil = "2026-11-04"
			students[id] = student
		case 3:
			rows[len(rows)-1].RequestKind = "pickup_change"
		case 4:
			rows[len(rows)-1].RequestKind = "pickup_permission"
		}
	}
	for _, id := range []int64{11, 22} {
		student := students[id]
		student.GroupID = &groupID
		students[id] = student
	}
	// Repeated requests reuse the same child's existing plan memo.
	rows = append(rows, rows[0])
	cache := s.preloadReviewPlans(ctx, rows, students, careplan.ReviewScope{GroupIDs: []int64{groupID}}, "2026-11-05")
	require.Len(t, cache.plans, 2)
	for _, stage := range []string{"pickups", "arrivals", "classes", "modes", "bookings", "offerings"} {
		require.Contains(t, reads.reads, stage+":[11 22]")
	}
	require.Len(t, reads.reads, 6)
}
