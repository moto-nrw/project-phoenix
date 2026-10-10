package care

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/internal/timezone"
	"github.com/moto-nrw/project-phoenix/modules/careplan"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// weekdayArrivals answers the weekly arrival of one weekday and records the
// weekdays it was asked for.
type weekdayArrivals struct {
	careplan.ArrivalScheduleService
	weekday int
	asked   []int
}

func (w *weekdayArrivals) GetStudentArrivalExceptionForDate(context.Context, int64, calendar.Date) (*careplan.ArrivalException, error) {
	return nil, nil
}

func (w *weekdayArrivals) GetStudentArrivalScheduleForWeekday(_ context.Context, studentID int64, weekday int) (*careplan.ArrivalSchedule, error) {
	w.asked = append(w.asked, weekday)
	if weekday != w.weekday {
		return nil, nil
	}
	return &careplan.ArrivalSchedule{StudentID: studentID, Weekday: weekday, ExpectedArrival: time.Date(0, 1, 1, 11, 45, 0, 0, time.UTC)}, nil
}

// Parents see a weekend that follows Friday's plan (#3921) as Friday's care
// day; without the setting the weekend is never one and the plan is not asked.
func TestResolveExpectedArrivalOnAFollowingWeekend(t *testing.T) {
	t.Parallel()

	saturday := timezone.NewDate(2026, time.September, 12)
	for _, follows := range []bool{true, false} {
		arrivals := &weekdayArrivals{weekday: 5}
		svc := &Service{ArrivalSchedules: arrivals}
		ctx := calendar.WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return follows, nil })
		got, err := svc.resolveExpectedArrival(ctx, 7, saturday)
		if err != nil {
			t.Fatalf("follows=%v: %v", follows, err)
		}
		if !got.resolved || got.isCareDay != follows {
			t.Fatalf("follows=%v: got %+v", follows, got)
		}
		if follows && (got.hhmm != "11:45" || len(arrivals.asked) != 1 || arrivals.asked[0] != 5) {
			t.Fatalf("Friday's plan expected, got %+v, asked %v", got, arrivals.asked)
		}
		if !follows && len(arrivals.asked) != 0 {
			t.Fatalf("a plain weekend must not ask the weekly plan, asked %v", arrivals.asked)
		}
	}
}
