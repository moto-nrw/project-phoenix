package application

import (
	"context"
	"errors"
	"fmt"

	"github.com/moto-nrw/project-phoenix/modules/enrollment"
	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Children of the hosting school itself in the weeks it hosts (#6678, ADR
// 0120 addendum). They stay in their own group; this file finds the student
// behind each confirmed registration, plans its booked days in Care Plan and
// shows the team what the registration says. Nothing is copied: every read
// goes to the Träger's registration through the host functions, which answer
// until the last day of the week.

// holidayOwnChildCleanupDays is how far back the reconciliation still
// removes planned days: a week that ended, or a cancellation, clears its
// plan within these days. Older days stay untouched; they have no effect.
const holidayOwnChildCleanupDays = 14

// HolidayOwnChildStore reads the school's own children of its hosted weeks
// in the tenant transaction of the hosting school.
type HolidayOwnChildStore interface {
	// ListOwnChildren reads the confirmed children of the school itself in
	// every hosted week of a published period that ends on today or later.
	ListOwnChildren(ctx context.Context, schoolID int64, today calendar.Date) ([]enrollment.HolidayOwnChildBooking, error)
}

// ownChildMatcher finds the student of the school a registration names, by
// the rule of the participant list (#4722) and of Soll und Ist (#5994): the
// same names (Unicode NFC, without regard to case and surrounding spaces)
// and the same date of birth, among the children that belong to the school
// on the booked days.
type ownChildMatcher struct {
	byIdentity map[holidayIdentityKey][]enrollment.HolidayStudentCandidate
}

// newOwnChildMatcher reads the candidates of all identities in one read.
func newOwnChildMatcher(ctx context.Context, children enrollment.HolidayStudentsByBirthDate, identities []enrollment.HolidayStudentIdentity) (ownChildMatcher, error) {
	matcher := ownChildMatcher{byIdentity: map[holidayIdentityKey][]enrollment.HolidayStudentCandidate{}}
	births := make([]calendar.Date, 0, len(identities))
	seen := map[calendar.Date]bool{}
	for _, identity := range identities {
		if identity.BirthDate != "" && !seen[identity.BirthDate] {
			seen[identity.BirthDate] = true
			births = append(births, identity.BirthDate)
		}
	}
	if len(births) == 0 {
		return matcher, nil
	}
	candidates, err := children.HolidayStudentsBornOn(ctx, births)
	if err != nil {
		return ownChildMatcher{}, fmt.Errorf("enrollment: read the children of the school: %w", err)
	}
	for _, candidate := range candidates {
		key := identityKey(candidate.Identity)
		matcher.byIdentity[key] = append(matcher.byIdentity[key], candidate)
	}
	return matcher, nil
}

// match names the one student the registration fits on its booked days and
// the days it belongs to the school on. A registration that fits no student
// or several is assigned to none: it must not plan a day or show a contact
// at the wrong child.
func (m ownChildMatcher) match(identity enrollment.HolidayStudentIdentity, days []calendar.Date) (studentID int64, assignment string, memberDays []calendar.Date) {
	candidates := m.byIdentity[identityKey(identity)]
	students := map[int64]bool{}
	for _, day := range days {
		members := membersOn(candidates, day)
		for _, member := range members {
			students[member] = true
		}
		if len(members) == 1 {
			memberDays = append(memberDays, day)
		}
	}
	switch len(students) {
	case 0:
		return 0, enrollment.HolidayOwnChildNotFound, nil
	case 1:
		for student := range students {
			studentID = student
		}
		return studentID, enrollment.HolidayOwnChildMatched, memberDays
	}
	return 0, enrollment.HolidayOwnChildAmbiguous, nil
}

// bookedDays lists the selected dates, or every day of the booked week.
func bookedDays(days []calendar.Date, startsOn, endsOn calendar.Date) []calendar.Date {
	if len(days) > 0 {
		return days
	}
	var week []calendar.Date
	for day := startsOn; !day.After(endsOn); day = day.AddDays(1) {
		week = append(week, day)
	}
	return week
}

// ownChildMatch is a booking with the student it was assigned to; studentID
// is zero unless assignment is HolidayOwnChildMatched.
type ownChildMatch struct {
	enrollment.HolidayOwnChildBooking
	assignment string
	studentID  int64
	days       []calendar.Date
}

// matchedOwnChildren reads the school's own children of its open weeks and
// keeps those assigned to exactly one student.
func (h *HolidayGuests) matchedOwnChildren(ctx context.Context, schoolID int64, today calendar.Date) ([]ownChildMatch, error) {
	assigned, err := h.assignedOwnChildren(ctx, schoolID, today)
	if err != nil {
		return nil, err
	}
	matches := make([]ownChildMatch, 0, len(assigned))
	for _, match := range assigned {
		if match.assignment == enrollment.HolidayOwnChildMatched {
			matches = append(matches, match)
		}
	}
	return matches, nil
}

// assignedOwnChildren reads the school's own children of its open weeks and
// says for each which student its registration fits.
func (h *HolidayGuests) assignedOwnChildren(ctx context.Context, schoolID int64, today calendar.Date) ([]ownChildMatch, error) {
	bookings, err := h.store.ListOwnChildren(ctx, schoolID, today)
	if err != nil || len(bookings) == 0 {
		return nil, err
	}
	identities := make([]enrollment.HolidayStudentIdentity, len(bookings))
	for i, booking := range bookings {
		identities[i] = booking.Identity
	}
	matcher, err := newOwnChildMatcher(ctx, h.children, identities)
	if err != nil {
		return nil, err
	}
	matches := make([]ownChildMatch, 0, len(bookings))
	for _, booking := range bookings {
		studentID, assignment, days := matcher.match(booking.Identity, bookedDays(booking.Days, booking.Week.StartsOn, booking.Week.EndsOn))
		matches = append(matches, ownChildMatch{HolidayOwnChildBooking: booking, assignment: assignment, studentID: studentID, days: days})
	}
	return matches, nil
}

// reconcileOwnChildren brings the planned days of the school's own children
// in line with their confirmed bookings and returns how many children each
// week plans. The booking is the author of a planned day, so the plan needs
// neither a holiday group nor a staff member.
func (h *HolidayGuests) reconcileOwnChildren(ctx context.Context, schoolID int64, today calendar.Date) (map[int64]int, error) {
	matches, err := h.matchedOwnChildren(ctx, schoolID, today)
	if err != nil {
		return nil, err
	}
	planned := map[int64]int{}
	var wanted []enrollment.HolidayOwnChildDay
	for _, match := range matches {
		planned[match.Week.ID]++
		for _, day := range match.days {
			wanted = append(wanted, enrollment.HolidayOwnChildDay{
				StudentID: match.studentID, Day: day, Arrival: match.TimeModelStarts, Pickup: match.TimeModelEnds,
			})
		}
	}
	if err := h.ownCare.SyncHolidayOwnChildDays(ctx, today.AddDays(-holidayOwnChildCleanupDays), today, wanted); err != nil {
		return nil, fmt.Errorf("plan the care days of the school's own holiday children: %w", err)
	}
	return planned, nil
}

// HolidayStudentRegistrations returns what the confirmed registrations say
// about a child of the hosting school for the weeks it hosts, and records
// the read in the same transaction when there is any.
func (h *HolidayGuests) HolidayStudentRegistrations(ctx context.Context, host enrollment.HolidayHost, studentID int64) ([]enrollment.HolidayStudentRegistration, error) {
	if host.SchoolID <= 0 || host.AccountID <= 0 {
		return nil, errors.New("enrollment: the hosting school and the account are required")
	}
	entries := []enrollment.HolidayStudentRegistration{}
	if studentID <= 0 {
		return entries, nil
	}
	matches, err := h.matchedOwnChildren(ctx, host.SchoolID, h.today())
	if err != nil {
		return nil, err
	}
	access := enrollment.HolidayGuestAccess{
		Kind: enrollment.HolidayGuestAccessOwnChild, ActorAccountID: host.AccountID, ActorRole: host.Role, StudentID: studentID,
	}
	for _, match := range matches {
		if match.studentID != studentID {
			continue
		}
		entries = append(entries, ownChildRegistration(match))
		access.WeekIDs = append(access.WeekIDs, match.Week.ID)
		if access.StartsOn == "" || match.Week.StartsOn.Before(access.StartsOn) {
			access.StartsOn = match.Week.StartsOn
		}
		if access.EndsOn == "" || match.Week.EndsOn.After(access.EndsOn) {
			access.EndsOn = match.Week.EndsOn
		}
	}
	if len(entries) == 0 {
		return entries, nil
	}
	access.WeekID = access.WeekIDs[0]
	if err := h.log.RecordHolidayGuestAccess(ctx, access); err != nil {
		return nil, fmt.Errorf("record holiday student registration read: %w", err)
	}
	return entries, nil
}

func ownChildRegistration(match ownChildMatch) enrollment.HolidayStudentRegistration {
	entry := enrollment.HolidayStudentRegistration{
		WeekID: match.Week.ID, PeriodName: match.Week.PeriodName, StartsOn: match.Week.StartsOn, EndsOn: match.Week.EndsOn,
		Days: match.Days, TimeModelName: match.TimeModelName, TimeModelStarts: match.TimeModelStarts, TimeModelEnds: match.TimeModelEnds,
		Emergency: match.Emergency, Pickup: match.Pickup,
	}
	if entry.Days == nil {
		entry.Days = []calendar.Date{}
	}
	if entry.Emergency == nil {
		entry.Emergency = []enrollment.HolidayGuestContact{}
	}
	if entry.Pickup == nil {
		entry.Pickup = []enrollment.HolidayGuestContact{}
	}
	return entry
}

// HolidayOwnChildAssignments lists how each child of the school itself in
// one hosted week was assigned to a student (#6678).
func (h *HolidayGuests) HolidayOwnChildAssignments(ctx context.Context, host enrollment.HolidayHost, weekID int64) ([]enrollment.HolidayOwnChildAssignment, error) {
	if host.SchoolID <= 0 {
		return nil, errors.New("enrollment: the hosting school is required")
	}
	assignments := []enrollment.HolidayOwnChildAssignment{}
	if weekID <= 0 {
		return assignments, nil
	}
	assigned, err := h.assignedOwnChildren(ctx, host.SchoolID, h.today())
	if err != nil {
		return nil, err
	}
	for _, match := range assigned {
		if match.Week.ID != weekID {
			continue
		}
		assignment := enrollment.HolidayOwnChildAssignment{ChildID: match.ChildID, Assignment: match.assignment}
		if match.assignment == enrollment.HolidayOwnChildMatched {
			studentID := match.studentID
			assignment.StudentID = &studentID
		}
		assignments = append(assignments, assignment)
	}
	return assignments, nil
}
