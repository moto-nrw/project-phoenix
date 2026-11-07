package application

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

type guestBookingsStub []enrollment.HolidayGuest

func (s guestBookingsStub) ListHolidayBookings(context.Context, []int64, calendar.Date, calendar.Date) ([]enrollment.HolidayGuest, []enrollment.HolidayOwnBooking, error) {
	return s, nil, nil
}

// identitiesStub is the whole school: every child born on a day asked for.
type identitiesStub map[int64]enrollment.HolidayStudentIdentity

func (s identitiesStub) HolidayStudentsBornOn(_ context.Context, births []calendar.Date) ([]enrollment.HolidayStudentCandidate, error) {
	var result []enrollment.HolidayStudentCandidate
	for id, identity := range s {
		for _, birth := range births {
			if identity.BirthDate == birth {
				result = append(result, enrollment.HolidayStudentCandidate{StudentID: id, Identity: identity})
			}
		}
	}
	return result, nil
}

func bookedDates(days map[calendar.Date]enrollment.HolidayGuestBookedDay) []calendar.Date {
	var dates []calendar.Date
	for date := calendar.NewDate(2026, 6, 1); date.Before(calendar.NewDate(2026, 8, 1)); date = date.AddDays(1) {
		if _, ok := days[date]; ok {
			dates = append(dates, date)
		}
	}
	return dates
}

// The booked days of a guest are calendar days of its week within the
// window; a cancellation ends them on the day it was recorded (#5994).
func TestHolidayGuestBookedDaysFollowTheWeekTheWindowAndTheCancellation(t *testing.T) {
	t.Parallel()
	mon, fri := calendar.NewDate(2026, 7, 6), calendar.NewDate(2026, 7, 10)
	wed := mon.AddDays(2)
	before := mon.AddDays(-3)
	care := NewHolidayGuestBookedCare(guestBookingsStub{
		{StudentID: 1, StartsOn: mon, EndsOn: fri, TimeModelStarts: "08:00", TimeModelEnds: "16:00"},
		{StudentID: 2, StartsOn: mon, EndsOn: fri, EndedOn: &wed, TimeModelStarts: "08:00", TimeModelEnds: "13:00"},
		{StudentID: 3, StartsOn: mon, EndsOn: fri, EndedOn: &before, TimeModelStarts: "08:00", TimeModelEnds: "13:00"},
	}, identitiesStub{})

	days, err := care.HolidayGuestBookedDays(context.Background(), []int64{1, 2, 3}, mon.AddDays(1), calendar.NewDate(2026, 7, 31))
	require.NoError(t, err)
	assert.Equal(t, []calendar.Date{mon.AddDays(1), wed, mon.AddDays(3), fri}, bookedDates(days[1]),
		"from the window's first day to the last day of the week")
	assert.Equal(t, "16:00", days[1][fri].EndsAt.Time().Format("15:04"))
	assert.Equal(t, []calendar.Date{mon.AddDays(1)}, bookedDates(days[2]), "cancelled on Wednesday: booked until Tuesday")
	assert.NotContains(t, days, int64(3), "cancelled before the week: never booked")
}

func TestHolidayGuestBookedDaysRefuseAnUnreadableTimeModel(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 6)
	care := NewHolidayGuestBookedCare(guestBookingsStub{
		{StudentID: 1, StartsOn: mon, EndsOn: mon, TimeModelStarts: "08:00", TimeModelEnds: "29:00"},
	}, identitiesStub{})
	_, err := care.HolidayGuestBookedDays(context.Background(), []int64{1}, mon, mon)
	require.Error(t, err, "an unreadable end is an error, never a day without a booking")
}

type ownBookingsStub []enrollment.HolidayOwnBooking

func (s ownBookingsStub) ListHolidayBookings(context.Context, []int64, calendar.Date, calendar.Date) ([]enrollment.HolidayGuest, []enrollment.HolidayOwnBooking, error) {
	return nil, s, nil
}

// A child of the school is found by the rule of the participant list
// (#4722): the same names, regardless of case and spaces, and the same date
// of birth. A booking that fits several children or none books no one.
func TestOwnHolidayBookingsFindTheirChildByNamesAndBirthDate(t *testing.T) {
	t.Parallel()
	mon, wed := calendar.NewDate(2026, 7, 6), calendar.NewDate(2026, 7, 8)
	born := calendar.NewDate(2018, 3, 4)
	booking := func(first, last string) enrollment.HolidayOwnBooking {
		return enrollment.HolidayOwnBooking{
			Identity: enrollment.HolidayStudentIdentity{FirstName: first, LastName: last, BirthDate: born},
			StartsOn: mon, EndsOn: wed, StartsAt: "08:00", EndsAt: "16:00",
		}
	}
	care := NewHolidayGuestBookedCare(ownBookingsStub{booking(" eva ", "MUSTER"), booking("Zwilling", "Doppelt"), booking("Niemand", "Hier")},
		identitiesStub{
			1: {FirstName: "Eva", LastName: "Muster", BirthDate: born},
			2: {FirstName: "Zwilling", LastName: "Doppelt", BirthDate: born},
			3: {FirstName: "Zwilling", LastName: "Doppelt", BirthDate: born},
			4: {FirstName: "Eva", LastName: "Muster", BirthDate: calendar.NewDate(2017, 3, 4)},
		})

	days, err := care.HolidayGuestBookedDays(context.Background(), []int64{1, 2, 3, 4}, mon, wed)
	require.NoError(t, err)
	assert.Equal(t, []calendar.Date{mon, mon.AddDays(1), wed}, bookedDates(days[1]), "Eva is found despite case and spaces")
	assert.Equal(t, "16:00", days[1][mon].EndsAt.Time().Format("15:04"))
	assert.NotContains(t, days, int64(2), "a booking that fits two children books neither")
	assert.NotContains(t, days, int64(3))
	assert.NotContains(t, days, int64(4), "another date of birth is another child")
}

// A report asks for a part of the school (a class, a group, one child). A
// booking that fits one child asked for but two children of the school
// books no one, so the answer does not depend on the filter (#5994).
func TestOwnHolidayBookingIsUniqueInTheWholeSchool(t *testing.T) {
	t.Parallel()
	mon := calendar.NewDate(2026, 7, 6)
	born := calendar.NewDate(2018, 3, 4)
	care := NewHolidayGuestBookedCare(ownBookingsStub{{
		Identity: enrollment.HolidayStudentIdentity{FirstName: "Zwilling", LastName: "Doppelt", BirthDate: born},
		StartsOn: mon, EndsOn: mon, StartsAt: "08:00", EndsAt: "16:00",
	}}, identitiesStub{
		2: {FirstName: "Zwilling", LastName: "Doppelt", BirthDate: born},
		5: {FirstName: "zwilling", LastName: "doppelt", BirthDate: born},
	})

	days, err := care.HolidayGuestBookedDays(context.Background(), []int64{2}, mon, mon)
	require.NoError(t, err)
	assert.Empty(t, days, "the view of one twin books neither, as the report of both does")
}
