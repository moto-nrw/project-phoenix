package enrollment

import (
	"context"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Children of the hosting school itself in a holiday week it hosts (#6678,
// ADR 0120 addendum). Such a child is no holiday guest: it is a student of
// the school already and stays in its own group. Enrollment finds it by the
// rule of the participant list (#4722): the registration names the school as
// its home school and carries the same names and date of birth. For a child
// it finds exactly once it plans the booked days in Care Plan, so the
// check-in expects the child on those days with the times of its time model,
// and it shows the team what the parents wrote into the registration:
// booked care, emergency contacts and who may collect. Nothing is copied:
// the school's master data stay as they are, the registration stays with the
// Träger, and the access ends with the last day of the week.

// How the registration of a child of the hosting school was assigned to one
// of its students.
const (
	// HolidayOwnChildMatched names the one student the registration fits.
	HolidayOwnChildMatched = "matched"
	// HolidayOwnChildNotFound means no student of the school carries these
	// names and this date of birth on the booked days.
	HolidayOwnChildNotFound = "not_found"
	// HolidayOwnChildAmbiguous means two or more students do.
	HolidayOwnChildAmbiguous = "ambiguous"
)

// HolidayOwnChildCareReason opens the reason of every day Enrollment plans
// for a child of the hosting school. Care Plan shows the reason to the team;
// Enrollment finds its own plan by it and never touches a day with another
// reason.
const HolidayOwnChildCareReason = "Ferienbetreuung laut Anmeldung"

// HolidayOwnChildSource is one confirmed child of a hosted week whose
// registration names the hosting school as its home school, with the booked
// time model and the contacts of the registration.
type HolidayOwnChildSource struct {
	ChildID         int64
	Identity        HolidayStudentIdentity
	TimeModelName   string
	TimeModelStarts string
	TimeModelEnds   string
	// Days names the booked dates; empty books the whole week.
	Days      []calendar.Date
	Emergency []HolidayGuestContact
	Pickup    []HolidayGuestContact
}

// HolidayOwnChildWeek is a hosted week that has not ended, with the period
// it belongs to and the account that picked its holiday group; zero before
// the school picked one.
type HolidayOwnChildWeek struct {
	HolidayGuestWeek
	PeriodName          string
	AssignedByAccountID int64
}

// HolidayOwnChildBooking is the confirmed booking of a child of the hosting
// school in one hosted week.
type HolidayOwnChildBooking struct {
	Week HolidayOwnChildWeek
	HolidayOwnChildSource
}

// HolidayOwnChildDay is one day Enrollment wants planned for a child of the
// hosting school: it arrives at Arrival and is collected at Pickup (clock
// times "15:04"). ActorAccountID is the account the plan is written for,
// zero when the week has no holiday group yet: such a day is kept if it is
// planned, and not planned anew.
type HolidayOwnChildDay struct {
	StudentID      int64
	Day            calendar.Date
	Arrival        string
	Pickup         string
	ActorAccountID int64
}

// HolidayOwnChildCareDays plans the booked days of the school's own children
// in Care Plan, in the tenant transaction of the hosting school. The root
// binds Care Plan's dated exceptions.
type HolidayOwnChildCareDays interface {
	// SyncHolidayOwnChildDays brings the days planned from holiday
	// registrations in line with wanted, from the day `from` on: a missing day
	// from today on is planned, a day whose booking went or whose times
	// changed is removed or replaced. A day that carries a plan the team or
	// the parents wrote is left alone, as is a planned day the team edited.
	SyncHolidayOwnChildDays(ctx context.Context, from, today calendar.Date, wanted []HolidayOwnChildDay) error
}

// HolidayStudentRegistration is what a confirmed holiday registration says
// about a child of the hosting school for one week it hosts (#6678): the
// booked care, the emergency contacts and who may collect it. It stands in
// the child's file as "Angaben aus der Ferienanmeldung", beside the school's
// master data, until the last day of the week.
type HolidayStudentRegistration struct {
	WeekID     int64         `json:"week_id"`
	PeriodName string        `json:"period_name"`
	StartsOn   calendar.Date `json:"starts_on"`
	EndsOn     calendar.Date `json:"ends_on"`
	// Days are the booked days of a week by days, empty for the whole week.
	Days            []calendar.Date       `json:"days"`
	TimeModelName   string                `json:"time_model_name"`
	TimeModelStarts string                `json:"time_model_starts_at"`
	TimeModelEnds   string                `json:"time_model_ends_at"`
	Emergency       []HolidayGuestContact `json:"emergency_contacts"`
	Pickup          []HolidayGuestContact `json:"pickup_contacts"`
	// Planned is true once the school picked the holiday group of the week:
	// from then on the check-in expects the child on its booked days.
	Planned bool `json:"planned"`
}

// HolidayGuestGroupHint tells a group that the school picked it as the
// holiday group of a week that has not begun (#6680): the group is empty on
// purpose, Guests children join it on StartsOn.
type HolidayGuestGroupHint struct {
	GroupID  int64         `json:"group_id"`
	WeekID   int64         `json:"week_id"`
	StartsOn calendar.Date `json:"starts_on"`
	EndsOn   calendar.Date `json:"ends_on"`
	Guests   int           `json:"guests"`
}

// HolidayOwnChildAssignment says, for one confirmed child of a hosted week
// whose registration names the hosting school as its home school, whether
// the registration was assigned to one of the school's students:
// HolidayOwnChildMatched with StudentID, HolidayOwnChildNotFound or
// HolidayOwnChildAmbiguous. ChildID is the child of the participant list.
type HolidayOwnChildAssignment struct {
	ChildID    int64  `json:"child_id"`
	Assignment string `json:"assignment"`
	StudentID  *int64 `json:"student_id"`
}

// HolidayOwnChildren serves the school's own children of its hosted weeks.
// Every operation runs in the tenant transaction of the hosting school.
type HolidayOwnChildren interface {
	// HolidayStudentRegistrations returns what the confirmed registrations
	// say about a child of the school for the weeks it hosts, and records the
	// read when there is any. A registration that fits no student or several
	// is shown at none of them.
	HolidayStudentRegistrations(ctx context.Context, host HolidayHost, studentID int64) ([]HolidayStudentRegistration, error)
	// HolidayOwnChildAssignments lists, for one hosted week, how each child
	// of the school itself was assigned to a student, so the school sees a
	// registration that fits no student or several instead of losing it
	// silently. It names children only by the ID of the participant list.
	HolidayOwnChildAssignments(ctx context.Context, host HolidayHost, weekID int64) ([]HolidayOwnChildAssignment, error)
	// HolidayGuestGroupHints lists the holiday groups of weeks that have not
	// begun, with the number of guests waiting for them. It reads no personal
	// data.
	HolidayGuestGroupHints(ctx context.Context, schoolID int64) ([]HolidayGuestGroupHint, error)
}
