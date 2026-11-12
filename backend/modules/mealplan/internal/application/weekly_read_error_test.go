package application

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/moto-ogs/project-phoenix/modules/mealplan"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-ogs/project-phoenix/modules/mealplan/internal/ports"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type weeklyReadSettings struct{ ports.Settings }

func (weeklyReadSettings) MealPlanEnabled(context.Context) (bool, error)         { return true, nil }
func (weeklyReadSettings) MealRegistrationEnabled(context.Context) (bool, error) { return true, nil }
func (weeklyReadSettings) MealOrderingSettings(context.Context) (ports.OrderingSettings, error) {
	return ports.OrderingSettings{Model: "ongoing_single", RegistrationKind: "days_before", RegistrationTime: "09:00", CancellationKind: "days_before", CancellationTime: "09:00", AnnualChanges: "parents_cancel", SickAfterDeadline: "stays_ordered", OrderRhythm: "daily", OrderDetail: "by_class"}, nil
}

// A weekly read stops at Monday's roster failure. A later day's calendar
// decode failure is not reachable, even though the week API supports it.
// This uses ordinary consumer-port errors, not ACL/RLS fault injection.
func TestWeeklyListStopsBeforeLaterCalendarFailure(t *testing.T) {
	t.Parallel()
	monday := domain.Date("2026-09-07")
	rosterErr := errors.New("roster unavailable")
	calendarErr := errors.New("invalid later holiday period")
	var calendarReads [][2]string
	var rosterReads []string
	directory := NewParticipantDirectory(func(_ context.Context, date string) ([]mealplan.ParticipantCandidate, error) {
		rosterReads = append(rosterReads, date)
		return nil, rosterErr
	}, func(context.Context, []int64) ([]mealplan.ParticipantCandidate, error) {
		t.Fatal("names are not reached")
		return nil, nil
	}, unavailableParticipantDays)
	calendar := NewSchoolCalendar(func(_ context.Context, from, to string) (map[string]bool, []mealplan.HolidayPeriod, error) {
		calendarReads = append(calendarReads, [2]string{from, to})
		if to > monday.String() {
			return nil, nil, calendarErr
		}
		return map[string]bool{}, nil, nil
	})
	service := New(seamStore{}, directory, calendar, weeklyReadSettings{}, seamTransaction{}, func(ports.Observation) {}, func() time.Time { return monday.BerlinMidnight() })
	_, err := service.WeeklyList(context.Background(), mealplan.Date(monday.String()))
	require.ErrorIs(t, err, rosterErr)
	assert.NotErrorIs(t, err, calendarErr)
	assert.Equal(t, [][2]string{{monday.String(), monday.String()}}, calendarReads)
	assert.Equal(t, []string{monday.String()}, rosterReads)
}

type weeklyReadStore struct{ seamStore }

func (weeklyReadStore) FindDailyParticipation(context.Context, []int64, domain.Date, time.Time) (map[int64]domain.DailyParticipation, domain.OperationStats, error) {
	return map[int64]domain.DailyParticipation{}, domain.OperationStats{}, nil
}

func TestWeeklyListRangeFailureKeepsEarlierRosterFailure(t *testing.T) {
	t.Parallel()
	monday := domain.Date("2026-09-07")
	rosterErr := errors.New("Tuesday roster unavailable")
	calendarErr := errors.New("invalid later holiday period")
	var rosterReads []string
	directory := NewParticipantDirectory(func(_ context.Context, date string) ([]mealplan.ParticipantCandidate, error) {
		rosterReads = append(rosterReads, date)
		if date != monday.String() {
			return nil, rosterErr
		}
		return nil, nil
	}, func(context.Context, []int64) ([]mealplan.ParticipantCandidate, error) {
		t.Fatal("names are not reached")
		return nil, nil
	}, unavailableParticipantDays)
	calendar := NewSchoolCalendar(func(_ context.Context, from, to string) (map[string]bool, []mealplan.HolidayPeriod, error) {
		if from != to {
			return nil, nil, calendarErr
		}
		return map[string]bool{}, nil, nil
	})
	service := New(weeklyReadStore{}, directory, calendar, weeklyReadSettings{}, seamTransaction{}, func(ports.Observation) {}, func() time.Time { return monday.BerlinMidnight() })
	_, err := service.WeeklyList(context.Background(), mealplan.Date(monday.String()))

	require.ErrorIs(t, err, rosterErr)
	assert.NotErrorIs(t, err, calendarErr)
	assert.Equal(t, []string{monday.String(), monday.AddDays(1).String()}, rosterReads)
}
