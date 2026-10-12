package common

import (
	"context"
	"net/http/httptest"
	"testing"

	configSvc "github.com/moto-nrw/project-phoenix/modules/settings"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type weekendSettingsStub struct {
	value bool
	keys  []string
}

func (s *weekendSettingsStub) ResolveBool(_ context.Context, key string) (bool, error) {
	s.keys = append(s.keys, key)
	return s.value, nil
}

// Every request carries the weekend plan (#3921): a weekend lookup reads the
// school's setting, a weekday lookup never does.
func TestWithWeekendPlanReadsTheSettingForWeekendsOnly(t *testing.T) {
	t.Parallel()

	settings := &weekendSettingsStub{value: true}
	ctx := WithWeekendPlan(httptest.NewRequest("GET", "/", nil), settings).Context()

	monday := calendar.NewDate(2026, 9, 7)
	if weekday, err := calendar.PlanWeekday(ctx, monday); err != nil || weekday != 1 || len(settings.keys) != 0 {
		t.Fatalf("Monday: weekday %d, err %v, reads %v", weekday, err, settings.keys)
	}
	weekday, err := calendar.PlanWeekday(ctx, monday.AddDays(5))
	if err != nil || weekday != 5 {
		t.Fatalf("Saturday: weekday %d, err %v", weekday, err)
	}
	if len(settings.keys) != 1 || settings.keys[0] != configSvc.KeyWeekendFollowsFriday {
		t.Fatalf("read keys %v", settings.keys)
	}
}
