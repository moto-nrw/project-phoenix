package enrollment

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Closing days in booked holiday weeks (#6434). A closing day the Träger or a
// school plans after parents booked a holiday week takes the care of that day
// from every booking of the week at the closed school. The bookings stay as
// they are: no booking is deleted or changed, no place is freed and nobody
// moves up the waiting list; only a new booking of the week is refused
// (#4702). What the parents of such a booking learn, and when:
//
//   - Before a Träger closing day is saved, the Träger-Büro sees the weeks and
//     bookings it affects (HolidayClosureImpact). The save sends the token of
//     that preview; a save whose bookings differ from it is refused, so the
//     mails the save sends are those the Büro saw.
//   - The save enqueues one mail per family (recipient address) with all its
//     children, weeks and closed days, in the Träger transaction of the
//     closing day, and records each told day per registration and week. A day
//     already told is never told again, so saving unchanged sends nothing.
//   - A closing day the school enters itself is written in the school's
//     transaction, which cannot reach the Träger's families. Its days, and any
//     other closed day of a booked week nobody told yet, stay pending for the
//     Träger-Büro (PendingHolidayClosures) until it tells the parents.
//
// Statutory holidays are no closing days: a week with one was never bookable
// (#4702), so its parents are not told about it.

// MailKindHolidayClosureNotice tells the parents that a holiday week they
// booked has a day without care (#6434), a mail of the Träger.
const MailKindHolidayClosureNotice = "holiday_closure_notice"

// HolidayClosureKeepsContribution says what a closed day does to the holiday
// contribution. A contribution is frozen when the registration is confirmed
// and never changes (#4664), whether its week is priced as a week or by its
// booked days (#4701), so a closed day leaves the contribution as it is and
// the mail says so. Whether a Träger lowers it is
// an open question (#6434); a correction goes through the existing
// cancellation and credit paths of the contribution.
const HolidayClosureKeepsContribution = true

// ErrHolidayClosureChanged reports a save or a notice whose affected bookings
// are no longer those of the preview the caller confirmed.
var ErrHolidayClosureChanged = errors.New("the holiday bookings a closing day affects changed since the preview")

// HolidayClosure is a closing day range of some host schools as Enrollment
// sees it: the schools, the days and the reason the parents read.
type HolidayClosure struct {
	SchoolIDs []int64
	StartsOn  calendar.Date
	EndsOn    calendar.Date
	Reason    string
}

// HolidayClosedDay is a day without care in a booked week with its reason.
type HolidayClosedDay struct {
	Date   calendar.Date `json:"date"`
	Reason string        `json:"reason"`
}

// HolidayClosureWeek is one affected week: its period, dates and host school,
// the days of the week without care nobody told the parents about, and how
// many children and families booked it. Waitlisted counts the children of
// Children that wait for a place.
type HolidayClosureWeek struct {
	WeekID       int64              `json:"week_id"`
	PeriodID     int64              `json:"period_id"`
	PeriodName   string             `json:"period_name"`
	StartsOn     calendar.Date      `json:"starts_on"`
	EndsOn       calendar.Date      `json:"ends_on"`
	HostSchoolID int64              `json:"host_school_id"`
	ClosedDays   []HolidayClosedDay `json:"closed_days"`
	Children     int                `json:"children"`
	Waitlisted   int                `json:"waitlisted"`
	Families     int                `json:"families"`
}

// HolidayClosureImpact is what a closing day means for the holiday bookings:
// the affected weeks in date order, the children and families (recipient
// addresses) across them, and the token a save or a notice confirms. It
// names no child: the Büro planning a closing day needs the extent, the
// parents get the details by mail.
type HolidayClosureImpact struct {
	Weeks             []HolidayClosureWeek `json:"weeks"`
	Children          int                  `json:"children"`
	Families          int                  `json:"families"`
	Token             string               `json:"token"`
	KeepsContribution bool                 `json:"keeps_contribution"`
}

// HolidayClosureNotice is the result of telling the parents.
type HolidayClosureNotice struct {
	Families int `json:"families"`
	Children int `json:"children"`
}

// HolidayClosureMailWeek is one booked week of a child in the notice with
// its closed days, formatted for the mail.
type HolidayClosureMailWeek struct {
	Week       string   `json:"week"`
	ClosedDays []string `json:"closed_days"`
	Waitlisted bool     `json:"waitlisted"`
}

// HolidayClosureMailChild is a child of the notice with its affected weeks.
type HolidayClosureMailChild struct {
	Name  string                   `json:"name"`
	Guest bool                     `json:"guest"`
	Weeks []HolidayClosureMailWeek `json:"weeks"`
}

// HolidayClosureMail is the payload of the notice: the family's children
// with their weeks and closed days, the reasons, and what happens to the
// contribution. No date of birth, no health information.
type HolidayClosureMail struct {
	// RegistrationIDs links every included registration for demo erasure.
	RegistrationIDs   []int64                   `json:"registration_ids"`
	Recipient         string                    `json:"recipient_email"`
	GuardianName      string                    `json:"guardian_name"`
	TraegerName       string                    `json:"traeger_name"`
	Reasons           []string                  `json:"reasons"`
	Children          []HolidayClosureMailChild `json:"children"`
	KeepsContribution bool                      `json:"keeps_contribution"`
}

// HolidayClosures lets the Träger tell the parents of holiday bookings about
// closing days in their weeks. Every operation runs in the Träger
// transaction of organizationID, which the caller opens.
type HolidayClosures interface {
	// HolidayClosureImpact previews which bookings closure affects. A
	// school that is not an undeleted school of the Träger is refused with
	// ErrHolidayHostSchoolOutsideTraeger.
	HolidayClosureImpact(ctx context.Context, organizationID int64, closure HolidayClosure) (HolidayClosureImpact, error)
	// NotifyHolidayClosure tells the parents of the bookings closure
	// affects, after the caller saved it in the same transaction. token is
	// that of the preview the caller confirmed; a differing impact with
	// bookings is ErrHolidayClosureChanged.
	NotifyHolidayClosure(ctx context.Context, organizationID int64, closure HolidayClosure, token string) (HolidayClosureNotice, error)
	// PendingHolidayClosures lists the closed days of booked weeks that no
	// parent was told about yet, those a school entered itself included.
	PendingHolidayClosures(ctx context.Context, organizationID int64) (HolidayClosureImpact, error)
	// NotifyPendingHolidayClosures tells the parents of the pending days;
	// token is that of the pending list the caller saw.
	NotifyPendingHolidayClosures(ctx context.Context, organizationID int64, token string) (HolidayClosureNotice, error)
}

// HolidayClosureBookingRecord is a booking as the store reads it for a
// closure: the registration with its guardian, the child, the week with its
// period and host school, the time model and the placement.
type HolidayClosureBookingRecord struct {
	RegistrationID    int64
	GuardianEmail     string
	GuardianFirstName string
	GuardianLastName  string
	ChildID           int64
	FirstName         string
	LastName          string
	Guest             bool
	PeriodID          int64
	PeriodName        string
	WeekID            int64
	StartsOn          calendar.Date
	EndsOn            calendar.Date
	HostSchoolID      int64
	TimeModelName     string
	StartsAt          string
	EndsAt            string
	Placement         string
	// Days are the booked days of a booking of days (#4701); empty for a
	// booking of the whole week. A closing day affects only these days.
	Days []calendar.Date
}

// HolidayClosureNoticeRecord is a closed day of a week the parents of a
// registration were told about.
type HolidayClosureNoticeRecord struct {
	RegistrationID int64
	WeekID         int64
	ClosedOn       calendar.Date
}
