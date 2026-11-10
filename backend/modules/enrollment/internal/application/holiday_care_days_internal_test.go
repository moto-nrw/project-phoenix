package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Invariant (#4702): a day that is a statutory holiday or a closing day of the
// host school is never bookable. These tests turn red without the checks in
// CreateHolidayPeriod, UpdateHolidayPeriod and the booking paths.

const (
	careDayHostSchool  int64 = 41
	careDayOtherSchool int64 = 42
)

// whitMonday is a statutory holiday in every federal state.
var whitMonday = enrollment.HolidayNoCareDay{Date: calendar.NewDate(2027, 5, 17), Kind: enrollment.HolidayNoCareHoliday, Name: "Pfingstmontag"}

// careDaySchoolCalendar answers fixed days without care per school.
type careDaySchoolCalendar map[int64][]enrollment.HolidayNoCareDay

func (c careDaySchoolCalendar) HolidayNoCareDays(_ context.Context, schoolIDs []int64, from, to calendar.Date) (map[int64][]enrollment.HolidayNoCareDay, error) {
	result := map[int64][]enrollment.HolidayNoCareDay{}
	for _, id := range schoolIDs {
		for _, day := range c[id] {
			if !day.Date.Before(from) && !day.Date.After(to) {
				result[id] = append(result[id], day)
			}
		}
	}
	return result, nil
}

// careDayPeriodStore records writes; a refused period writes nothing.
type careDayPeriodStore struct {
	HolidayPeriodStore
	stored  enrollment.HolidayPeriod
	written int
}

func (s *careDayPeriodStore) FindHolidayPeriod(context.Context, int64, int64, bool) (enrollment.HolidayPeriod, bool, error) {
	return s.stored, true, nil
}

func (s *careDayPeriodStore) InsertHolidayPeriod(context.Context, int64, enrollment.HolidayPeriodInput) (int64, error) {
	s.written++
	return s.stored.ID, nil
}

func (s *careDayPeriodStore) UpdateHolidayPeriod(context.Context, int64, int64, enrollment.HolidayPeriodInput, calendar.Date) error {
	s.written++
	return nil
}

func (s *careDayPeriodStore) HolidayWeeksBookedTogether(context.Context, int64, int64) ([][2]int64, error) {
	return nil, nil
}

type careDayFormStore struct{ HolidayFormStore }

func (careDayFormStore) InsertHolidayPeriodForm(context.Context, int64, int64, int64, []enrollment.HolidayFormField, []enrollment.FormLegalBlock, enrollment.HolidayAddressRequirement) (enrollment.HolidayPeriodForm, error) {
	return enrollment.HolidayPeriodForm{}, nil
}

func careDayWeek(id int64, startsOn calendar.Date, school int64) enrollment.HolidayWeekInput {
	return enrollment.HolidayWeekInput{ID: id, StartsOn: startsOn, EndsOn: startsOn.AddDays(4), HostSchoolID: school,
		TimeModels: []enrollment.HolidayTimeModelInput{{Name: "bis 13 Uhr", StartsAt: "08:00", EndsAt: "13:00", PriceCents: 4500}}}
}

func careDayPeriod(weeks ...enrollment.HolidayWeekInput) enrollment.HolidayPeriodInput {
	return enrollment.HolidayPeriodInput{Name: "Pfingstferien", RegistrationOpensOn: calendar.NewDate(2027, 3, 1),
		RegistrationClosesOn: calendar.NewDate(2027, 4, 30), Weeks: weeks}
}

func newCareDayPeriods(store *careDayPeriodStore, schools enrollment.HolidaySchoolCalendar) *HolidayPeriods {
	return NewHolidayPeriods(store, careDayFormStore{}, func() calendar.Date { return calendar.NewDate(2027, 2, 1) }, schools)
}

func TestHolidayWeekOnAHolidayOfItsHostSchoolIsNeverWritten(t *testing.T) {
	t.Parallel()
	store := &careDayPeriodStore{stored: enrollment.HolidayPeriod{ID: 9}}
	periods := newCareDayPeriods(store, careDaySchoolCalendar{careDayHostSchool: {whitMonday}})

	_, err := periods.CreateHolidayPeriod(context.Background(), 7, careDayPeriod(
		careDayWeek(0, calendar.NewDate(2027, 5, 24), careDayHostSchool),
		careDayWeek(0, calendar.NewDate(2027, 5, 17), careDayHostSchool)))
	var refusal *enrollment.HolidayWeekNoCareDayRefusal
	require.ErrorAs(t, err, &refusal)
	require.ErrorIs(t, err, enrollment.ErrHolidayWeekNoCareDay)
	assert.Equal(t, 1, refusal.Week, "the second week holds Whit Monday")
	assert.Equal(t, whitMonday, refusal.Day)
	assert.Zero(t, store.written, "a refused period writes nothing")

	_, err = periods.CreateHolidayPeriod(context.Background(), 7, careDayPeriod(careDayWeek(0, calendar.NewDate(2027, 5, 18), careDayHostSchool)))
	require.NoError(t, err, "the week after the holiday is bookable")
}

func TestHolidayWeekChecksTheDaysOfItsOwnHostSchool(t *testing.T) {
	t.Parallel()
	closing := enrollment.HolidayNoCareDay{Date: calendar.NewDate(2027, 5, 26), Kind: enrollment.HolidayNoCareClosingDay, Name: "Betriebsausflug"}
	store := &careDayPeriodStore{stored: enrollment.HolidayPeriod{ID: 9}}
	periods := newCareDayPeriods(store, careDaySchoolCalendar{careDayOtherSchool: {closing}})

	_, err := periods.CreateHolidayPeriod(context.Background(), 7, careDayPeriod(careDayWeek(0, calendar.NewDate(2027, 5, 24), careDayHostSchool)))
	require.NoError(t, err, "a closing day of another school does not block the week")
	_, err = periods.CreateHolidayPeriod(context.Background(), 7, careDayPeriod(careDayWeek(0, calendar.NewDate(2027, 5, 24), careDayOtherSchool)))
	var refusal *enrollment.HolidayWeekNoCareDayRefusal
	require.ErrorAs(t, err, &refusal, "the closing day of the host school blocks it")
	assert.Equal(t, closing, refusal.Day)
}

// A stored week left as it is stays when a closing day is planned later: its
// bookings stay, only a new booking is refused. Moving it onto the closing
// day, or giving it another host school, is checked again.
func TestHolidayPeriodUpdateChecksOnlyAddedOrMovedWeeks(t *testing.T) {
	t.Parallel()
	monday := calendar.NewDate(2027, 5, 17)
	stored := enrollment.HolidayPeriod{ID: 9, Status: enrollment.HolidayPeriodDraft, Weeks: []enrollment.HolidayWeek{{
		ID: 3, StartsOn: monday, EndsOn: monday.AddDays(4), HostSchoolID: careDayHostSchool,
		TimeModels: []enrollment.HolidayTimeModel{{ID: 5, Name: "bis 13 Uhr", StartsAt: "08:00", EndsAt: "13:00", PriceCents: 4500}},
	}}}
	store := &careDayPeriodStore{stored: stored}
	periods := newCareDayPeriods(store, careDaySchoolCalendar{careDayHostSchool: {whitMonday}})
	kept := careDayWeek(3, monday, careDayHostSchool)
	kept.TimeModels[0].ID = 5

	_, err := periods.UpdateHolidayPeriod(context.Background(), 7, 9, careDayPeriod(kept))
	require.NoError(t, err, "the unchanged week stays writable")
	moved := kept
	moved.HostSchoolID = careDayOtherSchool
	_, err = periods.UpdateHolidayPeriod(context.Background(), 7, 9, careDayPeriod(moved))
	require.NoError(t, err, "another host school without the holiday is fine")

	periods = newCareDayPeriods(store, careDaySchoolCalendar{careDayHostSchool: {whitMonday}, careDayOtherSchool: {whitMonday}})
	_, err = periods.UpdateHolidayPeriod(context.Background(), 7, 9, careDayPeriod(moved))
	require.ErrorIs(t, err, enrollment.ErrHolidayWeekNoCareDay, "a new host school with the holiday is refused")
}

func TestHolidayWeekIsNotWrittenWithoutTheHostSchoolsCalendar(t *testing.T) {
	t.Parallel()
	store := &careDayPeriodStore{stored: enrollment.HolidayPeriod{ID: 9}}
	periods := newCareDayPeriods(store, nil)
	_, err := periods.CreateHolidayPeriod(context.Background(), 7, careDayPeriod(careDayWeek(0, calendar.NewDate(2027, 5, 24), careDayHostSchool)))
	require.ErrorIs(t, err, enrollment.ErrHolidaySchoolCalendarUnavailable, "fail closed")
	assert.Zero(t, store.written)

	registrations := &HolidayRegistrations{}
	err = registrations.refuseWeeksWithoutCare(context.Background(), enrollment.HolidayPeriod{Weeks: []enrollment.HolidayWeek{{ID: 3, StartsOn: whitMonday.Date, EndsOn: whitMonday.Date, HostSchoolID: careDayHostSchool}}}, nil)
	require.ErrorIs(t, err, enrollment.ErrHolidaySchoolCalendarUnavailable, "no registration without the calendar")
}

func TestHolidayBookingOfAWeekWithoutCareIsRefusedUnlessKept(t *testing.T) {
	t.Parallel()
	period := enrollment.HolidayPeriod{Weeks: []enrollment.HolidayWeek{
		{ID: 3, StartsOn: whitMonday.Date, EndsOn: whitMonday.Date.AddDays(4), HostSchoolID: careDayHostSchool, NoCareDays: []enrollment.HolidayNoCareDay{whitMonday}},
		{ID: 4, StartsOn: whitMonday.Date.AddDays(7), EndsOn: whitMonday.Date.AddDays(11), HostSchoolID: careDayHostSchool, NoCareDays: []enrollment.HolidayNoCareDay{}},
	}}
	booking := []enrollment.HolidayBookingInput{{WeekID: 4, TimeModelID: 1}, {WeekID: 3, TimeModelID: 1}}

	var refusal *enrollment.HolidayWeekNoCareDayRefusal
	require.ErrorAs(t, refuseHolidayBookingsWithoutCare(booking, period, nil), &refusal)
	assert.Equal(t, int64(3), refusal.WeekID)
	assert.Equal(t, 1, refusal.Week)
	require.NoError(t, refuseHolidayBookingsWithoutCare(booking, period, []enrollment.HolidayBookingInput{{WeekID: 3, TimeModelID: 2}}),
		"a week the child had booked before the day became a closing day stays")
}

func TestHolidayOfferMarksTheDaysWithoutCare(t *testing.T) {
	t.Parallel()
	period := enrollment.HolidayPeriod{Weeks: []enrollment.HolidayWeek{
		{ID: 3, StartsOn: whitMonday.Date, EndsOn: whitMonday.Date.AddDays(4), HostSchoolID: careDayHostSchool},
		{ID: 4, StartsOn: whitMonday.Date, EndsOn: whitMonday.Date.AddDays(4), HostSchoolID: careDayOtherSchool},
	}}
	marked, err := withHolidayNoCareDays(context.Background(), careDaySchoolCalendar{careDayHostSchool: {whitMonday}}, period)
	require.NoError(t, err)
	assert.Equal(t, []enrollment.HolidayNoCareDay{whitMonday}, marked.Weeks[0].NoCareDays)
	assert.Empty(t, marked.Weeks[1].NoCareDays, "the parallel week at the other school is bookable")
	assert.Nil(t, period.Weeks[0].NoCareDays, "the read period is not changed in place")
}
