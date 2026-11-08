package application

import (
	"context"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/mealplan"
	"github.com/moto-nrw/project-phoenix/modules/mealplan/internal/domain"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// releaseStore is the store of the release guards: what is released, the
// Träger of the school and what the release inserted.
type releaseStore struct {
	released     map[calendar.Date]domain.StoredBillingMonth
	organization int64
	inserted     []domain.StoredBillingMonth
}

func (s *releaseStore) ReleasedMonths(context.Context, calendar.Date, calendar.Date) ([]domain.StoredBillingMonth, error) {
	months := []domain.StoredBillingMonth{}
	for _, month := range s.released {
		months = append(months, month)
	}
	return months, nil
}

func (s *releaseStore) ReleasedMonth(_ context.Context, month calendar.Date) (domain.StoredBillingMonth, bool, error) {
	stored, ok := s.released[month]
	return stored, ok, nil
}

func (s *releaseStore) SchoolOrganization(context.Context) (int64, bool, error) {
	return s.organization, s.organization > 0, nil
}

func (s *releaseStore) InsertRelease(_ context.Context, organizationID, _ int64, month domain.StoredBillingMonth) (time.Time, error) {
	if organizationID != s.organization {
		panic("the release names another Träger")
	}
	s.inserted = append(s.inserted, month)
	return time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC), nil
}

func (s *releaseStore) TraegerMonths(context.Context, int64, calendar.Date) ([]domain.ReleasedMealRow, error) {
	return nil, nil
}

// releaseMonths is March 2026 with Mia registered on Monday 2 March and
// present: one child, one meal. Today is 2 April 2026.
func releaseMonths(store *releaseStore) *BillingMonths {
	mia := mealplan.DailyParticipant{StudentID: 1, FirstName: "Mia", LastName: "Albers"}
	weeks := func(_ context.Context, monday mealplan.Date) (mealplan.WeeklyList, error) {
		if monday == "2026-03-02" {
			return mealplan.WeeklyList{WeekStart: monday, Days: []mealplan.DailyList{{Date: "2026-03-02", Participants: []mealplan.DailyParticipant{mia}}}}, nil
		}
		return mealplan.WeeklyList{WeekStart: monday}, nil
	}
	noHolidays := func(context.Context, string, string) (map[string]bool, []mealplan.HolidayPeriod, error) {
		return nil, nil, nil
	}
	present := func(context.Context, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error) {
		return map[calendar.Date]map[int64]bool{"2026-03-02": {1: true}}, nil
	}
	return NewBillingMonths(store, weeks, noHolidays, present,
		func(context.Context) (calendar.Date, error) { return "2025-04-02", nil },
		func() calendar.Date { return calendar.NewDate(2026, 4, 2) })
}

func TestReleaseBillingMonthGuards(t *testing.T) {
	t.Parallel()
	release := func(month string, children, meals int) mealplan.BillingMonthRelease {
		return mealplan.BillingMonthRelease{Month: mealplan.Date(month), AccountID: 42, ChildCount: children, MealCount: meals}
	}
	cases := []struct {
		name    string
		store   *releaseStore
		release mealplan.BillingMonthRelease
		want    error
	}{
		{"the month has not ended", &releaseStore{organization: 7}, release("2026-04-01", 0, 0), mealplan.ErrBillingMonthRunning},
		{"the counts the OGS-Leitung saw changed", &releaseStore{organization: 7}, release("2026-03-01", 1, 2), mealplan.ErrBillingMonthChanged},
		{"a month is released once", &releaseStore{organization: 7, released: map[calendar.Date]domain.StoredBillingMonth{
			"2026-03-01": {Month: "2026-03-01"}}}, release("2026-03-01", 1, 1), mealplan.ErrBillingMonthReleased},
		{"a school without a Träger bills no Mensa", &releaseStore{}, release("2026-03-01", 1, 1), mealplan.ErrBillingMonthNoTraeger},
		{"a month outside the window", &releaseStore{organization: 7}, release("2025-03-01", 0, 0), mealplan.ErrInvalidBillingMonth},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			_, err := releaseMonths(tc.store).ReleaseBillingMonth(context.Background(), tc.release)
			assert.ErrorIs(t, err, tc.want)
			assert.Empty(t, tc.store.inserted, "a refused release stores nothing")
		})
	}
}

func TestReleaseBillingMonthStoresWhatTheLeitungSaw(t *testing.T) {
	t.Parallel()
	store := &releaseStore{organization: 7}
	month, err := releaseMonths(store).ReleaseBillingMonth(context.Background(),
		mealplan.BillingMonthRelease{Month: "2026-03-01", AccountID: 42, ChildCount: 1, MealCount: 1})
	require.NoError(t, err)
	require.Len(t, store.inserted, 1)
	assert.Equal(t, []domain.BillingChild{{StudentID: 1, FirstName: "Mia", LastName: "Albers", RegisteredDays: 1, MealDays: 1}}, store.inserted[0].Children)
	assert.Equal(t, mealplan.BillingMonthReleased, month.Status)
	assert.Equal(t, 1, month.MealCount)
	assert.Empty(t, month.Children[0].AbsentDates, "a released month shows no absence dates")
}

func TestBillingMonthWithExpiredAttendanceCannotBeReleased(t *testing.T) {
	t.Parallel()
	store := &releaseStore{organization: 7}
	months := releaseMonths(store)
	months.availableFrom = func(context.Context) (calendar.Date, error) {
		return "2026-03-03", nil
	}
	months.presence = func(context.Context, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error) {
		return map[calendar.Date]map[int64]bool{}, nil
	}
	_, err := months.ReleaseBillingMonth(context.Background(),
		mealplan.BillingMonthRelease{Month: "2026-03-01", AccountID: 42, ChildCount: 1, MealCount: 0})
	require.ErrorIs(t, err, mealplan.ErrBillingMonthAttendanceExpired)
	assert.Empty(t, store.inserted, "expired attendance must never become immutable billing counts")

	store.released = map[calendar.Date]domain.StoredBillingMonth{"2026-03-01": {Month: "2026-03-01", MealCount: 1}}
	stored, err := months.BillingMonth(context.Background(), "2026-03-01")
	require.NoError(t, err, "retention does not hide a release that already stored the counts")
	assert.Equal(t, mealplan.BillingMonthReleased, stored.Status)
	assert.Equal(t, 1, stored.MealCount)
}
