package enrollment

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Closing days in booked holiday weeks (#6434, #6442, #6457, ADR 0196). A
// closing day the Träger or a school plans after parents booked a holiday
// week takes the care of that day from every booking of the week at the
// closed school; a deleted or shortened one gives it back. The bookings stay
// as they are: no booking is deleted or changed, no place is freed and nobody
// moves up the waiting list; only a new booking of the week is refused
// (#4702). What the parents learn is kept per registration, week and day as
// the state they were last told: closed or open again.
//
//   - Before a Träger closing day is saved, changed or deleted, the
//     Träger-Büro sees the weeks and bookings it affects, the days it closes
//     and the days it opens again (HolidayClosureImpact). The save confirms
//     the token of that preview and tells the parents in the same Träger
//     transaction.
//   - A change of a school closing day only proposes a notice. A click of the
//     school admin or of the Träger-Büro confirms it; the Träger's worker
//     then tells the parents (NotifyHolidayClosureOperation) under the
//     operation of the proposal, with the same rules.
//   - Each family (recipient address) gets one mail per operation with all
//     its children, weeks and days: "closed" and "open again" in two
//     sections when a closing day moved. A day told before is not told
//     again, so saving unchanged sends nothing; a day closed after it was
//     opened again is told again.
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

// HolidayClosure is a Träger closing day as Enrollment sees it: the schools,
// the days and the reason the parents read. TraegerClosingDayID names the
// saved closing day it replaces, whose stored version no longer counts, and
// Previous that stored version, whose days may open again. Deleted removes
// the closing day: SchoolIDs and the dates then name nothing new.
type HolidayClosure struct {
	SchoolIDs           []int64
	StartsOn            calendar.Date
	EndsOn              calendar.Date
	Reason              string
	TraegerClosingDayID int64
	Previous            *HolidayClosureSpan
	Deleted             bool
}

// HolidayClosureSpan is the stored version of a Träger closing day.
type HolidayClosureSpan struct {
	SchoolIDs []int64       `json:"school_ids"`
	StartsOn  calendar.Date `json:"start_date"`
	EndsOn    calendar.Date `json:"end_date"`
}

// HolidayClosureOperation is a confirmed or proposed change of a school
// closing day (ADR 0196): the operation of its proposal, the school and the
// days it touches.
type HolidayClosureOperation struct {
	OperationID string
	SchoolID    int64
	From        calendar.Date
	To          calendar.Date
}

// HolidayClosureCalendar is Enrollment's port onto the days without care of
// one current school of the Träger in [from, to], read in the caller's
// Träger transaction through School Calendar's narrow read channel
// (ADR 0196 section 3.4). A notice reads it under Enrollment's lock, so a
// Träger closing day saved meanwhile counts. The root binds it.
type HolidayClosureCalendar interface {
	HolidayClosureNoCareDays(ctx context.Context, organizationID, schoolID int64, from, to calendar.Date) ([]HolidayNoCareDay, error)
}

// HolidayClosureOperationResult is what the worker learns of a processed
// operation: how many families it told now and how many mails the
// operation holds in all.
type HolidayClosureOperationResult struct {
	Families int
	Mails    int
	Retried  int64
}

// HolidayClosureDelivery sums the mails of an operation up by their
// transport outcome.
type HolidayClosureDelivery struct {
	Queued  int
	Sent    int
	Failed  int
	Unknown int
}

// Transport outcomes of a Träger mail as Enrollment reads them back.
const (
	TraegerMailQueued  = "queued"
	TraegerMailSent    = "sent"
	TraegerMailFailed  = "failed"
	TraegerMailUnknown = "unknown"
)

// TraegerMailState is the transport outcome of a Träger mail.
type TraegerMailState struct {
	IdempotencyKey string
	Outcome        string
}

// TraegerMailDelivery is the outbox of the closing day notices with the
// transport outcomes of their mails and the protected repetition of a known
// failure (ADR 0196 section 5). The root binds it to Delivery; every
// operation runs in the caller's Träger transaction.
type TraegerMailDelivery interface {
	TraegerMailOutbox
	TraegerMailStates(ctx context.Context, organizationID int64, idempotencyKeys []string) ([]TraegerMailState, error)
	// RetryFailedTraegerEmails makes the mails with a known transport
	// failure due again; a mail sent, waiting or with an unknown outcome
	// stays as it is. It returns how many it made due.
	RetryFailedTraegerEmails(ctx context.Context, organizationID int64, idempotencyKeys []string) (int64, error)
}

// HolidayClosedDay is a day of a booked week with its reason: without care,
// or open again.
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
	ReopenedDays []HolidayClosedDay `json:"reopened_days"`
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
// with their weeks and closed days, those with days open again, the
// reasons, and what happens to the contribution. No date of birth, no
// health information.
type HolidayClosureMail struct {
	// RegistrationIDs links every included registration for demo erasure.
	RegistrationIDs   []int64                   `json:"registration_ids"`
	Recipient         string                    `json:"recipient_email"`
	GuardianName      string                    `json:"guardian_name"`
	TraegerName       string                    `json:"traeger_name"`
	Reasons           []string                  `json:"reasons"`
	Children          []HolidayClosureMailChild `json:"children"`
	ReopenedChildren  []HolidayClosureMailChild `json:"reopened_children,omitempty"`
	KeepsContribution bool                      `json:"keeps_contribution"`
}

// HolidayClosures lets the Träger tell the parents of holiday bookings about
// closing days in their weeks and days that open again. Every operation runs
// in the Träger transaction of organizationID, which the caller opens.
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
	// PlanHolidayClosureOperation previews what telling the parents of
	// operation would do now, without writing anything: the same
	// computation NotifyHolidayClosureOperation sends.
	PlanHolidayClosureOperation(ctx context.Context, organizationID int64, operation HolidayClosureOperation) (HolidayClosureImpact, error)
	// NotifyHolidayClosureOperation tells the parents of operation once per
	// family and makes the operation's mails with a known transport failure
	// due again.
	NotifyHolidayClosureOperation(ctx context.Context, organizationID int64, operation HolidayClosureOperation) (HolidayClosureOperationResult, error)
	// HolidayClosureOperationDelivery sums the mails of the operation up.
	HolidayClosureOperationDelivery(ctx context.Context, organizationID int64, operationID string) (HolidayClosureDelivery, error)
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

// Notice states of a day.
const (
	HolidayClosureDayClosed = "closed"
	HolidayClosureDayOpen   = "open"
)

// HolidayClosureNoticeRecord is a day of a week the parents of a
// registration were last told about: closed or open again.
type HolidayClosureNoticeRecord struct {
	RegistrationID int64
	WeekID         int64
	ClosedOn       calendar.Date
	State          string
}

// HolidayClosureSendRecord is one registration a family's mail of an
// operation names, under the mail's idempotency key.
type HolidayClosureSendRecord struct {
	OperationID    string
	FamilyKey      string
	RegistrationID int64
	IdempotencyKey string
}
