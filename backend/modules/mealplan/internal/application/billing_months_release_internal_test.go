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
	month.ReleasedAt = time.Date(2026, 4, 2, 9, 0, 0, 0, time.UTC)
	entry := domain.ReleaseVersion{Version: month.Version, ReleasedAt: month.ReleasedAt, Reason: month.Reason,
		ChildCount: month.ChildCount, MealCount: month.MealCount}
	if earlier, ok := s.released[month.Month]; ok {
		month.Versions = append([]domain.ReleaseVersion{entry}, earlier.Versions...)
	} else {
		month.Versions = []domain.ReleaseVersion{entry}
	}
	if s.released == nil {
		s.released = map[calendar.Date]domain.StoredBillingMonth{}
	}
	s.released[month.Month] = month
	return month.ReleasedAt, nil
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
		{"a released month is released again only with a reason", &releaseStore{organization: 7, released: map[calendar.Date]domain.StoredBillingMonth{
			"2026-03-01": {Month: "2026-03-01"}}}, release("2026-03-01", 1, 1), mealplan.ErrBillingMonthReasonRequired},
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
	months := releaseMonths(store)
	seen, err := months.BillingMonth(context.Background(), "2026-03-01")
	require.NoError(t, err)
	require.NotEmpty(t, seen.Fingerprint)
	month, err := months.ReleaseBillingMonth(context.Background(),
		mealplan.BillingMonthRelease{Month: "2026-03-01", AccountID: 42, ChildCount: 1, MealCount: 1, Fingerprint: seen.Fingerprint})
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

// A month that ended before the attendance window is no open month: the
// list and the month name it "expired" without counting it, so nobody waits
// for a release that cannot come (#4686).
func TestBillingMonthBeforeTheAttendanceWindowShowsAsExpired(t *testing.T) {
	t.Parallel()
	months := releaseMonths(&releaseStore{organization: 7})
	months.availableFrom = func(context.Context) (calendar.Date, error) { return "2026-03-03", nil }
	months.presence = func(context.Context, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error) {
		panic("an expired month reads no attendance")
	}

	overview, err := months.BillingMonthOverview(context.Background())
	require.NoError(t, err)
	statuses := map[mealplan.Date]mealplan.BillingMonthStatus{}
	for _, summary := range overview {
		statuses[summary.Month] = summary.Status
	}
	assert.Equal(t, mealplan.BillingMonthRunning, statuses["2026-04-01"])
	assert.Equal(t, mealplan.BillingMonthExpired, statuses["2026-03-01"], "March starts before the window")
	assert.Equal(t, mealplan.BillingMonthExpired, statuses["2026-02-01"])

	month, err := months.BillingMonth(context.Background(), "2026-03-01")
	require.NoError(t, err)
	assert.Equal(t, mealplan.BillingMonthExpired, month.Status)
	assert.Empty(t, month.Children)
	assert.Zero(t, month.MealCount)
}

// A correction that moves a meal from one child to another keeps both
// totals; the fingerprint of what the OGS-Leitung saw still refuses the
// release, so no child is billed with counts nobody looked at (#4686).
func TestReleaseBillingMonthRefusesAMealMovedBetweenChildren(t *testing.T) {
	t.Parallel()
	mia := mealplan.DailyParticipant{StudentID: 1, FirstName: "Mia", LastName: "Albers"}
	ben := mealplan.DailyParticipant{StudentID: 2, FirstName: "Ben", LastName: "Brandt"}
	present := map[calendar.Date]map[int64]bool{"2026-03-02": {1: true}}
	store := &releaseStore{organization: 7}
	months := releaseMonths(store)
	months.weeks = func(_ context.Context, monday mealplan.Date) (mealplan.WeeklyList, error) {
		if monday == "2026-03-02" {
			return mealplan.WeeklyList{WeekStart: monday, Days: []mealplan.DailyList{
				{Date: "2026-03-02", Participants: []mealplan.DailyParticipant{mia, ben}}}}, nil
		}
		return mealplan.WeeklyList{WeekStart: monday}, nil
	}
	months.presence = func(context.Context, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error) {
		return present, nil
	}
	seen, err := months.BillingMonth(context.Background(), "2026-03-01")
	require.NoError(t, err)

	present["2026-03-02"] = map[int64]bool{2: true}
	_, err = months.ReleaseBillingMonth(context.Background(), mealplan.BillingMonthRelease{Month: "2026-03-01", AccountID: 42,
		ChildCount: seen.ChildCount, MealCount: seen.MealCount, Fingerprint: seen.Fingerprint})
	require.ErrorIs(t, err, mealplan.ErrBillingMonthChanged)
	assert.Empty(t, store.inserted, "a refused release stores nothing")
}

// releaseAgainMonths is March 2026 with Mia registered on 2 and 3 March;
// present marks the days with her attendance on file.
func releaseAgainMonths(store *releaseStore, present map[calendar.Date]map[int64]bool) *BillingMonths {
	months := releaseMonths(store)
	mia := mealplan.DailyParticipant{StudentID: 1, FirstName: "Mia", LastName: "Albers"}
	months.weeks = func(_ context.Context, monday mealplan.Date) (mealplan.WeeklyList, error) {
		if monday == "2026-03-02" {
			return mealplan.WeeklyList{WeekStart: monday, Days: []mealplan.DailyList{
				{Date: "2026-03-02", Participants: []mealplan.DailyParticipant{mia}},
				{Date: "2026-03-03", Participants: []mealplan.DailyParticipant{mia}}}}, nil
		}
		return mealplan.WeeklyList{WeekStart: monday}, nil
	}
	months.presence = func(context.Context, calendar.Date, calendar.Date) (map[calendar.Date]map[int64]bool, error) {
		return present, nil
	}
	return months
}

// A corrected attendance after the release (#4686): the month shows what
// it counts now, and the OGS-Leitung releases it again with a reason as a
// new version; the first version stays.
func TestReleaseBillingMonthAgainAddsAVersionWithItsReason(t *testing.T) {
	t.Parallel()
	ctx := context.Background()
	store := &releaseStore{organization: 7}
	present := map[calendar.Date]map[int64]bool{"2026-03-02": {1: true}}
	months := releaseAgainMonths(store, present)
	first, err := months.BillingMonth(ctx, "2026-03-01")
	require.NoError(t, err)
	_, err = months.ReleaseBillingMonth(ctx, mealplan.BillingMonthRelease{Month: "2026-03-01", AccountID: 42,
		ChildCount: first.ChildCount, MealCount: first.MealCount, Fingerprint: first.Fingerprint})
	require.NoError(t, err)

	released, err := months.BillingMonth(ctx, "2026-03-01")
	require.NoError(t, err)
	assert.Nil(t, released.Current, "nothing changed since the release")

	present["2026-03-03"] = map[int64]bool{1: true}
	changed, err := months.BillingMonth(ctx, "2026-03-01")
	require.NoError(t, err)
	require.NotNil(t, changed.Current, "the corrected attendance counts one meal more")
	assert.Equal(t, 1, changed.MealCount, "the release keeps its count")
	assert.Equal(t, 2, changed.Current.MealCount)

	again := mealplan.BillingMonthRelease{Month: "2026-03-01", AccountID: 42, ChildCount: changed.Current.ChildCount,
		MealCount: changed.Current.MealCount, Fingerprint: changed.Current.Fingerprint}
	_, err = months.ReleaseBillingMonth(ctx, again)
	require.ErrorIs(t, err, mealplan.ErrBillingMonthReasonRequired)
	again.Reason = "Anwesenheit am 3. März nachgetragen"
	result, err := months.ReleaseBillingMonth(ctx, again)
	require.NoError(t, err)
	assert.Equal(t, 2, result.Version)
	assert.Equal(t, 2, result.MealCount)
	require.Len(t, result.Releases, 2)
	assert.Equal(t, "Anwesenheit am 3. März nachgetragen", result.Releases[0].Reason)
	assert.Equal(t, 1, result.Releases[1].MealCount, "the first version stays with its count")
	require.Len(t, store.inserted, 2)

	_, err = months.ReleaseBillingMonth(ctx, again)
	assert.ErrorIs(t, err, mealplan.ErrBillingMonthReleased, "nothing new to release")
}
