package application

import (
	"context"
	"fmt"
	"time"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// HolidayGuestBookings lists the confirmed holiday bookings of the school in
// the caller's tenant transaction that overlap [from, to]: the guests among
// studentIDs and the bookings of its own children.
type HolidayGuestBookings interface {
	ListHolidayBookings(ctx context.Context, studentIDs []int64, from, to calendar.Date) ([]enrollment.HolidayGuest, []enrollment.HolidayOwnBooking, error)
}

// HolidayGuestBookedCare serves enrollment.HolidayGuestBookedCare (#5994).
type HolidayGuestBookedCare struct {
	store    HolidayGuestBookings
	children enrollment.HolidayStudentsByBirthDate
}

var _ enrollment.HolidayGuestBookedCare = HolidayGuestBookedCare{}

// NewHolidayGuestBookedCare builds the read of the booked holiday days;
// children finds the school's own children of a booking.
func NewHolidayGuestBookedCare(store HolidayGuestBookings, children enrollment.HolidayStudentsByBirthDate) HolidayGuestBookedCare {
	if store == nil || children == nil {
		panic("enrollment: the holiday guest store and the school's children are required")
	}
	return HolidayGuestBookedCare{store: store, children: children}
}

// HolidayGuestBookedDays lists the booked days of each guest within
// [from, to]: every day of its week, the last one the day before a
// cancellation ended it. A time model whose clock cannot be read is an
// error, never a day without a booking.
func (c HolidayGuestBookedCare) HolidayGuestBookedDays(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay, error) {
	result := map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay{}
	if len(studentIDs) == 0 || to.Before(from) {
		return result, nil
	}
	guests, own, err := c.store.ListHolidayBookings(ctx, studentIDs, from, to)
	if err != nil {
		return nil, err
	}
	for _, guest := range guests {
		day, err := guestBookedDay(guest)
		if err != nil {
			return nil, err
		}
		first, last := guestBookedSpan(guest, from, to)
		addBookedDays(result, guest.StudentID, first, last, day)
	}
	if len(own) == 0 {
		return result, nil
	}
	if err := c.addOwnBookings(ctx, result, studentIDs, own, from, to); err != nil {
		return nil, err
	}
	return result, nil
}

// addOwnBookings finds the child of each day of an own booking by the rule
// of the participant list (#4722): the same names (Unicode NFC, without
// regard to case and surrounding spaces) and the same date of birth, among
// the children that belonged to the school on that day. The children come
// in one read for all bookings, from the whole school, so the answer does
// not depend on which children a report asks for. A day the booking fits no
// child on keeps the Soll beneath it; a day it fits two or more children on
// books none of them and is marked ambiguous, so it is not compared.
func (c HolidayGuestBookedCare) addOwnBookings(ctx context.Context, result map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay,
	studentIDs []int64, own []enrollment.HolidayOwnBooking, from, to calendar.Date) error {
	candidates, err := c.children.HolidayStudentsBornOn(ctx, birthDatesOf(own))
	if err != nil {
		return fmt.Errorf("enrollment: read the children of the school: %w", err)
	}
	byIdentity := map[holidayIdentityKey][]enrollment.HolidayStudentCandidate{}
	for _, candidate := range candidates {
		key := identityKey(candidate.Identity)
		byIdentity[key] = append(byIdentity[key], candidate)
	}
	asked := make(map[int64]bool, len(studentIDs))
	for _, id := range studentIDs {
		asked[id] = true
	}
	for _, booking := range own {
		day, err := guestBookedDay(enrollment.HolidayGuest{TimeModelStarts: booking.StartsAt, TimeModelEnds: booking.EndsAt})
		if err != nil {
			return err
		}
		first, last := guestBookedSpan(enrollment.HolidayGuest{StartsOn: booking.StartsOn, EndsOn: booking.EndsOn}, from, to)
		bookOwnDays(result, asked, byIdentity[identityKey(booking.Identity)], first, last, day)
	}
	return nil
}

// birthDatesOf lists each date of birth of the bookings once.
func birthDatesOf(own []enrollment.HolidayOwnBooking) []calendar.Date {
	births := make([]calendar.Date, 0, len(own))
	seen := map[calendar.Date]bool{}
	for _, booking := range own {
		if !seen[booking.Identity.BirthDate] {
			seen[booking.Identity.BirthDate] = true
			births = append(births, booking.Identity.BirthDate)
		}
	}
	return births
}

// bookOwnDays books each day from first to last for the one child among
// matches that belonged to the school on it; with two or more members the
// day is ambiguous for each of them.
func bookOwnDays(result map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay, asked map[int64]bool,
	matches []enrollment.HolidayStudentCandidate, first, last calendar.Date, day enrollment.HolidayGuestBookedDay) {
	for date := first; first != "" && !date.After(last); date = date.AddDays(1) {
		members := membersOn(matches, date)
		booked := day
		booked.Ambiguous = len(members) > 1
		for _, studentID := range members {
			if asked[studentID] {
				addBookedDays(result, studentID, date, date, booked)
			}
		}
	}
}

// membersOn are the candidates that belonged to the school on day.
func membersOn(candidates []enrollment.HolidayStudentCandidate, day calendar.Date) []int64 {
	var members []int64
	for _, candidate := range candidates {
		if (candidate.EnrolledFrom == nil || !day.Before(*candidate.EnrolledFrom)) &&
			(candidate.EnrolledUntil == nil || !day.After(*candidate.EnrolledUntil)) {
			members = append(members, candidate.StudentID)
		}
	}
	return members
}

type holidayIdentityKey struct {
	firstName, lastName string
	birthDate           calendar.Date
}

// identityKey compares names as the participant list does (#4722).
func identityKey(identity enrollment.HolidayStudentIdentity) holidayIdentityKey {
	return holidayIdentityKey{
		firstName: childNameKey(identity.FirstName),
		lastName:  childNameKey(identity.LastName),
		birthDate: identity.BirthDate,
	}
}

func addBookedDays(result map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay, studentID int64, first, last calendar.Date, day enrollment.HolidayGuestBookedDay) {
	if first.After(last) || first == "" {
		return
	}
	if result[studentID] == nil {
		result[studentID] = map[calendar.Date]enrollment.HolidayGuestBookedDay{}
	}
	for date := first; !date.After(last); date = date.AddDays(1) {
		result[studentID][date] = day
	}
}

// guestBookedSpan is the part of the guest's week within [from, to] its
// booking still held: a cancellation ends it on ended_on.
func guestBookedSpan(guest enrollment.HolidayGuest, from, to calendar.Date) (calendar.Date, calendar.Date) {
	first, last := guest.StartsOn, guest.EndsOn
	if guest.EndedOn != nil && !guest.EndedOn.After(last) {
		last = guest.EndedOn.AddDays(-1)
	}
	if first.Before(from) {
		first = from
	}
	if last.After(to) {
		last = to
	}
	return first, last
}

func guestBookedDay(guest enrollment.HolidayGuest) (enrollment.HolidayGuestBookedDay, error) {
	starts, err := timeModelClock(guest.TimeModelStarts)
	if err != nil {
		return enrollment.HolidayGuestBookedDay{}, fmt.Errorf("enrollment: start of the time model of guest %d: %w", guest.StudentID, err)
	}
	ends, err := timeModelClock(guest.TimeModelEnds)
	if err != nil {
		return enrollment.HolidayGuestBookedDay{}, fmt.Errorf("enrollment: end of the time model of guest %d: %w", guest.StudentID, err)
	}
	return enrollment.HolidayGuestBookedDay{StartsAt: starts, EndsAt: ends}, nil
}

// timeModelClock reads a clock time "15:04" of a time model.
func timeModelClock(value string) (calendar.WallClock, error) {
	parsed, err := time.Parse("15:04", value)
	if err != nil {
		return calendar.WallClock{}, err
	}
	return calendar.NewWallClock(parsed.Hour(), parsed.Minute(), 0, 0)
}
