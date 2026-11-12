package enrollment

import (
	"context"
	"errors"

	"github.com/moto-ogs/project-phoenix/sharedkernel/calendar"
)

// Holiday guests in the daily care of the hosting school (spec #4611 user
// stories 43 and 44, #4666, ADR 0120). A holiday guest is a child of another
// school of the Träger, or a child without a school of the Träger, whose
// booking for a week the school hosts is confirmed. The school picks one of
// its groups for the week; Enrollment then copies each confirmed guest into
// the school as a child whose care covers exactly that week, in the group the
// school picked, with the booked time model (Betreuung), the emergency
// contacts (Notfall) and the persons allowed to collect it (Abholung). The
// school's groups, its check-in and the kiosk work with the copy unchanged;
// from the day after the week the copy is no longer cared for and leaves the
// group and the check-in by itself. A cancellation or rebooking ends the copy
// at the next reconciliation. The contacts and the shared answers stay
// readable for seven days after the week and are deleted then. Every operation runs in the tenant
// transaction of the hosting school, which the caller opens.

// ErrHolidayGuestNotFound reports a child that is no holiday guest of the
// school, or one whose access has ended.
var ErrHolidayGuestNotFound = errors.New("holiday guest not found")

// ErrHolidayGuestGroupInvalid reports a group that is not one of the
// hosting school's.
var ErrHolidayGuestGroupInvalid = errors.New("holiday guest group is not a group of the school")

// HolidayGuestReadOnlyDays is how long the sheet of a guest stays readable
// after the last day of its week (ADR 0084 section 6).
const HolidayGuestReadOnlyDays = 7

// HolidayGuestContact is an emergency contact or a person allowed to collect
// a guest.
type HolidayGuestContact struct {
	Name         string `json:"name"`
	Phone        string `json:"phone"`
	Relationship string `json:"relationship"`
}

// HolidayGuestSheet is everything the hosting school sees of a guest beyond
// its name: the care it booked, its emergency contacts and who may collect
// it, and the answers the Träger shares (#5202). A nil HomeSchoolID marks a
// child without a school of the Träger; GuestSchoolName is the school its
// parents named, if any. SchoolName is the school the hint „Gast aus …“
// names (#5630).
type HolidayGuestSheet struct {
	StudentID       int64                 `json:"student_id"`
	WeekID          int64                 `json:"week_id"`
	StartsOn        calendar.Date         `json:"starts_on"`
	EndsOn          calendar.Date         `json:"ends_on"`
	TimeModelName   string                `json:"time_model_name"`
	TimeModelStarts string                `json:"time_model_starts_at"`
	TimeModelEnds   string                `json:"time_model_ends_at"`
	HomeSchoolID    *int64                `json:"home_school_id"`
	GuestSchoolName string                `json:"guest_school_name"`
	SchoolName      string                `json:"school_name"`
	Emergency       []HolidayGuestContact `json:"emergency_contacts"`
	Pickup          []HolidayGuestContact `json:"pickup_contacts"`
	Answers         []HolidayHostAnswer   `json:"answers"`
	// Health is the health information the Träger shares with the hosting
	// school (#4722); nil without any.
	Health *HolidayHostHealth `json:"health"`
	// ReadOnly is true after the last day of the week: the guest is no
	// longer cared for and the sheet stays only for questions afterwards.
	ReadOnly bool `json:"read_only"`
}

// HolidayGuestSchoolName is the school the hint on a guest names (#5630),
// by the rule of the participant list: the home school of the Träger, or the
// school the parents named for a child without one. Empty means the hint
// says „Feriengast“: a child without any school, or a home school that no
// longer exists.
func HolidayGuestSchoolName(homeSchoolID *int64, homeSchoolName, guestSchoolName string) string {
	if homeSchoolID != nil {
		return homeSchoolName
	}
	return guestSchoolName
}

// HolidayGuestBadge is the hint on a guest in the school's lists (#5630):
// the child and the school „Gast aus …“ names, empty for „Feriengast“.
type HolidayGuestBadge struct {
	StudentID  int64  `json:"student_id"`
	SchoolName string `json:"school_name"`
}

// HolidayGuestTransfer is the result of picking the group of a hosted week:
// the group and how many guests of the week it holds now.
type HolidayGuestTransfer struct {
	WeekID  int64 `json:"week_id"`
	GroupID int64 `json:"group_id"`
	Guests  int   `json:"guests"`
	// OwnChildren counts the school's own children of the week whose booked
	// days are planned now (#6678).
	OwnChildren int `json:"own_children"`
}

// HolidayGuestChild is the child People Directory keeps for a guest: its
// names and date of birth, the group, and the care interval. Before its week
// the child is Pending and in no group, like a newly admitted child before
// its first care day, so neither the group nor the check-in shows it yet.
type HolidayGuestChild struct {
	FirstName     string
	LastName      string
	BirthDate     calendar.Date
	GroupID       *int64
	EnrolledFrom  calendar.Date
	EnrolledUntil calendar.Date
	Pending       bool
}

// HolidayGuestPeople creates, reads and updates the child behind a guest in
// People Directory, in the tenant transaction of the hosting school. The
// root binds it to the People Directory's enrollment commands.
type HolidayGuestPeople interface {
	CreateHolidayGuestChild(ctx context.Context, child HolidayGuestChild) (studentID int64, err error)
	ReadHolidayGuestChild(ctx context.Context, studentID int64) (HolidayGuestChild, error)
	UpdateHolidayGuestChild(ctx context.Context, studentID int64, child HolidayGuestChild) error
	// SetHolidayGuestHealthInfo writes the health information of a guest
	// into the health field of its child, so the team sees it where it sees
	// every child (#4722, ADR 0120 addendum); nil clears it when the guest's
	// health information goes.
	SetHolidayGuestHealthInfo(ctx context.Context, studentID int64, healthInfo *string) error
}

// HolidayGuestCareDays plans the booked days of a guest in Care Plan, so the
// group and the check-in show it as expected on those days and no other:
// on each day of the week it arrives at the start and is collected at the
// end of its time model (clock times "15:04"). The booking is the author of
// the plan, not the account that picked the group (ADR 0120, addendum of
// 2026-11-11). ClearHolidayGuestDays removes the plan of a guest that ended.
// UnplannedHolidayGuests returns those of the given children that have no
// planned day at all, so the reconciliation plans them. The root binds Care
// Plan's dated exceptions.
type HolidayGuestCareDays interface {
	PlanHolidayGuestDays(ctx context.Context, studentID int64, days []calendar.Date, arrival, pickup string) error
	ClearHolidayGuestDays(ctx context.Context, studentID int64) error
	UnplannedHolidayGuests(ctx context.Context, studentIDs []int64) ([]int64, error)
}

// Kinds of a holiday guest access in the access log of the hosting school.
const (
	HolidayGuestAccessTransfer = "transfer"
	HolidayGuestAccessSheet    = "sheet"
	// HolidayGuestAccessOwnChild is a read of what the registrations say
	// about a child of the hosting school itself (#6678).
	HolidayGuestAccessOwnChild = "own_child"
	// HolidayGuestAccessHomeSchool is a read of the bookings of the
	// school's own children at other schools of its Träger (#6679); Guests
	// counts the children, WeekIDs names their weeks.
	HolidayGuestAccessHomeSchool = "home_school"
)

// HolidayGuestAccess is one access to holiday guest data for the access log
// of the hosting school: a transfer of the guests of a week into the school,
// or a read of the sheet of one guest. Automatic is true for a transfer the
// hourly reconciliation made under the group choice of ActorAccountID.
type HolidayGuestAccess struct {
	Kind           string
	ActorAccountID int64
	ActorRole      string
	WeekID         int64
	StudentID      int64
	StartsOn       calendar.Date
	EndsOn         calendar.Date
	Guests         int
	Automatic      bool
	// WeekIDs names the weeks an own-child or home-school read covered.
	WeekIDs []int64
}

// HolidayGuestAccessLog writes an access to holiday guest data into the
// access log of the hosting school, in the caller's tenant transaction. The
// root binds it to the Audit Platform.
type HolidayGuestAccessLog interface {
	RecordHolidayGuestAccess(ctx context.Context, access HolidayGuestAccess) error
}

// HolidayGuests serves the holiday guests to the hosting school. Every
// operation runs in the tenant transaction of the school.
type HolidayGuests interface {
	// AssignHolidayGuestGroup picks the group of a hosted week and copies
	// the week's confirmed guests into it at once. ErrHostedHolidayWeekNotFound
	// for a week the school does not host or that has ended.
	AssignHolidayGuestGroup(ctx context.Context, host HolidayHost, weekID, groupID int64) (HolidayGuestTransfer, error)
	// ReconcileHolidayGuests brings the guests of every hosted week with a
	// group in line with the confirmed bookings: new guests are copied,
	// cancelled ones end, and contacts and shared answers past the
	// read-only days go.
	ReconcileHolidayGuests(ctx context.Context, schoolID int64) error
	// HolidayGuestSheet returns the sheet of a guest and records the read;
	// ErrHolidayGuestNotFound for a child that is no guest or whose access
	// has ended.
	HolidayGuestSheet(ctx context.Context, host HolidayHost, studentID int64) (HolidayGuestSheet, error)
	HolidayGuestBadgeList
	HolidayOwnChildren
}

// HolidayGuestBadgeList lists the hints „Gast aus …“ of a hosting school,
// in the tenant transaction of the school. The school portal and the
// tablet (PyrePortal#499) read it. It also lists the Feriengruppen, which
// the school's group lists explain or keep back before their week.
type HolidayGuestBadgeList interface {
	HolidayGuestGroupList
	// HolidayGuestBadges lists the hint of every guest the school cares for
	// today, from the first to the last day of its week, so group,
	// check-in, child list, supervision and the tablet name its home school
	// (#5630).
	HolidayGuestBadges(ctx context.Context, schoolID int64) ([]HolidayGuestBadge, error)
}

// HolidayGuestBookedDay is the care the confirmed booking of a holiday
// guest promises on one day (#5994): it comes at the start and is collected
// at the end of its time model, as Berlin wall clocks.
type HolidayGuestBookedDay struct {
	StartsAt calendar.WallClock
	EndsAt   calendar.WallClock
	// Ambiguous marks a day an own booking fits two or more children of the
	// school on: no child is booked, and the day is not compared (#5994).
	Ambiguous bool
}

// HolidayOwnBooking is a confirmed holiday booking of a child of the school
// in a week the school hosts (#5994): the child as the registration names it,
// the week and the booked time model ("15:04").
type HolidayOwnBooking struct {
	Identity         HolidayStudentIdentity
	StartsOn, EndsOn calendar.Date
	StartsAt, EndsAt string
}

// HolidayStudentCandidate is a child of the school with the names and the
// date of birth a registration finds it by and the interval it belonged to
// the school (#5994); a nil end is an open membership.
type HolidayStudentCandidate struct {
	StudentID     int64
	Identity      HolidayStudentIdentity
	EnrolledFrom  *calendar.Date
	EnrolledUntil *calendar.Date
}

// HolidayStudentsByBirthDate reads, in the school's tenant transaction and
// in one read, every child of the school born on one of birthDates, also
// one whose care has ended (#5994): a match is unique among the children
// that belonged to the school on the booked day.
type HolidayStudentsByBirthDate interface {
	HolidayStudentsBornOn(ctx context.Context, birthDates []calendar.Date) ([]HolidayStudentCandidate, error)
}

// HolidayGuestBookedCare reads the booked days of the confirmed holiday
// bookings at the school whose tenant transaction the caller holds (#5994):
// Soll und Ist compares them with the attendance. A holiday guest's days
// run from the first day of its week to the last one, and stop on the day
// its booking was cancelled. A child of the school itself is found by the
// rule the participant list uses (#4722): the registration names the school
// as its home school, the same names and the same date of birth; a booking
// that fits no child or several is left out. A child without a holiday
// booking on a day has no entry.
type HolidayGuestBookedCare interface {
	HolidayGuestBookedDays(ctx context.Context, studentIDs []int64, from, to calendar.Date) (map[int64]map[calendar.Date]HolidayGuestBookedDay, error)
}

// HolidayHostGroup is the group a school picked for the guests of a week,
// and who picked it.
type HolidayHostGroup struct {
	OrganizationID      int64
	WeekID              int64
	GroupID             int64
	AssignedByAccountID int64
}

// HolidayGuestWeek is a week of a published period the school hosts whose
// last day has not passed.
type HolidayGuestWeek struct {
	ID             int64
	OrganizationID int64
	StartsOn       calendar.Date
	EndsOn         calendar.Date
}

// HolidayGuestSource is one confirmed child of a hosted week that is no
// student of the school, with its booked time model, its contacts (the
// second guardian among the emergency contacts) and its shared answers, as
// the Träger's registration holds it.
type HolidayGuestSource struct {
	ChildID      int64
	FirstName    string
	LastName     string
	BirthDate    calendar.Date
	HomeSchoolID *int64
	// HomeSchoolName is the name of the home school the service looked up
	// for the copy (#5630); empty without one or for a school gone.
	HomeSchoolName  string
	GuestSchoolName string
	TimeModelName   string
	TimeModelStarts string
	TimeModelEnds   string
	// Days names the booked dates; empty books the whole week.
	Days      []calendar.Date
	Emergency []HolidayGuestContact
	Pickup    []HolidayGuestContact
	Answers   []HolidayHostAnswer
	// Health is the opened health information of the child (#4722), nil
	// without any; Sealed carries it from the store to the service.
	Health *HolidayHostHealth
	Sealed *HolidaySealedHostHealth
}

// HolidayGuest is the stored copy of a guest at the hosting school. EndedOn
// is the day its access ended early (cancellation or rebooking). Answers go
// with the contacts.
type HolidayGuest struct {
	ID              int64
	OrganizationID  int64
	WeekID          int64
	ChildID         int64
	StudentID       int64
	StartsOn        calendar.Date
	EndsOn          calendar.Date
	EndedOn         *calendar.Date
	HomeSchoolID    *int64
	HomeSchoolName  string
	GuestSchoolName string
	TimeModelName   string
	TimeModelStarts string
	TimeModelEnds   string
	// Days names the booked dates; empty books the whole week.
	Days      []calendar.Date
	Emergency []HolidayGuestContact
	Pickup    []HolidayGuestContact
	Answers   []HolidayHostAnswer
	// Health is the copied health information (#4722), nil without any. The
	// store keeps it sealed for the school and the child; HealthSealed
	// carries the stored value between the store and the service.
	Health       *HolidayHostHealth
	HealthSealed []byte
}
