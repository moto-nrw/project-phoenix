package calendar

import (
	"context"
	"errors"
	"testing"
	"time"
)

// 2026-09-11 is a Friday.
var (
	planFriday   = NewDate(2026, time.September, 11)
	planSaturday = NewDate(2026, time.September, 12)
	planSunday   = NewDate(2026, time.September, 13)
	planMonday   = NewDate(2026, time.September, 14)
)

func countingSource(value bool, err error, calls *int) WeekendPlanSource {
	return func(context.Context) (bool, error) {
		*calls++
		return value, err
	}
}

func TestPlanWeekdayNeverResolvesTheSettingOnAWeekday(t *testing.T) {
	t.Parallel()
	calls := 0
	ctx := WithWeekendPlan(context.Background(), countingSource(true, nil, &calls))
	for want, day := range map[int]Date{1: planMonday, 5: planFriday} {
		got, err := PlanWeekday(ctx, day)
		if err != nil || got != want {
			t.Fatalf("PlanWeekday(%s) = %d, %v; want %d", day, got, err, want)
		}
		care, err := IsCareWeekday(ctx, day)
		if err != nil || !care {
			t.Fatalf("IsCareWeekday(%s) = %v, %v; want true", day, care, err)
		}
	}
	if calls != 0 {
		t.Fatalf("weekday lookups resolved the setting %d times", calls)
	}
}

func TestPlanWeekdayOnTheWeekend(t *testing.T) {
	t.Parallel()
	for _, tc := range []struct {
		name    string
		ctx     context.Context
		weekday int
		care    bool
	}{
		{"no source keeps the weekend", context.Background(), 6, false},
		{"setting off keeps the weekend", WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return false, nil }), 6, false},
		{"setting on follows Friday", WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return true, nil }), 5, true},
	} {
		got, err := PlanWeekday(tc.ctx, planSaturday)
		if err != nil || got != tc.weekday {
			t.Fatalf("%s: PlanWeekday = %d, %v; want %d", tc.name, got, err, tc.weekday)
		}
		care, err := IsCareWeekday(tc.ctx, planSaturday)
		if err != nil || care != tc.care {
			t.Fatalf("%s: IsCareWeekday = %v, %v; want %v", tc.name, care, err, tc.care)
		}
	}
	if got := ISOWeekday(planSunday); got != 7 {
		t.Fatalf("ISOWeekday(Sunday) = %d; want 7", got)
	}
}

func TestPlanWeekdayReturnsAResolutionError(t *testing.T) {
	t.Parallel()
	boom := errors.New("settings unavailable")
	ctx := WithWeekendPlan(context.Background(), func(context.Context) (bool, error) { return true, boom })
	weekday, err := PlanWeekday(ctx, planSunday)
	if !errors.Is(err, boom) || weekday != 7 {
		t.Fatalf("PlanWeekday = %d, %v; want the error and the own weekday", weekday, err)
	}
	if _, err := IsCareWeekday(ctx, planSunday); !errors.Is(err, boom) {
		t.Fatalf("IsCareWeekday error = %v; want the resolution error", err)
	}
}
