package compose_test

import (
	"context"
	"testing"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/timetable"
	"github.com/moto-nrw/project-phoenix/modules/timetable/timetabletest"
	testpkg "github.com/moto-nrw/project-phoenix/test"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRecurrenceEventsPreserveCalendarContracts(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := timetabletest.New(t, db)
	ctx := testpkg.Ctx(t)
	cases := []struct {
		name       string
		rule       timetable.RecurrenceRuleInput
		start, end string
		want       []string
	}{
		{"daily interval", timetable.RecurrenceRuleInput{Frequency: "daily", IntervalCount: 2},
			"2027-01-01", "2027-01-06", []string{"2027-01-01", "2027-01-03", "2027-01-05"}},
		{"weekly interval", timetable.RecurrenceRuleInput{Frequency: "weekly", IntervalCount: 2, Weekdays: []string{"MON", "WED"}},
			"2027-01-04", "2027-01-20", []string{"2027-01-04", "2027-01-06", "2027-01-18", "2027-01-20"}},
		{"monthly clamping", timetable.RecurrenceRuleInput{Frequency: "monthly", IntervalCount: 1, MonthDays: []int{31}},
			"2027-01-01", "2027-03-31", []string{"2027-01-31", "2027-02-28", "2027-03-31"}},
		{"leap day", timetable.RecurrenceRuleInput{Frequency: "yearly", IntervalCount: 1},
			"2024-02-29", "2026-03-01", []string{"2024-02-29", "2025-02-28", "2026-02-28"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			rule, err := module.CreateRecurrenceRule(ctx, tc.rule)
			require.NoError(t, err)
			start, err := time.Parse(time.DateOnly, tc.start)
			require.NoError(t, err)
			end, err := time.Parse(time.DateOnly, tc.end)
			require.NoError(t, err)
			events, err := module.GenerateRecurrenceEvents(ctx, rule.ID, start, end)
			require.NoError(t, err)
			dates := make([]string, 0, len(events))
			for _, event := range events {
				dates = append(dates, event.Format(time.DateOnly))
			}
			assert.Equal(t, tc.want, dates)
		})
	}
}

func TestRecurrenceEventsRespectTenantAndReadFailures(t *testing.T) {
	t.Parallel()
	db := testpkg.SetupTestDB(t)
	module := timetabletest.New(t, db)
	ctx := testpkg.Ctx(t)
	rule, err := module.CreateRecurrenceRule(ctx, timetable.RecurrenceRuleInput{Frequency: "daily", IntervalCount: 1})
	require.NoError(t, err)
	start := time.Date(2027, time.January, 1, 0, 0, 0, 0, time.UTC)
	otherTenant, _ := testpkg.CreateTestTenant(t, db)
	events, err := module.GenerateRecurrenceEvents(testpkg.ContextForTenant(ctx, otherTenant), rule.ID, start, start)
	require.ErrorIs(t, err, timetable.ErrRecurrenceRuleNotFound)
	assert.Nil(t, events)
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	events, err = module.GenerateRecurrenceEvents(cancelled, rule.ID, start, start)
	require.ErrorIs(t, err, context.Canceled)
	assert.Nil(t, events)
	_, err = module.GenerateRecurrenceEvents(ctx, rule.ID, start.AddDate(0, 0, 1), start)
	require.ErrorIs(t, err, timetable.ErrInvalidRecurrenceRange)
}
