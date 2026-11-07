package application

import (
	"context"
	"fmt"
	"strings"
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
	store      HolidayGuestBookings
	identities enrollment.HolidayStudentIdentityList
}

var _ enrollment.HolidayGuestBookedCare = HolidayGuestBookedCare{}

// NewHolidayGuestBookedCare builds the read of the booked holiday days; the
// identities find the school's own children.
func NewHolidayGuestBookedCare(store HolidayGuestBookings, identities enrollment.HolidayStudentIdentityList) HolidayGuestBookedCare {
	if store == nil || identities == nil {
		panic("enrollment: the holiday guest store and the student identities are required")
	}
	return HolidayGuestBookedCare{store: store, identities: identities}
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

// addOwnBookings finds the children of the school its own bookings name, by
// the rule of the participant list (#4722): the same names and the same date
// of birth. A booking that fits no child or several, among the children
// asked for or in the whole school, is left out; the Soll beneath it stays.
// The school-wide check keeps the answer independent of which children a
// report asks for.
func (c HolidayGuestBookedCare) addOwnBookings(ctx context.Context, result map[int64]map[calendar.Date]enrollment.HolidayGuestBookedDay,
	studentIDs []int64, own []enrollment.HolidayOwnBooking, from, to calendar.Date) error {
	identities, err := c.identities.HolidayStudentIdentities(ctx, studentIDs)
	if err != nil {
		return fmt.Errorf("enrollment: read the children of the school: %w", err)
	}
	byIdentity := map[holidayIdentityKey][]int64{}
	for studentID, identity := range identities {
		key := identityKey(identity)
		byIdentity[key] = append(byIdentity[key], studentID)
	}
	var fitting []enrollment.HolidayOwnBooking
	var candidates []int64
	for _, booking := range own {
		if matches := byIdentity[identityKey(booking.Identity)]; len(matches) == 1 {
			fitting, candidates = append(fitting, booking), append(candidates, matches[0])
		}
	}
	if len(fitting) == 0 {
		return nil
	}
	identityList := make([]enrollment.HolidayStudentIdentity, len(fitting))
	for i, booking := range fitting {
		identityList[i] = trimmedIdentity(booking.Identity)
	}
	unique, err := c.identities.UniqueHolidayStudents(ctx, identityList)
	if err != nil {
		return fmt.Errorf("enrollment: check the children of own holiday bookings: %w", err)
	}
	for i, booking := range fitting {
		if studentID, ok := unique[i]; !ok || studentID != candidates[i] {
			continue
		}
		day, err := guestBookedDay(enrollment.HolidayGuest{StudentID: candidates[i], TimeModelStarts: booking.StartsAt, TimeModelEnds: booking.EndsAt})
		if err != nil {
			return err
		}
		first, last := guestBookedSpan(enrollment.HolidayGuest{StartsOn: booking.StartsOn, EndsOn: booking.EndsOn}, from, to)
		addBookedDays(result, candidates[i], first, last, day)
	}
	return nil
}

type holidayIdentityKey struct {
	firstName, lastName string
	birthDate           calendar.Date
}

// identityKey compares names as the participant list does: case and
// surrounding spaces do not count (lower(btrim(...)), #4722).
func identityKey(identity enrollment.HolidayStudentIdentity) holidayIdentityKey {
	return holidayIdentityKey{
		firstName: strings.ToLower(strings.TrimSpace(identity.FirstName)),
		lastName:  strings.ToLower(strings.TrimSpace(identity.LastName)),
		birthDate: identity.BirthDate,
	}
}

func trimmedIdentity(identity enrollment.HolidayStudentIdentity) enrollment.HolidayStudentIdentity {
	identity.FirstName, identity.LastName = strings.TrimSpace(identity.FirstName), strings.TrimSpace(identity.LastName)
	return identity
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
