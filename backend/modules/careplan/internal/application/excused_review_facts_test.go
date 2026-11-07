package application

import (
	"context"
	"slices"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func absenceFactRows() []careplan.ExcusedAbsenceRequest {
	first := pendingRow(fakeStudentID, fakeTomorrow)
	second := pendingRow(otherStudentID, fakeNextWeek)
	second.StudentID = otherStudentID
	third := pendingRow(endedStudentID, fakeNextWeek)
	third.CreatedAt = first.CreatedAt.Add(time.Hour)
	return []careplan.ExcusedAbsenceRequest{first, second, third}
}

func absenceFactStudents() map[int64]careplan.ReviewStudent {
	return map[int64]careplan.ReviewStudent{
		fakeStudentID:  activeStudent(),
		otherStudentID: {ID: otherStudentID, PersonID: fakePersonID},
	}
}

func absenceFactPlan(rows []careplan.ExcusedAbsenceRequest) *fakeCarePlan {
	days := []careplan.StudentStatusDay{
		{StudentID: fakeStudentID, Date: fakeTomorrow, Status: careplan.StudentStatusDaySick, ReportedAt: rows[0].CreatedAt.Add(time.Minute)},
		{StudentID: fakeStudentID, Date: fakeNextWeek, Status: careplan.StudentStatusDayExcused, ReportedAt: rows[2].CreatedAt.Add(-time.Minute)},
		{StudentID: otherStudentID, Date: fakeNextWeek, Status: careplan.StudentStatusDayClassTrip, ReportedAt: rows[1].CreatedAt.Add(time.Minute)},
	}
	clock := time.Date(2000, time.January, 1, 12, 0, 0, 0, time.UTC)
	pickups := []careplan.PickupException{
		{StudentID: fakeStudentID, ExceptionDate: fakeTomorrow, ExcusedFrom: &clock, ExcusedAuto: true},
		{StudentID: otherStudentID, ExceptionDate: fakeNextWeek, ExcusedFrom: &clock},
		// A manual absence on another requested child's day must not leak.
		{StudentID: otherStudentID, ExceptionDate: fakeTomorrow, ExcusedFrom: &clock},
	}
	return &fakeCarePlan{
		listStatusDays: func(_ context.Context, f careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
			var out []careplan.StudentStatusDay
			for _, row := range days {
				if slices.Contains(f.StudentIDs, row.StudentID) && !row.Date.Before(f.From) && !row.Date.After(f.To) {
					out = append(out, row)
				}
			}
			return out, nil
		},
		listPickups: func(_ context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
			var out []careplan.PickupException
			for _, row := range pickups {
				if slices.Contains(f.StudentIDs, row.StudentID) && !row.ExceptionDate.Before(f.From) && !row.ExceptionDate.After(f.To) {
					out = append(out, row)
				}
			}
			return out, nil
		},
	}
}

func TestAbsenceReviewBatchPreservesPerRequestFacts(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	rows, students := absenceFactRows(), absenceFactStudents()
	plan := absenceFactPlan(rows)
	h := newHarness(t, plan, &fakeStudents{})
	var want []*careplan.ExcusedRequestReviewItem
	for i := range rows {
		item, err := h.workflow.pendingReviewItem(ctx, &rows[i], students[rows[i].StudentID], fakeToday, absenceReviewFacts{})
		require.NoError(t, err)
		want = append(want, item)
	}
	statusRead, pickupRead := plan.listStatusDays, plan.listPickups
	var statuses, pickups int
	plan.listStatusDays = func(ctx context.Context, f careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
		statuses++
		assert.ElementsMatch(t, []int64{fakeStudentID, otherStudentID}, f.StudentIDs)
		assert.True(t, f.ActiveOnly)
		assert.Equal(t, fakeTomorrow, f.From)
		assert.Equal(t, fakeNextWeek, f.To)
		return statusRead(ctx, f)
	}
	plan.listPickups = func(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
		pickups++
		assert.ElementsMatch(t, []int64{fakeStudentID, otherStudentID}, f.StudentIDs)
		return pickupRead(ctx, f)
	}
	got, err := h.workflow.pendingReviewItems(ctx, rows, students, nil, h.scope, fakeToday)
	require.NoError(t, err)
	require.Equal(t, want, got)
	assert.Equal(t, 1, statuses)
	assert.Equal(t, 1, pickups)
	assert.Equal(t, careplan.BulkIneligibleStale, got[0].BulkIneligibleReason)
	assert.Equal(t, careplan.BulkIneligibleConflict, got[1].BulkIneligibleReason)
	assert.True(t, got[2].BulkEligible)
	assert.False(t, *got[2].CurrentValueChanged)
}

func TestAbsenceReviewBatchFailuresRetryScalarReads(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"statuses", "pickups", "both"} {
		t.Run(failure, func(t *testing.T) {
			rows, students := absenceFactRows(), absenceFactStudents()
			plan := absenceFactPlan(rows)
			h := newHarness(t, plan, &fakeStudents{})
			want, err := h.workflow.pendingReviewItems(context.Background(), rows, students, nil, h.scope, fakeToday)
			require.NoError(t, err)
			statusRead, pickupRead := plan.listStatusDays, plan.listPickups
			var statusScalars, pickupScalars []int64
			plan.listStatusDays = func(ctx context.Context, f careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
				if failure != "pickups" {
					if len(f.StudentIDs) > 1 {
						return nil, errBoom
					}
					statusScalars = append(statusScalars, f.StudentIDs[0])
				}
				return statusRead(ctx, f)
			}
			plan.listPickups = func(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
				if failure != "statuses" {
					if len(f.StudentIDs) > 1 {
						return nil, errBoom
					}
					pickupScalars = append(pickupScalars, f.StudentIDs[0])
				}
				return pickupRead(ctx, f)
			}
			got, err := h.workflow.pendingReviewItems(context.Background(), rows, students, nil, h.scope, fakeToday)
			require.NoError(t, err)
			assert.Equal(t, want, got, "batch failures must not lose a successful neighbour's enrichment")
			if failure != "pickups" {
				assert.Equal(t, []int64{fakeStudentID, otherStudentID, fakeStudentID}, statusScalars)
			}
			if failure != "statuses" {
				assert.Equal(t, []int64{fakeStudentID, otherStudentID, fakeStudentID}, pickupScalars)
			}
		})
	}
}

func TestAbsenceReviewScalarFailureKeepsValidationOrder(t *testing.T) {
	t.Parallel()
	for _, failure := range []string{"statuses", "pickups"} {
		t.Run(failure, func(t *testing.T) {
			rows, students := absenceFactRows(), absenceFactStudents()
			plan := absenceFactPlan(rows)
			h := newHarness(t, plan, &fakeStudents{})
			statusRead, pickupRead := plan.listStatusDays, plan.listPickups
			plan.listStatusDays = func(ctx context.Context, f careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
				if failure == "statuses" && (len(f.StudentIDs) > 1 || f.StudentIDs[0] == otherStudentID) {
					return nil, errBoom
				}
				return statusRead(ctx, f)
			}
			plan.listPickups = func(ctx context.Context, f careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
				if failure == "pickups" && (len(f.StudentIDs) > 1 || f.StudentIDs[0] == otherStudentID) {
					return nil, errBoom
				}
				return pickupRead(ctx, f)
			}
			got, err := h.workflow.pendingReviewItems(context.Background(), rows, students, nil, h.scope, fakeToday)
			require.ErrorIs(t, err, errBoom)
			assert.Nil(t, got, "a scalar read error remains fatal to this queue")
			if failure == "statuses" {
				assert.EqualError(t, err, "care plan: resolve current absence statuses: boom")
			} else {
				assert.EqualError(t, err, "care plan: resolve absence bulk eligibility: boom")
			}
		})
	}
}

func TestAbsenceReviewHiddenChildrenAreNotBatchRead(t *testing.T) {
	t.Parallel()
	for _, hidden := range []string{"missing", "outside_scope", "alumnus", "ended"} {
		t.Run(hidden, func(t *testing.T) {
			rows, students := absenceFactRows(), absenceFactStudents()
			rows = rows[:1]
			student := students[fakeStudentID]
			scope := careplan.ReviewScope{SchoolWide: true}
			switch hidden {
			case "missing":
				delete(students, fakeStudentID)
			case "outside_scope":
				scope = careplan.ReviewScope{GroupIDs: []int64{fakeGroupID}}
			case "alumnus":
				student.Alumnus = true
			case "ended":
				student.EnrolledUntil = fakePast
			}
			if hidden != "missing" {
				students[fakeStudentID] = student
			}
			plan := &fakeCarePlan{
				listStatusDays: func(context.Context, careplan.StudentStatusDayFilter) ([]careplan.StudentStatusDay, error) {
					t.Fatal("hidden children must not be read for status enrichment")
					return nil, errBoom
				},
				listPickups: func(context.Context, careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
					t.Fatal("hidden children must not be read for partial absences")
					return nil, errBoom
				},
			}
			h := newHarness(t, plan, &fakeStudents{})
			got, err := h.workflow.pendingReviewItems(context.Background(), rows, students, nil, scope, fakeToday)
			require.NoError(t, err)
			assert.Empty(t, got)
		})
	}
}

func TestAbsenceReviewEligibilityShortCircuitsBeforeGuardianAccess(t *testing.T) {
	t.Parallel()
	rows, students := absenceFactRows(), absenceFactStudents()
	rows[0].Dates = []careplan.Date{fakePast}
	students[fakeStudentID] = careplan.ReviewStudent{ID: fakeStudentID, EnrolledUntil: fakeTomorrow}
	plan := absenceFactPlan(rows)
	pickupRead := plan.listPickups
	plan.listPickups = func(ctx context.Context, filter careplan.StudentScheduleFilter) ([]careplan.PickupException, error) {
		assert.Equal(t, []int64{otherStudentID}, filter.StudentIDs,
			"past dates and dates beyond care end must not reach the partial-absence read")
		return pickupRead(ctx, filter)
	}
	h := newHarness(t, plan, &fakeStudents{})
	h.messenger.accessErr = errBoom
	got, err := h.workflow.pendingReviewItems(context.Background(), rows, students, nil, h.scope, fakeToday)
	require.NoError(t, err, "all three requests stop before guardian access, despite the newer status")
	require.Len(t, got, len(rows))
	assert.Equal(t, careplan.BulkIneligiblePast, got[0].BulkIneligibleReason)
	assert.Equal(t, careplan.BulkIneligibleConflict, got[1].BulkIneligibleReason)
	assert.Equal(t, careplan.BulkIneligibleChildUnavailable, got[2].BulkIneligibleReason)
}
