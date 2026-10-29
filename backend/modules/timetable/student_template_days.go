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
}

// StudentDayBooking is one offering the student books on one ISO weekday.
//
// A booking carries a Regeltermin on that weekday when the Regeltermin ends
// no later than the offering's pickup time: the booked care runs until the
// child is picked up. A care Regeltermin needs an offering that counts as
// care. Without a pickup time, or for a Regeltermin without an end, the
// times cannot decide; then the Regeltermin counts as lost on a net-loss
// weekday only (#5342).
type StudentDayBooking struct {
	Weekday      int
	CountsAsCare bool
	// PickupTime is the offering's pickup time on the weekday as "HH:MM" or
	// "HH:MM:SS"; empty when the offering sets none for that day.
	PickupTime string
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
}

// StudentTemplateDaysRemoval takes a student off a Regeltermin series on the
// given ISO weekdays from From on.
type StudentTemplateDaysRemoval struct {
	TemplateID int64
	StudentID  int64
	Weekdays   []int
	From       calendar.Date
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
	// weekdays from From on. Booking-created rows, earlier days, other
	// weekdays and rows recording attendance or a hand decision stay. It
	// takes the tenant recurrence gate. From must not lie in the past.
	RemoveStudentTemplateDays(ctx context.Context, removal StudentTemplateDaysRemoval) (StudentTemplateDaysRemoved, error)
}
