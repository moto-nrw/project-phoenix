package enrollment

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
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
// at the next reconciliation. The contacts stay readable for seven days after
// the week and are deleted then. Every operation runs in the tenant
// transaction of the hosting school, which the caller opens.

// ErrHolidayGuestNotFound reports a child that is no holiday guest of the
// school, or one whose access has ended.
var ErrHolidayGuestNotFound = errors.New("holiday guest not found")

// ErrHolidayGuestPlanAuthorMissing reports that the account the days of a
// guest are planned for has no staff record, which Care Plan requires as the
// author of a day plan.
var ErrHolidayGuestPlanAuthorMissing = errors.New("holiday guest plan author has no staff record")

// ErrHolidayGuestGroupInvalid reports a group that is not one of the
// hosting school's.
var ErrHolidayGuestGroupInvalid = errors.New("holiday guest group is not a group of the school")

// HolidayGuestSchoolClass is the class of the child a guest becomes: People
// Directory requires one, and the groups and lists of the school show it
// where they show a class, so the staff tell a guest from their own
// children at a glance.
const HolidayGuestSchoolClass = "Feriengast"

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
// it. A nil HomeSchoolID marks a child without a school of the Träger;
// GuestSchoolName is the school its parents named, if any.
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
	Emergency       []HolidayGuestContact `json:"emergency_contacts"`
	Pickup          []HolidayGuestContact `json:"pickup_contacts"`
	// ReadOnly is true after the last day of the week: the guest is no
	// longer cared for and the sheet stays only for questions afterwards.
	ReadOnly bool `json:"read_only"`
}

// HolidayGuestTransfer is the result of picking the group of a hosted week:
// the group and how many guests of the week it holds now.
type HolidayGuestTransfer struct {
	WeekID  int64 `json:"week_id"`
	GroupID int64 `json:"group_id"`
	Guests  int   `json:"guests"`
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
}

// HolidayGuestCareDays plans the booked days of a guest in Care Plan, so the
// group and the check-in show it as expected on those days and no other:
// on each day of the week it arrives at the start and is collected at the
// end of its time model (clock times "15:04"). actorAccountID is the account
// the plan is written for. ClearHolidayGuestDays removes the plan of a
// guest that ended. The root binds Care Plan's dated exceptions.
type HolidayGuestCareDays interface {
	PlanHolidayGuestDays(ctx context.Context, studentID, actorAccountID int64, days []calendar.Date, arrival, pickup string) error
	ClearHolidayGuestDays(ctx context.Context, studentID int64) error
}

// Kinds of a holiday guest access in the access log of the hosting school.
const (
	HolidayGuestAccessTransfer = "transfer"
	HolidayGuestAccessSheet    = "sheet"
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
	// cancelled ones end, and contacts past the read-only days go.
	ReconcileHolidayGuests(ctx context.Context, schoolID int64) error
	// HolidayGuestSheet returns the sheet of a guest and records the read;
	// ErrHolidayGuestNotFound for a child that is no guest or whose access
	// has ended.
	HolidayGuestSheet(ctx context.Context, host HolidayHost, studentID int64) (HolidayGuestSheet, error)
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
// student of the school, with its booked time model and its contacts, as
// the Träger's registration holds it.
type HolidayGuestSource struct {
	ChildID         int64
	FirstName       string
	LastName        string
	BirthDate       calendar.Date
	HomeSchoolID    *int64
	GuestSchoolName string
	TimeModelName   string
	TimeModelStarts string
	TimeModelEnds   string
	Emergency       []HolidayGuestContact
	Pickup          []HolidayGuestContact
}

// HolidayGuest is the stored copy of a guest at the hosting school. EndedOn
// is the day its access ended early (cancellation or rebooking).
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
	GuestSchoolName string
	TimeModelName   string
	TimeModelStarts string
	TimeModelEnds   string
	Emergency       []HolidayGuestContact
	Pickup          []HolidayGuestContact
}
