package timetable

import (
	"context"
	"errors"

	"github.com/moto-nrw/project-phoenix/sharedkernel/calendar"
)

// Student template days (#5102): after a booking change takes weekdays away
// from a child, Regeltermine that staff maintain by hand still plan the child
// on those days. The offering resync only rewrites rows a booking created
// (enrollment_request_child_id set); a hand row, a Klasse/Jahrgang/Gruppe
// target and the roster of an occurrence created earlier stay. This
// capability names those Regeltermine and, after a confirmation in the UI,
// takes the child off exactly those weekdays from a date on.

// ErrStudentTemplateDaysInvalid rejects a malformed query or removal.
var ErrStudentTemplateDaysInvalid = errors.New("invalid student template days request")

// ManualStudentPlanQuery asks where a student stays planned from From on, on
// the given ISO weekdays (1 = Monday … 7 = Sunday), other than through a
// roster row a booking created, without a booking that carries the
// Regeltermin (#5342).
type ManualStudentPlanQuery struct {
	StudentID int64
	From      calendar.Date
	// Weekdays are the days the booking change dropped a booking on.
	Weekdays []int
	// NetLossWeekdays are the days the change drops more offerings on than
	// it newly books. They decide where pickup time or end is missing.
	NetLossWeekdays []int
	// Bookings are what the student books after the change, one entry per
	// offering and weekday. None means the student books nothing.
	Bookings []StudentDayBooking
	// OtherBookings are what the student books through other enrollments
	// the change leaves as they are (#5366). They carry a Regeltermin like
	// Bookings, and one without a pickup time keeps the day on the net-loss
	// rule like one of Bookings.
	OtherBookings []StudentDayBooking
}

// StudentDayBooking is one offering the student books on one ISO weekday.
//
// A booking carries a Regeltermin on that weekday when the offering fits the
// Regeltermin's kind (RosterOfferingKindFor) and the Regeltermin ends no
// later than the offering's pickup time: the booked care runs until the child
// is picked up. The times decide only when the Regeltermin has an end and
// every booking of the day, in any enrollment, has a pickup time; otherwise
// the Regeltermin
// counts as lost exactly on a net-loss weekday, whatever the kinds (#5342).
type StudentDayBooking struct {
	Weekday       int
	CountsAsCare  bool
	IncludesLunch bool
	// PickupTime is the offering's pickup time on the weekday as "HH:MM" or
	// "HH:MM:SS"; empty when the offering sets none for that day.
	PickupTime string
	// From is the day the booking starts when that lies after the query's
	// From (#5497); nil when it is in force then. A later booking does not
	// keep the change's date, but it ends the gap of a weekday it carries.
	From *calendar.Date
}

// ManualStudentPlan is one Regeltermin series that keeps planning the student
// on some of the asked weekdays.
type ManualStudentPlan struct {
	// TemplateID is the latest segment of the series that plans the student;
	// the removal resolves the whole series from it.
	TemplateID int64
	Name       string
	// Weekdays are the asked ISO weekdays the series still plans the student
	// on, ascending.
	Weekdays []int
	// ViaTarget is true when a Klasse, Jahrgang or Gruppe target of the
	// series includes the student: occurrences created later add the child
	// again.
	ViaTarget bool
	// Until is the day a later booking carries the series again on all of
	// Weekdays (exclusive end, #5497); nil when none does. A removal ends
	// there.
	Until *calendar.Date
}

// StudentTemplateDaysRemoval takes a student off a Regeltermin series on the
// given ISO weekdays from From on, until Until (exclusive) when set: from
// then a later booking carries the series again and the student stays
// planned there (#5497).
type StudentTemplateDaysRemoval struct {
	TemplateID int64
	StudentID  int64
	Weekdays   []int
	From       calendar.Date
	Until      *calendar.Date
}

// StudentTemplateDaysKeep records the school's "Bleibt so" (#5102): the
// child stays planned by hand on the weekdays although a booking ending on
// BookingEndedOn no longer carries the Regeltermin. Kept false withdraws the
// decision.
type StudentTemplateDaysKeep struct {
	TemplateID     int64
	StudentID      int64
	Weekdays       []int
	BookingEndedOn calendar.Date
	Kept           bool
}

// StudentTemplateDaysRemoved counts what a removal changed.
type StudentTemplateDaysRemoved struct {
	// RosterRows are hand roster rows ended or narrowed.
	RosterRows int
	// Occurrences are already-created occurrences the student left.
	Occurrences int
}

// StudentTemplateDays reads and removes the hand planning of one student.
// Both run in the caller's tenant transaction.
type StudentTemplateDays interface {
	// ManualStudentPlan lists the series, in name order, that plan the
	// student on the asked weekdays from From on through a hand roster row,
	// a dynamic target or an occurrence roster. Days a booking-created row
	// still covers or a remaining booking carries are left out.
	ManualStudentPlan(ctx context.Context, query ManualStudentPlanQuery) ([]ManualStudentPlan, error)
	// RemoveStudentTemplateDays narrows the student's hand roster rows of the
	// series and removes the student from its planned occurrences on the
	// weekdays from From on, up to Until when set. Booking-created rows,
	// earlier and later days, other weekdays and rows recording attendance or
	// a hand decision stay. It takes the tenant recurrence gate. From must not
	// lie in the past, and Until must lie after From.
	RemoveStudentTemplateDays(ctx context.Context, removal StudentTemplateDaysRemoval) (StudentTemplateDaysRemoved, error)
	// KeepStudentTemplateDays marks or unmarks, on exactly these weekdays,
	// every hand row of the student in the series that plans one of them and
	// returns how many rows it marked. Plans stay as they are. It takes the
	// tenant recurrence gate. ErrGroupNotFound for an unknown series.
	KeepStudentTemplateDays(ctx context.Context, keep StudentTemplateDaysKeep) (int, error)
}
